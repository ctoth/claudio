package cli

import (
	"bytes"
	"io"
	"strings"
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

func TestInstallCommandRejectsInvalidAgent(t *testing.T) {
	cmd := newInstallCommand()
	cmd.SetArgs([]string{"--agent", "gemini", "--dry-run"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for invalid agent, got nil")
	}
}

func TestInstallCommandCodexDryRunShowsTrustReminder(t *testing.T) {
	cmd := newInstallCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--agent", "codex", "--dry-run"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := out.String()
	if !strings.Contains(s, "hooks.json") {
		t.Errorf("expected codex hooks.json path in output, got: %s", s)
	}
	if !strings.Contains(s, "/hooks") {
		t.Errorf("expected /hooks trust reminder in output, got: %s", s)
	}
}

func TestInstallCommandDefaultsToClaude(t *testing.T) {
	cmd := newInstallCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--dry-run"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "settings.json") {
		t.Errorf("expected claude settings.json path by default, got: %s", out.String())
	}
}
