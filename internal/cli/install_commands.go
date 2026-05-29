package cli

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// claudioCommandContent is the content of the claudio.md slash command file
const claudioCommandContent = `---
allowed-tools: Bash(claudio:*)
argument-hint: [volume 0.0-1.0 | mute | unmute | status]
description: Control Claudio audio feedback
---
Control Claudio audio using the claudio CLI.

Available commands:
- volume [0.0-1.0]: Set volume level
- mute: Disable audio persistently
- unmute: Enable audio persistently
- status: Show current settings

Run: claudio $ARGUMENTS
`

// newInstallCommandsCommand creates the install-commands subcommand
func newInstallCommandsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install-commands",
		Short: "Install slash commands for Claude Code",
		Long: `Install slash commands for controlling Claudio from within Claude Code.

This command creates a claudio.md file in ~/.claude/commands/ that enables
you to use /claudio slash commands directly in Claude Code sessions.

After installation, you can use commands like:
  /claudio volume 0.5
  /claudio mute
  /claudio unmute
  /claudio status`,
		RunE: runInstallCommandsE,
	}

	return cmd
}

// runInstallCommandsE handles the install-commands subcommand execution
func runInstallCommandsE(cmd *cobra.Command, args []string) error {
	slog.Debug("install-commands started")

	// Get user home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	// Build paths
	commandsDir := filepath.Join(homeDir, ".claude", "commands")
	claudioMdPath := filepath.Join(commandsDir, "claudio.md")

	// Install the command file
	err = installCommandsToPath(commandsDir, claudioMdPath)
	if err != nil {
		return fmt.Errorf("failed to install slash commands: %w", err)
	}

	// Print success message
	cmd.Printf("Installed slash command: %s\n\n", claudioMdPath)
	cmd.Printf("You can now use /claudio in Claude Code:\n")
	cmd.Printf("  /claudio volume 0.5   - Set volume to 50%%\n")
	cmd.Printf("  /claudio mute         - Disable audio\n")
	cmd.Printf("  /claudio unmute       - Enable audio\n")
	cmd.Printf("  /claudio status       - Show current settings\n")

	slog.Info("slash commands installed successfully", "path", claudioMdPath)

	return nil
}

// installCommandsToPath creates the commands directory and writes claudio.md
// This function is exported for testing
func installCommandsToPath(commandsDir, claudioMdPath string) error {
	slog.Debug("installing commands", "dir", commandsDir, "file", claudioMdPath)

	// Create commands directory if it doesn't exist
	err := os.MkdirAll(commandsDir, 0755)
	if err != nil {
		slog.Error("failed to create commands directory", "path", commandsDir, "error", err)
		return fmt.Errorf("failed to create commands directory: %w", err)
	}

	slog.Debug("commands directory ready", "path", commandsDir)

	// Write claudio.md file
	err = os.WriteFile(claudioMdPath, []byte(claudioCommandContent), 0644)
	if err != nil {
		slog.Error("failed to write claudio.md", "path", claudioMdPath, "error", err)
		return fmt.Errorf("failed to write claudio.md: %w", err)
	}

	slog.Debug("claudio.md written successfully", "path", claudioMdPath)

	return nil
}
