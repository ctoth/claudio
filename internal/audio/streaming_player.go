package audio

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"

	"github.com/gen2brain/malgo"
)

// StreamingPlayer implements audio playback using streaming
type StreamingPlayer struct {
	context *Context
	volume  float32
	mutex   sync.RWMutex
	closed  bool
}

// NewStreamingPlayer creates a new streaming audio player
func NewStreamingPlayer() *StreamingPlayer {
	slog.Debug("creating new streaming audio player")
	
	player := &StreamingPlayer{
		volume: 1.0,
	}
	
	slog.Info("streaming audio player created", "default_volume", player.volume)
	return player
}

// PlayStreamingSound plays audio from a reader
func (p *StreamingPlayer) PlayStreamingSound(ctx context.Context, audioData *StreamingAudioData) error {
	p.mutex.RLock()
	if p.closed {
		p.mutex.RUnlock()
		return fmt.Errorf("player is closed")
	}
	p.mutex.RUnlock()
	
	// Initialize audio context if needed
	if p.context == nil {
		slog.Debug("initializing audio context for streaming playback")
		audioCtx, err := NewContext()
		if err != nil {
			return fmt.Errorf("failed to initialize audio context: %w", err)
		}
		p.context = audioCtx
	}
	
	// Create device configuration
	deviceConfig := malgo.DefaultDeviceConfig(malgo.Playback)
	deviceConfig.Playback.Format = audioData.Format
	deviceConfig.Playback.Channels = audioData.Channels
	deviceConfig.SampleRate = audioData.SampleRate
	deviceConfig.Alsa.NoMMap = 1
	
	slog.Debug("streaming device configuration",
		"format", audioData.Format,
		"channels", audioData.Channels,
		"sample_rate", audioData.SampleRate)
	
	// Audio callback - simple streaming
	onSamples := func(pOutputSample, pInputSamples []byte, framecount uint32) {
		// Check context cancellation
		select {
		case <-ctx.Done():
			// Fill with silence on cancellation
			for i := range pOutputSample {
				pOutputSample[i] = 0
			}
			return
		default:
		}
		
		// Read directly from the audio reader
		n, err := io.ReadFull(audioData.Reader, pOutputSample)
		if err != nil {
			// Fill remaining with silence
			for i := n; i < len(pOutputSample); i++ {
				pOutputSample[i] = 0
			}
			
			if err != io.EOF && err != io.ErrUnexpectedEOF {
				slog.Debug("error reading audio data", "error", err)
			}
		}
		
		// Apply volume if needed
		volume := p.GetVolume()
		if volume != 1.0 && n > 0 {
			// Apply volume to 16-bit samples
			for i := 0; i < n-1; i += 2 {
				sample := int16(pOutputSample[i]) | int16(pOutputSample[i+1])<<8
				sample = int16(float32(sample) * volume)
				pOutputSample[i] = byte(sample)
				pOutputSample[i+1] = byte(sample >> 8)
			}
		}
	}
	
	deviceCallbacks := malgo.DeviceCallbacks{
		Data: onSamples,
	}
	
	// Create and start device
	device, err := malgo.InitDevice(p.context.GetContext().Context, deviceConfig, deviceCallbacks)
	if err != nil {
		return fmt.Errorf("failed to initialize playback device: %w", err)
	}
	defer device.Uninit()
	
	err = device.Start()
	if err != nil {
		return fmt.Errorf("failed to start playback: %w", err)
	}
	defer device.Stop()
	
	slog.Info("streaming playback started")
	
	// Wait for context cancellation or a reasonable timeout
	<-ctx.Done()
	
	slog.Info("streaming playback stopped")
	return nil
}

// GetVolume returns the current volume level
func (p *StreamingPlayer) GetVolume() float32 {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.volume
}

// SetVolume sets the volume level (0.0 to 1.0)
func (p *StreamingPlayer) SetVolume(volume float32) error {
	if volume < 0.0 || volume > 1.0 {
		return fmt.Errorf("invalid volume: %f", volume)
	}
	
	p.mutex.Lock()
	p.volume = volume
	p.mutex.Unlock()
	
	return nil
}

// Close shuts down the audio player
func (p *StreamingPlayer) Close() error {
	p.mutex.Lock()
	p.closed = true
	p.mutex.Unlock()
	
	if p.context != nil {
		return p.context.Close()
	}
	
	return nil
}