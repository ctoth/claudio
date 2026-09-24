package install

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"strings"

	captainhook "github.com/ctoth/captain-hook"
)

// HooksMap represents an agent settings hooks section.
type HooksMap map[string]any

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

// GenerateHookSpecs returns the agent's claudio hooks in captain-hook's
// representation, one spec per enabled hook in registry order.
func GenerateHookSpecs(executablePath string, agent Agent) ([]captainhook.HookSpec, error) {
	spec, err := agent.concreteSpec()
	if err != nil {
		return nil, err
	}
	hooks := agent.EnabledHooks()
	specs := make([]captainhook.HookSpec, 0, len(hooks))
	for _, hook := range hooks {
		specs = append(specs, spec.hookSpec(executablePath, hook.Name))
	}
	slog.Debug("generated claudio hook specs", "agent", agent, "hook_count", len(specs))
	return specs, nil
}

// hookSpec returns the captain-hook spec for one of the agent's events.
func (s agentSpec) hookSpec(executablePath, event string) captainhook.HookSpec {
	hs := captainhook.HookSpec{
		Event:   event,
		Matcher: s.matcher,
		Command: s.hookCommand(executablePath, event),
		Flat:    s.shape == shapeFlatCommands,
	}
	if s.powerShellCommand {
		hs.Command, hs.CommandWindows = powerShellHookCommands(executablePath)
	}
	extra := make(map[string]any)
	if s.commandName != "" {
		extra["name"] = s.commandName
	}
	if s.timeoutSec > 0 {
		extra["timeoutSec"] = s.timeoutSec
	}
	if len(extra) > 0 {
		hs.Extra = extra
	}
	return hs
}

// powerShellHookCommands returns the portable command and the Windows
// PowerShell override for an agent that runs hooks through PowerShell on
// Windows. Both use forward slashes. PowerShell double quotes expand dollar
// signs and backticks in paths, so the override single-quotes the path.
func powerShellHookCommands(executablePath string) (command, commandWindows string) {
	executablePath = strings.ReplaceAll(executablePath, `\`, "/")
	return quoteCommandArg(executablePath),
		"& '" + powerShellSingleQuoteEscaper.Replace(executablePath) + "'"
}

// GenerateClaudioHooksForAgent creates hook configuration for the given agent
// using its registry and config shape.
func GenerateClaudioHooksForAgent(executablePath string, agent Agent) (any, error) {
	spec, err := agent.concreteSpec()
	if err != nil {
		return nil, err
	}
	enabledHooks := agent.EnabledHooks()

	hooks := make(HooksMap)

	// Helper function to create hook config structure
	createHookConfig := func(hookDef HookDefinition) any {
		commandConfig := map[string]any{
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
			return []any{commandConfig}
		}

		return []any{
			map[string]any{
				"matcher": spec.matcher,
				"hooks": []any{
					commandConfig,
				},
			},
		}
	}

	// Generate hooks for all enabled hooks in the agent's registry
	for _, hookDef := range enabledHooks {
		hooks[hookDef.Name] = createHookConfig(hookDef)
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
func MergeHooksIntoSettings(existingSettings *SettingsMap, claudioHooks any) (*SettingsMap, error) {
	// Validate inputs
	if existingSettings == nil {
		return nil, errors.New("settings cannot be nil")
	}

	if claudioHooks == nil {
		return nil, errors.New("hooks cannot be nil")
	}

	// Validate Claudio hooks type
	claudioHooksMap, ok := claudioHooks.(HooksMap)
	if !ok {
		// Try to convert from map[string]interface{}
		if genericMap, isGeneric := claudioHooks.(map[string]any); isGeneric {
			claudioHooksMap = HooksMap(genericMap)
		} else {
			return nil, fmt.Errorf("invalid hooks type: expected map[string]interface{}, got %T", claudioHooks)
		}
	}

	// Create deep copy of existing settings using JSON round-trip
	settingsCopy, err := deepCopySettings(existingSettings)
	if err != nil {
		return nil, fmt.Errorf("failed to create deep copy of settings: %w", err)
	}

	// Get or create hooks section in the copy
	var existingHooks HooksMap
	if hooksInterface, exists := (*settingsCopy)["hooks"]; exists {
		// Validate existing hooks type
		if hooksMap, ok := hooksInterface.(map[string]any); ok {
			existingHooks = HooksMap(hooksMap)
		} else {
			return nil, fmt.Errorf("existing hooks invalid: expected map[string]interface{}, got %T", hooksInterface)
		}
	} else {
		// Create new hooks section
		existingHooks = make(HooksMap)
	}

	// Merge Claudio hooks into existing hooks
	// This preserves existing hooks while adding/updating Claudio hooks
	mergedHooks := make(HooksMap)

	// First, copy all existing hooks
	maps.Copy(mergedHooks, existingHooks)

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
		}
	}

	// Update the hooks section in the settings copy
	(*settingsCopy)["hooks"] = map[string]any(mergedHooks)

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
func mergeHookValues(existingValue, claudioValue any) (any, error) {
	claudioArray, ok := claudioValue.([]any)
	if !ok {
		return nil, fmt.Errorf("claudio hook value must be an array, got %T", claudioValue)
	}

	var existingArray []any
	switch v := existingValue.(type) {
	case string:
		existingArray = []any{
			map[string]any{
				"matcher": ".*",
				"hooks": []any{
					map[string]any{
						"type":    "command",
						"command": v,
					},
				},
			},
		}
	case []any:
		existingArray = v
	default:
		return nil, fmt.Errorf("unsupported existing hook value: expected a string or an array, got %T", existingValue)
	}

	kept, _ := stripClaudioEntries(existingArray)
	merged := make([]any, 0, len(kept)+len(claudioArray))
	merged = append(merged, kept...)
	merged = append(merged, claudioArray...)

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

// GetExecutablePath returns the running executable's path with forward
// slashes, so the path works in bash (Claude Code passes hook commands
// through /usr/bin/bash on all platforms).
func GetExecutablePath() (string, error) {
	p, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(p), nil
}
