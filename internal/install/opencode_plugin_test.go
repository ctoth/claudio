package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/afero"
)

// TestOpenCodePluginDetectionAndUninstallWorkflow covers the plugin branches
// of auto-detection and the uninstall workflow, using OPENCODE_CONFIG_DIR.
func TestOpenCodePluginDetectionAndUninstallWorkflow(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OPENCODE_CONFIG_DIR", dir)
	path := filepath.Join(dir, "plugins", "claudio.js")

	if hasExistingClaudioHooks(AgentOpenCode, ScopeGlobal) {
		t.Fatal("detected a plugin before one was written")
	}
	if err := WriteOpenCodePlugin(path, "/usr/local/bin/claudio"); err != nil {
		t.Fatal(err)
	}
	if !hasExistingClaudioHooks(AgentOpenCode, ScopeGlobal) {
		t.Fatal("did not detect the written plugin")
	}

	if err := RunUninstallWorkflow(afero.NewOsFs(), AgentTarget{Agent: AgentOpenCode, ConfigPath: path}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("plugin still present after uninstall workflow: %v", err)
	}
}

func TestWriteOpenCodePluginFailsWhenDirectoryIsAFile(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "plugins")
	if err := os.WriteFile(blocker, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteOpenCodePlugin(filepath.Join(blocker, "claudio.js"), "claudio"); err == nil {
		t.Fatal("expected an error when the plugin directory is a file")
	}
}

func TestOpenCodePlugin(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plugins", "claudio.js")
	exe := `C:\Program Files\claudio\claudio.exe`

	if err := WriteOpenCodePlugin(path, exe); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := `const CLAUDIO = "C:\\Program Files\\claudio\\claudio.exe"`; !strings.Contains(string(data), want) {
		t.Errorf("plugin does not embed %s:\n%s", want, data)
	}
	if !HasOpenCodePlugin(path) {
		t.Fatal("HasOpenCodePlugin = false after write")
	}

	if err := RemoveOpenCodePlugin(path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("plugin still present after remove: %v", err)
	}
	if err := RemoveOpenCodePlugin(path); err != nil {
		t.Errorf("removing a missing plugin: %v", err)
	}

	// A plugin claudio did not write is never deleted.
	if err := os.WriteFile(path, []byte("export const Mine = async () => ({})\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := RemoveOpenCodePlugin(path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("user plugin was removed: %v", err)
	}
}
