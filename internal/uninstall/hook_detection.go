package uninstall

import (
	"log/slog"
	"path/filepath"

	"github.com/ctoth/claudio/internal/install"
)

// DetectClaudioHooks finds all hook names that reference claudio (public interface)
func DetectClaudioHooks(settings *install.SettingsMap) []string {
	return detectClaudioHooks(settings)
}

// detectClaudioHooks finds all hook names that reference claudio
func detectClaudioHooks(settings *install.SettingsMap) []string {
	slog.Debug("detecting claudio hooks in settings")

	if settings == nil {
		slog.Debug("settings is nil, returning empty list")
		return []string{}
	}

	hooksInterface, exists := (*settings)["hooks"]
	if !exists {
		slog.Debug("no hooks section found in settings")
		return []string{}
	}

	hooksMap, ok := hooksInterface.(map[string]interface{})
	if !ok {
		slog.Warn("hooks section is not a map", "type", typeof(hooksInterface))
		return []string{}
	}

	var claudioHooks []string

	for hookName, hookValue := range hooksMap {
		slog.Debug("checking hook", "name", hookName, "value", hookValue)

		// Check for simple string hook: "PreToolUse": "claudio" or "/path/to/claudio"
		if stringValue, ok := hookValue.(string); ok {
			if isClaudioCommand(stringValue) {
				slog.Debug("found simple claudio hook", "name", hookName, "command", stringValue)
				claudioHooks = append(claudioHooks, hookName)
			}
			continue
		}

		// Check for complex array hook
		if arrayValue, ok := hookValue.([]interface{}); ok {
			if containsClaudioCommand(arrayValue) {
				slog.Debug("found complex claudio hook", "name", hookName)
				claudioHooks = append(claudioHooks, hookName)
			}
			continue
		}

		slog.Debug("hook is neither string nor array", "name", hookName, "type", typeof(hookValue))
	}

	slog.Info("claudio hook detection completed", "found_hooks", claudioHooks)
	return claudioHooks
}

// isClaudioCommand checks if a command string represents a claudio executable
func isClaudioCommand(cmdStr string) bool {
	// Remove quotes if present (handles quoted paths in JSON)
	unquoted := cmdStr
	if len(cmdStr) >= 2 && cmdStr[0] == '"' && cmdStr[len(cmdStr)-1] == '"' {
		unquoted = cmdStr[1 : len(cmdStr)-1]
	}
	
	baseName := filepath.Base(unquoted)
	// Handle production "claudio" and test executables "install.test", "uninstall.test"
	return baseName == "claudio" || baseName == "install.test" || baseName == "uninstall.test"
}

// containsClaudioCommand checks if an array contains a claudio command
func containsClaudioCommand(array []interface{}) bool {
	for _, item := range array {
		if itemMap, ok := item.(map[string]interface{}); ok {
			// Check if this item has a "hooks" array
			if hooksInterface, exists := itemMap["hooks"]; exists {
				if hooksArray, ok := hooksInterface.([]interface{}); ok {
					for _, hookItem := range hooksArray {
						if hookMap, ok := hookItem.(map[string]interface{}); ok {
							if command, exists := hookMap["command"]; exists {
								if commandStr, ok := command.(string); ok && isClaudioCommand(commandStr) {
									return true
								}
							}
						}
					}
				}
			}
		}
	}
	return false
}

// typeof returns a string representation of the type for debugging
func typeof(v interface{}) string {
	if v == nil {
		return "nil"
	}
	switch v.(type) {
	case string:
		return "string"
	case []interface{}:
		return "[]interface{}"
	case map[string]interface{}:
		return "map[string]interface{}"
	default:
		return "unknown"
	}
}
