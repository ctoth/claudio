package install

import (
	"encoding/json"
	"fmt"
	"log/slog"
)

// HooksMap represents the hooks section of Claude Code settings
type HooksMap map[string]interface{}

// GenerateClaudiaHooks creates the hook configuration for Claudio installation
// Returns a hooks map that can be integrated into Claude Code settings.json
func GenerateClaudiaHooks() (interface{}, error) {
	slog.Debug("generating Claudio hooks configuration")
	
	hooks := make(HooksMap)
	
	// Helper function to create hook config structure
	createHookConfig := func() interface{} {
		return []interface{}{
			map[string]interface{}{
				"hooks": []interface{}{
					map[string]interface{}{
						"type":    "command",
						"command": "claudio",
					},
				},
			},
		}
	}
	
	// PreToolUse: Play loading/thinking sounds before tool execution
	hooks["PreToolUse"] = createHookConfig()
	slog.Debug("added PreToolUse hook", "command", "claudio")
	
	// PostToolUse: Play success/error sounds after tool execution  
	hooks["PostToolUse"] = createHookConfig()
	slog.Debug("added PostToolUse hook", "command", "claudio")
	
	// UserPromptSubmit: Play interaction sounds when user submits prompts
	hooks["UserPromptSubmit"] = createHookConfig()
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

// MergeHooksIntoSettings merges Claudio hooks into existing Claude Code settings
// Creates a deep copy of existing settings and safely merges hooks without modifying originals
// Preserves existing non-Claudio hooks and all other settings
func MergeHooksIntoSettings(existingSettings *SettingsMap, claudiaHooks interface{}) (*SettingsMap, error) {
	slog.Debug("starting hook merge operation")
	
	// Validate inputs
	if existingSettings == nil {
		return nil, fmt.Errorf("settings cannot be nil")
	}
	
	if claudiaHooks == nil {
		return nil, fmt.Errorf("hooks cannot be nil")
	}
	
	// Validate Claudio hooks type
	claudiaHooksMap, ok := claudiaHooks.(HooksMap)
	if !ok {
		// Try to convert from map[string]interface{}
		if genericMap, isGeneric := claudiaHooks.(map[string]interface{}); isGeneric {
			claudiaHooksMap = HooksMap(genericMap)
		} else {
			return nil, fmt.Errorf("invalid hooks type: expected map[string]interface{}, got %T", claudiaHooks)
		}
	}
	
	slog.Debug("validated inputs", "claudio_hooks_count", len(claudiaHooksMap))
	
	// Create deep copy of existing settings using JSON round-trip
	settingsCopy, err := deepCopySettings(existingSettings)
	if err != nil {
		return nil, fmt.Errorf("failed to create deep copy of settings: %w", err)
	}
	
	// Get or create hooks section in the copy
	var existingHooks HooksMap
	if hooksInterface, exists := (*settingsCopy)["hooks"]; exists {
		// Validate existing hooks type
		if hooksMap, ok := hooksInterface.(map[string]interface{}); ok {
			existingHooks = HooksMap(hooksMap)
			slog.Debug("found existing hooks", "existing_hooks_count", len(existingHooks))
		} else {
			return nil, fmt.Errorf("existing hooks invalid: expected map[string]interface{}, got %T", hooksInterface)
		}
	} else {
		// Create new hooks section
		existingHooks = make(HooksMap)
		slog.Debug("created new hooks section")
	}
	
	// Merge Claudio hooks into existing hooks
	// This preserves existing hooks while adding/updating Claudio hooks
	mergedHooks := make(HooksMap)
	
	// First, copy all existing hooks
	for hookName, hookValue := range existingHooks {
		mergedHooks[hookName] = hookValue
		slog.Debug("preserved existing hook", "hook_name", hookName, "hook_value", hookValue)
	}
	
	// Then, add/update Claudio hooks
	for hookName, hookValue := range claudiaHooksMap {
		if _, exists := mergedHooks[hookName]; exists {
			// Hook already exists - for arrays/objects we can't directly compare
			// so we'll just log and update
			slog.Info("updating existing hook", 
				"hook_name", hookName, 
				"action", "replacing")
		} else {
			slog.Debug("adding new Claudio hook", 
				"hook_name", hookName)
		}
		
		// Update/add the Claudio hook
		mergedHooks[hookName] = hookValue
	}
	
	// Update the hooks section in the settings copy
	(*settingsCopy)["hooks"] = map[string]interface{}(mergedHooks)
	
	slog.Info("completed hook merge", 
		"total_hooks", len(mergedHooks),
		"claudio_hooks_merged", len(claudiaHooksMap))
	
	return settingsCopy, nil
}

// deepCopySettings creates a deep copy of settings using JSON round-trip
// This ensures that modifications to the copy don't affect the original
func deepCopySettings(original *SettingsMap) (*SettingsMap, error) {
	// Marshal to JSON
	jsonData, err := json.Marshal(original)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal original settings: %w", err)
	}
	
	// Unmarshal to new copy
	var copy SettingsMap
	err = json.Unmarshal(jsonData, &copy)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal settings copy: %w", err)
	}
	
	return &copy, nil
}

// IsClaudioHook checks if a hook value represents a claudio hook,
// supporting both the old string format and new array format
func IsClaudioHook(hookValue interface{}) bool {
	// Check old string format
	if str, ok := hookValue.(string); ok {
		return str == "claudio"
	}
	
	// Check new array format
	if arr, ok := hookValue.([]interface{}); ok && len(arr) > 0 {
		if config, ok := arr[0].(map[string]interface{}); ok {
			if hooks, ok := config["hooks"].([]interface{}); ok && len(hooks) > 0 {
				if cmd, ok := hooks[0].(map[string]interface{}); ok {
					if cmdStr, ok := cmd["command"].(string); ok {
						return cmdStr == "claudio"
					}
				}
			}
		}
	}
	
	return false
}