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

// newInstallCommand creates the install subcommand with flags
func newInstallCommand() *cobra.Command {
	var flags hookTargetFlags
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install claudio hooks into agent settings",
		Long:  "Install claudio hooks into supported coding-agent settings to enable audio feedback for tool usage and events.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runInstallCommand(cmd, &flags)
		},
	}
	flags.register(cmd, hookTargetHelp{
		scope:  "Installation scope",
		dryRun: "Show what would be done without making changes (simulation mode)",
		print:  "Print configuration that would be written",
	})
	return cmd
}

// runInstallCommand handles the install subcommand execution
func runInstallCommand(cmd *cobra.Command, flags *hookTargetFlags) error {
	scope, targets, err := flags.resolve()
	if err != nil {
		return err
	}

	switch {
	case flags.print:
		flags.printHookTargets(cmd, "install", scope, targets, nil)
		return nil
	case flags.dryRun:
		handleDryRunInstall(cmd, flags, scope, targets)
		return nil
	default:
		return runInstallTargets(cmd, flags, scope, targets)
	}
}

func handleDryRunInstall(cmd *cobra.Command, flags *hookTargetFlags, scope string, targets []install.AgentTarget) {
	if flags.quiet {
		for _, target := range targets {
			cmd.Printf("DRY-RUN: %s %s -> %s\n", scope, target.Agent, target.ConfigPath)
		}
		return
	}
	cmd.Printf("DRY-RUN: Claudio installation simulation for %s scope\n", scope)
	for _, target := range targets {
		flags.printTarget(cmd, target)
		cmd.Printf("Would install hooks: %s\n", strings.Join(enabledHookNames(target.Agent), ", "))
		if hint := target.Agent.TrustHint(); hint != "" {
			cmd.Printf("After install, %s\n", lowerFirst(hint))
		}
	}
	cmd.Printf("No changes will be made.\n")
}

// lowerFirst lower-cases the first byte of an ASCII sentence so it can
// follow a lead-in clause.
func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}

func enabledHookNames(agent install.Agent) []string {
	definitions := agent.EnabledHooks()
	names := make([]string, len(definitions))
	for i, definition := range definitions {
		names[i] = definition.Name
	}
	return names
}

func runInstallTargets(cmd *cobra.Command, flags *hookTargetFlags, scope string, targets []install.AgentTarget) error {
	err := flags.applyToTargets(cmd, "Installing", scope, targets, func(target install.AgentTarget) error {
		if err := runInstallWorkflow(target.Agent, scope, target.ConfigPath); err != nil {
			return fmt.Errorf("installation failed for %s: %w", target.Agent, err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	if flags.quiet {
		cmd.Printf("Install: %s ✅\n", scope)
		return nil
	}
	cmd.Printf("✅ Claudio installation completed successfully!\n")
	cmd.Printf("Audio hooks have been added to selected agent settings.\n")
	for _, target := range targets {
		if hint := target.Agent.TrustHint(); hint != "" {
			cmd.Printf("%s\n", hint)
			break
		}
	}
	return nil
}

// runInstallWorkflow installs the agent's claudio hooks into settingsPath:
// one locked read-merge-write through install.ModifySettings. The merged
// settings are not read back to "verify" them: that check only repeated
// the recognizer the merge had just used.
func runInstallWorkflow(agent install.Agent, scope string, settingsPath string) error {
	slog.Info("starting Claudio installation workflow",
		"agent", agent,
		"scope", scope,
		"settings_path", settingsPath)

	if _, err := install.NormalizeScope(scope); err != nil {
		return err
	}

	settingsDir := filepath.Dir(settingsPath)
	if err := os.MkdirAll(settingsDir, 0755); err != nil {
		return fmt.Errorf("failed to create settings directory %s: %w", settingsDir, err)
	}

	execPath, err := install.GetExecutablePath()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	if agent.UsesPluginFile() {
		if err := install.WriteOpenCodePlugin(settingsPath, execPath); err != nil {
			return fmt.Errorf("install: %w", err)
		}
		slog.Info("Claudio plugin written", "agent", agent, "plugin_path", settingsPath)
		return nil
	}

	err = install.ModifySettings(afero.NewOsFs(), settingsPath, func(settings *install.SettingsMap) (*install.SettingsMap, error) {
		return install.InstallAgentHooks(settings, agent, execPath)
	})
	if err != nil {
		return fmt.Errorf("install: %w", err)
	}

	slog.Info("Claudio installation workflow completed successfully",
		"agent", agent,
		"settings_path", settingsPath)
	return nil
}
