package cli

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"os/exec"

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

	// Tests opt out of detach via env var to keep hook processing in-process.
	if os.Getenv("CLAUDIO_DETACH_DISABLE") == "1" {
		return false
	}

	return true
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

	// Keep detached worker independent from the caller's stdio. If the
	// platform refuses to open os.DevNull (e.g. sandboxed Windows
	// AppContainer), fall back to closed pipes / nil so the child still
	// cannot chatter on the user's terminal — previously fell through to
	// exec.Cmd's parent-stdio defaults, defeating detach.
	detachStdio(childCmd)

	childCmd.Env = append(os.Environ(), "CLAUDIO_DAEMON_CHILD=1")

	if err := childCmd.Start(); err != nil {
		_ = os.Remove(hookPath)
		return fmt.Errorf("failed to start detached worker: %w", err)
	}

	slog.Debug("detached hook worker started", "pid", childCmd.Process.Pid, "input_file", hookPath)
	return nil
}

// detachStdio wires the child command's stdio so it can never inherit the
// parent terminal. Preferred path: open os.DevNull and point all three
// streams at it. Fallback (DevNull open fails): give stdin an empty reader
// and stdout/stderr a write end of a pipe whose read end is closed —
// writes go nowhere. Last resort: leave the fields nil, which makes
// os/exec connect the child's stdio to the system DevNull on most
// platforms; this is still strictly better than inheriting parent stdio.
func detachStdio(childCmd *exec.Cmd) {
	if devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0); err == nil {
		childCmd.Stdin = devNull
		childCmd.Stdout = devNull
		childCmd.Stderr = devNull
		return
	}

	// Fallback: empty stdin, closed-pipe stdout/stderr.
	childCmd.Stdin = bytes.NewReader(nil)
	rNull, wNull, perr := os.Pipe()
	if perr == nil {
		_ = rNull.Close()
		childCmd.Stdout = wNull
		childCmd.Stderr = wNull
		return
	}

	// Last resort: explicitly nil so the child does not inherit our
	// terminal even though writes may fail at runtime.
	childCmd.Stdout = nil
	childCmd.Stderr = nil
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

	if hookAgent, _ := cmd.Flags().GetString("hook-agent"); hookAgent != "" {
		args = append(args, "--hook-agent", hookAgent)
	}

	if hookEvent, _ := cmd.Flags().GetString("hook-event"); hookEvent != "" {
		args = append(args, "--hook-event", hookEvent)
	}

	return args
}
