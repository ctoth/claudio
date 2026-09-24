package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"claudio.click/internal/cli/testenv"
)

// TestInstallWithoutHomeFailsAndCreatesNoTildeDir pins that a global
// install with no home directory errors instead of writing into a literal
// "~" directory under the current working directory.
func TestInstallWithoutHomeFailsAndCreatesNoTildeDir(t *testing.T) {
	testenv.IsolateXDG(t)
	for _, key := range []string{"HOME", "USERPROFILE", "HOMEDRIVE", "HOMEPATH", "CLAUDE_CONFIG_DIR", "CODEX_HOME", "COPILOT_HOME"} {
		t.Setenv(key, "")
	}
	t.Setenv("CLAUDIO_TEST_RECOGNIZE_GO_TEST", "1")
	t.Chdir(t.TempDir())

	for _, agent := range []string{"claude", "codex", "gemini", "qwen", "copilot"} {
		stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
		code := NewCLI().Run([]string{"claudio", "install", "--agent", agent, "--scope", "global", "--quiet"}, strings.NewReader(""), stdout, stderr)
		if code == 0 {
			t.Errorf("install --agent %s without a home directory succeeded; stdout=%q", agent, stdout)
		}
		if _, err := os.Stat("~"); !os.IsNotExist(err) {
			t.Fatalf("install --agent %s created a literal ~ directory (err=%v)", agent, err)
		}
	}
}
