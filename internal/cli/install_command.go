package cli

import (
	"fmt"
	"log/slog"
	"strings"

	"claudio.click/internal/install"
	"claudio.click/internal/util"
	"github.com/spf13/cobra"
)

// InstallScope represents the scope of installation
type InstallScope string

const (
	ScopeUser    InstallScope = "user"
	ScopeProject InstallScope = "project"
)

// String returns the string representation of InstallScope
func (s InstallScope) String() string {
	return string(s)
}

// IsValid returns true if the scope is valid
func (s InstallScope) IsValid() bool {
	return s == ScopeUser || s == ScopeProject
}

// newInstallCommand creates the install subcommand with flags
func newInstallCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install claudio hooks into Claude Code settings",
		Long:  "Install claudio hooks into Claude Code settings to enable audio feedback for tool usage and events.",
		RunE:  runInstallCommandE,
	}

	// Add --scope flag with validation
	cmd.Flags().StringP("scope", "s", "user", "Installation scope: 'user' for user-specific settings, 'project' for project-specific settings")

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

	scope := InstallScope(scopeStr)
	if !scope.IsValid() {
		return fmt.Errorf("invalid scope '%s': must be 'user' or 'project'", scopeStr)
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

	slog.Info("install command executing", "scope", scope, "dry_run", dryRun, "quiet", quiet, "print", print)

	// Find Claude Code settings paths for the specified scope
	settingsPaths, err := install.FindClaudeSettingsPaths(scope.String())
	if err != nil {
		return fmt.Errorf("failed to find Claude Code settings paths: %w", err)
	}

	if len(settingsPaths) == 0 {
		return fmt.Errorf("no Claude Code settings paths found for scope: %s", scope)
	}

	// Use the first available path
	settingsPath := settingsPaths[0]
	slog.Debug("using settings path", "path", settingsPath, "scope", scope)

	// Handle print flag - shows configuration details
	if print {
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
		cmd.Printf("  Settings Path: %s\n", settingsPath)
		return nil
	}

	// Handle dry-run mode - show what would be done without making changes
	if dryRun {
		if !quiet {
			cmd.Printf("DRY-RUN: Claudio installation simulation for %s scope\n", scope.String())
			cmd.Printf("Settings path: %s\n", settingsPath)

			// Use registry to show hook names instead of hardcoded list
			hookNames := install.GetHookNames()
			hookList := strings.Join(hookNames, ", ")
			cmd.Printf("Would install hooks: %s\n", hookList)
			cmd.Printf("No changes will be made.\n")
		} else {
			cmd.Printf("DRY-RUN: %s -> %s\n", scope.String(), settingsPath)
		}
		return nil
	}

	// Run the actual installation workflow
	if !quiet {
		cmd.Printf("Installing Claudio hooks for %s scope...\n", scope.String())
		cmd.Printf("Settings path: %s\n", settingsPath)
	}

	err = runInstallWorkflow(scope.String(), settingsPath)
	if err != nil {
		return fmt.Errorf("installation failed: %w", err)
	}

	// Success message
	if !quiet {
		cmd.Printf("✅ Claudio installation completed successfully!\n")
		cmd.Printf("Audio hooks have been added to Claude Code settings.\n")
	} else {
		cmd.Printf("Install: %s ✅\n", scope.String())
	}

	return nil
}

// runInstallWorkflow orchestrates the complete Claudio installation process
// Workflow: Detect paths → Read settings → Generate hooks → Merge → Write → Verify
func runInstallWorkflow(scope string, settingsPath string) error {
	slog.Info("starting Claudio installation workflow",
		"scope", scope,
		"settings_path", settingsPath)

	// Step 1: Validate scope
	if scope != "user" && scope != "project" {
		return fmt.Errorf("invalid scope '%s': must be 'user' or 'project'", scope)
	}

	slog.Debug("validated installation scope", "scope", scope)

	// Step 2: Read existing settings
	slog.Debug("reading existing settings", "path", settingsPath)
	factory := install.GetFilesystemFactory()
	prodFS := factory.Production()
	existingSettings, err := install.ReadSettingsFile(prodFS, settingsPath)
	if err != nil {
		return fmt.Errorf("failed to read existing settings from %s: %w", settingsPath, err)
	}

	slog.Info("loaded existing settings",
		"path", settingsPath,
		"settings_keys", util.GetSettingsKeys(existingSettings))

	// Step 3: Generate Claudio hooks configuration
	slog.Debug("generating Claudio hooks configuration")
	
	// Get current executable path - must succeed
	execPath, err := install.GetExecutablePath()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}
	
	// Use production filesystem (reuse existing variables)
	
	claudioHooks, err := install.GenerateClaudioHooks(prodFS, execPath)
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
		"merged_settings_keys", util.GetSettingsKeys(mergedSettings))

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

	// Check that all Claudio hooks are present
	if hooks, exists := (*verifySettings)["hooks"]; exists {
		if hooksMap, ok := hooks.(map[string]interface{}); ok {
			expectedHooks := install.GetHookNames() // Use registry instead of hardcoded list
			for _, hookName := range expectedHooks {
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

