//go:build windows

package install

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
)

func TestCodexWindowsHookExecutesLiteralPath(t *testing.T) {
	// Use a system utility so the test exercises command parsing without
	// building or invoking Claudio and without touching agent configuration.
	systemExecutable, err := exec.LookPath("where.exe")
	if err != nil {
		t.Skip("where.exe unavailable")
	}
	contents, err := os.ReadFile(systemExecutable)
	if err != nil {
		t.Fatal(err)
	}
	// PowerShell treats U+2018-U+201B as single quotes too, so each must be
	// doubled just like the ASCII apostrophe.
	for _, name := range []string{
		"Cash$rate O'Brien",
		"O\u2018Brien",
		"O\u2019Brien",
		"O\u201aBrien",
		"O\u201bBrien",
	} {
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), name)
			if err := os.Mkdir(dir, 0700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "claudio.exe")
			if err := os.WriteFile(path, contents, 0700); err != nil {
				t.Fatal(err)
			}
			command := GenerateCodexHookSpecs(path)[0].CommandWindows + " /?"
			cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", command)
			cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("hook did not execute the literal executable path: %v\n%s", err, output)
			}
		})
	}
}
