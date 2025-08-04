package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

// TestInstallCommandExists verifies that the install subcommand exists
// This is a RED test - will fail until we implement the install subcommand
func TestInstallCommandExists(t *testing.T) {
	cli := NewCLI()

	// This test expects the CLI to have an install subcommand
	// The install command should be accessible via cli.rootCmd.Commands()

	installCmd := findCommand(cli.rootCmd, "install")
	if installCmd == nil {
		t.Fatal("install subcommand not found - expected 'claudio install' to be available")
	}

	if installCmd.Use != "install" {
		t.Errorf("Expected install command Use to be 'install', got %q", installCmd.Use)
	}

	if installCmd.Short == "" {
		t.Error("Expected install command to have a Short description")
	}

	t.Logf("Install command found: Use=%q, Short=%q", installCmd.Use, installCmd.Short)
}

// findCommand searches for a subcommand by name in the given command
func findCommand(rootCmd *cobra.Command, name string) *cobra.Command {
	// This helper will need to be implemented to find cobra subcommands
	// For now, return nil to make the test fail (RED phase)
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == name {
			return cmd
		}
	}
	return nil
}
