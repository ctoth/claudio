//go:build windows

package cli

import (
	"os/exec"
	"syscall"
)

const (
	createNewProcessGroup  = 0x00000200
	detachedProcess        = 0x00000008
	createBreakawayFromJob = 0x01000000
)

func configureDetachedProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: createNewProcessGroup | detachedProcess | createBreakawayFromJob,
		HideWindow:    true,
	}
}
