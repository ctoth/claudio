package cli

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"claudio.click/internal/install"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

// InstallScope represents the scope of installation
type InstallScope string

const (
	ScopeGlobal  InstallScope = InstallScope(install.ScopeGlobal)
	ScopeProject InstallScope = InstallScope(install.ScopeProject)
)

// String returns the string representation of InstallScope
func (s InstallScope) String() string {
	return string(s)
}

// IsValid returns true if the scope is valid
func (s InstallScope) IsValid() bool {
	_, err := install.NormalizeScope(s.String())
	return err == nil
}

// newInstallCommand creates the install subcommand with flags
func newInstallCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install claudio hooks into agent settings",
		Long:  "Install claudio hooks into supported coding-agent settings to enable audio feedback for tool usage and events.",
		RunE:  runInstallCommandE,
	}

	// Add --scope flag with validation
	cmd.Flags().StringP("scope", "s", install.ScopeGlobal, "Installation scope: 'global' for user-wide settings, 'project' for project-specific settings")

	// Add --agent flag with validation
	cmd.Flags().StringP("agent", "a", string(install.AgentAuto), "Target agent: 'auto', 'claude', 'codex', 'gemini', 'qwen', or 'all'")

	// Add --dry-run flag
	cmd.Flags().BoolP("dry-run", "d", false, "Show what would be done without making changes (simulation mode)")

	// Add --quiet flag
	cmd.Flags().BoolP("quiet", "q", false, "Suppress output (no progress messages)")

	// Add --print flag
	cmd.Flags().BoolP("print", "p", false, "Print configuration that would be written")

	return cmd
}

// runInstallCommandE handles the install subcommand execution
func runInstallCommandE(cmd *cobra.Command, args []string) error {
	slog.Debug("install command started", "args", args)

	// Get and validate scope flag
	scopeStr, err := cmd.Flags().GetString("scope")
	if err != nil {
		return fmt.Errorf("failed to get scope flag: %w", err)
	}

	normalizedScope, err := install.NormalizeScope(scopeStr)
	if err != nil {
		return err
	}
	scope := InstallScope(normalizedScope)

	// Get and validate agent flag
	agentStr, err := cmd.Flags().GetString("agent")
	if err != nil {
		return fmt.Errorf("failed to get agent flag: %w", err)
	}
	agent, err := install.ParseAgent(agentStr)
	if err != nil {
		return err
	}

	// Get dry-run flag
	dryRun, err := cmd.Flags().GetBool("dry-run")
	if err != nil {
		return fmt.Errorf("failed to get dry-run flag: %w", err)
	}

	// Get quiet flag
	quiet, err := cmd.Flags().GetBool("quiet")
	if err != nil {
		return fmt.Errorf("failed to get quiet flag: %w", err)
	}

	// Get print flag
	print, err := cmd.Flags().GetBool("print")
	if err != nil {
		return fmt.Errorf("failed to get print flag: %w", err)
	}

	slog.Info("install command executing", "scope", scope, "agent", agent, "dry_run", dryRun, "quiet", quiet, "print", print)

	targets, err := install.ResolveAgentTargets(agent, scope.String())
	if err != nil {
		return err
	}

	slog.Debug("resolved install targets", "scope", scope, "agent", agent, "count", len(targets))

	// Handle print flag - shows configuration details
	if print {
		return handlePrintInstall(cmd, scope, targets, dryRun, quiet)
	}

	// Handle dry-run mode - show what would be done without making changes
	if dryRun {
		return handleDryRunInstall(cmd, scope, targets, quiet)
	}

	return runInstallTargets(cmd, scope, targets, quiet)
}

func handlePrintInstall(cmd *cobra.Command, scope InstallScope, targets []install.AgentTarget, dryRun bool, quiet bool) error {
	var configDetails string
	if dryRun {
		configDetails = "PRINT: DRY-RUN configuration for scope: " + scope.String()
	} else {
		configDetails = "PRINT: Install configuration for scope: " + scope.String()
	}

	cmd.Printf("%s\n", configDetails)
	if dryRun {
		cmd.Printf("  Mode: Simulation (no changes will be made)\n")
	}
	if quiet {
		cmd.Printf("  Output: Quiet mode (minimal messages)\n")
	}
	cmd.Printf("  Scope: %s\n", scope.String())
	for _, target := range targets {
		cmd.Printf("  Target agent: %s\n", target.Agent)
		cmd.Printf("  Settings Path: %s\n", target.ConfigPath)
	}
	return nil
}

func handleDryRunInstall(cmd *cobra.Command, scope InstallScope, targets []install.AgentTarget, quiet bool) error {
	if !quiet {
		cmd.Printf("DRY-RUN: Claudio installation simulation for %s scope\n", scope.String())
		for _, target := range targets {
			cmd.Printf("Target agent: %s\n", target.Agent)
			cmd.Printf("Settings path: %s\n", target.ConfigPath)

			hookList := strings.Join(target.Agent.HookNames(), ", ")
			cmd.Printf("Would install hooks: %s\n", hookList)
			if target.Agent == install.AgentCodex {
				cmd.Printf("After install, run /hooks in Codex to trust the claudio hook.\n")
			}
		}
		cmd.Printf("No changes will be made.\n")
	} else {
		for _, target := range targets {
			cmd.Printf("DRY-RUN: %s %s -> %s\n", scope.String(), target.Agent, target.ConfigPath)
		}
	}
	return nil
}

func runInstallTargets(cmd *cobra.Command, scope InstallScope, targets []install.AgentTarget, quiet bool) error {
	if !quiet {
		cmd.Printf("Installing Claudio hooks for %s scope...\n", scope.String())
	}

	for _, target := range targets {
		if !quiet {
			cmd.Printf("Target agent: %s\n", target.Agent)
			cmd.Printf("Settings path: %s\n", target.ConfigPath)
		}

		err := runInstallWorkflow(target.Agent, scope.String(), target.ConfigPath)
		if err != nil {
			return fmt.Errorf("installation failed for %s: %w", target.Agent, err)
		}
	}

	if !quiet {
		cmd.Printf("✅ Claudio installation completed successfully!\n")
		cmd.Printf("Audio hooks have been added to selected agent settings.\n")
		for _, target := range targets {
			if target.Agent == install.AgentCodex {
				cmd.Printf("Run /hooks in Codex to trust the claudio hook.\n")
				break
			}
		}
	} else {
		cmd.Printf("Install: %s ✅\n", scope.String())
	}

	return nil
}

// runInstallWorkflow orchestrates the complete Claudio installation process
// Workflow: Detect paths → Read settings → Generate hooks → Merge → Write → Verify
func runInstallWorkflow(agent install.Agent, scope string, settingsPath string) error {
	slog.Info("starting Claudio installation workflow",
		"scope", scope,
		"settings_path", settingsPath)

	normalizedScope, err := install.NormalizeScope(scope)
	if err != nil {
		return err
	}
	scope = normalizedScope

	slog.Debug("validated installation scope", "scope", scope)

	settingsDir := filepath.Dir(settingsPath)
	if err := os.MkdirAll(settingsDir, 0755); err != nil {
		return fmt.Errorf("failed to create settings directory %s: %w", settingsDir, err)
	}

	// Acquire advisory lock around the full read-mutate-write window so
	// concurrent install/uninstall processes serialise. This must happen
	// BEFORE the initial ReadSettingsFile — putting it inside
	// WriteSettingsFile would not prevent the classic read-modify-write
	// race two install processes hit when they both read the same
	// starting state.
	lock, err := install.LockSettingsDir(settingsPath)
	if err != nil {
		return fmt.Errorf("install: %w", err)
	}
	defer func() {
		if unlockErr := lock.Unlock(); unlockErr != nil {
			slog.Warn("failed to release settings lock", "err", unlockErr)
		}
	}()

	// Step 2: Read existing settings
	slog.Debug("reading existing settings", "path", settingsPath)
	prodFS := afero.NewOsFs()
	existingSettings, err := install.ReadSettingsFile(prodFS, settingsPath)
	if err != nil {
		return fmt.Errorf("failed to read existing settings from %s: %w", settingsPath, err)
	}

	slog.Info("loaded existing settings",
		"path", settingsPath,
		"settings_keys", install.SettingsKeys(existingSettings))

	// Step 3: Generate Claudio hooks configuration
	slog.Debug("generating Claudio hooks configuration")

	// Get current executable path - must succeed
	execPath, err := install.GetExecutablePath()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	claudioHooks, err := install.GenerateClaudioHooksForAgent(execPath, agent)
	if err != nil {
		return fmt.Errorf("failed to generate Claudio hooks: %w", err)
	}

	slog.Info("generated Claudio hooks", "hooks", claudioHooks)

	// Step 4: Merge Claudio hooks into existing settings
	slog.Debug("merging Claudio hooks into existing settings")
	mergedSettings, err := install.MergeHooksIntoSettings(existingSettings, claudioHooks)
	if err != nil {
		return fmt.Errorf("failed to merge Claudio hooks into settings: %w", err)
	}

	slog.Info("merged Claudio hooks into settings",
		"merged_settings_keys", install.SettingsKeys(mergedSettings))

	// Step 5: Write merged settings back to file
	slog.Debug("writing merged settings to file", "path", settingsPath)
	err = install.WriteSettingsFile(prodFS, settingsPath, mergedSettings)
	if err != nil {
		return fmt.Errorf("failed to write merged settings to %s: %w", settingsPath, err)
	}

	slog.Info("wrote merged settings to file", "path", settingsPath)

	// Step 6: Verify installation by reading back and checking hooks
	slog.Debug("verifying installation by reading back settings")
	verifySettings, err := install.ReadSettingsFile(prodFS, settingsPath)
	if err != nil {
		return fmt.Errorf("failed to verify installation by reading %s: %w", settingsPath, err)
	}

	// Check that the default-enabled Claudio hooks are present. We iterate
	// EnabledHooks (not HookNames) so we match the set the write step
	// (install/hooks.go) actually writes — a DefaultEnabled=false hook
	// must NOT cause a verify mismatch because it was deliberately
	// skipped on write.
	if hooks, exists := (*verifySettings)["hooks"]; exists {
		if hooksMap, ok := hooks.(map[string]interface{}); ok {
			expectedHooks := agent.EnabledHooks()
			for _, h := range expectedHooks {
				hookName := h.Name
				if val, exists := hooksMap[hookName]; !exists {
					return fmt.Errorf("verification failed: Claudio hook '%s' missing after installation", hookName)
				} else if !install.IsClaudioHook(val) {
					return fmt.Errorf("verification failed: Claudio hook '%s' has wrong value '%v', expected a claudio hook", hookName, val)
				}
			}

			slog.Info("installation verification successful",
				"total_hooks", len(hooksMap),
				"claudio_hooks_verified", len(expectedHooks))
		} else {
			return fmt.Errorf("verification failed: hooks section is not a valid map type: %T", hooks)
		}
	} else {
		return fmt.Errorf("verification failed: no hooks section found after installation")
	}

	slog.Info("Claudio installation workflow completed successfully",
		"scope", scope,
		"settings_path", settingsPath)

	return nil
}
