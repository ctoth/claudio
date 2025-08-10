package uninstall

import (
	"fmt"
	"log/slog"

	"github.com/ctoth/claudio/internal/install"
)

// RunUninstallWorkflow orchestrates the complete Claudio uninstall process (public interface)
// Workflow: Validate scope → Read settings → Detect hooks → Remove hooks → Write → Verify
func RunUninstallWorkflow(scope string, settingsPath string) error {
	return runUninstallWorkflow(scope, settingsPath)
}

// runUninstallWorkflow orchestrates the complete Claudio uninstall process
// Workflow: Validate scope → Read settings → Detect hooks → Remove hooks → Write → Verify
func runUninstallWorkflow(scope string, settingsPath string) error {
	slog.Info("starting Claudio uninstall workflow",
		"scope", scope,
		"settings_path", settingsPath)

	// Step 1: Validate scope
	if scope != "user" && scope != "project" {
		return fmt.Errorf("invalid scope '%s': must be 'user' or 'project'", scope)
	}

	slog.Debug("validated uninstall scope", "scope", scope)

	// Step 2: Read existing settings (uses file locking for safety)
	slog.Debug("reading existing settings", "path", settingsPath)
	factory := install.GetFilesystemFactory()
	prodFS := factory.Production()
	existingSettings, err := install.ReadSettingsFile(prodFS, settingsPath)
	if err != nil {
		return fmt.Errorf("failed to read existing settings from %s: %w", settingsPath, err)
	}

	slog.Info("loaded existing settings",
		"path", settingsPath,
		"settings_keys", getSettingsKeys(existingSettings))

	// Step 3: Detect Claudio hooks in settings
	slog.Debug("detecting claudio hooks in settings")
	claudioHooks := detectClaudioHooks(existingSettings)

	if len(claudioHooks) == 0 {
		slog.Info("no claudio hooks found, uninstall is idempotent",
			"settings_path", settingsPath)
		return nil // Idempotent operation - no error if nothing to remove
	}

	slog.Info("detected claudio hooks for removal", "hooks", claudioHooks)

	// Step 4: Remove simple claudio hooks (string format)
	slog.Debug("removing simple claudio hooks")
	removeSimpleClaudioHooks(existingSettings, claudioHooks)

	// Step 5: Remove complex claudio hooks (array format)
	slog.Debug("removing complex claudio hooks")
	removeComplexClaudioHooks(existingSettings, claudioHooks)

	// Step 6: Write updated settings back to file (uses file locking for safety)
	slog.Debug("writing updated settings to file", "path", settingsPath)
	err = install.WriteSettingsFile(prodFS, settingsPath, existingSettings)
	if err != nil {
		return fmt.Errorf("failed to write updated settings to %s: %w", settingsPath, err)
	}

	slog.Info("wrote updated settings to file", "path", settingsPath)

	// Step 7: Verify uninstall by reading back and checking for claudio hooks
	slog.Debug("verifying uninstall by reading back settings")
	verifySettings, err := install.ReadSettingsFile(prodFS, settingsPath)
	if err != nil {
		return fmt.Errorf("failed to verify uninstall by reading %s: %w", settingsPath, err)
	}

	// Check that no claudio hooks remain
	remainingClaudioHooks := detectClaudioHooks(verifySettings)
	if len(remainingClaudioHooks) > 0 {
		return fmt.Errorf("verification failed: claudio hooks still present after uninstall: %v", remainingClaudioHooks)
	}

	slog.Info("uninstall verification successful",
		"no_claudio_hooks_remaining", true,
		"settings_path", settingsPath)

	slog.Info("Claudio uninstall workflow completed successfully",
		"scope", scope,
		"settings_path", settingsPath,
		"removed_hooks", claudioHooks)

	return nil
}

// getSettingsKeys returns a list of top-level keys in settings for logging
func getSettingsKeys(settings *install.SettingsMap) []string {
	if settings == nil {
		return []string{}
	}

	keys := make([]string, 0, len(*settings))
	for key := range *settings {
		keys = append(keys, key)
	}
	return keys
}
