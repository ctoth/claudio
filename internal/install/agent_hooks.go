package install

import (
	"fmt"

	captainhook "github.com/ctoth/captain-hook"
)

// InstallAgentHooks returns settings with the agent's claudio hooks
// installed, replacing any claudio entries already present. The input
// settings are not modified.
func InstallAgentHooks(settings *SettingsMap, agent Agent, executablePath string) (*SettingsMap, error) {
	spec, err := agent.concreteSpec()
	if err != nil {
		return nil, err
	}
	if spec.captainHook {
		copied, err := deepCopySettings(settings)
		if err != nil {
			return nil, fmt.Errorf("failed to copy settings: %w", err)
		}
		captainSettings := captainhook.SettingsMap(*copied)
		if err := captainhook.Install(
			&captainSettings,
			GenerateCodexHookSpecs(executablePath),
			captainhook.IdentityFunc(IsClaudioCommandString),
		); err != nil {
			return nil, fmt.Errorf("failed to install %s hooks: %w", agent, err)
		}
		converted := SettingsMap(captainSettings)
		return &converted, nil
	}

	claudioHooks, err := GenerateClaudioHooksForAgent(executablePath, agent)
	if err != nil {
		return nil, fmt.Errorf("failed to generate Claudio hooks: %w", err)
	}
	merged, err := MergeHooksIntoSettings(settings, claudioHooks)
	if err != nil {
		return nil, fmt.Errorf("failed to merge Claudio hooks into settings: %w", err)
	}
	return merged, nil
}

// UsesCaptainHook reports whether the agent's hooks are managed through
// the captain-hook module rather than claudio's own merge engine.
func (a Agent) UsesCaptainHook() bool {
	s, _ := a.spec()
	return s.captainHook
}
