package install

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"claudio.click/internal/fs"
)

// HooksMap represents an agent settings hooks section.
type HooksMap map[string]interface{}

// executableRecognizer decides whether a basename refers to the claudio
// executable. Production matches only claudio and claudio.exe. End-to-end
// install tests that thread the go test binary path through
// GetExecutablePath need the recognizer to accept names like
// "cli.test.exe"; those tests opt in by setting
// CLAUDIO_TEST_RECOGNIZE_GO_TEST=1 via t.Setenv. The env var is read at
// call time so the production binary never imports the testing package
// for a test-only seam.
var executableRecognizer = func(name string) bool {
	if name == "claudio" || name == "claudio.exe" {
		return true
	}
	if os.Getenv("CLAUDIO_TEST_RECOGNIZE_GO_TEST") == "1" {
		if strings.HasSuffix(name, ".test") || strings.HasSuffix(name, ".test.exe") {
			return true
		}
	}
	return false
}

// GenerateClaudioHooks creates the Claude Code hook configuration (backward-compatible default).
func GenerateClaudioHooks(executablePath string) (interface{}, error) {
	return GenerateClaudioHooksForAgent(executablePath, AgentClaude)
}

// GenerateClaudioHooksForAgent creates hook configuration for the given agent
// using its registry and config shape.
func GenerateClaudioHooksForAgent(executablePath string, agent Agent) (interface{}, error) {
	slog.Debug("generating Claudio hooks configuration",
		"agent", agent, "executable_path", executablePath)

	enabledHooks := agent.EnabledHooks()
	matcher := agent.Matcher()
	slog.Debug("retrieved enabled hooks for agent", "agent", agent, "count", len(enabledHooks))

	hooks := make(HooksMap)

	// Helper function to create hook config structure
	createHookConfig := func() interface{} {
		commandConfig := map[string]interface{}{
			"type":    "command",
			"command": hookCommandForAgent(executablePath, agent),
		}
		addAgentHookMetadata(commandConfig, agent)

		if agent == AgentCopilot {
			return []interface{}{commandConfig}
		}

		return []interface{}{
			map[string]interface{}{
				"matcher": matcher,
				"hooks": []interface{}{
					commandConfig,
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

func hookCommandForAgent(executablePath string, agent Agent) string {
	switch agent {
	case AgentGemini, AgentQwen, AgentCopilot:
		return quoteCommandArg(executablePath) + " --hook-agent " + string(agent)
	default:
		return executablePath
	}
}

func addAgentHookMetadata(commandConfig map[string]interface{}, agent Agent) {
	switch agent {
	case AgentGemini:
		commandConfig["name"] = "claudio"
	case AgentQwen:
		commandConfig["name"] = "claudio"
	case AgentCopilot:
		commandConfig["timeoutSec"] = 30
	}
}

func quoteCommandArg(arg string) string {
	if !strings.ContainsAny(arg, " \t\r\n\"") {
		return arg
	}
	return `"` + strings.ReplaceAll(arg, `"`, `\"`) + `"`
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

	// Then, add/update Claudio hooks with strip-and-replace merging.
	// mergeHookValues now handles both cases uniformly: it strips any
	// pre-existing Claudio entries from the existing array and appends the
	// new Claudio entries. This preserves the user's non-Claudio entries
	// regardless of ordering and is idempotent across repeated merges.
	for hookName, claudioValue := range claudioHooksMap {
		if existingValue, exists := mergedHooks[hookName]; exists {
			mergedHooks[hookName] = mergeHookValues(existingValue, claudioValue)
			slog.Debug("merged existing hook with Claudio (strip-and-replace)",
				"hook_name", hookName)
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
// Returns the merged result in array format, preserving existing non-Claudio
// commands and replacing any pre-existing Claudio entries with the new ones.
// The merge is idempotent regardless of element ordering: any Claudio entry in
// the existing array is filtered out before the new Claudio entries are
// appended, so merge(merge(existing)) == merge(existing).
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
		existingArray = existingArr
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

	// Strip any pre-existing Claudio entries from the existing array, then
	// append the new Claudio entries. Filtering operates at HOOK granularity
	// inside each item's "hooks" sub-array (mirroring removeClaudioFromArray
	// in internal/uninstall/hook_removal.go): for each existing item, build
	// a new item whose hooks sub-array contains only the non-Claudio
	// entries. Drop the item only when removal emptied its hooks sub-array.
	// Items with no Claudio commands pass through verbatim. This preserves
	// user non-Claudio hooks that share a matcher's hooks sub-array with a
	// Claudio command (Chunk 5 analyst Finding 1).
	filteredExisting := make([]interface{}, 0, len(existingArray))
	strippedCount := 0
	for _, item := range existingArray {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			// Preserve non-map entries verbatim
			filteredExisting = append(filteredExisting, item)
			continue
		}
		if cmdStr, ok := itemMap["command"].(string); ok {
			if isClaudioCommandString(cmdStr) {
				strippedCount++
				continue
			}
			filteredExisting = append(filteredExisting, item)
			continue
		}
		hooks, ok := itemMap["hooks"].([]interface{})
		if !ok {
			// Preserve items without a hooks sub-array verbatim
			filteredExisting = append(filteredExisting, item)
			continue
		}
		keptHooks := make([]interface{}, 0, len(hooks))
		itemStripped := 0
		for _, h := range hooks {
			hookMap, ok := h.(map[string]interface{})
			if !ok {
				keptHooks = append(keptHooks, h)
				continue
			}
			cmdStr, ok := hookMap["command"].(string)
			if !ok {
				keptHooks = append(keptHooks, h)
				continue
			}
			if isClaudioCommandString(cmdStr) {
				itemStripped++
				continue
			}
			keptHooks = append(keptHooks, h)
		}
		if itemStripped == 0 {
			// No Claudio commands in this item; preserve verbatim.
			filteredExisting = append(filteredExisting, item)
			continue
		}
		strippedCount += itemStripped
		if len(keptHooks) == 0 {
			// Item was Claudio-only; drop it. The new Claudio entries will
			// be appended below.
			continue
		}
		// Item had Claudio + non-Claudio siblings; preserve the non-Claudio
		// siblings in a copied item so we never mutate the input map.
		newItem := make(map[string]interface{}, len(itemMap))
		for k, v := range itemMap {
			newItem[k] = v
		}
		newItem["hooks"] = keptHooks
		filteredExisting = append(filteredExisting, newItem)
	}

	mergedArray := make([]interface{}, 0, len(filteredExisting)+len(claudioArray))
	mergedArray = append(mergedArray, filteredExisting...)
	mergedArray = append(mergedArray, claudioArray...)

	slog.Debug("completed hook value merge",
		"existing_elements", len(existingArray),
		"existing_claudio_entries_stripped", strippedCount,
		"claudio_elements", len(claudioArray),
		"merged_elements", len(mergedArray))

	return mergedArray
}

// itemContainsClaudioCommand returns true if the given hook-array element
// (a map with a "hooks" sub-array) contains any hook whose command resolves
// to the claudio executable per executableRecognizer.
func itemContainsClaudioCommand(item map[string]interface{}) bool {
	if cmdStr, ok := item["command"].(string); ok && isClaudioCommandString(cmdStr) {
		return true
	}
	hooks, ok := item["hooks"].([]interface{})
	if !ok {
		return false
	}
	for _, h := range hooks {
		cmd, ok := h.(map[string]interface{})
		if !ok {
			continue
		}
		cmdStr, ok := cmd["command"].(string)
		if !ok {
			continue
		}
		if isClaudioCommandString(cmdStr) {
			return true
		}
	}
	return false
}

// IsClaudioCommandString reports whether a command string refers to the
// claudio executable. Shared between IsClaudioHook, the merge filter,
// and the uninstall package's hook detection so the three predicates
// cannot drift apart. (Chunk 3 analyst F1: previously install and
// uninstall maintained two recognizers with divergent code shapes;
// they happened to agree on production inputs only by accident.)
func IsClaudioCommandString(cmdStr string) bool {
	cmdStr = strings.TrimSpace(cmdStr)
	if cmdStr == "" {
		return false
	}

	// Preserve support for legacy unquoted Windows paths with spaces, such as
	// C:\Program Files\claudio.exe, by trying the full string before treating
	// whitespace as argument separation.
	if executableRecognizer(commandBasename(stripSurroundingQuotes(cmdStr))) {
		return true
	}

	executable, _ := leadingCommandToken(cmdStr)
	return executableRecognizer(commandBasename(executable))
}

func stripSurroundingQuotes(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

func leadingCommandToken(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", false
	}

	if s[0] == '"' || s[0] == '\'' {
		quote := s[0]
		for i := 1; i < len(s); i++ {
			if s[i] == quote {
				return s[1:i], true
			}
		}
		return stripSurroundingQuotes(s), true
	}

	if i := strings.IndexAny(s, " \t\r\n"); i >= 0 {
		return s[:i], true
	}
	return s, true
}

// commandBasename returns the final path segment of a command string,
// splitting on both '/' and '\' regardless of the host OS. filepath.Base
// only honors the running platform's separator, so on Linux/macOS it left
// a Windows-style hook command like `C:\Program Files\claudio.exe` intact
// and the recognizer never saw the bare `claudio.exe`. settings.json is
// portable data — a hook authored on Windows can be inspected on Linux and
// vice versa — so recognition must not depend on the reader's OS.
func commandBasename(cmdStr string) string {
	if i := strings.LastIndexAny(cmdStr, `/\`); i >= 0 {
		return cmdStr[i+1:]
	}
	return cmdStr
}

// isClaudioCommandString is the previous (unexported) spelling, kept as
// a thin alias so adjacent install-package call sites stay readable.
func isClaudioCommandString(cmdStr string) bool {
	return IsClaudioCommandString(cmdStr)
}

// IsClaudioHook reports whether a hook value contains any reference to the
// claudio executable. Supports the old string format and the new array
// format. The array form is scanned exhaustively — return true if ANY array
// element contains ANY hooks-sub-array entry whose command refers to claudio.
// This any-element semantics matches the merge-side filter so a mixed array
// like [customHook, claudioHook] is correctly identified as containing
// Claudio regardless of element ordering.
func IsClaudioHook(hookValue interface{}) bool {
	// Check old string format (backward compatibility)
	if str, ok := hookValue.(string); ok {
		return isClaudioCommandString(str)
	}

	// Check new array format — scan every element, not just arr[0].
	if arr, ok := hookValue.([]interface{}); ok && len(arr) > 0 {
		for _, item := range arr {
			itemMap, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			if itemContainsClaudioCommand(itemMap) {
				return true
			}
		}
		return false
	}

	return false
}

// GetExecutablePath returns the current executable path using filesystem abstraction.
func GetExecutablePath() (string, error) {
	return fs.ExecutablePath()
}
