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
	cmd.Flags().StringP("agent", "a", string(install.AgentAuto), "Target agent: 'auto', 'claude', 'codex', 'gemini', 'qwen', 'copilot', or 'all'")

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

			hookList := strings.Join(enabledHookNames(target.Agent), ", ")
			cmd.Printf("Would install hooks: %s\n", hookList)
			if hint := target.Agent.TrustHint(); hint != "" {
				cmd.Printf("After install, %s\n", lowerFirst(hint))
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
			if hint := target.Agent.TrustHint(); hint != "" {
				cmd.Printf("%s\n", hint)
				break
			}
		}
	} else {
		cmd.Printf("Install: %s ✅\n", scope.String())
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
