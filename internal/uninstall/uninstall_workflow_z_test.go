package uninstall

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"claudio.click/internal/install"
	"github.com/spf13/afero"
)

// TestRunUninstallWorkflowUsesAgentResolvedPath asserts that the workflow
// targets the path returned by agentResolver — not any value passed by the
// caller. Regression test for the latent vulnerability that a caller could
// pass scope=user with an unrelated settingsPath and the validation would
// not prevent the write going elsewhere.
func TestRunUninstallWorkflowUsesAgentResolvedPath(t *testing.T) {
	tempDir := t.TempDir()
	resolvedPath := filepath.Join(tempDir, "resolved", "settings.json")
	decoyPath := filepath.Join(tempDir, "decoy", "settings.json")

	if err := os.MkdirAll(filepath.Dir(resolvedPath), 0755); err != nil {
		t.Fatalf("mkdir resolved dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(decoyPath), 0755); err != nil {
		t.Fatalf("mkdir decoy dir: %v", err)
	}

	// Seed both files with claudio hooks so we can tell which one the
	// workflow rewrote.
	initial := install.SettingsMap{
		"hooks": map[string]interface{}{
			"PreToolUse": "/usr/local/bin/claudio",
		},
		"version": "test",
	}
	data, err := json.Marshal(initial)
	if err != nil {
		t.Fatalf("marshal initial settings: %v", err)
	}
	if err := os.WriteFile(resolvedPath, data, 0644); err != nil {
		t.Fatalf("write resolved file: %v", err)
	}
	if err := os.WriteFile(decoyPath, data, 0644); err != nil {
		t.Fatalf("write decoy file: %v", err)
	}

	// Inject the resolved path via the resolver — the workflow's only
	// source of truth for the target path.
	swapAgentResolver(t, fixedPathResolver(resolvedPath))

	if err := RunUninstallWorkflow(afero.NewOsFs(), "user", install.AgentClaude); err != nil {
		t.Fatalf("workflow failed: %v", err)
	}

	// The resolved path must have had its claudio hook removed.
	resolvedAfter, err := os.ReadFile(resolvedPath)
	if err != nil {
		t.Fatalf("read resolved after: %v", err)
	}
	var resolvedSettings install.SettingsMap
	if err := json.Unmarshal(resolvedAfter, &resolvedSettings); err != nil {
		t.Fatalf("unmarshal resolved after: %v", err)
	}
	if hooks, ok := resolvedSettings["hooks"].(map[string]interface{}); ok {
		if _, present := hooks["PreToolUse"]; present {
			t.Errorf("resolved path's PreToolUse hook should have been removed, but it is still present: %v", hooks)
		}
	}

	// The decoy path must be untouched.
	decoyAfter, err := os.ReadFile(decoyPath)
	if err != nil {
		t.Fatalf("read decoy after: %v", err)
	}
	if string(decoyAfter) != string(data) {
		t.Errorf("decoy path was modified — workflow should only write to the agent-resolved path.\nbefore: %s\nafter:  %s", data, decoyAfter)
	}
}

// TestRunUninstallWorkflowAgentResolverError asserts the workflow surfaces
// errors from the agent path resolution rather than silently writing to an
// empty path.
func TestRunUninstallWorkflowAgentResolverError(t *testing.T) {
	swapAgentResolver(t, func(install.Agent, string) (string, error) {
		return "", os.ErrNotExist
	})

	err := RunUninstallWorkflow(afero.NewOsFs(), "user", install.AgentClaude)
	if err == nil {
		t.Fatalf("expected resolver error to surface, got nil")
	}
	if !strings.Contains(err.Error(), "failed to resolve settings path") {
		t.Errorf("expected wrapping error message; got %q", err.Error())
	}
}

func TestRunUninstallWorkflowMissingSettingsFileIsNoop(t *testing.T) {
	settingsPath := filepath.Join(t.TempDir(), "missing", "settings.json")
	swapAgentResolver(t, fixedPathResolver(settingsPath))

	if err := RunUninstallWorkflow(afero.NewOsFs(), install.ScopeGlobal, install.AgentClaude); err != nil {
		t.Fatalf("missing settings file should be an idempotent uninstall, got: %v", err)
	}

	if _, err := os.Stat(filepath.Dir(settingsPath)); !os.IsNotExist(err) {
		t.Fatalf("uninstall should not create missing settings directory, stat err: %v", err)
	}
}

func TestRunUninstallWorkflowCodexPreservesMixedGroupSibling(t *testing.T) {
	settingsPath := filepath.Join(t.TempDir(), "settings.json")
	initial := install.SettingsMap{
		"hooks": map[string]interface{}{
			"Stop": []interface{}{
				map[string]interface{}{
					"matcher": "*",
					"hooks": []interface{}{
						map[string]interface{}{
							"type":           "command",
							"command":        "C:/Users/Q/bin/claudio.exe",
							"commandWindows": `& "C:/Users/Q/bin/claudio.exe"`,
						},
						map[string]interface{}{
							"type":    "command",
							"command": "custom-stop-hook",
						},
					},
				},
			},
		},
		"version": "test",
	}
	data, err := json.Marshal(initial)
	if err != nil {
		t.Fatalf("marshal initial settings: %v", err)
	}
	if err := os.WriteFile(settingsPath, data, 0644); err != nil {
		t.Fatalf("write settings: %v", err)
	}

	swapAgentResolver(t, fixedPathResolver(settingsPath))
	if err := RunUninstallWorkflow(afero.NewOsFs(), install.ScopeGlobal, install.AgentCodex); err != nil {
		t.Fatalf("workflow failed: %v", err)
	}

	after, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read settings after uninstall: %v", err)
	}
	var settings install.SettingsMap
	if err := json.Unmarshal(after, &settings); err != nil {
		t.Fatalf("unmarshal settings after uninstall: %v", err)
	}
	hooks := settings["hooks"].(map[string]interface{})
	groups := hooks["Stop"].([]interface{})
	group := groups[0].(map[string]interface{})
	entries := group["hooks"].([]interface{})
	if len(entries) != 1 {
		t.Fatalf("expected one surviving custom command, got %v", entries)
	}
	entry := entries[0].(map[string]interface{})
	if command := entry["command"]; command != "custom-stop-hook" {
		t.Fatalf("expected custom sibling to survive, got command %v", command)
	}
}
