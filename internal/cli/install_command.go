package cli

import (
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"
	"github.com/ctoth/claudio/internal/install"
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
	
	// Add --force flag
	cmd.Flags().BoolP("force", "f", false, "Overwrite existing hooks without prompting")

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

	// Get force flag
	force, err := cmd.Flags().GetBool("force")
	if err != nil {
		return fmt.Errorf("failed to get force flag: %w", err)
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

	slog.Info("install command executing", "scope", scope, "dry_run", dryRun, "force", force, "quiet", quiet, "print", print)

	// TODO: Implement actual installation logic in later commits
	// For now, just validate the flags and return success

	// Handle print flag - shows configuration details
	if print {
		var configDetails string
		if dryRun && force {
			configDetails = "PRINT: DRY-RUN + FORCE configuration for scope: " + scope.String()
		} else if dryRun {
			configDetails = "PRINT: DRY-RUN configuration for scope: " + scope.String()
		} else if force {
			configDetails = "PRINT: FORCE configuration for scope: " + scope.String()
		} else {
			configDetails = "PRINT: Install configuration for scope: " + scope.String()
		}
		
		cmd.Printf("%s\n", configDetails)
		if dryRun {
			cmd.Printf("  Mode: Simulation (no changes will be made)\n")
		}
		if force {
			cmd.Printf("  Mode: Force (will overwrite without prompting)\n")
		}
		if quiet {
			cmd.Printf("  Output: Quiet mode (minimal messages)\n")
		}
		cmd.Printf("  Scope: %s\n", scope.String())
		return nil
	}

	// Handle quiet mode - minimal output
	if quiet {
		// Only show essential information in quiet mode
		if dryRun {
			cmd.Printf("DRY-RUN: %s\n", scope.String())
		} else if force {
			cmd.Printf("FORCE: %s\n", scope.String())
		} else {
			cmd.Printf("Install: %s\n", scope.String())
		}
		return nil
	}

	// Normal verbose output mode
	var prefix string
	if dryRun && force {
		prefix = "DRY-RUN + FORCE:"
	} else if dryRun {
		prefix = "DRY-RUN:"
	} else if force {
		prefix = "FORCE:"
	} else {
		prefix = ""
	}

	if prefix != "" {
		cmd.Printf("%s Install command would run with scope: %s", prefix, scope)
		if dryRun {
			cmd.Printf(" (no changes will be made)")
		}
		if force {
			cmd.Printf(" (will overwrite without prompting)")
		}
		cmd.Printf("\n")
	} else {
		cmd.Printf("Install command would run with scope: %s\n", scope)
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
	
	// Step 2: Read existing settings (uses file locking for safety)
	slog.Debug("reading existing settings", "path", settingsPath)
	existingSettings, err := install.ReadSettingsFileWithLock(settingsPath)
	if err != nil {
		return fmt.Errorf("failed to read existing settings from %s: %w", settingsPath, err)
	}
	
	slog.Info("loaded existing settings", 
		"path", settingsPath,
		"settings_keys", getSettingsKeys(existingSettings))
	
	// Step 3: Generate Claudio hooks configuration
	slog.Debug("generating Claudio hooks configuration")
	claudiaHooks, err := install.GenerateClaudiaHooks()
	if err != nil {
		return fmt.Errorf("failed to generate Claudio hooks: %w", err)
	}
	
	slog.Info("generated Claudio hooks", "hooks", claudiaHooks)
	
	// Step 4: Merge Claudio hooks into existing settings
	slog.Debug("merging Claudio hooks into existing settings")
	mergedSettings, err := install.MergeHooksIntoSettings(existingSettings, claudiaHooks)
	if err != nil {
		return fmt.Errorf("failed to merge Claudio hooks into settings: %w", err)
	}
	
	slog.Info("merged Claudio hooks into settings", 
		"merged_settings_keys", getSettingsKeys(mergedSettings))
	
	// Step 5: Write merged settings back to file (uses file locking for safety)
	slog.Debug("writing merged settings to file", "path", settingsPath)
	err = install.WriteSettingsFileWithLock(settingsPath, mergedSettings)
	if err != nil {
		return fmt.Errorf("failed to write merged settings to %s: %w", settingsPath, err)
	}
	
	slog.Info("wrote merged settings to file", "path", settingsPath)
	
	// Step 6: Verify installation by reading back and checking hooks
	slog.Debug("verifying installation by reading back settings")
	verifySettings, err := install.ReadSettingsFileWithLock(settingsPath)
	if err != nil {
		return fmt.Errorf("failed to verify installation by reading %s: %w", settingsPath, err)
	}
	
	// Check that all Claudio hooks are present
	if hooks, exists := (*verifySettings)["hooks"]; exists {
		if hooksMap, ok := hooks.(map[string]interface{}); ok {
			expectedHooks := []string{"PreToolUse", "PostToolUse", "UserPromptSubmit"}
			for _, hookName := range expectedHooks {
				if val, exists := hooksMap[hookName]; !exists {
					return fmt.Errorf("verification failed: Claudio hook '%s' missing after installation", hookName)
				} else if val != "claudio" {
					return fmt.Errorf("verification failed: Claudio hook '%s' has wrong value '%v', expected 'claudio'", hookName, val)
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