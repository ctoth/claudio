//go:build cgo

package audio

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
)

// MalgoBackend implements AudioBackend using AudioPlayer and DecoderRegistry
type MalgoBackend struct {
	audioPlayer *AudioPlayer
	registry    *DecoderRegistry
	closed      bool
	mutex       sync.RWMutex
}

// NewMalgoBackend creates a new MalgoBackend using AudioPlayer and DecoderRegistry
func NewMalgoBackend() *MalgoBackend {
	slog.Debug("creating new MalgoBackend with unified audio system")
	
	return &MalgoBackend{
		audioPlayer: NewAudioPlayer(),
		registry:    NewDefaultRegistry(), // Includes AIFF support
	}
}

// Start initializes the backend
func (mb *MalgoBackend) Start() error {
	mb.mutex.Lock()
	defer mb.mutex.Unlock()

	if mb.closed {
		return ErrBackendClosed
	}

	slog.Debug("MalgoBackend started with unified audio system")
	return nil
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
	if filePath, err := source.AsFilePath(); err == nil {
		audioData, err = mb.loadAudioFile(filePath)
	} else {
		// Use reader
		reader, format, err := source.AsReader()
		if err != nil {
			slog.Error("failed to get reader from source", "error", err)
			return fmt.Errorf("failed to get audio data from source: %w", err)
		}
		defer reader.Close()

		audioData, err = mb.loadAudioFromReader(reader, format)
	}

	if err != nil {
		slog.Error("failed to load audio data", "error", err)
		return fmt.Errorf("failed to load audio data: %w", err)
	}

	if audioData == nil {
		slog.Error("audio data is nil after loading")
		return fmt.Errorf("audio data is nil")
	}

	// Generate unique sound ID for this playback
	soundID := fmt.Sprintf("play_%d", len(audioData.Samples))

	// Preload and play
	err = mb.audioPlayer.PreloadSound(soundID, audioData)
	if err != nil {
		slog.Error("failed to preload sound", "sound_id", soundID, "error", err)
		return fmt.Errorf("failed to preload sound: %w", err)
	}

	err = mb.audioPlayer.PlaySoundWithContext(ctx, soundID)
	if err != nil {
		// Clean up on error
		mb.audioPlayer.UnloadSound(soundID)
		slog.Error("failed to play sound", "sound_id", soundID, "error", err)
		return fmt.Errorf("failed to play sound: %w", err)
	}

	// Clean up after playback
	go func() {
		<-ctx.Done()
		mb.audioPlayer.UnloadSound(soundID)
		slog.Debug("sound cleanup completed", "sound_id", soundID)
	}()

	slog.Debug("unified playback completed successfully")
	return nil
}

// loadAudioFile loads an audio file using the registry
func (mb *MalgoBackend) loadAudioFile(filePath string) (*AudioData, error) {
	slog.Debug("loading audio file with registry", "file", filePath)

	file, err := os.Open(filePath)
	if err != nil {
		slog.Error("failed to open audio file", "file", filePath, "error", err)
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Use registry to decode - this handles AIFF/WAV/MP3 automatically
	audioData, err := mb.registry.DecodeFile(filePath, file)
	if err != nil {
		slog.Error("registry decode failed", "file", filePath, "error", err)
		return nil, fmt.Errorf("decode failed: %w", err)
	}

	slog.Info("audio file loaded successfully via registry",
		"file", filePath,
		"channels", audioData.Channels,
		"sample_rate", audioData.SampleRate,
		"data_size", len(audioData.Samples))

	return audioData, nil
}

// loadAudioFromReader loads audio from a reader using the registry
func (mb *MalgoBackend) loadAudioFromReader(reader io.Reader, format string) (*AudioData, error) {
	slog.Debug("loading audio from reader with registry", "format", format)

	// Create filename for format detection
	filename := "stream." + format

	// Use registry to decode
	audioData, err := mb.registry.DecodeFile(filename, reader)
	if err != nil {
		slog.Error("registry decode from reader failed", "format", format, "error", err)
		return nil, fmt.Errorf("decode from reader failed: %w", err)
	}

	slog.Info("audio reader loaded successfully via registry",
		"format", format,
		"channels", audioData.Channels,
		"sample_rate", audioData.SampleRate,
		"data_size", len(audioData.Samples))

	return audioData, nil
}
