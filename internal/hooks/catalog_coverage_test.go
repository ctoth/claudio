package hooks

import (
	"testing"

	captainhook "github.com/ctoth/captain-hook"

	"claudio.click/internal/install"
)

// TestParserKnowsEveryInstalledEvent fails when install can write a hook the
// parser does not recognize. Such an event would fall through to
// unknownEvent and play the generic interactive sound. Each event name is
// parsed the way the CLI parses it: as the --hook-event default for a
// payload that does not name the event.
func TestParserKnowsEveryInstalledEvent(t *testing.T) {
	type source struct {
		agent  string
		events []string
	}
	var sources []source
	for _, agent := range captainhook.Agents() {
		keys := make([]string, len(agent.Events))
		for i, event := range agent.Events {
			keys[i] = event.Key()
		}
		sources = append(sources, source{string(agent.Agent), keys})
	}
	opencode := make([]string, len(install.OpenCodeHooks))
	for i, hook := range install.OpenCodeHooks {
		opencode[i] = hook.Name
	}
	sources = append(sources, source{"opencode", opencode})

	for _, src := range sources {
		for _, name := range src.events {
			t.Run(src.agent+"/"+name, func(t *testing.T) {
				event, err := ParseHookEventWithDefault([]byte(`{"session_id":"s","cwd":"/c"}`), name)
				if err != nil {
					t.Fatalf("parse: %v", err)
				}
				if got := event.GetContext(); got.Operation == unknownEvent.operation {
					t.Errorf("parser does not recognize %s event %q (normalized to %q)", src.agent, name, event.EventName)
				}
			})
		}
	}
}
