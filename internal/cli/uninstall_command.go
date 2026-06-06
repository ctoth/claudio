package cli

import (
	"fmt"
	"log/slog"

	"claudio.click/internal/install"
	"claudio.click/internal/uninstall"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

// newUninstallCommand creates the uninstall subcommand with flags
func newUninstallCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove claudio hooks from agent settings",
		Long:  "Remove claudio hooks from supported coding-agent settings to disable audio feedback for tool usage and events.",
		RunE:  runUninstallCommandE,
	}

	// Add --scope flag with validation
	cmd.Flags().StringP("scope", "s", install.ScopeGlobal, "Uninstall scope: 'global' for user-wide settings, 'project' for project-specific settings")

	// Add --agent flag with validation
	cmd.Flags().StringP("agent", "a", string(install.AgentAuto), "Target agent: 'auto', 'claude', 'codex', 'gemini', 'qwen', 'copilot', or 'all'")

	// Add --dry-run flag
	cmd.Flags().BoolP("dry-run", "d", false, "Show what would be removed without making changes (simulation mode)")

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

	slog.Info("uninstall command executing", "scope", scope, "agent", agent, "dry_run", dryRun, "quiet", quiet, "print", print)

	targets, err := install.ResolveAgentTargets(agent, scope.String())
	if err != nil {
		return err
	}

	slog.Debug("resolved uninstall targets", "scope", scope, "agent", agent, "count", len(targets))

	// Handle print flag - shows what hooks would be removed
	if print {
		return handlePrintUninstall(cmd, scope, targets, dryRun, quiet)
	}

	// Handle dry-run mode - show what would be done without making changes
	if dryRun {
		return handleDryRunUninstall(cmd, scope, targets, quiet)
	}

	return runUninstallTargets(cmd, scope, targets, quiet)
}

func runUninstallTargets(cmd *cobra.Command, scope InstallScope, targets []install.AgentTarget, quiet bool) error {
	if !quiet {
		cmd.Printf("Uninstalling Claudio hooks for %s scope...\n", scope.String())
	}

	for _, target := range targets {
		if !quiet {
			cmd.Printf("Target agent: %s\n", target.Agent)
			cmd.Printf("Settings path: %s\n", target.ConfigPath)
		}

		err := uninstall.RunUninstallWorkflow(afero.NewOsFs(), scope.String(), target.Agent)
		if err != nil {
			return fmt.Errorf("uninstall failed for %s: %w", target.Agent, err)
		}
	}

	// Success message
	if !quiet {
		cmd.Printf("✅ Claudio uninstall completed successfully!\n")
		cmd.Printf("Audio hooks have been removed from selected agent settings.\n")
	} else {
		cmd.Printf("Uninstall: %s ✅\n", scope.String())
	}

	return nil
}

// handlePrintUninstall shows configuration details about what would be removed
func handlePrintUninstall(cmd *cobra.Command, scope InstallScope, targets []install.AgentTarget, dryRun bool, quiet bool) error {
	var configDetails string
	if dryRun {
		configDetails = "PRINT: DRY-RUN uninstall configuration for scope: " + scope.String()
	} else {
		configDetails = "PRINT: Uninstall configuration for scope: " + scope.String()
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

		// Try to read settings and show what hooks would be removed
		prodFS := afero.NewOsFs()
		settings, err := install.ReadSettingsFile(prodFS, target.ConfigPath)
		if err != nil {
			cmd.Printf("  Warning: Could not read settings file: %v\n", err)
			continue
		}

		claudioHooks := uninstall.DetectClaudioHooks(settings)
		if len(claudioHooks) == 0 {
			cmd.Printf("  Hooks to remove: None (no claudio hooks found)\n")
		} else {
			cmd.Printf("  Hooks to remove: %v\n", claudioHooks)
		}
	}

	return nil
}

// handleDryRunUninstall shows what would be done without making changes
func handleDryRunUninstall(cmd *cobra.Command, scope InstallScope, targets []install.AgentTarget, quiet bool) error {
	if !quiet {
		cmd.Printf("DRY-RUN: Claudio uninstall simulation for %s scope\n", scope.String())
	}

	for _, target := range targets {
		if !quiet {
			cmd.Printf("Target agent: %s\n", target.Agent)
			cmd.Printf("Settings path: %s\n", target.ConfigPath)
		}

		// Try to read settings and show what would be removed
		prodFS := afero.NewOsFs()
		settings, err := install.ReadSettingsFile(prodFS, target.ConfigPath)
		if err != nil {
			if !quiet {
				cmd.Printf("Would attempt to read settings, but got error: %v\n", err)
			} else {
				cmd.Printf("DRY-RUN: %s %s -> ERROR: %v\n", scope.String(), target.Agent, err)
			}
			continue
		}

		claudioHooks := uninstall.DetectClaudioHooks(settings)
		if len(claudioHooks) == 0 {
			if !quiet {
				cmd.Printf("No claudio hooks found to remove.\n")
			} else {
				cmd.Printf("DRY-RUN: %s %s -> No hooks to remove\n", scope.String(), target.Agent)
			}
		} else {
			if !quiet {
				cmd.Printf("Would remove hooks: %v\n", claudioHooks)
			} else {
				cmd.Printf("DRY-RUN: %s %s -> Would remove: %v\n", scope.String(), target.Agent, claudioHooks)
			}
		}
	}
	if !quiet {
		cmd.Printf("No changes will be made.\n")
	}

	return nil
}
