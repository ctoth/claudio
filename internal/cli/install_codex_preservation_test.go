package cli

import (
	"os"
	"path/filepath"
	"testing"

	"claudio.click/internal/install"
)

func TestCodexInstallPreservesInvalidHookShapes(t *testing.T) {
	t.Setenv("CLAUDIO_TEST_RECOGNIZE_GO_TEST", "1")
	for _, input := range []string{
		`{"hooks":"preserve this"}`,
		`{"hooks":{"PreToolUse":{"user":"preserve this"}}}`,
	} {
		t.Run(input, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "hooks.json")
			if err := os.WriteFile(path, []byte(input), 0600); err != nil {
				t.Fatal(err)
			}
			if err := runInstallWorkflow(install.AgentCodex, install.ScopeGlobal, path); err == nil {
				t.Fatal("expected invalid hook configuration to be rejected")
			}
			contents, err := os.ReadFile(path)
			if err != nil || string(contents) != input {
				t.Fatalf("existing configuration changed: %q, %v", contents, err)
			}
		})
	}
}
