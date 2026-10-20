//go:build windows

package cli

import (
	"errors"
	"log/slog"
	"os/exec"
	"syscall"
)

const (
	createNewProcessGroup  = 0x00000200
	detachedProcess        = 0x00000008
	createBreakawayFromJob = 0x01000000
)

// startDetachedProcess starts the worker outside the agent's job object, so
// it outlives the hook. Claude Code's job does not set
// JOB_OBJECT_LIMIT_BREAKAWAY_OK, so breakaway fails with "Access is denied"
// (#18); the worker then starts inside the job instead.
func startDetachedProcess(newCmd func() *exec.Cmd) (*exec.Cmd, error) {
	cmd, err := startWithFlags(newCmd(), createNewProcessGroup|detachedProcess|createBreakawayFromJob)
	if errors.Is(err, syscall.ERROR_ACCESS_DENIED) {
		slog.Debug("job forbids breakaway, starting worker inside it", "error", err)
		cmd, err = startWithFlags(newCmd(), createNewProcessGroup|detachedProcess)
	}
	return cmd, err
}

func startWithFlags(cmd *exec.Cmd, flags uint32) (*exec.Cmd, error) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: flags,
		HideWindow:    true,
	}
	return cmd, cmd.Start()
}
