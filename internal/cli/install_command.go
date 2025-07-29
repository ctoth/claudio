package cli

import (
	"fmt"
	"log/slog"

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
	
	// Add --force flag
	cmd.Flags().BoolP("force", "f", false, "Overwrite existing hooks without prompting")

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

	slog.Info("install command executing", "scope", scope, "dry_run", dryRun, "force", force)

	// TODO: Implement actual installation logic in later commits
	// For now, just validate the flags and return success
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