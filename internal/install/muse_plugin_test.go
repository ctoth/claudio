package install

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
)

func TestMusePluginManifestHooks(t *testing.T) {
	var m museManifest
	if err := json.Unmarshal(MusePluginManifest(`C:\tools\claudio.exe`), &m); err != nil {
		t.Fatalf("manifest does not parse: %v", err)
	}
	if m.Name != MusePluginName || m.SchemaVersion != 1 {
		t.Errorf("manifest identity = %q v%d, want %q v1", m.Name, m.SchemaVersion, MusePluginName)
	}
	hooks := m.Capabilities.Hooks
	if len(hooks) != len(MuseHooks) {
		t.Fatalf("manifest hooks = %d, want %d (MuseHooks)", len(hooks), len(MuseHooks))
	}
	seen := map[string]bool{}
	for i, h := range hooks {
		if h.Event != MuseHooks[i].Name {
			t.Errorf("hook %d event = %q, want %q", i, h.Event, MuseHooks[i].Name)
		}
		if h.ID != MuseHookID(h.Event) || seen[h.ID] {
			t.Errorf("hook %q id = %q, want unique kebab id", h.Event, h.ID)
		}
		seen[h.ID] = true
		if len(h.Command) != 1 || h.Command[0] != `C:\tools\claudio.exe` {
			t.Errorf("hook %q command = %v, want the claudio executable", h.Event, h.Command)
		}
	}
}

func TestMuseHookID(t *testing.T) {
	for event, want := range map[string]string{
		"PreToolUse": "pre-tool-use", "PostToolBatch": "post-tool-batch", "Stop": "stop",
	} {
		if got := MuseHookID(event); got != want {
			t.Errorf("MuseHookID(%q) = %q, want %q", event, got, want)
		}
	}
}

func TestWriteMusePluginRefusesForeignBundle(t *testing.T) {
	dir := t.TempDir()
	manifest := filepath.Join(dir, museManifestPath)
	if err := os.MkdirAll(filepath.Dir(manifest), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifest, []byte(`{"name":"someone-else"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteMusePlugin(dir, "claudio"); err == nil {
		t.Error("expected refusal to overwrite a foreign plugin bundle")
	}
	if HasMusePlugin(dir) {
		t.Error("foreign bundle reported as claudio's")
	}
	if err := RemoveMusePlugin(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(manifest); err != nil {
		t.Error("RemoveMusePlugin deleted a foreign bundle")
	}
}

func TestWriteAndRemoveMusePlugin(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "claudio")
	if err := WriteMusePlugin(dir, "claudio"); err != nil {
		t.Fatal(err)
	}
	if !HasMusePlugin(dir) {
		t.Fatal("written bundle not recognized as claudio's")
	}
	// Rewriting (reinstall) is allowed and stays ours.
	if err := WriteMusePlugin(dir, "claudio"); err != nil {
		t.Fatal(err)
	}
	if err := RemoveMusePlugin(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Error("RemoveMusePlugin left the bundle behind")
	}
}

func TestHasMusePluginInvalidManifest(t *testing.T) {
	dir := t.TempDir()
	if HasMusePlugin(dir) {
		t.Error("missing manifest reported as claudio's")
	}
	manifest := filepath.Join(dir, museManifestPath)
	if err := os.MkdirAll(filepath.Dir(manifest), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifest, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if HasMusePlugin(dir) {
		t.Error("invalid manifest reported as claudio's")
	}
}

func TestWriteMusePluginCreateDirFails(t *testing.T) {
	dir := t.TempDir()
	// A regular file where the bundle's .muse-plugin directory must go.
	if err := os.WriteFile(filepath.Join(dir, ".muse-plugin"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteMusePlugin(dir, "claudio"); err == nil {
		t.Error("expected error when the plugin directory cannot be created")
	}
}

func TestRunUninstallWorkflowMuse(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "claudio")
	if err := WriteMusePlugin(dir, "claudio"); err != nil {
		t.Fatal(err)
	}
	target := AgentTarget{Agent: AgentMuse, ConfigPath: dir}
	if err := RunUninstallWorkflow(afero.NewOsFs(), target); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Error("uninstall left the muse bundle behind")
	}
	// Missing bundle is a no-op.
	if err := RunUninstallWorkflow(afero.NewOsFs(), target); err != nil {
		t.Fatal(err)
	}
}

func TestMuseConfigPaths(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "xdg"))
	paths, err := AgentMuse.ConfigPaths(ScopeGlobal)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "muse", "plugins", "claudio")
	if len(paths) != 1 || paths[0] != want {
		t.Errorf("global paths = %v, want [%s]", paths, want)
	}
	project, err := AgentMuse.ConfigPaths(ScopeProject)
	if err != nil {
		t.Fatal(err)
	}
	if len(project) != 1 || project[0] != filepath.Join(".muse-plugin", "claudio") {
		t.Errorf("project paths = %v, want [.muse-plugin/claudio]", project)
	}
	if !AgentMuse.UsesPluginBundle() || AgentMuse.UsesPluginFile() {
		t.Error("muse should use a plugin bundle, not a plugin file or settings")
	}
	if got := AgentMuse.Registry(); len(got) != len(MuseHooks) {
		t.Errorf("muse registry = %d hooks, want %d", len(got), len(MuseHooks))
	}
}
