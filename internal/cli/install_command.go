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