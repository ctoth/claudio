package install

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/afero"
)

func TestOpenCodePluginProjectPrecedence(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is needed to execute the generated plugin")
	}
	root := t.TempDir()
	global := filepath.Join(root, "global", "claudio.js")
	project := filepath.Join(root, "repo", ".opencode", "plugins", "claudio.js")
	for _, path := range []string{global, project} {
		if err := WriteOpenCodePlugin(path, "claudio"); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"type":"module"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	worktree := filepath.Join(root, "repo")
	directory := filepath.Join(worktree, "sub")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	script := `import { pathToFileURL } from "node:url";
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
const [globalPath, projectPath, directory, worktree] = process.argv.slice(1);
const load = async (path) => (await import(pathToFileURL(path))).ClaudioPlugin({ directory, worktree });
const active = (hooks) => Object.keys(hooks).length > 0;
if (active(await load(globalPath)) || !active(await load(projectPath))) throw Error("project copy must win from subdirectory");
writeFileSync(projectPath, "export const Mine = () => ({});\n");
const outside = join(dirname(worktree), ".opencode", "plugins", "claudio.js");
mkdirSync(dirname(outside), { recursive: true });
writeFileSync(outside, readFileSync(globalPath));
if (!active(await load(globalPath))) throw Error("foreign project file silenced global plugin");
`
	output, err := exec.Command(node, "--input-type=module", "-e", script, global, project, directory, worktree).CombinedOutput()
	if err != nil {
		t.Fatalf("generated plugin check: %v\n%s", err, output)
	}
}

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

func TestWriteOpenCodePluginRefusesForeignFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plugins", "claudio.js")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	// A claudio.js the user wrote themselves must survive install.
	if err := os.WriteFile(path, []byte("export const Mine = async () => ({})\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteOpenCodePlugin(path, "/usr/local/bin/claudio"); err == nil {
		t.Fatal("expected an error when claudio.js was not written by claudio")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "export const Mine = async () => ({})\n" {
		t.Errorf("user plugin was modified:\n%s", data)
	}
}

func TestWriteOpenCodePluginRewritesOwnFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plugins", "claudio.js")
	if err := WriteOpenCodePlugin(path, "/usr/local/bin/claudio"); err != nil {
		t.Fatal(err)
	}
	// Rewriting our own plugin (e.g. after the executable moved) still works.
	if err := WriteOpenCodePlugin(path, "/opt/claudio/claudio"); err != nil {
		t.Fatalf("rewriting our own plugin: %v", err)
	}
	if !HasOpenCodePlugin(path) {
		t.Fatal("HasOpenCodePlugin = false after rewrite")
	}
}

func TestOpenCodePluginTemplateGuards(t *testing.T) {
	source := OpenCodePluginSource("/usr/local/bin/claudio")

	// The stale process-wide guard must be gone: it silenced every load
	// after the first, including project reloads and extra directories.
	if strings.Contains(source, "__claudioPlugin") {
		t.Error("template still uses the process-wide __claudioPlugin guard")
	}
	// Instead, the global copy stands down when a project copy exists,
	// decided on every load.
	for _, want := range []string{
		`".opencode", "plugins", "claudio.js"`,
		"readFileSync(projectPlugin",
		"fileURLToPath(import.meta.url)",
	} {
		if !strings.Contains(source, want) {
			t.Errorf("template missing project-copy stand-down logic %q", want)
		}
	}
	// Esc interrupts arrive as MessageAbortedError and must not play the
	// error sound.
	if !strings.Contains(source, `p.error?.name !== "MessageAbortedError"`) {
		t.Error("template does not skip MessageAbortedError on session.error")
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
