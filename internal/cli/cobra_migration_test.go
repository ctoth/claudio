package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

// TestRootCommandExists verifies that the CLI has a cobra root command
func TestRootCommandExists(t *testing.T) {
	cli := NewCLI()

	if cli == nil {
		t.Fatal("NewCLI returned nil")
	}

	// This test expects the CLI to have a rootCmd field with a cobra.Command
	// This will fail initially until we modify the CLI struct to include rootCmd
	if cli.rootCmd == nil {
		t.Fatal("CLI.rootCmd is nil - expected *cobra.Command")
	}

	// Verify it's actually a cobra command
	if _, ok := interface{}(cli.rootCmd).(*cobra.Command); !ok {
		t.Fatal("CLI.rootCmd is not a *cobra.Command")
	}

	// Verify basic command properties
	if cli.rootCmd.Use != "claudio" {
		t.Errorf("Expected rootCmd.Use to be 'claudio', got %q", cli.rootCmd.Use)
	}

	if cli.rootCmd.Short == "" {
		t.Error("Expected rootCmd.Short to be set")
	}
}