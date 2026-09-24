package install

import (
	"reflect"
	"strings"
	"testing"
)

// TestInstallRejectsUnknownHookValueShape pins that an existing hook value
// that is neither a string nor an array is an error naming the agent and
// the event, not a "%v" command, and that the input is left alone.
func TestInstallRejectsUnknownHookValueShape(t *testing.T) {
	for _, agent := range ConcreteAgents() {
		for name, value := range map[string]any{
			"number": float64(42),
			"object": map[string]any{"command": "echo hi"},
			"bool":   true,
		} {
			t.Run(string(agent)+"/"+name, func(t *testing.T) {
				event := firstEvent(agent)
				settings := SettingsMap{"hooks": map[string]any{event: value}}
				before := SettingsMap{"hooks": map[string]any{event: value}}
				_, err := InstallAgentHooks(&settings, agent, "/usr/local/bin/claudio")
				if err == nil {
					t.Fatal("expected error for unknown hook value shape")
				}
				for _, want := range []string{string(agent), event} {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("error %q does not name %q", err, want)
					}
				}
				if !reflect.DeepEqual(settings, before) {
					t.Errorf("input mutated: %v", settings)
				}
			})
		}
	}
}

// TestInstallRejectsNonObjectHooksSection pins that a hooks section that is
// not a JSON object is an error rather than being overwritten.
func TestInstallRejectsNonObjectHooksSection(t *testing.T) {
	for _, agent := range ConcreteAgents() {
		for _, hooks := range []any{"not-a-map", []any{}, float64(1)} {
			settings := SettingsMap{"hooks": hooks}
			if _, err := InstallAgentHooks(&settings, agent, "/usr/local/bin/claudio"); err == nil {
				t.Errorf("%s: expected error for hooks = %v", agent, hooks)
			}
		}
	}
}

// TestInstallTreatsNullAsAbsent pins that a JSON null hooks section or
// event value is treated as absent: install succeeds and writes exactly
// what it writes into settings without them.
func TestInstallTreatsNullAsAbsent(t *testing.T) {
	for _, agent := range ConcreteAgents() {
		t.Run(string(agent), func(t *testing.T) {
			want, err := InstallAgentHooks(&SettingsMap{}, agent, "/usr/local/bin/claudio")
			if err != nil {
				t.Fatal(err)
			}
			for name, settings := range map[string]SettingsMap{
				"null hooks section": {"hooks": nil},
				"null event":         {"hooks": map[string]any{firstEvent(agent): nil}},
			} {
				got, err := InstallAgentHooks(&settings, agent, "/usr/local/bin/claudio")
				if err != nil {
					t.Fatalf("%s: %v", name, err)
				}
				if !reflect.DeepEqual(got, want) {
					t.Errorf("%s:\n got %s\nwant %s", name, canonicalJSON(t, got), canonicalJSON(t, want))
				}
			}
		})
	}
}

func TestInstallAgentHooksRejectsNilSettings(t *testing.T) {
	if _, err := InstallAgentHooks(nil, AgentClaude, "/usr/local/bin/claudio"); err == nil ||
		!strings.Contains(err.Error(), "settings cannot be nil") {
		t.Fatalf("err = %v, want settings cannot be nil", err)
	}
}
