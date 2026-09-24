package install

import (
	"reflect"
	"testing"

	captainhook "github.com/ctoth/captain-hook"
)

func TestGenerateHookSpecsPerAgent(t *testing.T) {
	const exe = `C:\Program Files\Claudio\claudio.exe`
	cases := []struct {
		agent Agent
		// want returns the expected spec for one enabled hook.
		want func(event string) captainhook.HookSpec
	}{
		{AgentClaude, func(event string) captainhook.HookSpec {
			return captainhook.HookSpec{Event: event, Matcher: ".*", Command: exe}
		}},
		{AgentCodex, func(event string) captainhook.HookSpec {
			return captainhook.HookSpec{
				Event:          event,
				Matcher:        "*",
				Command:        `"C:/Program Files/Claudio/claudio.exe"`,
				CommandWindows: `& 'C:/Program Files/Claudio/claudio.exe'`,
			}
		}},
		{AgentGemini, func(event string) captainhook.HookSpec {
			return captainhook.HookSpec{
				Event:   event,
				Command: `"` + exe + `" --hook-agent gemini`,
				Extra:   map[string]any{"name": "claudio"},
			}
		}},
		{AgentQwen, func(event string) captainhook.HookSpec {
			return captainhook.HookSpec{
				Event:   event,
				Matcher: ".*",
				Command: `"` + exe + `" --hook-agent qwen`,
				Extra:   map[string]any{"name": "claudio"},
			}
		}},
		{AgentCopilot, func(event string) captainhook.HookSpec {
			command := `"` + exe + `" --hook-agent copilot`
			if event == "subagentStart" {
				command += " --hook-event subagentStart"
			}
			return captainhook.HookSpec{
				Event:   event,
				Command: command,
				Flat:    true,
				Extra:   map[string]any{"timeoutSec": 30},
			}
		}},
	}
	for _, tc := range cases {
		t.Run(string(tc.agent), func(t *testing.T) {
			specs, err := GenerateHookSpecs(exe, tc.agent)
			if err != nil {
				t.Fatal(err)
			}
			hooks := tc.agent.EnabledHooks()
			if len(specs) != len(hooks) {
				t.Fatalf("specs = %d, want %d", len(specs), len(hooks))
			}
			for i, hook := range hooks {
				if want := tc.want(hook.Name); !reflect.DeepEqual(specs[i], want) {
					t.Errorf("spec %d:\n got %#v\nwant %#v", i, specs[i], want)
				}
			}
		})
	}
}

func TestGenerateHookSpecsCodexEscapesPowerShellQuotes(t *testing.T) {
	specs, err := GenerateHookSpecs("/opt/O'Brien\u2019s/claudio", AgentCodex)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := specs[0].CommandWindows, "& '/opt/O''Brien\u2019\u2019s/claudio'"; got != want {
		t.Errorf("commandWindows = %q, want %q", got, want)
	}
	if got, want := specs[0].Command, "/opt/O'Brien\u2019s/claudio"; got != want {
		t.Errorf("command = %q, want %q", got, want)
	}
}

func TestGenerateHookSpecsRejectsNonConcreteAgent(t *testing.T) {
	for _, agent := range []Agent{AgentAuto, AgentAll, Agent("bogus")} {
		if _, err := GenerateHookSpecs("/usr/local/bin/claudio", agent); err == nil {
			t.Errorf("GenerateHookSpecs(%q) should error", agent)
		}
	}
}
