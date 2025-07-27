package audio

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/gen2brain/malgo"
)

// Player interface defines audio playback capabilities
type Player interface {
	// State management
	IsPlaying() bool
	GetVolume() float32
	SetVolume(volume float32) error
	
	// Sound management
	IsSoundLoaded(soundID string) bool
	PreloadSound(soundID string, audioData *AudioData) error
	UnloadSound(soundID string) error
	
	// Playback control
	PlaySound(soundID string) error
	PlaySoundWithContext(ctx context.Context, soundID string) error
	Stop() error
	StopAll() error
	
	// Lifecycle
	Close() error
}

// AudioPlayer implements memory-based audio playback using malgo
type AudioPlayer struct {
	context     *Context
	sounds      map[string]*AudioData
	devices     map[string]*malgo.Device
	volume      float32
	isPlaying   bool
	mutex       sync.RWMutex
	deviceMutex sync.Mutex
	closed      bool
}

// NewAudioPlayer creates a new audio player instance
func NewAudioPlayer() *AudioPlayer {
	slog.Debug("creating new audio player instance")
	
	player := &AudioPlayer{
		sounds:  make(map[string]*AudioData),
		devices: make(map[string]*malgo.Device),
		volume:  1.0, // Default full volume
		mutex:   sync.RWMutex{},
	}
	
	slog.Info("audio player created successfully", "default_volume", player.volume)
	return player
}

// IsPlaying returns true if any audio is currently playing
func (p *AudioPlayer) IsPlaying() bool {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.isPlaying
}

// GetVolume returns the current volume level (0.0 to 1.0)
func (p *AudioPlayer) GetVolume() float32 {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.volume
}

// SetVolume sets the volume level (0.0 to 1.0)
func (p *AudioPlayer) SetVolume(volume float32) error {
	if volume < 0.0 || volume > 1.0 {
		err := fmt.Errorf("invalid volume level: %f (must be 0.0-1.0)", volume)
		slog.Error("invalid volume setting", "volume", volume, "error", err)
		return err
	}
	
	p.mutex.Lock()
	oldVolume := p.volume
	p.volume = volume
	p.mutex.Unlock()
	
	slog.Info("volume changed", "old_volume", oldVolume, "new_volume", volume)
	return nil
}

// IsSoundLoaded checks if a sound is preloaded
func (p *AudioPlayer) IsSoundLoaded(soundID string) bool {
	if soundID == "" {
		return false
	}
	
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	
	_, exists := p.sounds[soundID]
	slog.Debug("sound load status check", "sound_id", soundID, "loaded", exists)
	return exists
}

// PreloadSound loads audio data into memory for quick playback
func (p *AudioPlayer) PreloadSound(soundID string, audioData *AudioData) error {
	if soundID == "" {
		err := fmt.Errorf("sound ID cannot be empty")
		slog.Error("preload failed: empty sound ID", "error", err)
		return err
	}
	
	if audioData == nil {
		err := fmt.Errorf("audio data cannot be nil")
		slog.Error("preload failed: nil audio data", "sound_id", soundID, "error", err)
		return err
	}
	
	slog.Debug("preloading sound", 
		"sound_id", soundID,
		"channels", audioData.Channels,
		"sample_rate", audioData.SampleRate,
		"format", audioData.Format,
		"data_size", len(audioData.Samples))
	
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	// Check if we're overwriting an existing sound
	if _, exists := p.sounds[soundID]; exists {
		slog.Info("overwriting existing preloaded sound", "sound_id", soundID)
	}
	
	p.sounds[soundID] = audioData
	
	slog.Info("sound preloaded successfully", 
		"sound_id", soundID,
		"total_preloaded", len(p.sounds))
	
	return nil
}

// UnloadSound removes a sound from memory
func (p *AudioPlayer) UnloadSound(soundID string) error {
	if soundID == "" {
		return fmt.Errorf("sound ID cannot be empty")
	}
	
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	if _, exists := p.sounds[soundID]; !exists {
		slog.Debug("attempted to unload non-existent sound", "sound_id", soundID)
		return nil // Not an error, already unloaded
	}
	
	delete(p.sounds, soundID)
	
	slog.Info("sound unloaded successfully", 
		"sound_id", soundID,
		"remaining_preloaded", len(p.sounds))
	
	return nil
}

// PlaySound plays a preloaded sound
func (p *AudioPlayer) PlaySound(soundID string) error {
	return p.PlaySoundWithContext(context.Background(), soundID)
}

// PlaySoundWithContext plays a preloaded sound with context cancellation
func (p *AudioPlayer) PlaySoundWithContext(ctx context.Context, soundID string) error {
	if soundID == "" {
		err := fmt.Errorf("sound ID cannot be empty")
		slog.Error("playback failed: empty sound ID", "error", err)
		return err
	}
	
	p.mutex.RLock()
	if p.closed {
		p.mutex.RUnlock()
		err := fmt.Errorf("player is closed")
		slog.Error("playback failed: player closed", "sound_id", soundID, "error", err)
		return err
	}
	
	audioData, exists := p.sounds[soundID]
	if !exists {
		p.mutex.RUnlock()
		err := fmt.Errorf("sound not found: %s", soundID)
		slog.Error("playback failed: sound not preloaded", "sound_id", soundID, "error", err)
		return err
	}
	p.mutex.RUnlock()
	
	slog.Debug("starting sound playback", "sound_id", soundID)
	
	// Check context cancellation before starting
	select {
	case <-ctx.Done():
		err := ctx.Err()
		slog.Error("playback cancelled before start", "sound_id", soundID, "error", err)
		return err
	default:
	}
	
	// Initialize audio context if needed
	if p.context == nil {
		slog.Debug("initializing audio context for playback")
		audioCtx, err := NewContext()
		if err != nil {
			slog.Error("failed to initialize audio context", "sound_id", soundID, "error", err)
			return fmt.Errorf("failed to initialize audio context: %w", err)
		}
		p.context = audioCtx
	}
	
	// Create device configuration for this sound
	deviceConfig := malgo.DefaultDeviceConfig(malgo.Playback)
	deviceConfig.Playback.Format = audioData.Format
	deviceConfig.Playback.Channels = audioData.Channels
	deviceConfig.SampleRate = audioData.SampleRate
	deviceConfig.Alsa.NoMMap = 1
	
	slog.Debug("device configuration", 
		"sound_id", soundID,
		"format", audioData.Format,
		"channels", audioData.Channels,
		"sample_rate", audioData.SampleRate)
	
	// Track playback position for this sound instance
	var frameOffset uint32
	totalFrames := uint32(len(audioData.Samples) / int(audioData.Channels) / 2) // Assuming 16-bit samples
	
	// Audio callback function
	onSamples := func(pOutputSample, pInputSamples []byte, framecount uint32) {
		// Check context cancellation during playback
		select {
		case <-ctx.Done():
			slog.Debug("playback cancelled during audio callback", "sound_id", soundID)
			return
		default:
		}
		
		// Calculate byte offset in our audio data
		bytesPerFrame := int(audioData.Channels * 2) // 2 bytes per 16-bit sample
		startByte := int(frameOffset) * bytesPerFrame
		requestedBytes := int(framecount) * bytesPerFrame
		
		// Check if we've reached the end
		if startByte >= len(audioData.Samples) {
			// Fill with silence
			for i := range pOutputSample {
				pOutputSample[i] = 0
			}
			return
		}
		
		// Calculate how many bytes we can actually copy
		availableBytes := len(audioData.Samples) - startByte
		bytesToCopy := requestedBytes
		if bytesToCopy > availableBytes {
			bytesToCopy = availableBytes
		}
		
		// Copy audio data
		copy(pOutputSample[:bytesToCopy], audioData.Samples[startByte:startByte+bytesToCopy])
		
		// CRITICAL: Fill any remaining space with silence
		// We MUST fill the entire buffer or we'll get garbage/crackling
		for i := bytesToCopy; i < len(pOutputSample); i++ {
			pOutputSample[i] = 0
		}
		
		// Apply volume if needed
		volume := p.GetVolume()
		if volume != 1.0 {
			for i := 0; i < bytesToCopy; i += 2 {
				if i+1 < bytesToCopy {
					// Read 16-bit sample (little endian)
					sample := int16(pOutputSample[i]) | int16(pOutputSample[i+1])<<8
					
					// Apply volume
					sample = int16(float32(sample) * volume)
					
					// Write back (little endian)
					pOutputSample[i] = byte(sample)
					pOutputSample[i+1] = byte(sample >> 8)
				}
			}
		}
		
		frameOffset += framecount
		
		if frameOffset >= totalFrames {
			slog.Debug("sound playback completed", "sound_id", soundID, "frames_played", frameOffset)
		}
	}
	
	deviceCallbacks := malgo.DeviceCallbacks{
		Data: onSamples,
	}
	
	// Create and start device
	device, err := malgo.InitDevice(p.context.GetContext().Context, deviceConfig, deviceCallbacks)
	if err != nil {
		slog.Error("failed to initialize playback device", "sound_id", soundID, "error", err)
		return fmt.Errorf("failed to initialize playback device: %w", err)
	}
	
	slog.Debug("playback device initialized", "sound_id", soundID)
	
	// Store device for cleanup
	p.deviceMutex.Lock()
	p.devices[soundID] = device
	p.deviceMutex.Unlock()
	
	// Start playback
	err = device.Start()
	if err != nil {
		device.Uninit()
		p.deviceMutex.Lock()
		delete(p.devices, soundID)
		p.deviceMutex.Unlock()
		slog.Error("failed to start playback", "sound_id", soundID, "error", err)
		return fmt.Errorf("failed to start playback: %w", err)
	}
	
	p.mutex.Lock()
	p.isPlaying = true
	p.mutex.Unlock()
	
	slog.Info("sound playback started successfully", "sound_id", soundID)
	
	// Wait for playback to complete or context cancellation
	go func() {
		// Estimate playback duration
		duration := time.Duration(totalFrames) * time.Second / time.Duration(audioData.SampleRate)
		timer := time.NewTimer(duration + 100*time.Millisecond) // Add small buffer
		
		select {
		case <-ctx.Done():
			slog.Debug("playback context cancelled", "sound_id", soundID)
		case <-timer.C:
			slog.Debug("playback duration elapsed", "sound_id", soundID)
		}
		
		// Cleanup device
		device.Stop()
		device.Uninit()
		
		p.deviceMutex.Lock()
		delete(p.devices, soundID)
		p.deviceMutex.Unlock()
		
		// Update playing status if no more devices
		p.deviceMutex.Lock()
		stillPlaying := len(p.devices) > 0
		p.deviceMutex.Unlock()
		
		p.mutex.Lock()
		p.isPlaying = stillPlaying
		p.mutex.Unlock()
		
		slog.Info("sound playback cleanup completed", "sound_id", soundID, "still_playing", stillPlaying)
	}()
	
	return nil
}

// Stop stops the current sound (for single-sound scenarios)
func (p *AudioPlayer) Stop() error {
	slog.Debug("stopping current sound playback")
	
	p.mutex.Lock()
	p.isPlaying = false
	p.mutex.Unlock()
	
	slog.Info("sound playback stopped")
	return nil
}

// StopAll stops all currently playing sounds
func (p *AudioPlayer) StopAll() error {
	slog.Debug("stopping all sound playback")
	
	p.deviceMutex.Lock()
	devices := make([]*malgo.Device, 0, len(p.devices))
	for _, device := range p.devices {
		devices = append(devices, device)
	}
	p.devices = make(map[string]*malgo.Device) // Clear map
	p.deviceMutex.Unlock()
	
	// Stop and cleanup all devices
	for _, device := range devices {
		device.Stop()
		device.Uninit()
	}
	
	p.mutex.Lock()
	p.isPlaying = false
	p.mutex.Unlock()
	
	slog.Info("all sound playback stopped", "devices_stopped", len(devices))
	return nil
}

// Close shuts down the audio player and releases resources
func (p *AudioPlayer) Close() error {
	slog.Debug("closing audio player")
	
	p.mutex.Lock()
	if p.closed {
		p.mutex.Unlock()
		slog.Debug("audio player already closed")
		return nil
	}
	p.closed = true
	p.mutex.Unlock()
	
	// Stop all playback
	err := p.StopAll()
	if err != nil {
		slog.Error("error stopping playback during close", "error", err)
	}
	
	// Close audio context
	if p.context != nil {
		err := p.context.Close()
		if err != nil {
			slog.Error("error closing audio context", "error", err)
		}
	}
	
	// Clear preloaded sounds
	p.mutex.Lock()
	soundCount := len(p.sounds)
	p.sounds = make(map[string]*AudioData)
	p.mutex.Unlock()
	
	slog.Info("audio player closed successfully", "sounds_cleared", soundCount)
	return nil
}