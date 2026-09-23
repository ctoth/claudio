package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"claudio.click/internal/cli/testenv"
)

// TestInstallUnknownHookShapeLeavesSettingsUnchanged pins that install
// refuses a hook value it does not understand and does not rewrite the
// user's settings file.
func TestInstallUnknownHookShapeLeavesSettingsUnchanged(t *testing.T) {
	root := testenv.IsolateXDG(t)
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("CLAUDIO_TEST_RECOGNIZE_GO_TEST", "1")

	settingsPath := filepath.Join(root, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		t.Fatal(err)
	}
	original := []byte(`{"hooks":{"PreToolUse":{"command":"echo user"}},"theme":"dark"}`)
	if err := os.WriteFile(settingsPath, original, 0644); err != nil {
		t.Fatal(err)
	}

	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	code := NewCLI().Run([]string{"claudio", "install", "--agent", "claude", "--quiet"}, strings.NewReader(""), stdout, stderr)
	if code == 0 {
		t.Fatalf("install over an unknown hook shape succeeded; stdout=%q", stdout)
	}
	got, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, original) {
		t.Fatalf("settings file changed:\n got %s\nwant %s", got, original)
	}
}
