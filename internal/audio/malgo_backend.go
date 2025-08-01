package audio

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
)

// MalgoBackend implements AudioBackend by wrapping the existing SimplePlayer
type MalgoBackend struct {
	player  *SimplePlayer
	volume  float32
	closed  bool
	mutex   sync.RWMutex
}

// NewMalgoBackend creates a new MalgoBackend using the existing SimplePlayer
func NewMalgoBackend() *MalgoBackend {
	slog.Debug("creating new MalgoBackend")
	return &MalgoBackend{
		player: NewSimplePlayer(),
		volume: 1.0, // Default full volume
	}
}

// Start initializes the backend (no-op for malgo)
func (mb *MalgoBackend) Start() error {
	mb.mutex.Lock()
	defer mb.mutex.Unlock()
	
	if mb.closed {
		return ErrBackendClosed
	}
	
	slog.Debug("MalgoBackend started")
	return nil
}

// Stop stops any ongoing playback
func (mb *MalgoBackend) Stop() error {
	mb.mutex.Lock()
	defer mb.mutex.Unlock()
	
	if mb.closed {
		return ErrBackendClosed
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
	
	if mb.player != nil {
		err := mb.player.Close()
		if err != nil {
			slog.Error("error closing SimplePlayer", "error", err)
			return fmt.Errorf("error closing SimplePlayer: %w", err)
		}
	}
	
	slog.Debug("MalgoBackend closed")
	return nil
}

// IsPlaying returns the current playing state (simplified for now)
func (mb *MalgoBackend) IsPlaying() bool {
	mb.mutex.RLock()
	defer mb.mutex.RUnlock()
	return !mb.closed
}

// SetVolume sets the volume level (0.0 to 1.0)
func (mb *MalgoBackend) SetVolume(volume float32) error {
	if volume < 0.0 || volume > 1.0 {
		err := fmt.Errorf("invalid volume level: %f (must be 0.0-1.0)", volume)
		slog.Error("invalid volume setting", "volume", volume, "error", err)
		return err
	}
	
	mb.mutex.Lock()
	defer mb.mutex.Unlock()
	
	if mb.closed {
		return ErrBackendClosed
	}
	
	oldVolume := mb.volume
	mb.volume = volume
	slog.Debug("volume changed", "old_volume", oldVolume, "new_volume", volume)
	return nil
}

// GetVolume returns the current volume level
func (mb *MalgoBackend) GetVolume() float32 {
	mb.mutex.RLock()
	defer mb.mutex.RUnlock()
	return mb.volume
}

// Play plays audio from the given source using the existing SimplePlayer
func (mb *MalgoBackend) Play(ctx context.Context, source AudioSource) error {
	mb.mutex.RLock()
	if mb.closed {
		mb.mutex.RUnlock()
		return ErrBackendClosed
	}
	mb.mutex.RUnlock()
	
	slog.Debug("MalgoBackend starting playback")
	
	// Try file path first (most efficient for SimplePlayer)
	if filePath, err := source.AsFilePath(); err == nil {
		return mb.playFile(ctx, filePath)
	}
	
	// Fall back to reader via temporary file
	reader, format, err := source.AsReader()
	if err != nil {
		slog.Error("failed to get reader from source", "error", err)
		return fmt.Errorf("failed to get audio data from source: %w", err)
	}
	defer reader.Close()
	
	return mb.playReaderViaTempFile(ctx, reader, format)
}

// playFile plays a file using the SimplePlayer
func (mb *MalgoBackend) playFile(ctx context.Context, filePath string) error {
	slog.Debug("playing file via SimplePlayer", "file", filePath)
	
	// Use the existing SimplePlayer.PlayFile method
	err := mb.player.PlayFile(filePath, mb.volume)
	if err != nil {
		slog.Error("SimplePlayer failed to play file", "file", filePath, "error", err)
		return fmt.Errorf("SimplePlayer failed to play file: %w", err)
	}
	
	slog.Debug("file playback completed successfully", "file", filePath)
	return nil
}

// playReaderViaTempFile writes reader data to a temporary file and plays it
func (mb *MalgoBackend) playReaderViaTempFile(ctx context.Context, reader io.Reader, format string) error {
	slog.Debug("playing reader via temporary file", "format", format)
	
	// Create temporary file with appropriate extension
	tempFile, err := os.CreateTemp("", "claudio-malgo-*."+format)
	if err != nil {
		slog.Error("failed to create temporary file", "format", format, "error", err)
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	
	// Ensure cleanup
	tempPath := tempFile.Name()
	defer func() {
		os.Remove(tempPath)
		slog.Debug("temporary file cleaned up", "path", tempPath)
	}()
	
	// Copy reader data to temporary file
	_, err = io.Copy(tempFile, reader)
	if err != nil {
		tempFile.Close()
		slog.Error("failed to write audio data to temporary file", "path", tempPath, "error", err)
		return fmt.Errorf("failed to write audio data to temporary file: %w", err)
	}
	
	// Close file before playing
	err = tempFile.Close()
	if err != nil {
		slog.Error("failed to close temporary file", "path", tempPath, "error", err)
		return fmt.Errorf("failed to close temporary file: %w", err)
	}
	
	slog.Debug("temporary file created successfully", "path", tempPath, "format", format)
	
	// Play the temporary file
	return mb.playFile(ctx, tempPath)
}