package install

import (
	"reflect"
	"testing"
)

// userSettingsFor returns settings holding only user-owned hooks in the
// agent's shape.
func userSettingsFor(agent Agent) SettingsMap {
	if agent == AgentCopilot {
		return SettingsMap{"theme": "dark", "hooks": map[string]interface{}{
			"PreToolUse": []interface{}{map[string]interface{}{"type": "command", "command": "echo user"}},
		}}
	}
	return SettingsMap{"theme": "dark", "hooks": map[string]interface{}{
		"PreToolUse": []interface{}{map[string]interface{}{
			"matcher": "Bash",
			"hooks":   []interface{}{map[string]interface{}{"type": "command", "command": "echo user"}},
		}},
	}}
}

// TestRemoveAgentHooksUndoesInstallForEveryAgent pins that the one
// install/uninstall engine is symmetric per agent shape.
func TestRemoveAgentHooksUndoesInstallForEveryAgent(t *testing.T) {
	for _, agent := range ConcreteAgents() {
		t.Run(string(agent), func(t *testing.T) {
			user := userSettingsFor(agent)
			installed, err := InstallAgentHooks(&user, agent, "/usr/local/bin/claudio")
			if err != nil {
				t.Fatal(err)
			}
			names := ClaudioHookNames(installed)
			if len(names) != len(agent.EnabledHooks()) {
				t.Fatalf("ClaudioHookNames = %v, want %d names", names, len(agent.EnabledHooks()))
			}
			removed, removedNames, err := RemoveAgentHooks(installed, agent)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(removedNames, names) {
				t.Errorf("removed names = %v, want %v", removedNames, names)
			}
			if want := userSettingsFor(agent); !reflect.DeepEqual(*removed, want) {
				t.Fatalf("after remove = %v, want %v", *removed, want)
			}
			if len(ClaudioHookNames(installed)) == 0 {
				t.Error("RemoveAgentHooks mutated its input")
			}
		})
	}
}

func TestRemoveAgentHooksRejectsNonConcreteAgent(t *testing.T) {
	if _, _, err := RemoveAgentHooks(&SettingsMap{}, AgentAll); err == nil {
		t.Fatal("expected error for a non-concrete agent")
	}
}

func TestClaudioHookNamesIsSortedAndIgnoresUserHooks(t *testing.T) {
	settings := SettingsMap{"hooks": map[string]interface{}{
		"Stop":       "claudio",
		"Other":      "echo hi",
		"PreToolUse": []interface{}{map[string]interface{}{"type": "command", "command": "/opt/claudio"}},
		"Weird":      float64(3),
	}}
	if got, want := ClaudioHookNames(&settings), []string{"PreToolUse", "Stop"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ClaudioHookNames = %v, want %v", got, want)
	}
	for _, s := range []*SettingsMap{nil, {}, {"hooks": "not-a-map"}} {
		if got := ClaudioHookNames(s); len(got) != 0 {
			t.Errorf("ClaudioHookNames(%v) = %v, want none", s, got)
		}
	}
}
