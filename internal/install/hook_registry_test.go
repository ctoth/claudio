package install

import (
	"slices"
	"testing"

	"claudio.click/internal/hooks"
	captainhook "github.com/ctoth/captain-hook"
)

// catalogAgents maps claudio's settings-hook agents to captain-hook's catalog.
var catalogAgents = map[Agent]captainhook.Agent{
	AgentClaude:      captainhook.AgentClaude,
	AgentCodex:       captainhook.AgentCodex,
	AgentGemini:      captainhook.AgentGemini,
	AgentQwen:        captainhook.AgentQwen,
	AgentCopilot:     captainhook.AgentCopilot,
	AgentCommandCode: captainhook.AgentCommandCode,
}

func TestRegistryFollowsCatalog(t *testing.T) {
	for agent, catalogAgent := range catalogAgents {
		catalog, ok := captainhook.Lookup(catalogAgent)
		if !ok {
			t.Fatalf("catalog has no %s", catalogAgent)
		}
		var want []string
		for _, event := range catalog.Events {
			want = append(want, event.Key())
		}
		if got := agent.HookNames(); !slices.Equal(got, want) {
			t.Errorf("%s hook names = %v, want catalog keys %v", agent, got, want)
		}
	}
}

func TestDefaultEnabledStatus(t *testing.T) {
	disabled := map[string]bool{"FileChanged": true, "MessageDisplay": true}
	for _, agent := range ConcreteAgents() {
		for _, hook := range agent.Registry() {
			if hook.DefaultEnabled == disabled[hook.Name] {
				t.Errorf("%s %s: DefaultEnabled = %v", agent, hook.Name, hook.DefaultEnabled)
			}
		}
	}
}

// Every event claudio installs must map to a sound. An unmapped event falls
// through to the "unknown" context and plays a generic sound.
func TestEveryRegisteredEventHasASoundMapping(t *testing.T) {
	for _, agent := range ConcreteAgents() {
		for _, hook := range agent.Registry() {
			event := &hooks.HookEvent{EventName: hooks.NormalizeEventName(hook.Name)}
			if ctx := event.GetContext(); ctx.Operation == "unknown" {
				t.Errorf("%s %s has no sound mapping in internal/hooks/parser.go", agent, hook.Name)
			}
		}
	}
}

// Copilot events with no PascalCase key send camelCase payloads, so the
// hook command names the event itself.
func TestCopilotCamelCaseOnlyEventsPassTheirName(t *testing.T) {
	flagged := map[string]bool{}
	for _, hook := range AgentCopilot.Registry() {
		flagged[hook.Name] = hook.EventFlag
	}
	for _, name := range []string{"notification", "subagentStart", "userPromptTransformed"} {
		if !flagged[name] {
			t.Errorf("copilot %s should pass --hook-event", name)
		}
	}
	for _, name := range []string{"Stop", "PreToolUse", "SessionStart"} {
		if flagged[name] {
			t.Errorf("copilot %s has a PascalCase key and should not pass --hook-event", name)
		}
	}
}

func TestOpenCodeRegistryIsClaudiosPluginList(t *testing.T) {
	if got, want := AgentOpenCode.HookNames(), len(OpenCodeHooks); len(got) != want {
		t.Errorf("opencode hook names = %v, want the %d plugin events", got, want)
	}
}

func enabledHookNames(agent Agent) []string {
	definitions := agent.EnabledHooks()
	names := make([]string, len(definitions))
	for i, definition := range definitions {
		names[i] = definition.Name
	}
	return names
}
