package install

import (
	"reflect"
	"testing"
)

// userSettingsFor returns settings holding only user-owned hooks in the
// agent's shape.
func userSettingsFor(agent Agent) SettingsMap {
	if agent == AgentCopilot {
		return SettingsMap{"theme": "dark", "hooks": map[string]any{
			"PreToolUse": []any{map[string]any{"type": "command", "command": "echo user"}},
		}}
	}
	return SettingsMap{"theme": "dark", "hooks": map[string]any{
		"PreToolUse": []any{map[string]any{
			"matcher": "Bash",
			"hooks":   []any{map[string]any{"type": "command", "command": "echo user"}},
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
	settings := SettingsMap{"hooks": map[string]any{
		"Stop":       "claudio",
		"Other":      "echo hi",
		"PreToolUse": []any{map[string]any{"type": "command", "command": "/opt/claudio"}},
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

func TestIsClaudioHookFormats(t *testing.T) {
	if !IsClaudioHook("/usr/local/bin/claudio") {
		t.Error("expected string claudio command recognized")
	}
	if !IsClaudioHook(`"/usr/local/bin/claudio.exe"`) {
		t.Error("expected quoted windows claudio recognized")
	}
	if !IsClaudioHook(`/usr/local/bin/claudio --hook-agent gemini`) {
		t.Error("expected claudio command with arguments recognized")
	}
	if !IsClaudioHook(`"C:\Program Files\claudio.exe" --hook-agent gemini`) {
		t.Error("expected quoted claudio command with arguments recognized")
	}
	if IsClaudioHook("/usr/bin/other") {
		t.Error("non-claudio command must not be recognized")
	}
	arr := []any{
		map[string]any{
			"hooks": []any{
				map[string]any{"command": "/opt/claudio"},
			},
		},
	}
	if !IsClaudioHook(arr) {
		t.Error("expected array-format claudio recognized")
	}
}

func TestIsClaudioHookFindsClaudioInMergedHookArrays(t *testing.T) {
	cases := []struct {
		name string
		arr  []any
	}{
		{
			name: "claudio after existing hook",
			arr: []any{
				map[string]any{
					"matcher": ".*",
					"hooks": []any{
						map[string]any{"command": "/usr/bin/logger"},
					},
				},
				map[string]any{
					"matcher": "*",
					"hooks": []any{
						map[string]any{"command": "/usr/local/bin/claudio"},
					},
				},
			},
		},
		{
			name: "claudio before existing hook",
			arr: []any{
				map[string]any{
					"matcher": "*",
					"hooks": []any{
						map[string]any{"command": `C:\tools\claudio.exe`},
					},
				},
				map[string]any{
					"matcher": ".*",
					"hooks": []any{
						map[string]any{"command": "/usr/bin/logger"},
					},
				},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !IsClaudioHook(tc.arr) {
				t.Error("expected merged hook array to be recognized when any entry is claudio")
			}
		})
	}
}

func TestHookCommandQuotingAndRecognitionBranches(t *testing.T) {
	if got := mustSpec(t, AgentClaude).hookCommand("/usr/local/bin/claudio", "PreToolUse"); got != "/usr/local/bin/claudio" {
		t.Errorf("claude hook command = %q", got)
	}
	if got := mustSpec(t, AgentGemini).hookCommand(`/opt/Claudio Tools/claudio`, "PreToolUse"); got != `"/opt/Claudio Tools/claudio" --hook-agent gemini` {
		t.Errorf("gemini hook command = %q", got)
	}
	if got := mustSpec(t, AgentQwen).hookCommand(`/opt/cla"udio`, "PreToolUse"); got != `"/opt/cla\"udio" --hook-agent qwen` {
		t.Errorf("qwen hook command = %q", got)
	}
	if got := quoteCommandArg("plain"); got != "plain" {
		t.Errorf("plain arg quoted as %q", got)
	}

	if IsClaudioCommandString("") {
		t.Error("empty command must not be recognized as claudio")
	}
	if IsClaudioCommandString("/usr/bin/other --flag") {
		t.Error("other command must not be recognized as claudio")
	}
	if !IsClaudioCommandString(`"/usr/local/bin/claudio" --silent`) {
		t.Error("quoted executable with arguments should be recognized")
	}
	if !IsClaudioCommandString(`'/usr/local/bin/claudio' --silent`) {
		t.Error("single-quoted executable with arguments should be recognized")
	}
	if !IsClaudioCommandString(`C:\Program Files\claudio.exe`) {
		t.Error("legacy unquoted Windows path with spaces should be recognized")
	}
	if IsClaudioCommandString(`"/usr/local/bin/other --silent`) {
		t.Error("unclosed quoted non-claudio command must not be recognized")
	}

	if token, ok := leadingCommandToken("  "); ok || token != "" {
		t.Errorf("blank leading token = %q, %v; want empty false", token, ok)
	}
	if token, ok := leadingCommandToken(`"/usr/local/bin/claudio --silent`); !ok || token != `"/usr/local/bin/claudio --silent` {
		t.Errorf("unclosed quoted leading token = %q, %v", token, ok)
	}
}

func TestHookArrayDetectionAdditionalBranches(t *testing.T) {
	if IsClaudioHook([]any{}) {
		t.Error("empty hook array must not be recognized")
	}
	if IsClaudioHook([]any{"raw", map[string]any{"hooks": []any{}}}) {
		t.Error("array without claudio command must not be recognized")
	}

	directCommand := map[string]any{"command": "/opt/claudio"}
	if !IsClaudioHook([]any{directCommand}) {
		t.Error("direct command item should be recognized")
	}
	noHookArray := map[string]any{"matcher": "*"}
	if IsClaudioHook([]any{noHookArray}) {
		t.Error("item without hooks array must not be recognized")
	}
	nestedCommand := map[string]any{
		"hooks": []any{
			"raw-hook",
			map[string]any{"command": 42},
			map[string]any{"command": "/opt/claudio"},
		},
	}
	if !IsClaudioHook([]any{nestedCommand}) {
		t.Error("nested claudio command should be recognized")
	}
}
