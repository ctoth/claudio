package cli

import (
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"
	"github.com/ctoth/claudio/internal/uninstall"
	"github.com/ctoth/claudio/internal/install"
)

// newUninstallCommand creates the uninstall subcommand with flags
func newUninstallCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove claudio hooks from Claude Code settings",
		Long:  "Remove claudio hooks from Claude Code settings to disable audio feedback for tool usage and events.",
		RunE:  runUninstallCommandE,
	}

	// Add --scope flag with validation
	cmd.Flags().StringP("scope", "s", "user", "Uninstall scope: 'user' for user-specific settings, 'project' for project-specific settings")
	
	// Add --dry-run flag
	cmd.Flags().BoolP("dry-run", "d", false, "Show what would be removed without making changes (simulation mode)")
	
	// Add --force flag
	cmd.Flags().BoolP("force", "f", false, "Remove hooks without prompting")

	// Add --quiet flag
	cmd.Flags().BoolP("quiet", "q", false, "Suppress output (no progress messages)")
	
	// Add --print flag
	cmd.Flags().BoolP("print", "p", false, "Print hooks that would be removed")

	return cmd
}

// runUninstallCommandE handles the uninstall subcommand execution
func runUninstallCommandE(cmd *cobra.Command, args []string) error {
	slog.Debug("uninstall command started", "args", args)

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

	slog.Info("uninstall command executing", "scope", scope, "dry_run", dryRun, "force", force, "quiet", quiet, "print", print)

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

	// Handle print flag - shows what hooks would be removed
	if print {
		return handlePrintUninstall(cmd, scope, settingsPath, dryRun, force, quiet)
	}

	// Handle dry-run mode - show what would be done without making changes
	if dryRun {
		return handleDryRunUninstall(cmd, scope, settingsPath, quiet)
	}

	// Run the actual uninstall workflow
	if !quiet {
		cmd.Printf("Uninstalling Claudio hooks for %s scope...\n", scope.String())
		cmd.Printf("Settings path: %s\n", settingsPath)
	}

	err = uninstall.RunUninstallWorkflow(scope.String(), settingsPath)
	if err != nil {
		return fmt.Errorf("uninstall failed: %w", err)
	}

	// Success message
	if !quiet {
		cmd.Printf("✅ Claudio uninstall completed successfully!\n")
		cmd.Printf("Audio hooks have been removed from Claude Code settings.\n")
		if force {
			cmd.Printf("Force mode was used - hooks were removed without prompting.\n")
		}
	} else {
		cmd.Printf("Uninstall: %s ✅\n", scope.String())
	}

	return nil
}

// handlePrintUninstall shows configuration details about what would be removed
func handlePrintUninstall(cmd *cobra.Command, scope InstallScope, settingsPath string, dryRun bool, force bool, quiet bool) error {
	var configDetails string
	if dryRun && force {
		configDetails = "PRINT: DRY-RUN + FORCE uninstall configuration for scope: " + scope.String()
	} else if dryRun {
		configDetails = "PRINT: DRY-RUN uninstall configuration for scope: " + scope.String()
	} else if force {
		configDetails = "PRINT: FORCE uninstall configuration for scope: " + scope.String()
	} else {
		configDetails = "PRINT: Uninstall configuration for scope: " + scope.String()
	}
	
	cmd.Printf("%s\n", configDetails)
	if dryRun {
		cmd.Printf("  Mode: Simulation (no changes will be made)\n")
	}
	if force {
		cmd.Printf("  Mode: Force (will remove without prompting)\n")
	}
	if quiet {
		cmd.Printf("  Output: Quiet mode (minimal messages)\n")
	}
	cmd.Printf("  Scope: %s\n", scope.String())
	cmd.Printf("  Settings Path: %s\n", settingsPath)

	// Try to read settings and show what hooks would be removed
	settings, err := install.ReadSettingsFileWithLock(settingsPath)
	if err != nil {
		cmd.Printf("  Warning: Could not read settings file: %v\n", err)
		return nil
	}

	claudioHooks := uninstall.DetectClaudioHooks(settings)
	if len(claudioHooks) == 0 {
		cmd.Printf("  Hooks to remove: None (no claudio hooks found)\n")
	} else {
		cmd.Printf("  Hooks to remove: %v\n", claudioHooks)
	}

	return nil
}

// handleDryRunUninstall shows what would be done without making changes
func handleDryRunUninstall(cmd *cobra.Command, scope InstallScope, settingsPath string, quiet bool) error {
	if !quiet {
		cmd.Printf("DRY-RUN: Claudio uninstall simulation for %s scope\n", scope.String())
		cmd.Printf("Settings path: %s\n", settingsPath)
	}

	// Try to read settings and show what would be removed
	settings, err := install.ReadSettingsFileWithLock(settingsPath)
	if err != nil {
		if !quiet {
			cmd.Printf("Would attempt to read settings, but got error: %v\n", err)
			cmd.Printf("No changes will be made.\n")
		} else {
			cmd.Printf("DRY-RUN: %s -> ERROR: %v\n", scope.String(), err)
		}
		return nil
	}

	claudioHooks := uninstall.DetectClaudioHooks(settings)
	if len(claudioHooks) == 0 {
		if !quiet {
			cmd.Printf("No claudio hooks found to remove.\n")
			cmd.Printf("No changes will be made.\n")
		} else {
			cmd.Printf("DRY-RUN: %s -> No hooks to remove\n", scope.String())
		}
	} else {
		if !quiet {
			cmd.Printf("Would remove hooks: %v\n", claudioHooks)
			cmd.Printf("No changes will be made.\n")
		} else {
			cmd.Printf("DRY-RUN: %s -> Would remove: %v\n", scope.String(), claudioHooks)
		}
	}

	return nil
}