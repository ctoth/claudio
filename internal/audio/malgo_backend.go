//go:build cgo

package audio

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
)

// MalgoBackend implements AudioBackend using AudioPlayer and DecoderRegistry
type MalgoBackend struct {
	audioPlayer *AudioPlayer
	registry    *DecoderRegistry
	closed      bool
	mutex       sync.RWMutex
	// soundIDCount monotonically increments per Play to produce a unique
	// soundID per playback. Previously the soundID was keyed on
	// len(audioData.Samples), which collided whenever two concurrent Plays
	// of distinct same-length buffers ran — silently overwriting each
	// other's device entry in AudioPlayer.devices and leaking the loser's
	// handle (review finding #40).
	soundIDCount atomic.Uint64
}

// NewMalgoBackend creates a new MalgoBackend using AudioPlayer and DecoderRegistry
func NewMalgoBackend() *MalgoBackend {
	slog.Debug("creating new MalgoBackend with unified audio system")
	
	return &MalgoBackend{
		audioPlayer: NewAudioPlayer(),
		registry:    NewDefaultRegistry(), // Includes AIFF support
	}
}

// Stop stops any ongoing playback
func (mb *MalgoBackend) Stop() error {
	mb.mutex.Lock()
	defer mb.mutex.Unlock()

	if mb.closed {
		return ErrBackendClosed
	}

	err := mb.audioPlayer.StopAll()
	if err != nil {
		slog.Error("error stopping audio player", "error", err)
		return fmt.Errorf("error stopping audio player: %w", err)
	}

	slog.Debug("MalgoBackend stopped")
	return nil
}

// Close shuts down the backend
func (mb *MalgoBackend) Close() error {
	mb.mutex.Lock()
	defer mb.mutex.Unlock()

	if mb.closed {
		slog.Debug("MalgoBackend already closed")
		return nil
	}

	mb.closed = true

	if mb.audioPlayer != nil {
		err := mb.audioPlayer.Close()
		if err != nil {
			slog.Error("error closing AudioPlayer", "error", err)
			return fmt.Errorf("error closing AudioPlayer: %w", err)
		}
	}

	slog.Debug("MalgoBackend closed")
	return nil
}

// IsPlaying returns the current playing state
func (mb *MalgoBackend) IsPlaying() bool {
	mb.mutex.RLock()
	defer mb.mutex.RUnlock()
	
	if mb.closed {
		return false
	}
	
	return mb.audioPlayer.IsPlaying()
}

// SetVolume sets the volume level (0.0 to 1.0)
func (mb *MalgoBackend) SetVolume(volume float32) error {
	mb.mutex.Lock()
	defer mb.mutex.Unlock()

	if mb.closed {
		return ErrBackendClosed
	}

	return mb.audioPlayer.SetVolume(volume)
}

// GetVolume returns the current volume level
func (mb *MalgoBackend) GetVolume() float32 {
	mb.mutex.RLock()
	defer mb.mutex.RUnlock()
	
	if mb.closed {
		return 0.0
	}
	
	return mb.audioPlayer.GetVolume()
}

// Play plays audio from the given source using unified audio system
func (mb *MalgoBackend) Play(ctx context.Context, source AudioSource) error {
	mb.mutex.RLock()
	if mb.closed {
		mb.mutex.RUnlock()
		return ErrBackendClosed
	}
	mb.mutex.RUnlock()

	slog.Debug("MalgoBackend starting playback with unified system")

	// Get audio data from source
	var audioData *AudioData
	var err error

	// Try file path first (most efficient)
	var loadErr error
	if filePath, fpErr := source.AsFilePath(); fpErr == nil {
		audioData, loadErr = mb.loadAudioFile(ctx, filePath)
	} else {
		// Use reader
		reader, format, rErr := source.AsReader()
		if rErr != nil {
			slog.Error("failed to get reader from source", "error", rErr)
			return fmt.Errorf("failed to get audio data from source: %w", rErr)
		}
		defer reader.Close()

		audioData, loadErr = mb.loadAudioFromReader(ctx, reader, format)
	}

	if loadErr != nil {
		slog.Error("failed to load audio data", "error", loadErr)
		return fmt.Errorf("failed to load audio data: %w", loadErr)
	}

	if audioData == nil {
		slog.Error("audio data is nil after loading")
		return fmt.Errorf("audio data is nil")
	}

	// Generate unique sound ID for this playback. atomic.Uint64.Add returns
	// the post-increment value, guaranteeing distinct IDs across
	// concurrent Plays regardless of buffer length.
	soundID := fmt.Sprintf("play_%d", mb.soundIDCount.Add(1))

	// Preload and play
	err = mb.audioPlayer.PreloadSound(soundID, audioData)
	if err != nil {
		slog.Error("failed to preload sound", "sound_id", soundID, "error", err)
		return fmt.Errorf("failed to preload sound: %w", err)
	}

	err = mb.audioPlayer.PlaySoundWithContext(ctx, soundID)
	if err != nil {
		// Clean up on error
		_ = mb.audioPlayer.UnloadSound(soundID)
		slog.Error("failed to play sound", "sound_id", soundID, "error", err)
		return fmt.Errorf("failed to play sound: %w", err)
	}

	// Playback is synchronous: PlaySoundWithContext returns when the buffer
	// has been consumed or ctx is cancelled. Unload inline. The previous
	// `go func() { <-ctx.Done(); UnloadSound(soundID) }()` pattern leaked one
	// goroutine plus a pinned *AudioData per call whenever the caller passed
	// context.Background() (Done() is nil and the receive blocked forever) —
	// which is exactly what the production CLI path does.
	if uerr := mb.audioPlayer.UnloadSound(soundID); uerr != nil {
		slog.Warn("failed to unload sound after playback", "sound_id", soundID, "error", uerr)
	}

	slog.Debug("unified playback completed successfully")
	return nil
}

// loadAudioFile loads an audio file using the registry
func (mb *MalgoBackend) loadAudioFile(ctx context.Context, filePath string) (*AudioData, error) {
	slog.Debug("loading audio file with registry", "file", filePath)

	file, err := os.Open(filePath)
	if err != nil {
		slog.Error("failed to open audio file", "file", filePath, "error", err)
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Use registry to decode - this handles AIFF/WAV/MP3 automatically
	audioData, err := mb.registry.DecodeFile(ctx, filePath, file)
	if err != nil {
		slog.Error("registry decode failed", "file", filePath, "error", err)
		return nil, fmt.Errorf("decode failed: %w", err)
	}

	slog.Debug("audio file loaded successfully via registry",
		"file", filePath,
		"channels", audioData.Channels,
		"sample_rate", audioData.SampleRate,
		"data_size", len(audioData.Samples))

	return audioData, nil
}

// loadAudioFromReader loads audio from a reader using the registry
func (mb *MalgoBackend) loadAudioFromReader(ctx context.Context, reader io.Reader, format string) (*AudioData, error) {
	slog.Debug("loading audio from reader with registry", "format", format)

	// Create filename for format detection
	filename := "stream." + format

	// Use registry to decode
	audioData, err := mb.registry.DecodeFile(ctx, filename, reader)
	if err != nil {
		slog.Error("registry decode from reader failed", "format", format, "error", err)
		return nil, fmt.Errorf("decode from reader failed: %w", err)
	}

	slog.Debug("audio reader loaded successfully via registry",
		"format", format,
		"channels", audioData.Channels,
		"sample_rate", audioData.SampleRate,
		"data_size", len(audioData.Samples))

	return audioData, nil
}
