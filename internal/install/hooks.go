package install

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"runtime"
	"strings"

	"claudio.click/internal/fs"
	"github.com/spf13/afero"
)

// HooksMap represents the hooks section of Claude Code settings
type HooksMap map[string]interface{}

// GenerateClaudioHooks creates the Claude Code hook configuration (backward-compatible default).
func GenerateClaudioHooks(filesystem afero.Fs, executablePath string) (interface{}, error) {
	return GenerateClaudioHooksForAgent(filesystem, executablePath, AgentClaude)
}

// GenerateClaudioHooksForAgent creates hook configuration for the given agent using its
// registry and matcher. Returns a hooks map suitable for Claude settings.json or Codex hooks.json.
// Accepts filesystem and executable path parameters to prevent config corruption during testing.
func GenerateClaudioHooksForAgent(filesystem afero.Fs, executablePath string, agent Agent) (interface{}, error) {
	slog.Debug("generating Claudio hooks configuration",
		"agent", agent, "executable_path", executablePath)

	enabledHooks := agent.EnabledHooks()
	matcher := agent.Matcher()
	slog.Debug("retrieved enabled hooks for agent", "agent", agent, "count", len(enabledHooks))

	hooks := make(HooksMap)

	// Helper function to create hook config structure
	createHookConfig := func() interface{} {
		return []interface{}{
			map[string]interface{}{
				"matcher": matcher,
				"hooks": []interface{}{
					map[string]interface{}{
						"type":    "command",
						"command": executablePath,
					},
				},
			},
		}
	}

	// Generate hooks for all enabled hooks in the agent's registry
	for _, hookDef := range enabledHooks {
		hooks[hookDef.Name] = createHookConfig()
		slog.Debug("added hook from registry",
			"agent", agent,
			"hook_name", hookDef.Name,
			"category", hookDef.Category,
			"description", hookDef.Description)
	}

	slog.Info("generated Claudio hooks configuration",
		"agent", agent,
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
func MergeHooksIntoSettings(existingSettings *SettingsMap, claudioHooks interface{}) (*SettingsMap, error) {
	slog.Debug("starting hook merge operation")

	// Validate inputs
	if existingSettings == nil {
		return nil, fmt.Errorf("settings cannot be nil")
	}

	if claudioHooks == nil {
		return nil, fmt.Errorf("hooks cannot be nil")
	}

	// Validate Claudio hooks type
	claudioHooksMap, ok := claudioHooks.(HooksMap)
	if !ok {
		// Try to convert from map[string]interface{}
		if genericMap, isGeneric := claudioHooks.(map[string]interface{}); isGeneric {
			claudioHooksMap = HooksMap(genericMap)
		} else {
			return nil, fmt.Errorf("invalid hooks type: expected map[string]interface{}, got %T", claudioHooks)
		}
	}

	slog.Debug("validated inputs", "claudio_hooks_count", len(claudioHooksMap))

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

	// Then, add/update Claudio hooks with proper merging
	for hookName, claudioValue := range claudioHooksMap {
		if existingValue, exists := mergedHooks[hookName]; exists {
			mergedHooks[hookName] = mergeHookValues(existingValue, claudioValue)
			if IsClaudioHook(existingValue) {
				slog.Debug("refreshed existing Claudio hook", "hook_name", hookName)
			} else {
				slog.Info("merged existing non-Claudio hook with Claudio",
					"hook_name", hookName, "action", "merging")
			}
		} else {
			// No conflict - add new Claudio hook
			mergedHooks[hookName] = claudioValue
			slog.Debug("adding new Claudio hook", "hook_name", hookName)
		}
	}

	// Update the hooks section in the settings copy
	(*settingsCopy)["hooks"] = map[string]interface{}(mergedHooks)

	slog.Info("completed hook merge",
		"total_hooks", len(mergedHooks),
		"claudio_hooks_merged", len(claudioHooksMap))

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

// mergeHookValues merges an existing hook value with a Claudio hook value
// Returns the merged result in array format, preserving existing commands and adding Claudio commands
// Handles deduplication when the existing hook is already a Claudio hook
func mergeHookValues(existingValue, claudioValue interface{}) interface{} {
	slog.Debug("merging hook values", "existing_type", fmt.Sprintf("%T", existingValue), "claudio_type", fmt.Sprintf("%T", claudioValue))

	// Convert Claudio value to array format (it should already be, but be safe)
	claudioArray, ok := claudioValue.([]interface{})
	if !ok {
		slog.Warn("claudio value is not array format, returning as-is", "type", fmt.Sprintf("%T", claudioValue))
		return claudioValue
	}

	// Convert existing value to array format
	var existingArray []interface{}
	if existingStr, ok := existingValue.(string); ok {
		if isClaudioCommand(existingStr) {
			slog.Debug("replacing existing string Claudio hook")
			return claudioValue
		}
		// Convert string hook to array format
		existingArray = []interface{}{
			map[string]interface{}{
				"matcher": ".*",
				"hooks": []interface{}{
					map[string]interface{}{
						"type":    "command",
						"command": existingStr,
					},
				},
			},
		}
		slog.Debug("converted existing string hook to array format", "command", existingStr)
	} else if existingArr, ok := existingValue.([]interface{}); ok {
		// Already in array format
		existingArray = removeClaudioCommands(existingArr)
		slog.Debug("existing hook already in array format")
	} else {
		slog.Warn("unknown existing hook format, treating as string", "type", fmt.Sprintf("%T", existingValue))
		// Fallback: treat as string
		existingArray = []interface{}{
			map[string]interface{}{
				"matcher": ".*",
				"hooks": []interface{}{
					map[string]interface{}{
						"type":    "command",
						"command": fmt.Sprintf("%v", existingValue),
					},
				},
			},
		}
	}

	// Merge arrays: existing commands first, then Claudio commands
	var mergedArray []interface{}

	// Add all existing array elements
	mergedArray = append(mergedArray, existingArray...)

	// Add Claudio array elements
	mergedArray = append(mergedArray, claudioArray...)

	slog.Debug("completed hook value merge",
		"existing_elements", len(existingArray),
		"claudio_elements", len(claudioArray),
		"merged_elements", len(mergedArray))

	return mergedArray
}

func removeClaudioCommands(entries []interface{}) []interface{} {
	filtered := make([]interface{}, 0, len(entries))
	for _, entry := range entries {
		config, ok := entry.(map[string]interface{})
		if !ok {
			filtered = append(filtered, entry)
			continue
		}
		hooksList, ok := config["hooks"].([]interface{})
		if !ok {
			filtered = append(filtered, entry)
			continue
		}

		keptHooks := make([]interface{}, 0, len(hooksList))
		removed := false
		for _, hook := range hooksList {
			cmd, ok := hook.(map[string]interface{})
			if !ok {
				keptHooks = append(keptHooks, hook)
				continue
			}
			cmdStr, ok := cmd["command"].(string)
			if ok && isClaudioCommand(cmdStr) {
				removed = true
				continue
			}
			keptHooks = append(keptHooks, hook)
		}
		if len(keptHooks) == 0 {
			continue
		}
		if !removed {
			filtered = append(filtered, entry)
			continue
		}

		configCopy := make(map[string]interface{}, len(config))
		for key, value := range config {
			configCopy[key] = value
		}
		configCopy["hooks"] = keptHooks
		filtered = append(filtered, configCopy)
	}
	return filtered
}

// IsClaudioHook checks if a hook value represents a claudio hook,
// supporting both the old string format and new array format
func IsClaudioHook(hookValue interface{}) bool {
	// Check old string format (backward compatibility)
	if str, ok := hookValue.(string); ok {
		return isClaudioCommand(str)
	}

	// Check new array format
	if arr, ok := hookValue.([]interface{}); ok && len(arr) > 0 {
		for _, item := range arr {
			config, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			if hooks, ok := config["hooks"].([]interface{}); ok && len(hooks) > 0 {
				for _, hook := range hooks {
					cmd, ok := hook.(map[string]interface{})
					if !ok {
						continue
					}
					if cmdStr, ok := cmd["command"].(string); ok {
						if isClaudioCommand(cmdStr) {
							return true
						}
					}
				}
			}
		}
	}

	return false
}

func isClaudioCommand(cmdStr string) bool {
	// Strip quotes if present (for Windows compatibility)
	if len(cmdStr) >= 2 && cmdStr[0] == '"' && cmdStr[len(cmdStr)-1] == '"' {
		cmdStr = cmdStr[1 : len(cmdStr)-1]
	}
	cmdStr = strings.ReplaceAll(cmdStr, "\\", "/")
	baseName := filepath.Base(cmdStr)
	// Handle production "claudio" and "claudio.exe" (Windows) and test executables "install.test", "uninstall.test", "cli.test"
	return baseName == "claudio" || baseName == "claudio.exe" || baseName == "install.test" || baseName == "uninstall.test" || baseName == "cli.test"
}

// GetExecutablePath returns the current executable path using filesystem abstraction.
// On Windows the result is converted to forward slashes so that the path works
// when Claude Code invokes the hook command through bash.
func GetExecutablePath() (string, error) {
	p, err := fs.ExecutablePath()
	if err != nil {
		return "", err
	}
	if runtime.GOOS == "windows" {
		p = filepath.ToSlash(p)
	}
	return p, nil
}

// GetFilesystemFactory returns the default filesystem factory
func GetFilesystemFactory() fs.Factory {
	return fs.NewDefaultFactory()
}
