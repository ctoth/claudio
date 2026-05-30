package audio

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
)

// SystemCommandBackend implements AudioBackend using system commands like paplay
type SystemCommandBackend struct {
	command          string
	volume           float32
	isPlaying        bool
	closed           bool
	mutex            sync.RWMutex
	warnNoVolumeOnce sync.Once // one WARN per backend instance for aplay
}

// NewSystemCommandBackend creates a new SystemCommandBackend with the specified command
func NewSystemCommandBackend(command string) *SystemCommandBackend {
	slog.Debug("creating new SystemCommandBackend", "command", command)
	return &SystemCommandBackend{
		command: command,
		volume:  1.0, // Default full volume
	}
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
	// NaN evaluates as false for both bounds checks (NaN<0.0 and NaN>1.0
	// both false), so reject non-finite values BEFORE the range check.
	// Otherwise NaN/Inf would slip through and reach the subprocess argv
	// (e.g. 'afplay -v NaN' or 'ffplay -volume <minint>').
	v64 := float64(volume)
	if math.IsNaN(v64) || math.IsInf(v64, 0) {
		err := fmt.Errorf("volume must be a finite float between 0.0 and 1.0; got %v", volume)
		slog.Error("invalid volume setting", "volume", volume, "error", err)
		return err
	}
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

	// Fast path: source can provide a file path directly (FileSource). Exec
	// the player binary against the path without the read-then-write-temp
	// dance.
	if fp, ok := source.(FilePather); ok {
		if filePath, err := fp.FilePath(); err == nil {
			return scb.playFile(ctx, filePath)
		}
	}

	// Fall back to reader via temporary file.
	reader, format, err := source.Reader()
	if err != nil {
		slog.Error("failed to get reader from source", "error", err)
		return fmt.Errorf("failed to get audio data from source: %w", err)
	}
	defer reader.Close()

	return scb.playReaderViaTempFile(ctx, reader, format)
}

// loadVolume returns the current volume under RLock. The subprocess fork-exec
// dominates the wall-clock cost of playFile, so a mutex here is a rounding
// error; we don't need atomic loads on this code path. (The malgo realtime
// callback in playback.go is a separate site with separate constraints.)
func (scb *SystemCommandBackend) loadVolume() float32 {
	scb.mutex.RLock()
	defer scb.mutex.RUnlock()
	return scb.volume
}

// buildPlayerArgv returns the argv (NOT including scb.command itself) to play
// filePath at volume v on the configured backend. v is in [0.0, 1.0]; the
// function scales it to the backend's native value space. Backends without a
// native volume flag (e.g. aplay) ignore v and log a one-time WARN.
//
// Verified mappings (paplay, ffplay, afplay) come from each player's
// authoritative documentation. afplay's mapping is identity — review finding
// #4 incorrectly claimed 0..255 scaling; afplay treats `-v 1.0` as 100%.
func (scb *SystemCommandBackend) buildPlayerArgv(filePath string, v float64) []string {
	switch filepath.Base(scb.command) {
	case "paplay":
		// PulseAudio: --volume=N where N is uint32, 65536 = 100%.
		n := uint32(math.Round(v * 65536))
		return []string{fmt.Sprintf("--volume=%d", n), filePath}
	case "ffplay":
		// ffmpeg: -volume N where N is int, 100 = 100%.
		// -nodisp prevents ffplay opening an SDL window for audio-only input.
		// -autoexit makes ffplay exit at EOF (without it, cmd.Run() hangs).
		n := int(math.Round(v * 100))
		return []string{"-nodisp", "-autoexit", "-volume", strconv.Itoa(n), filePath}
	case "afplay":
		// macOS: -v V where V is a float; 1.0 = 100%. Identity mapping.
		return []string{"-v", strconv.FormatFloat(v, 'f', 2, 64), filePath}
	case "aplay":
		// ALSA aplay has no native volume flag. Warn once per backend instance
		// when the configured volume is not full.
		if v != 1.0 {
			scb.warnNoVolumeOnce.Do(func() {
				slog.Warn("aplay has no native volume flag; configured volume ignored",
					"command", scb.command, "volume", v)
			})
		}
		return []string{filePath}
	default:
		// Unknown / test command (e.g. "echo" in TestSystemCommandBackend_Play):
		// pass only the file path, preserving prior behavior.
		return []string{filePath}
	}
}

// playFile plays a file directly using the system command
func (scb *SystemCommandBackend) playFile(ctx context.Context, filePath string) error {
	slog.Debug("playing file via system command", "file", filePath, "command", scb.command)

	v := scb.loadVolume()
	argv := scb.buildPlayerArgv(filePath, float64(v))

	// Create command with context for cancellation
	cmd := exec.CommandContext(ctx, scb.command, argv...)

	// Run the command and wait for completion
	err := cmd.Run()
	if err != nil {
		slog.Error("system command failed", "command", scb.command, "argv", argv, "file", filePath, "error", err)
		return fmt.Errorf("system command failed: %w", err)
	}

	slog.Debug("file playback completed successfully", "file", filePath, "argv", argv)
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
