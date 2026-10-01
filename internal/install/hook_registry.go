package install

import (
	"log/slog"

	captainhook "github.com/ctoth/captain-hook"
)

// HookDefinition is one hook event claudio can install for an agent.
type HookDefinition struct {
	Name           string // settings key (e.g. "PreToolUse")
	DefaultEnabled bool   // whether install writes it by default
	// EventFlag appends "--hook-event <Name>" to the hook command, for
	// events whose payload may not name the event.
	EventFlag bool
}

// defaultDisabledHooks fire too often to play a sound each time. They stay
// in the registry but install skips them.
var defaultDisabledHooks = map[string]bool{
	"FileChanged":    true,
	"MessageDisplay": true,
}

// catalogRegistry returns an agent's hook definitions from captain-hook's
// event catalog, in catalog order. With flagCamelCase, events that have no
// PascalCase key pass their name on the command line (Copilot sends those
// events camelCase payloads).
func catalogRegistry(agent captainhook.Agent, flagCamelCase bool) []HookDefinition {
	catalog, ok := captainhook.Lookup(agent)
	if !ok {
		slog.Error("agent missing from captain-hook catalog", "agent", agent)
		return nil
	}
	definitions := make([]HookDefinition, len(catalog.Events))
	for i, event := range catalog.Events {
		key := event.Key()
		definitions[i] = HookDefinition{
			Name:           key,
			DefaultEnabled: !defaultDisabledHooks[key],
			EventFlag:      flagCamelCase && event.PascalName == "",
		}
	}
	return definitions
}

// OpenCodeHooks are the Claude Code event names the OpenCode plugin sends.
// OpenCode has no settings hooks, so it is not in captain-hook's catalog;
// the plugin maps its own events to these.
var OpenCodeHooks = []HookDefinition{
	{Name: "SessionStart", DefaultEnabled: true},
	{Name: "SessionDelete", DefaultEnabled: true},
	{Name: "UserPromptSubmit", DefaultEnabled: true},
	{Name: "UserPromptExpansion", DefaultEnabled: true},
	{Name: "PreToolUse", DefaultEnabled: true},
	{Name: "PostToolUse", DefaultEnabled: true},
	{Name: "PermissionRequest", DefaultEnabled: true},
	{Name: "PermissionDenied", DefaultEnabled: true},
	{Name: "SubagentStart", DefaultEnabled: true},
	{Name: "SubagentStop", DefaultEnabled: true},
	{Name: "TodoCreated", DefaultEnabled: true},
	{Name: "TodoCompleted", DefaultEnabled: true},
	{Name: "PreCompact", DefaultEnabled: true},
	{Name: "PostCompact", DefaultEnabled: true},
	{Name: "Stop", DefaultEnabled: true},
	{Name: "StopFailure", DefaultEnabled: true},
}
