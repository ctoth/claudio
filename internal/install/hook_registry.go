package install

import (
	"log/slog"

	"github.com/ctoth/claudio/internal/hooks"
)

// HookDefinition represents a single hook configuration in the registry
type HookDefinition struct {
	Name           string                 // Hook name (e.g., "PreToolUse")
	Category       hooks.EventCategory    // Event category from parser
	Description    string                 // Human-readable description
	DefaultEnabled bool                   // Whether enabled by default
}

// AllHooks defines the complete registry of all Claude Code hooks supported by Claudio
var AllHooks = []HookDefinition{
	{
		Name:           "PreToolUse",
		Category:       hooks.Loading,
		Description:    "Play loading/thinking sounds before tool execution",
		DefaultEnabled: true,
	},
	{
		Name:           "PostToolUse", 
		Category:       hooks.Success, // Note: Can also be Error category depending on outcome
		Description:    "Play success/error sounds after tool execution",
		DefaultEnabled: true,
	},
	{
		Name:           "UserPromptSubmit",
		Category:       hooks.Interactive,
		Description:    "Play interaction sounds when user submits prompts",
		DefaultEnabled: true,
	},
	{
		Name:           "Notification",
		Category:       hooks.Interactive,
		Description:    "Play sounds for permission requests and idle notifications",
		DefaultEnabled: true,
	},
	{
		Name:           "Stop",
		Category:       hooks.Completion,
		Description:    "Play sounds when main Claude agent finishes responding",
		DefaultEnabled: true,
	},
	{
		Name:           "SubagentStop",
		Category:       hooks.Completion,
		Description:    "Play sounds when Task tool subagent finishes responding",
		DefaultEnabled: true,
	},
	{
		Name:           "PreCompact",
		Category:       hooks.System,
		Description:    "Play sounds before Claude Code context compression",
		DefaultEnabled: true,
	},
	{
		Name:           "SessionStart",
		Category:       hooks.System,
		Description:    "Play sounds when Claude Code session starts or resumes",
		DefaultEnabled: true,
	},
}

// GetAllHooks returns all hooks defined in the registry
func GetAllHooks() []HookDefinition {
	slog.Debug("retrieving all hooks from registry", "total_hooks", len(AllHooks))
	return AllHooks
}

// GetEnabledHooks returns only hooks that are enabled by default
func GetEnabledHooks() []HookDefinition {
	var enabled []HookDefinition
	
	for _, hook := range AllHooks {
		if hook.DefaultEnabled {
			enabled = append(enabled, hook)
		}
	}
	
	slog.Debug("retrieved enabled hooks from registry", 
		"enabled_count", len(enabled), 
		"total_count", len(AllHooks))
	
	return enabled
}

// GetHookByName looks up a hook by name and returns it with a found flag
func GetHookByName(name string) (HookDefinition, bool) {
	slog.Debug("looking up hook by name", "hook_name", name)
	
	for _, hook := range AllHooks {
		if hook.Name == name {
			slog.Debug("found hook in registry", "hook_name", name, "category", hook.Category)
			return hook, true
		}
	}
	
	slog.Debug("hook not found in registry", "hook_name", name)
	return HookDefinition{}, false
}

// GetHookNames returns a slice of all hook names from the registry
func GetHookNames() []string {
	names := make([]string, len(AllHooks))
	
	for i, hook := range AllHooks {
		names[i] = hook.Name
	}
	
	slog.Debug("retrieved hook names from registry", 
		"hook_count", len(names),
		"names", names)
	
	return names
}