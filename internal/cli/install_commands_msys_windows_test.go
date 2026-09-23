//go:build windows

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"claudio.click/internal/cli/testenv"
)

// msysPath converts C:\a\b to /c/a/b, the form Git Bash exports as HOME.
func msysPath(p string) string {
	return "/" + strings.ToLower(p[:1]) + filepath.ToSlash(p[2:])
}

// TestInstallCommandsHonorsMSYSHome pins that install-commands resolves
// the home directory the same way hook install does: with USERPROFILE
// unset, an MSYS-style HOME (/c/Users/x) is used after normalization.
func TestInstallCommandsHonorsMSYSHome(t *testing.T) {
	testenv.IsolateXDG(t)
	home := t.TempDir()
	t.Setenv("USERPROFILE", "")
	t.Setenv("HOME", msysPath(home))
	t.Setenv("CLAUDE_CONFIG_DIR", "")

	for _, tc := range []struct {
		agent string
		file  string
	}{
		{"claude", filepath.Join(home, ".claude", "commands", "claudio.md")},
		{"codex", filepath.Join(home, ".agents", "skills", "claudio", "SKILL.md")},
	} {
		stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
		code := NewCLI().Run([]string{"claudio", "install-commands", "--agent", tc.agent}, strings.NewReader(""), stdout, stderr)
		if code != 0 {
			t.Fatalf("install-commands --agent %s exit %d; stderr=%q", tc.agent, code, stderr)
		}
		if _, err := os.Stat(tc.file); err != nil {
			t.Errorf("install-commands --agent %s did not write %s: %v", tc.agent, tc.file, err)
		}
	}
}
