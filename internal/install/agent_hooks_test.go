package install

import (
	"reflect"
	"testing"
)

func TestInstallAgentHooksEveryAgentAddsClaudioWithoutMutatingInput(t *testing.T) {
	for _, agent := range ConcreteAgents() {
		t.Run(string(agent), func(t *testing.T) {
			settings := SettingsMap{"theme": "dark"}
			before := SettingsMap{"theme": "dark"}
			got, err := InstallAgentHooks(&settings, agent, "/usr/local/bin/claudio")
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(settings, before) {
				t.Errorf("input settings mutated: %v", settings)
			}
			hooks, ok := (*got)["hooks"].(map[string]any)
			if !ok || len(hooks) != len(agent.EnabledHooks()) {
				t.Fatalf("installed hooks = %v, want %d events", (*got)["hooks"], len(agent.EnabledHooks()))
			}
			for name, value := range hooks {
				if !IsClaudioHook(value) {
					t.Errorf("hook %s is not a claudio hook: %v", name, value)
				}
			}
		})
	}
}

// Reinstalling after an upgrade must drop claudio entries from events the
// new version no longer installs, such as Copilot's old PascalCase
// Notification key, while keeping other tools' entries there.
func TestInstallAgentHooksDropsClaudioFromRetiredEvents(t *testing.T) {
	settings := SettingsMap{"hooks": map[string]any{
		"Notification": []any{
			map[string]any{"type": "command", "command": "/usr/local/bin/claudio --hook-agent copilot"},
			map[string]any{"type": "command", "command": "other-tool"},
		},
		"Gone": []any{
			map[string]any{"type": "command", "command": "/usr/local/bin/claudio --hook-agent copilot"},
		},
	}}
	got, err := InstallAgentHooks(&settings, AgentCopilot, "/usr/local/bin/claudio")
	if err != nil {
		t.Fatal(err)
	}
	hooks := (*got)["hooks"].(map[string]any)
	if _, ok := hooks["Gone"]; ok {
		t.Errorf("retired event holding only claudio should be removed: %v", hooks["Gone"])
	}
	want := []any{map[string]any{"type": "command", "command": "other-tool"}}
	if !reflect.DeepEqual(hooks["Notification"], want) {
		t.Errorf("Notification = %v, want only other-tool", hooks["Notification"])
	}
	if _, ok := hooks["notification"]; !ok {
		t.Error("claudio should install the camelCase notification key")
	}
}

// Versions up to 1.15.0 installed claudio under WorktreeCreate and
// WorktreeRemove, which broke Claude Code worktrees. Reinstalling must
// remove those entries and keep another tool's worktree hook.
func TestInstallAgentHooksDropsClaudioFromWorktreeEvents(t *testing.T) {
	claudioGroup := map[string]any{"matcher": ".*", "hooks": []any{
		map[string]any{"type": "command", "command": "C:/Users/me/go/bin/claudio.exe"},
	}}
	userGroup := map[string]any{"hooks": []any{
		map[string]any{"type": "command", "command": "make-svn-worktree"},
	}}
	settings := SettingsMap{"hooks": map[string]any{
		"WorktreeCreate": []any{claudioGroup, userGroup},
		"WorktreeRemove": []any{claudioGroup},
	}}
	got, err := InstallAgentHooks(&settings, AgentClaude, "C:/Users/me/go/bin/claudio.exe")
	if err != nil {
		t.Fatal(err)
	}
	hooks := (*got)["hooks"].(map[string]any)
	if want := []any{userGroup}; !reflect.DeepEqual(hooks["WorktreeCreate"], want) {
		t.Errorf("WorktreeCreate = %v, want only the user's hook", hooks["WorktreeCreate"])
	}
	if _, ok := hooks["WorktreeRemove"]; ok {
		t.Errorf("WorktreeRemove holding only claudio should be removed: %v", hooks["WorktreeRemove"])
	}
	if !IsClaudioHook(hooks["Stop"]) {
		t.Error("reinstall should still install the other events")
	}
}

func TestInstallAgentHooksRejectsNonConcreteAgent(t *testing.T) {
	for _, agent := range []Agent{AgentAuto, AgentAll, Agent("bogus")} {
		if _, err := InstallAgentHooks(&SettingsMap{}, agent, "/usr/local/bin/claudio"); err == nil {
			t.Errorf("InstallAgentHooks(%q) should error", agent)
		}
	}
}

func TestInstallAgentHooksReportsMergeErrors(t *testing.T) {
	settings := SettingsMap{"hooks": "not-a-map"}
	if _, err := InstallAgentHooks(&settings, AgentClaude, "/usr/local/bin/claudio"); err == nil {
		t.Fatal("expected merge error for a non-map hooks section")
	}
}

func TestAgentSpecAccessors(t *testing.T) {
	if AgentCodex.TrustHint() == "" || AgentClaude.TrustHint() != "" {
		t.Error("only codex should have a trust hint")
	}
	if got := AgentAuto.Matcher(); got != ".*" {
		t.Errorf("non-concrete matcher = %q, want .*", got)
	}
	if got := Agent("bogus").Registry(); got != nil {
		t.Errorf("unknown agent registry = %v, want nil", got)
	}
}

// TestAgentHooksReportTooDeepSettings pins that settings nested deeper than
// the JSON decoder accepts are an error, not a panic or a silent loss.
func TestAgentHooksReportTooDeepSettings(t *testing.T) {
	var deep any = "leaf"
	for range 10001 {
		deep = map[string]any{"x": deep}
	}
	settings := SettingsMap{"deep": deep}
	if _, err := InstallAgentHooks(&settings, AgentClaude, "/usr/local/bin/claudio"); err == nil {
		t.Error("InstallAgentHooks: expected copy error")
	}
	if _, _, err := RemoveAgentHooks(&settings, AgentClaude); err == nil {
		t.Error("RemoveAgentHooks: expected copy error")
	}
}
