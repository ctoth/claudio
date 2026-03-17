package cli

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"claudio.click/internal/config"
	"github.com/spf13/cobra"
)

// shouldDetachHookProcessing determines whether hook processing should run in a detached worker.
// This is the default for non-test hook invocations with audio enabled.
func shouldDetachHookProcessing(cmd *cobra.Command, cfg *config.Config, inputData []byte) bool {
	if len(inputData) == 0 {
		return false
	}

	if cfg == nil || !cfg.Enabled {
		return false
	}

	daemonChild, _ := cmd.Flags().GetBool("daemon-child")
	if daemonChild {
		return false
	}

	// Keep unit/integration tests deterministic and in-process.
	if isGoTestBinary() {
		return false
	}

	return true
}

func isGoTestBinary() bool {
	base := strings.ToLower(filepath.Base(os.Args[0]))
	return strings.HasSuffix(base, ".test") || strings.HasSuffix(base, ".test.exe")
}

func spawnDetachedHookWorker(cmd *cobra.Command, inputData []byte) error {
	hookFile, err := os.CreateTemp("", "claudio-hook-*.json")
	if err != nil {
		return fmt.Errorf("failed to create hook payload file: %w", err)
	}

	hookPath := hookFile.Name()

	if _, err := hookFile.Write(inputData); err != nil {
		hookFile.Close()
		_ = os.Remove(hookPath)
		return fmt.Errorf("failed to write hook payload file: %w", err)
	}

	if err := hookFile.Close(); err != nil {
		_ = os.Remove(hookPath)
		return fmt.Errorf("failed to close hook payload file: %w", err)
	}

	childArgs := buildDetachedWorkerArgs(cmd, hookPath)

	executablePath, err := os.Executable()
	if err != nil {
		_ = os.Remove(hookPath)
		return fmt.Errorf("failed to resolve executable path: %w", err)
	}

	childCmd := exec.Command(executablePath, childArgs...)
	configureDetachedProcess(childCmd)

	// Keep detached worker independent from the caller's stdio.
	if devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0); err == nil {
		defer devNull.Close()
		childCmd.Stdout = devNull
		childCmd.Stderr = devNull
	}

	childCmd.Env = append(os.Environ(), "CLAUDIO_DAEMON_CHILD=1")

	if err := childCmd.Start(); err != nil {
		_ = os.Remove(hookPath)
		return fmt.Errorf("failed to start detached worker: %w", err)
	}

	slog.Debug("detached hook worker started", "pid", childCmd.Process.Pid, "input_file", hookPath)
	return nil
}

func buildDetachedWorkerArgs(cmd *cobra.Command, hookInputFile string) []string {
	args := []string{"--daemon-child", "--hook-input-file", hookInputFile}

	if configFile, _ := cmd.Flags().GetString("config"); configFile != "" {
		args = append(args, "--config", configFile)
	}

	if volume, _ := cmd.Flags().GetString("volume"); volume != "" {
		args = append(args, "--volume", volume)
	}

	if soundpack, _ := cmd.Flags().GetString("soundpack"); soundpack != "" {
		args = append(args, "--soundpack", soundpack)
	}

	if silent, _ := cmd.Flags().GetBool("silent"); silent {
		args = append(args, "--silent")
	}

	return args
}
