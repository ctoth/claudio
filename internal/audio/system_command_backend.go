package audio

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"sync"
)

// SystemCommandBackend implements AudioBackend using system commands like paplay
type SystemCommandBackend struct {
	command   string
	volume    float32
	isPlaying bool
	closed    bool
	mutex     sync.RWMutex
}

// NewSystemCommandBackend creates a new SystemCommandBackend with the specified command
func NewSystemCommandBackend(command string) *SystemCommandBackend {
	slog.Debug("creating new SystemCommandBackend", "command", command)
	return &SystemCommandBackend{
		command: command,
		volume:  1.0, // Default full volume
	}
}

// Start initializes the backend (no-op for system commands)
func (scb *SystemCommandBackend) Start() error {
	scb.mutex.Lock()
	defer scb.mutex.Unlock()

	if scb.closed {
		return ErrBackendClosed
	}

	slog.Debug("SystemCommandBackend started", "command", scb.command)
	return nil
}

// Stop stops any ongoing playback (limited control with system commands)
func (scb *SystemCommandBackend) Stop() error {
	scb.mutex.Lock()
	defer scb.mutex.Unlock()

	if scb.closed {
		return ErrBackendClosed
	}

	scb.isPlaying = false
	slog.Debug("SystemCommandBackend stopped")
	return nil
}

// Close shuts down the backend
func (scb *SystemCommandBackend) Close() error {
	scb.mutex.Lock()
	defer scb.mutex.Unlock()

	scb.closed = true
	scb.isPlaying = false
	slog.Debug("SystemCommandBackend closed")
	return nil
}

// IsPlaying returns the current playing state
func (scb *SystemCommandBackend) IsPlaying() bool {
	scb.mutex.RLock()
	defer scb.mutex.RUnlock()
	return scb.isPlaying && !scb.closed
}

// SetVolume sets the volume level (0.0 to 1.0)
func (scb *SystemCommandBackend) SetVolume(volume float32) error {
	if volume < 0.0 || volume > 1.0 {
		err := fmt.Errorf("invalid volume level: %f (must be 0.0-1.0)", volume)
		slog.Error("invalid volume setting", "volume", volume, "error", err)
		return err
	}

	scb.mutex.Lock()
	defer scb.mutex.Unlock()

	if scb.closed {
		return ErrBackendClosed
	}

	oldVolume := scb.volume
	scb.volume = volume
	slog.Debug("volume changed", "old_volume", oldVolume, "new_volume", volume)
	return nil
}

// GetVolume returns the current volume level
func (scb *SystemCommandBackend) GetVolume() float32 {
	scb.mutex.RLock()
	defer scb.mutex.RUnlock()
	return scb.volume
}

// Play plays audio from the given source using system commands
func (scb *SystemCommandBackend) Play(ctx context.Context, source AudioSource) error {
	scb.mutex.Lock()
	if scb.closed {
		scb.mutex.Unlock()
		return ErrBackendClosed
	}
	scb.isPlaying = true
	scb.mutex.Unlock()

	// Ensure we reset playing state when done
	defer func() {
		scb.mutex.Lock()
		scb.isPlaying = false
		scb.mutex.Unlock()
	}()

	slog.Debug("SystemCommandBackend starting playback", "command", scb.command)

	// Try file path first (most efficient for system commands)
	if filePath, err := source.AsFilePath(); err == nil {
		return scb.playFile(ctx, filePath)
	}

	// Fall back to reader via temporary file
	reader, format, err := source.AsReader()
	if err != nil {
		slog.Error("failed to get reader from source", "error", err)
		return fmt.Errorf("failed to get audio data from source: %w", err)
	}
	defer reader.Close()

	return scb.playReaderViaTempFile(ctx, reader, format)
}

// playFile plays a file directly using the system command
func (scb *SystemCommandBackend) playFile(ctx context.Context, filePath string) error {
	slog.Debug("playing file via system command", "file", filePath, "command", scb.command)

	// Create command with context for cancellation
	cmd := exec.CommandContext(ctx, scb.command, filePath)

	// Run the command and wait for completion
	err := cmd.Run()
	if err != nil {
		slog.Error("system command failed", "command", scb.command, "file", filePath, "error", err)
		return fmt.Errorf("system command failed: %w", err)
	}

	slog.Debug("file playback completed successfully", "file", filePath)
	return nil
}

// playReaderViaTempFile writes reader data to a temporary file and plays it
func (scb *SystemCommandBackend) playReaderViaTempFile(ctx context.Context, reader io.Reader, format string) error {
	slog.Debug("playing reader via temporary file", "format", format)

	// Create temporary file with appropriate extension
	tempFile, err := os.CreateTemp("", "claudio-*."+format)
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
	return scb.playFile(ctx, tempPath)
}
