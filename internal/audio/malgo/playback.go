//go:build cgo

package malgo

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gen2brain/malgo"
)

// deviceEntry wraps a malgo.Device with a sync.Once gating Uninit so that
// concurrent teardown paths (per-sound inline cleanup in PlaySoundWithContext,
// StopAll's snapshot iteration, Close's delegation to StopAll) cannot
// double-free the same C-side device. malgo.Device.Uninit calls
// `dev.free()` on the underlying C struct; a second call is a real
// double-free, not just a stale Go reference.
type deviceEntry struct {
	device     *malgo.Device
	uninitOnce sync.Once
}

// uninit stops and uninits the wrapped device exactly once, regardless of
// how many goroutines reach the teardown path. Safe to call any number of
// times from any goroutine.
func (e *deviceEntry) uninit() {
	e.uninitOnce.Do(func() {
		if e.device == nil {
			return
		}
		_ = e.device.Stop()
		e.device.Uninit()
	})
}

// AudioPlayer implements memory-based audio playback using malgo.
//
// Always construct via NewAudioPlayer — a zero-value AudioPlayer has volume 0
// (silent) because atomic.Uint32's zero value decodes to a 0.0 float32 via
// math.Float32frombits.
type AudioPlayer struct {
	context *Context
	sounds  map[string]*AudioData
	// devices maps soundID -> *deviceEntry. The map is the single source of
	// truth for "is anything playing": IsPlaying() returns len(devices) > 0
	// under deviceMutex, and there is no separate isPlaying bool to drift
	// out of sync.
	devices map[string]*deviceEntry
	// volume holds the playback gain (0.0..1.0) as a float32 encoded via
	// math.Float32bits and accessed atomically. Lock-free access is mandatory:
	// the malgo realtime audio callback reads this on every buffer fill and
	// must never block (see PlaySoundWithContext's onSamples closure).
	volume      atomic.Uint32
	mutex       sync.RWMutex
	deviceMutex sync.Mutex
	closed      bool
	// contextInitOnce gates the lazy malgo.InitContext allocation in
	// PlaySoundWithContext. Two concurrent first-Play goroutines used to both
	// observe p.context==nil and both call NewContext(), leaking the loser's
	// C-side handle. Once also publishes the p.context/contextInitErr writes
	// to every later observer.
	contextInitOnce sync.Once
	contextInitErr  error
}

// NewAudioPlayer creates a new audio player instance
func NewAudioPlayer() *AudioPlayer {
	slog.Debug("creating new audio player instance")

	player := &AudioPlayer{
		sounds:  make(map[string]*AudioData),
		devices: make(map[string]*deviceEntry),
		mutex:   sync.RWMutex{},
	}
	// atomic.Uint32 cannot be initialised in a struct literal with a non-zero
	// value; seed the default full-volume after construction.
	player.volume.Store(math.Float32bits(1.0))

	slog.Debug("audio player created successfully", "default_volume", float32(1.0))
	return player
}

// IsPlaying returns true if any audio is currently playing. The device map
// is the single source of truth — a non-empty map means at least one malgo
// device is live. Eliminates the prior isPlaying-vs-actual-state drift.
func (p *AudioPlayer) IsPlaying() bool {
	p.deviceMutex.Lock()
	defer p.deviceMutex.Unlock()
	return len(p.devices) > 0
}

// GetVolume returns the current volume level (0.0 to 1.0). Lock-free; safe to
// call from the malgo realtime audio callback.
func (p *AudioPlayer) GetVolume() float32 {
	return math.Float32frombits(p.volume.Load())
}

// SetVolume sets the volume level (0.0 to 1.0). Rejects NaN and Inf to match
// SystemCommandBackend.SetVolume's input contract.
func (p *AudioPlayer) SetVolume(volume float32) error {
	if math.IsNaN(float64(volume)) || math.IsInf(float64(volume), 0) {
		err := fmt.Errorf("invalid volume level: %f (must be finite)", volume)
		slog.Error("invalid volume setting (non-finite)", "volume", volume, "error", err)
		return err
	}
	if volume < 0.0 || volume > 1.0 {
		err := fmt.Errorf("invalid volume level: %f (must be 0.0-1.0)", volume)
		slog.Error("invalid volume setting", "volume", volume, "error", err)
		return err
	}

	// atomic.Swap returns the previous bits so we can preserve the existing
	// "old_volume" debug log without an extra Load round-trip.
	oldBits := p.volume.Swap(math.Float32bits(volume))
	slog.Debug("volume changed", "old_volume", math.Float32frombits(oldBits), "new_volume", volume)
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
		slog.Debug("overwriting existing preloaded sound", "sound_id", soundID)
	}
	
	p.sounds[soundID] = audioData
	
	slog.Debug("sound preloaded successfully",
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
	
	slog.Debug("sound unloaded successfully",
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
	
	// Initialize audio context if needed. sync.Once guarantees exactly one
	// NewContext() / malgo.InitContext call across concurrent first-Play
	// goroutines — preventing the C-side handle leak that occurred when the
	// nil-check + assignment was racy.
	p.contextInitOnce.Do(func() {
		slog.Debug("initializing audio context for playback")
		audioCtx, err := NewContext()
		if err != nil {
			p.contextInitErr = err
			return
		}
		p.context = audioCtx
	})
	if p.contextInitErr != nil {
		slog.Error("failed to initialize audio context", "sound_id", soundID, "error", p.contextInitErr)
		return fmt.Errorf("failed to initialize audio context: %w", p.contextInitErr)
	}
	if p.context == nil {
		err := fmt.Errorf("audio context not initialized")
		slog.Error("audio context unexpectedly nil", "sound_id", soundID, "error", err)
		return err
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
	
	// Calculate bytes per sample based on format. Unknown formats are
	// refused outright — silently defaulting to 2 bytes produced garbage
	// frame math (wrong duration, wrong callback offsets, distorted audio).
	bytesPerSample, err := getBytesPerSample(audioData.Format)
	if err != nil {
		slog.Error("cannot play sound: unsupported format", "sound_id", soundID, "format", audioData.Format, "error", err)
		return fmt.Errorf("cannot play sound %q: %w", soundID, err)
	}
	// Round UP to include any trailing partial frame. The previous
	// truncation caused the playback-complete timer to fire before the
	// final callback ran, uninitting the device mid-buffer. The audio
	// callback already pads the last frame with silence (line ~315), so
	// the timer just needs to wait long enough for that final fire.
	bytesPerFrame := int(audioData.Channels) * bytesPerSample
	totalFrames := uint32((len(audioData.Samples) + bytesPerFrame - 1) / bytesPerFrame)
	
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
		bytesPerFrame := int(audioData.Channels) * bytesPerSample
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
		
		// Apply volume if needed.
		//
		// REALTIME HOT PATH: this callback runs on malgo's audio thread under a
		// hard deadline. The volume read MUST remain lock-free — inline the
		// atomic load and skip the GetVolume method-call frame on purpose.
		volume := math.Float32frombits(p.volume.Load())
		if volume != 1.0 {
			applyVolumeToSamples(pOutputSample[:bytesToCopy], audioData.Format, volume)
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

	// Wrap in a deviceEntry whose uninitOnce makes Uninit idempotent. Both
	// this goroutine's inline cleanup and any concurrent StopAll/Close can
	// call entry.uninit() safely; only the first call reaches malgo.
	entry := &deviceEntry{device: device}

	// Store entry for cleanup
	p.deviceMutex.Lock()
	p.devices[soundID] = entry
	p.deviceMutex.Unlock()

	// Start playback
	err = device.Start()
	if err != nil {
		entry.uninit()
		p.deviceMutex.Lock()
		delete(p.devices, soundID)
		p.deviceMutex.Unlock()
		slog.Error("failed to start playback", "sound_id", soundID, "error", err)
		return fmt.Errorf("failed to start playback: %w", err)
	}

	slog.Debug("sound playback started successfully", "sound_id", soundID)

	// Estimate playback duration
	duration := time.Duration(totalFrames) * time.Second / time.Duration(audioData.SampleRate)

	// Wait for playback to complete or context cancellation
	timer := time.NewTimer(duration + 500*time.Millisecond) // Add buffer for callback processing
	defer timer.Stop()

	select {
	case <-ctx.Done():
		slog.Debug("playback context cancelled", "sound_id", soundID)
	case <-timer.C:
		slog.Debug("playback duration elapsed", "sound_id", soundID)
	}

	// Cleanup device — idempotent via deviceEntry.uninitOnce. Map removal
	// must precede the IsPlaying-derived state observation: with IsPlaying
	// now derived from len(p.devices), deleting first means concurrent
	// observers see false the instant the device is logically gone, and
	// the subsequent uninit (which can be slow — it joins the malgo worker
	// thread) proceeds at its own pace without lying about IsPlaying.
	p.deviceMutex.Lock()
	delete(p.devices, soundID)
	stillPlaying := len(p.devices) > 0
	p.deviceMutex.Unlock()

	entry.uninit()
	
	slog.Debug("sound playback cleanup completed", "sound_id", soundID, "still_playing", stillPlaying)
	return nil
}

// Stop halts all currently playing sounds. Previously this method only
// flipped an isPlaying flag without touching any malgo device — playback
// continued, but IsPlaying reported false. Stop is now an alias for
// StopAll, the only semantically-correct behaviour for a player with no
// per-sound handle in its public API.
func (p *AudioPlayer) Stop() error {
	slog.Debug("stopping current sound playback (delegating to StopAll)")
	return p.StopAll()
}

// StopAll stops all currently playing sounds. Each device's uninitOnce
// makes the teardown safe even if a per-sound goroutine is racing to its
// own inline cleanup — only the first uninit call reaches malgo.
func (p *AudioPlayer) StopAll() error {
	slog.Debug("stopping all sound playback")

	p.deviceMutex.Lock()
	entries := make([]*deviceEntry, 0, len(p.devices))
	for _, entry := range p.devices {
		entries = append(entries, entry)
	}
	p.devices = make(map[string]*deviceEntry) // Clear map
	p.deviceMutex.Unlock()

	// Uninit each entry. The uninitOnce inside deviceEntry guarantees this
	// is safe even when a per-sound goroutine is concurrently uninitting
	// the same entry from PlaySoundWithContext's deferred cleanup path.
	for _, entry := range entries {
		entry.uninit()
	}

	slog.Debug("all sound playback stopped", "devices_stopped", len(entries))
	return nil
}

// Close shuts down the audio player and releases resources
func (p *AudioPlayer) Close() error {
	slog.Debug("closing audio player")

	// Synchronize with any in-flight PlaySoundWithContext that's mid
	// context-init. If init has already completed, this Do is a free no-op
	// (sync.Once is one-shot). If init is racing, Close's no-op Do
	// participates in the Once barrier so the subsequent read of p.context
	// observes a settled state — closing the publication race the scout
	// flagged where Close read p.context without going through the Once.
	p.contextInitOnce.Do(func() {})

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
	
	slog.Debug("audio player closed successfully", "sounds_cleared", soundCount)
	return nil
}

// getBytesPerSample returns the number of bytes per sample for a given
// format. Unknown formats produce an error so callers can refuse playback
// rather than silently defaulting (the previous behaviour returned 2 for
// any unrecognised malgo.FormatType, generating garbage frame math).
func getBytesPerSample(format malgo.FormatType) (int, error) {
	switch format {
	case malgo.FormatU8:
		return 1, nil
	case malgo.FormatS16:
		return 2, nil
	case malgo.FormatS24:
		return 3, nil
	case malgo.FormatS32, malgo.FormatF32:
		return 4, nil
	default:
		return 0, fmt.Errorf("unsupported audio format: %v", format)
	}
}

// applyVolumeToSamples applies volume scaling to audio samples based on format
func applyVolumeToSamples(samples []byte, format malgo.FormatType, volume float32) {
	switch format {
	case malgo.FormatS16:
		// 16-bit signed samples
		for i := 0; i < len(samples)-1; i += 2 {
			sample := int16(samples[i]) | int16(samples[i+1])<<8
			sample = int16(float32(sample) * volume)
			samples[i] = byte(sample)
			samples[i+1] = byte(sample >> 8)
		}
	case malgo.FormatS24:
		// 24-bit signed samples (little endian)
		for i := 0; i < len(samples)-2; i += 3 {
			// Read 24-bit sample (little endian, sign-extended to 32-bit)
			sample := int32(samples[i]) | int32(samples[i+1])<<8 | int32(samples[i+2])<<16
			// Sign extend from 24-bit to 32-bit
			if sample&0x800000 != 0 {
				sample |= ^0xFFFFFF // Set upper 8 bits to 1 for negative numbers
			}
			
			// Apply volume
			sample = int32(float32(sample) * volume)
			
			// Write back (little endian, truncate to 24-bit)
			samples[i] = byte(sample)
			samples[i+1] = byte(sample >> 8)
			samples[i+2] = byte(sample >> 16)
		}
	case malgo.FormatS32:
		// 32-bit signed samples
		for i := 0; i < len(samples)-3; i += 4 {
			sample := int32(samples[i]) | int32(samples[i+1])<<8 | int32(samples[i+2])<<16 | int32(samples[i+3])<<24
			sample = int32(float32(sample) * volume)
			samples[i] = byte(sample)
			samples[i+1] = byte(sample >> 8)
			samples[i+2] = byte(sample >> 16)
			samples[i+3] = byte(sample >> 24)
		}
	default:
		slog.Warn("volume adjustment not implemented for format", "format", format)
	}
}