package audio

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"claudio.click/internal/volume"
)

// SystemCommandBackend implements AudioBackend using system commands like paplay
type SystemCommandBackend struct {
	commands         []string
	volume           float32
	closed           bool
	mutex            sync.RWMutex
	warnNoVolumeOnce sync.Once // one WARN per backend instance for aplay
}

// NewSystemCommandBackend creates a new SystemCommandBackend with the specified
// commands in priority order. A single command preserves the historical call
// shape; multiple commands enable best-effort fallback when the primary command
// fails or cannot handle the file format.
func NewSystemCommandBackend(commands ...string) *SystemCommandBackend {
	return &SystemCommandBackend{
		commands: append([]string(nil), commands...),
		volume:   1.0, // Default full volume
	}
}

// Close shuts down the backend
func (scb *SystemCommandBackend) Close() error {
	scb.mutex.Lock()
	defer scb.mutex.Unlock()

	scb.closed = true
	return nil
}

// SetVolume sets the volume level (0.0 to 1.0)
func (scb *SystemCommandBackend) SetVolume(v float32) error {
	// Non-finite values would otherwise reach the player argv
	// (e.g. 'afplay -v NaN').
	if err := volume.Validate(float64(v)); err != nil {
		return err
	}

	scb.mutex.Lock()
	defer scb.mutex.Unlock()

	if scb.closed {
		return ErrBackendClosed
	}

	oldVolume := scb.volume
	scb.volume = v
	slog.Debug("volume changed", "old_volume", oldVolume, "new_volume", v)
	return nil
}

// Play plays audio from the given source using system commands
func (scb *SystemCommandBackend) Play(ctx context.Context, source AudioSource) error {
	scb.mutex.RLock()
	closed := scb.closed
	scb.mutex.RUnlock()
	if closed {
		return ErrBackendClosed
	}

	slog.Debug("SystemCommandBackend starting playback", "commands", scb.commands)

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
		return fmt.Errorf("failed to get audio data from source: %w", err)
	}
	defer reader.Close()

	return scb.playReaderViaTempFile(ctx, reader, format)
}

// loadVolume returns the current volume under RLock. The subprocess fork-exec
// dominates the wall-clock cost of playFile, so a mutex here is a rounding
// error; we don't need atomic loads on this code path.
func (scb *SystemCommandBackend) loadVolume() float32 {
	scb.mutex.RLock()
	defer scb.mutex.RUnlock()
	return scb.volume
}

// buildPlayerArgvForCommand returns the argv (NOT including the command
// itself) to play filePath at volume v with command. v is in [0.0, 1.0]; the
// function scales it to the player's native value space. Players without a
// native volume flag (e.g. aplay) ignore v and log a one-time WARN.
//
// Verified mappings (paplay, ffplay, afplay) come from each player's
// authoritative documentation. afplay's mapping is identity: `-v 1.0` is 100%.
func (scb *SystemCommandBackend) buildPlayerArgvForCommand(command, filePath string, v float64) []string {
	switch filepath.Base(command) {
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
					"command", command, "volume", v)
			})
		}
		return []string{filePath}
	default:
		// Unknown / test command (e.g. "echo" in TestSystemCommandBackend_Play):
		// pass only the file path, preserving prior behavior.
		return []string{filePath}
	}
}

// commandSupportsFormat reports whether a system audio command should be tried
// for a file extension. aplay is limited to WAV on the supported platforms; the
// other known command players are treated as general-purpose decoders.
func commandSupportsFormat(command, ext string) bool {
	if filepath.Base(command) != "aplay" {
		return true
	}
	return strings.EqualFold(ext, ".wav")
}

// playFile plays a file directly using the configured system command chain.
func (scb *SystemCommandBackend) playFile(ctx context.Context, filePath string) error {
	slog.Debug("playing file via system command", "file", filePath, "commands", scb.commands)

	v := scb.loadVolume()
	ext := filepath.Ext(filePath)
	var lastErr error
	var attempted int

	for i, command := range scb.commands {
		if !commandSupportsFormat(command, ext) {
			slog.Debug("skipping system command unsupported for format",
				"command", command, "ext", ext, "file", filePath)
			continue
		}

		attempted++
		argv := scb.buildPlayerArgvForCommand(command, filePath, float64(v))
		cmd := exec.CommandContext(ctx, command, argv...)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr

		err := cmd.Run()
		if err == nil {
			if i > 0 {
				slog.Info("playback succeeded via fallback",
					"command", command, "argv", argv, "file", filePath, "attempt", i+1)
			} else {
				slog.Debug("file playback completed successfully", "file", filePath, "argv", argv)
			}
			return nil
		}

		lastErr = err
		stderrText := strings.TrimSpace(stderr.String())
		if attempted == 1 {
			slog.Warn("primary audio command failed",
				"command", command, "argv", argv, "file", filePath, "error", err, "stderr", stderrText)
		} else {
			slog.Warn("fallback audio command failed",
				"command", command, "argv", argv, "file", filePath, "error", err, "stderr", stderrText)
		}
	}

	if lastErr == nil {
		return fmt.Errorf("no audio commands support format %q", ext)
	}
	return fmt.Errorf("all audio commands failed for %s: %w", filepath.Base(filePath), lastErr)
}

// playReaderViaTempFile writes reader data to a temporary file and plays it
func (scb *SystemCommandBackend) playReaderViaTempFile(ctx context.Context, reader io.Reader, format string) error {
	slog.Debug("playing reader via temporary file", "format", format)

	// Create temporary file with appropriate extension
	tempFile, err := os.CreateTemp("", "claudio-*."+format)
	if err != nil {
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
		return fmt.Errorf("failed to write audio data to temporary file: %w", err)
	}

	// Close file before playing
	err = tempFile.Close()
	if err != nil {
		return fmt.Errorf("failed to close temporary file: %w", err)
	}

	slog.Debug("temporary file created successfully", "path", tempPath, "format", format)

	// Play the temporary file
	return scb.playFile(ctx, tempPath)
}
