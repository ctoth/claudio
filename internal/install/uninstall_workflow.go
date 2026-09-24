package install

import (
	"fmt"
	"log/slog"

	"github.com/spf13/afero"
)

// RunUninstallWorkflow removes every claudio hook from the target's
// resolved config file (target.ConfigPath) under the settings lock. A
// missing file, or one without claudio hooks, is left untouched.
func RunUninstallWorkflow(filesystem afero.Fs, target AgentTarget) error {
	settingsPath := target.ConfigPath
	slog.Info("starting Claudio uninstall workflow", "agent", target.Agent, "settings_path", settingsPath)

	exists, err := afero.Exists(filesystem, settingsPath)
	if err != nil {
		return fmt.Errorf("failed to check settings path %s: %w", settingsPath, err)
	}
	if !exists {
		slog.Info("settings file not found, uninstall is idempotent", "settings_path", settingsPath)
		return nil
	}

	var removed []string
	err = ModifySettings(filesystem, settingsPath, func(settings *SettingsMap) (*SettingsMap, error) {
		updated, names, err := RemoveAgentHooks(settings, target.Agent)
		removed = names
		return updated, err
	})
	if err != nil {
		return fmt.Errorf("uninstall: %w", err)
	}

	slog.Info("Claudio uninstall workflow completed", "settings_path", settingsPath, "removed_hooks", removed)
	return nil
}
