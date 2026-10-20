//go:build !windows

package cli

import (
	"os/exec"
	"syscall"
)

func startDetachedProcess(newCmd func() *exec.Cmd) (*exec.Cmd, error) {
	cmd := newCmd()
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	return cmd, cmd.Start()
}
