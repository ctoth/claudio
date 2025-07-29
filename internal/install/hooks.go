package install

import (
	"log/slog"
)

// HooksMap represents the hooks section of Claude Code settings
type HooksMap map[string]interface{}

// GenerateClaudiaHooks creates the hook configuration for Claudio installation
// Returns a hooks map that can be integrated into Claude Code settings.json
func GenerateClaudiaHooks() (interface{}, error) {
	slog.Debug("generating Claudio hooks configuration")
	
	hooks := make(HooksMap)
	
	// PreToolUse: Play loading/thinking sounds before tool execution
	hooks["PreToolUse"] = "claudio"
	slog.Debug("added PreToolUse hook", "command", "claudio")
	
	// PostToolUse: Play success/error sounds after tool execution  
	hooks["PostToolUse"] = "claudio"
	slog.Debug("added PostToolUse hook", "command", "claudio")
	
	// UserPromptSubmit: Play interaction sounds when user submits prompts
	hooks["UserPromptSubmit"] = "claudio"
	slog.Debug("added UserPromptSubmit hook", "command", "claudio")
	
	slog.Info("generated Claudio hooks configuration", 
		"hook_count", len(hooks),
		"hooks", getHookNamesList(hooks))
	
	return hooks, nil
}

// getHookNamesList returns a list of hook names for logging
func getHookNamesList(hooks HooksMap) []string {
	names := make([]string, 0, len(hooks))
	for name := range hooks {
		names = append(names, name)
	}
	return names
}