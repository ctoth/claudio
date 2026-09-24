package install

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	captainhook "github.com/ctoth/captain-hook"
)

// claudioIdentity recognizes claudio's hook commands for captain-hook.
var claudioIdentity = captainhook.IdentityFunc(IsClaudioCommandString)

// captainSettings views settings as captain-hook's settings type; both are
// map[string]interface{}, so the view shares the same map.
func captainSettings(settings *SettingsMap) *captainhook.SettingsMap {
	return (*captainhook.SettingsMap)(settings)
}

// InstallAgentHooks returns settings with the agent's claudio hooks
// installed, replacing any claudio entries already present. The input
// settings are not modified.
func InstallAgentHooks(settings *SettingsMap, agent Agent, executablePath string) (*SettingsMap, error) {
	specs, err := GenerateHookSpecs(executablePath, agent)
	if err != nil {
		return nil, err
	}
	if settings == nil {
		return nil, errors.New("settings cannot be nil")
	}
	copied, err := deepCopySettings(settings)
	if err != nil {
		return nil, fmt.Errorf("failed to copy settings: %w", err)
	}
	if err := captainhook.Install(captainSettings(copied), specs, claudioIdentity); err != nil {
		return nil, fmt.Errorf("failed to install %s hooks: %w", agent, err)
	}
	slog.Info("installed claudio hooks", "agent", agent, "hook_count", len(specs))
	return copied, nil
}

// RemoveAgentHooks returns a copy of settings with every claudio hook
// removed, and the sorted names of the events that held one. The input
// settings are not modified.
func RemoveAgentHooks(settings *SettingsMap, agent Agent) (*SettingsMap, []string, error) {
	if _, err := agent.concreteSpec(); err != nil {
		return nil, nil, err
	}
	copied, err := deepCopySettings(settings)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to copy settings: %w", err)
	}
	names := ClaudioHookNames(copied)
	captainhook.Uninstall(captainSettings(copied), claudioIdentity)
	slog.Debug("removed claudio hooks", "agent", agent, "hooks", names)
	return copied, names, nil
}

// ClaudioHookNames returns the sorted names of hook events that contain a
// claudio command.
func ClaudioHookNames(settings *SettingsMap) []string {
	return captainhook.OwnedEvents(captainSettings(settings), claudioIdentity)
}

// IsClaudioHook reports whether a hook event's value (legacy string or
// array of entries) contains any claudio command.
func IsClaudioHook(hookValue any) bool {
	settings := SettingsMap{"hooks": map[string]any{"event": hookValue}}
	return len(ClaudioHookNames(&settings)) > 0
}

// deepCopySettings returns an independent copy of settings via a JSON
// round-trip, so changes to the copy never reach the original.
func deepCopySettings(original *SettingsMap) (*SettingsMap, error) {
	data, err := json.Marshal(original)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal original settings: %w", err)
	}
	var copied SettingsMap
	if err := json.Unmarshal(data, &copied); err != nil {
		return nil, fmt.Errorf("failed to unmarshal settings copy: %w", err)
	}
	return &copied, nil
}
