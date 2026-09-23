package install

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"claudio.click/internal/fs"
	captainhook "github.com/ctoth/captain-hook"
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

// powerShellSingleQuoteEscaper doubles every character PowerShell treats as a
// single quote: the ASCII apostrophe and U+2018 through U+201B.
var powerShellSingleQuoteEscaper = strings.NewReplacer(
	"'", "''",
	"\u2018", "\u2018\u2018",
	"\u2019", "\u2019\u2019",
	"\u201a", "\u201a\u201a",
	"\u201b", "\u201b\u201b",
)

// GenerateCodexHookSpecs returns Claudio's desired Codex hooks in the shared
// Captain Hook representation.
func GenerateCodexHookSpecs(executablePath string) []captainhook.HookSpec {
	executablePath = strings.ReplaceAll(executablePath, `\`, "/")
	command := quoteCommandArg(executablePath)
	// PowerShell double quotes expand dollar signs and backticks in paths.
	commandWindows := "& '" + powerShellSingleQuoteEscaper.Replace(executablePath) + "'"

	hooks := AgentCodex.EnabledHooks()
	specs := make([]captainhook.HookSpec, 0, len(hooks))
	for _, hook := range hooks {
		specs = append(specs, captainhook.HookSpec{
			Event:          hook.Name,
			Matcher:        AgentCodex.Matcher(),
			Command:        command,
			CommandWindows: commandWindows,
		})
	}
	return specs
}

// GenerateClaudioHooksForAgent creates hook configuration for the given agent
// using its registry and config shape.
func GenerateClaudioHooksForAgent(executablePath string, agent Agent) (interface{}, error) {
	slog.Debug("generating Claudio hooks configuration",
		"agent", agent, "executable_path", executablePath)

	spec, err := agent.concreteSpec()
	if err != nil {
		return nil, err
	}
	enabledHooks := agent.EnabledHooks()
	slog.Debug("retrieved enabled hooks for agent", "agent", agent, "count", len(enabledHooks))

	hooks := make(HooksMap)

	// Helper function to create hook config structure
	createHookConfig := func(hookDef HookDefinition) interface{} {
		commandConfig := map[string]interface{}{
			"type":    "command",
			"command": spec.hookCommand(executablePath, hookDef.Name),
		}
		if spec.commandName != "" {
			commandConfig["name"] = spec.commandName
		}
		if spec.timeoutSec > 0 {
			commandConfig["timeoutSec"] = spec.timeoutSec
		}

		if spec.shape == shapeFlatCommands {
			return []interface{}{commandConfig}
		}

		return []interface{}{
			map[string]interface{}{
				"matcher": spec.matcher,
				"hooks": []interface{}{
					commandConfig,
				},
			},
		}
	}

	// Generate hooks for all enabled hooks in the agent's registry
	for _, hookDef := range enabledHooks {
		hooks[hookDef.Name] = createHookConfig(hookDef)
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

// hookCommand returns the command string the agent runs for hookName.
func (s agentSpec) hookCommand(executablePath, hookName string) string {
	if !s.hookAgentFlag {
		return executablePath
	}
	command := quoteCommandArg(executablePath) + " --hook-agent " + string(s.agent)
	if s.eventFlagHooks[hookName] {
		command += " --hook-event " + hookName
	}
	return command
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
			merged, err := mergeHookValues(existingValue, claudioValue)
			if err != nil {
				return nil, fmt.Errorf("hook %s: %w", hookName, err)
			}
			mergedHooks[hookName] = merged
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

// mergeHookValues merges an existing hook value with a Claudio hook value.
// It strips any pre-existing claudio entries (stripClaudioEntries, the same
// engine uninstall uses) and appends the new claudio entries, so the merge
// is idempotent regardless of element ordering and never touches the
// user's own entries. A legacy string value is converted to one matcher
// group first.
//
// An existing value that is neither a string nor an array is an error: it
// is not a hook shape claudio understands, so it must not be rewritten.
func mergeHookValues(existingValue, claudioValue interface{}) (interface{}, error) {
	claudioArray, ok := claudioValue.([]interface{})
	if !ok {
		return nil, fmt.Errorf("claudio hook value must be an array, got %T", claudioValue)
	}

	var existingArray []interface{}
	switch v := existingValue.(type) {
	case string:
		existingArray = []interface{}{
			map[string]interface{}{
				"matcher": ".*",
				"hooks": []interface{}{
					map[string]interface{}{
						"type":    "command",
						"command": v,
					},
				},
			},
		}
	case []interface{}:
		existingArray = v
	default:
		return nil, fmt.Errorf("unsupported existing hook value: expected a string or an array, got %T", existingValue)
	}

	kept, stripped := stripClaudioEntries(existingArray)
	merged := make([]interface{}, 0, len(kept)+len(claudioArray))
	merged = append(merged, kept...)
	merged = append(merged, claudioArray...)

	slog.Debug("completed hook value merge",
		"existing_elements", len(existingArray),
		"existing_claudio_entries_stripped", stripped,
		"claudio_elements", len(claudioArray),
		"merged_elements", len(merged))

	return merged, nil
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

// GetExecutablePath returns the current executable path using filesystem abstraction.
func GetExecutablePath() (string, error) {
	return fs.ExecutablePath()
}
