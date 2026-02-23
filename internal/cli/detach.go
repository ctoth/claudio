package cli

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"

	"claudio.click/internal/config"
	"github.com/spf13/cobra"
)

// shouldDetachHookProcessing returns true when the current invocation should
// spawn a detached child process to handle the hook event. This allows the
// Claude Code hook to return immediately rather than blocking on audio
// playback.
//
// Detaching is skipped when:
//   - Already running as a daemon child (--daemon-child flag)
//   - Running under "go test" (detected via os.Args[0])
//   - The plugin is disabled in config
//   - There is no input data to process
func shouldDetachHookProcessing(cmd *cobra.Command, cfg *config.Config, inputData []byte) bool {
	// Already a detached worker — don't recurse.
	if isDaemonChild, _ := cmd.Flags().GetBool("daemon-child"); isDaemonChild {
		slog.Debug("skipping detach: already a daemon child")
		return false
	}

	// Detect "go test" so tests run synchronously.
	if strings.HasSuffix(os.Args[0], ".test") || strings.HasSuffix(os.Args[0], ".test.exe") {
		slog.Debug("skipping detach: running under go test")
		return false
	}

	if !cfg.Enabled {
		slog.Debug("skipping detach: plugin disabled")
		return false
	}

	if len(inputData) == 0 {
		slog.Debug("skipping detach: no input data")
		return false
	}

	return true
}

// spawnDetachedHookWorker writes the hook payload to a temporary file and
// re-invokes the current executable with --daemon-child and --hook-input-file
// so that the calling process can exit immediately.
func spawnDetachedHookWorker(cmd *cobra.Command, inputData []byte) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot determine executable path: %w", err)
	}

	// Write payload to a temp file so the child can read it.
	tmpFile, err := os.CreateTemp("", "claudio-hook-*.json")
	if err != nil {
		return fmt.Errorf("cannot create temp file for hook input: %w", err)
	}

	if _, err := tmpFile.Write(inputData); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return fmt.Errorf("cannot write hook input to temp file: %w", err)
	}
	tmpFile.Close()

	// Build child args: carry over relevant flags.
	args := []string{"--daemon-child", "--hook-input-file", tmpFile.Name()}

	// Forward --config if set.
	if configPath, _ := cmd.Flags().GetString("config"); configPath != "" {
		args = append(args, "--config", configPath)
	}
	// Forward --volume if set.
	if volume, _ := cmd.Flags().GetString("volume"); volume != "" {
		args = append(args, "--volume", volume)
	}
	// Forward --soundpack if set.
	if sp, _ := cmd.Flags().GetString("soundpack"); sp != "" {
		args = append(args, "--soundpack", sp)
	}
	// Forward --silent if set.
	if silent, _ := cmd.Flags().GetBool("silent"); silent {
		args = append(args, "--silent")
	}

	child := exec.Command(exe, args...)
	child.Stdin = nil
	child.Stdout = nil
	child.Stderr = nil

	slog.Info("spawning detached hook worker",
		"exe", exe,
		"args", args,
		"hook_input_file", tmpFile.Name())

	if err := child.Start(); err != nil {
		os.Remove(tmpFile.Name())
		return fmt.Errorf("cannot start detached hook worker: %w", err)
	}

	// Release the child so it isn't reaped when we exit.
	if err := child.Process.Release(); err != nil {
		slog.Warn("failed to release detached process", "error", err)
	}

	return nil
}
