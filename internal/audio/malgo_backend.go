//go:build cgo

package audio

import (
	"context"
	"fmt"
	"log/slog"
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

	// Get audio data from source. malgo always goes through the registry
	// decoder, so we use the reader path uniformly — FileSource gives us a
	// real os.File via Reader(), ReaderSource gives us its in-memory
	// stream. The dual AsFilePath/AsReader fork that lived here was
	// finding #42's main consumer; both branches ultimately called
	// registry.DecodeFile, so the fork was paying for nothing on the malgo
	// side.
	var err error

	reader, format, rErr := source.Reader()
	if rErr != nil {
		slog.Error("failed to get reader from source", "error", rErr)
		return fmt.Errorf("failed to get audio data from source: %w", rErr)
	}
	defer reader.Close()

	// Prefer the file path for format detection when available (FileSource);
	// otherwise rely on the stream's declared format (ReaderSource).
	detectFilename := "stream." + format
	if fp, ok := source.(FilePather); ok {
		if filePath, pathErr := fp.FilePath(); pathErr == nil {
			detectFilename = filePath
		}
	}

	audioData, loadErr := mb.registry.DecodeFile(ctx, detectFilename, reader)
	if loadErr != nil {
		slog.Error("failed to load audio data", "filename", detectFilename, "error", loadErr)
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

