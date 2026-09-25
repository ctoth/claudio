package cli

import (
	"fmt"

	"claudio.click/internal/install"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

// newUninstallCommand creates the uninstall subcommand with flags
func newUninstallCommand() *cobra.Command {
	var flags hookTargetFlags
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove claudio hooks from agent settings",
		Long:  "Remove claudio hooks from supported coding-agent settings to disable audio feedback for tool usage and events.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runUninstallCommand(cmd, &flags)
		},
	}
	flags.register(cmd, hookTargetHelp{
		scope:  "Uninstall scope",
		dryRun: "Show what would be removed without making changes (simulation mode)",
		print:  "Print hooks that would be removed",
	})
	return cmd
}

// runUninstallCommand handles the uninstall subcommand execution
func runUninstallCommand(cmd *cobra.Command, flags *hookTargetFlags) error {
	scope, targets, err := flags.resolve()
	if err != nil {
		return err
	}

	switch {
	case flags.print:
		flags.printHookTargets(cmd, "uninstall", scope, targets, func(target install.AgentTarget) {
			hooks, err := claudioHooksIn(target)
			switch {
			case err != nil:
				cmd.Printf("  Warning: Could not read settings file: %v\n", err)
			case len(hooks) == 0:
				cmd.Printf("  Hooks to remove: None (no claudio hooks found)\n")
			default:
				cmd.Printf("  Hooks to remove: %v\n", hooks)
			}
		})
		return nil
	case flags.dryRun:
		handleDryRunUninstall(cmd, flags, scope, targets)
		return nil
	default:
		return runUninstallTargets(cmd, flags, scope, targets)
	}
}

// claudioHooksIn lists the claudio hook names in the target's settings.
func claudioHooksIn(target install.AgentTarget) ([]string, error) {
	if target.Agent.UsesPluginFile() {
		if install.HasOpenCodePlugin(target.ConfigPath) {
			return target.Agent.HookNames(), nil
		}
		return nil, nil
	}
	settings, err := install.ReadSettingsFile(afero.NewOsFs(), target.ConfigPath)
	if err != nil {
		return nil, err
	}
	return install.ClaudioHookNames(settings), nil
}

func runUninstallTargets(cmd *cobra.Command, flags *hookTargetFlags, scope string, targets []install.AgentTarget) error {
	err := flags.applyToTargets(cmd, "Uninstalling", scope, targets, func(target install.AgentTarget) error {
		if err := install.RunUninstallWorkflow(afero.NewOsFs(), target); err != nil {
			return fmt.Errorf("uninstall failed for %s: %w", target.Agent, err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	if flags.quiet {
		cmd.Printf("Uninstall: %s ✅\n", scope)
		return nil
	}
	cmd.Printf("✅ Claudio uninstall completed successfully!\n")
	cmd.Printf("Audio hooks have been removed from selected agent settings.\n")
	return nil
}

// handleDryRunUninstall shows what would be done without making changes
func handleDryRunUninstall(cmd *cobra.Command, flags *hookTargetFlags, scope string, targets []install.AgentTarget) {
	if !flags.quiet {
		cmd.Printf("DRY-RUN: Claudio uninstall simulation for %s scope\n", scope)
	}

	for _, target := range targets {
		flags.printTarget(cmd, target)

		hooks, err := claudioHooksIn(target)
		switch {
		case err != nil && flags.quiet:
			cmd.Printf("DRY-RUN: %s %s -> ERROR: %v\n", scope, target.Agent, err)
		case err != nil:
			cmd.Printf("Would attempt to read settings, but got error: %v\n", err)
		case len(hooks) == 0 && flags.quiet:
			cmd.Printf("DRY-RUN: %s %s -> No hooks to remove\n", scope, target.Agent)
		case len(hooks) == 0:
			cmd.Printf("No claudio hooks found to remove.\n")
		case flags.quiet:
			cmd.Printf("DRY-RUN: %s %s -> Would remove: %v\n", scope, target.Agent, hooks)
		default:
			cmd.Printf("Would remove hooks: %v\n", hooks)
		}
	}
	if !flags.quiet {
		cmd.Printf("No changes will be made.\n")
	}
}
