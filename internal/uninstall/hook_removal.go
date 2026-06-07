package uninstall

import (
	"log/slog"

	"claudio.click/internal/install"
)

// isSimpleClaudioArrayHook checks if an array hook contains only a single claudio command
func isSimpleClaudioArrayHook(arr []interface{}) bool {
	if len(arr) != 1 {
		return false
	}

	if config, ok := arr[0].(map[string]interface{}); ok {
		if cmdStr, ok := config["command"].(string); ok && isClaudioCommand(cmdStr) {
			return true
		}
		if hooks, ok := config["hooks"].([]interface{}); ok && len(hooks) == 1 {
			if cmd, ok := hooks[0].(map[string]interface{}); ok {
				if cmdStr, ok := cmd["command"].(string); ok && isClaudioCommand(cmdStr) {
					return true
				}
			}
		}
	}

	return false
}

// removeSimpleClaudioHooks removes simple string claudio hooks from settings
func removeSimpleClaudioHooks(settings *install.SettingsMap, hookNames []string) {
	slog.Debug("removing simple claudio hooks", "hook_names", hookNames)

	if settings == nil {
		slog.Debug("settings is nil, nothing to remove")
		return
	}

	hooksInterface, exists := (*settings)["hooks"]
	if !exists {
		slog.Debug("no hooks section found in settings")
		return
	}

	hooksMap, ok := hooksInterface.(map[string]interface{})
	if !ok {
		slog.Warn("hooks section is not a map", "type", typeof(hooksInterface))
		return
	}

	removedCount := 0
	for _, hookName := range hookNames {
		slog.Debug("checking hook for removal", "name", hookName)

		if hookValue, exists := hooksMap[hookName]; exists {
			// Handle claudio hooks (both string and simple array format)
			if stringValue, ok := hookValue.(string); ok && isClaudioCommand(stringValue) {
				slog.Debug("removing simple claudio hook", "name", hookName, "command", stringValue)
				delete(hooksMap, hookName)
				removedCount++
			} else if arr, ok := hookValue.([]interface{}); ok {
				// Handle new array format claudio hooks (only if they're single-command claudio hooks)
				if isSimpleClaudioArrayHook(arr) {
					slog.Debug("removing new format claudio hook", "name", hookName)
					delete(hooksMap, hookName)
					removedCount++
				} else {
					slog.Debug("array hook exists but is not simple claudio hook",
						"name", hookName, "value", hookValue)
				}
			} else {
				slog.Debug("hook exists but is not simple claudio hook",
					"name", hookName, "value", hookValue)
			}
		} else {
			slog.Debug("hook does not exist", "name", hookName)
		}
	}

	// If hooks map is now empty, remove the entire hooks section
	if len(hooksMap) == 0 {
		slog.Debug("hooks map is empty, removing hooks section")
		delete(*settings, "hooks")
	}

	slog.Info("simple claudio hook removal completed",
		"removed_count", removedCount,
		"remaining_hooks", len(hooksMap))
}

// removeComplexClaudioHooks removes claudio commands from complex array hooks
func removeComplexClaudioHooks(settings *install.SettingsMap, hookNames []string) {
	slog.Debug("removing complex claudio hooks", "hook_names", hookNames)

	if settings == nil {
		slog.Debug("settings is nil, nothing to remove")
		return
	}

	hooksInterface, exists := (*settings)["hooks"]
	if !exists {
		slog.Debug("no hooks section found in settings")
		return
	}

	hooksMap, ok := hooksInterface.(map[string]interface{})
	if !ok {
		slog.Warn("hooks section is not a map", "type", typeof(hooksInterface))
		return
	}

	removedHookCount := 0
	totalCommandsRemoved := 0

	for _, hookName := range hookNames {
		slog.Debug("checking complex hook for removal", "name", hookName)

		hookValue, exists := hooksMap[hookName]
		if !exists {
			slog.Debug("hook does not exist", "name", hookName)
			continue
		}

		// Only process array hooks
		arrayValue, ok := hookValue.([]interface{})
		if !ok {
			slog.Debug("hook is not an array, skipping", "name", hookName, "type", typeof(hookValue))
			continue
		}

		// Process the array and remove claudio commands
		filteredArray, commandsRemoved := removeClaudioFromArray(arrayValue)
		totalCommandsRemoved += commandsRemoved

		if len(filteredArray) == 0 {
			// If the array is now empty, remove the entire hook
			slog.Debug("removing entire hook as array became empty", "name", hookName)
			delete(hooksMap, hookName)
			removedHookCount++
		} else if commandsRemoved > 0 {
			// Replace with filtered array
			slog.Debug("updating hook with filtered array", "name", hookName,
				"original_elements", len(arrayValue), "filtered_elements", len(filteredArray))
			hooksMap[hookName] = filteredArray
		} else {
			slog.Debug("no claudio commands found in hook array", "name", hookName)
		}
	}

	// If hooks map is now empty, remove the entire hooks section
	if len(hooksMap) == 0 {
		slog.Debug("hooks map is empty, removing hooks section")
		delete(*settings, "hooks")
	}

	slog.Info("complex claudio hook removal completed",
		"removed_hooks", removedHookCount,
		"commands_removed", totalCommandsRemoved,
		"remaining_hooks", len(hooksMap))
}

// removeClaudioFromArray processes an array and removes claudio commands.
//
// An item is dropped from the result ONLY when this function actually removed
// a Claudio command from it AND the remaining hooks list is empty. A
// pre-existing item with an empty hooks array (e.g. a user-staged matcher
// block) is preserved verbatim because we did not remove anything from it.
func removeClaudioFromArray(array []interface{}) ([]interface{}, int) {
	var filteredArray []interface{}
	commandsRemoved := 0

	for _, item := range array {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			// Keep non-map items as-is
			filteredArray = append(filteredArray, item)
			continue
		}
		if command, exists := itemMap["command"]; exists {
			if commandStr, ok := command.(string); ok && isClaudioCommand(commandStr) {
				slog.Debug("removing direct claudio command from hooks array", "command", commandStr)
				commandsRemoved++
				continue
			}
		}

		// Check if this item has a "hooks" array
		hooksInterface, exists := itemMap["hooks"]
		if !exists {
			// Keep items without hooks array as-is
			filteredArray = append(filteredArray, item)
			continue
		}

		hooksArray, ok := hooksInterface.([]interface{})
		if !ok {
			// Keep items with non-array hooks as-is
			filteredArray = append(filteredArray, item)
			continue
		}

		// Filter out claudio commands from the hooks array.
		// Track per-item removals so the drop decision below can distinguish
		// "we emptied this item by removing Claudio" from "this item started
		// empty and we changed nothing".
		itemCommandsRemoved := 0
		var filteredHooks []interface{}
		for _, hookItem := range hooksArray {
			hookMap, ok := hookItem.(map[string]interface{})
			if !ok {
				// Keep non-map hook items as-is
				filteredHooks = append(filteredHooks, hookItem)
				continue
			}

			command, exists := hookMap["command"]
			if !exists {
				// Keep hooks without command field as-is
				filteredHooks = append(filteredHooks, hookItem)
				continue
			}

			commandStr, ok := command.(string)
			if !ok || !isClaudioCommand(commandStr) {
				// Keep non-string commands or non-claudio commands
				filteredHooks = append(filteredHooks, hookItem)
				continue
			}

			// This is a claudio command - remove it
			slog.Debug("removing claudio command from hooks array", "command", commandStr)
			commandsRemoved++
			itemCommandsRemoved++
		}

		if itemCommandsRemoved > 0 && len(filteredHooks) == 0 {
			// We removed a Claudio command and that was the only thing here.
			slog.Debug("removing entire array element as hooks became empty after Claudio removal")
			continue
		}

		newItem := make(map[string]interface{})
		for k, v := range itemMap {
			newItem[k] = v
		}
		// Preserve user's pre-existing empty array exactly. filteredHooks
		// may be nil here if hooksArray started empty; normalise to an
		// empty slice so the JSON round-trip emits "[]" not "null".
		if filteredHooks == nil {
			filteredHooks = []interface{}{}
		}
		newItem["hooks"] = filteredHooks
		filteredArray = append(filteredArray, newItem)
	}

	return filteredArray, commandsRemoved
}
