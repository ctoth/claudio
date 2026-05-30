package uninstall

import (
	"fmt"
	"log/slog"

	"claudio.click/internal/install"
	"github.com/spf13/afero"
)

// agentResolver returns the settings path for the given agent and scope.
// Production calls agent.BestConfigPath. Tests override via swapAgentResolver
// (defined in *_test.go) so a t.TempDir() path can be injected without
// changing the workflow's signature.
var agentResolver = func(agent install.Agent, scope string) (string, error) {
	return agent.BestConfigPath(scope)
}

// RunUninstallWorkflow orchestrates the complete Claudio uninstall process.
// The settings-file path is resolved internally from the agent and the
// validated scope, so the scope check becomes load-bearing: a caller cannot
// pass scope=user with a path that does not belong to that scope.
// Workflow: Validate scope → Resolve path → Read settings → Detect hooks → Remove hooks → Write → Verify
func RunUninstallWorkflow(filesystem afero.Fs, scope string, agent install.Agent) error {
	slog.Info("starting Claudio uninstall workflow",
		"scope", scope,
		"agent", agent)

	// Step 1: Validate scope
	if scope != "user" && scope != "project" {
		return fmt.Errorf("invalid scope '%s': must be 'user' or 'project'", scope)
	}

	slog.Debug("validated uninstall scope", "scope", scope)

	// Resolve the settings path from the validated scope using the agent.
	// Going through agentResolver lets tests inject a TempDir path without
	// changing this signature.
	settingsPath, err := agentResolver(agent, scope)
	if err != nil {
		return fmt.Errorf("failed to resolve settings path for agent %s scope %s: %w", agent, scope, err)
	}
	slog.Debug("resolved settings path from agent", "agent", agent, "scope", scope, "path", settingsPath)

	// Acquire advisory lock around the full read-mutate-write window so
	// concurrent install/uninstall processes serialise. See
	// install.LockSettingsDir for semantics.
	lock, err := install.LockSettingsDir(settingsPath)
	if err != nil {
		return fmt.Errorf("uninstall: %w", err)
	}
	defer func() {
		if unlockErr := lock.Unlock(); unlockErr != nil {
			slog.Warn("failed to release settings lock", "err", unlockErr)
		}
	}()

	// Step 2: Read existing settings
	slog.Debug("reading existing settings", "path", settingsPath)
	existingSettings, err := install.ReadSettingsFile(filesystem, settingsPath)
	if err != nil {
		return fmt.Errorf("failed to read existing settings from %s: %w", settingsPath, err)
	}

	slog.Info("loaded existing settings",
		"path", settingsPath,
		"settings_keys", install.SettingsKeys(existingSettings))

	// Step 3: Detect Claudio hooks in settings
	slog.Debug("detecting claudio hooks in settings")
	claudioHooks := DetectClaudioHooks(existingSettings)

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

	// Step 6: Write updated settings back to file
	slog.Debug("writing updated settings to file", "path", settingsPath)
	err = install.WriteSettingsFile(filesystem, settingsPath, existingSettings)
	if err != nil {
		return fmt.Errorf("failed to write updated settings to %s: %w", settingsPath, err)
	}

	slog.Info("wrote updated settings to file", "path", settingsPath)

	// Step 7: Verify uninstall by reading back and checking for claudio hooks
	slog.Debug("verifying uninstall by reading back settings")
	verifySettings, err := install.ReadSettingsFile(filesystem, settingsPath)
	if err != nil {
		return fmt.Errorf("failed to verify uninstall by reading %s: %w", settingsPath, err)
	}

	// Check that no claudio hooks remain
	remainingClaudioHooks := DetectClaudioHooks(verifySettings)
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
