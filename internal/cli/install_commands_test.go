package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestInstallCommandsCreation verifies the install-commands subcommand is created correctly
func TestInstallCommandsCreation(t *testing.T) {
	cmd := newInstallCommandsCommand()

	if cmd == nil {
		t.Fatal("newInstallCommandsCommand() returned nil")
	}

	if cmd.Use != "install-commands" {
		t.Errorf("expected Use 'install-commands', got %s", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("expected Short description to be set")
	}
}

// TestInstallCommandsHelp verifies --help flag works
func TestInstallCommandsHelp(t *testing.T) {
	cmd := newInstallCommandsCommand()

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("--help failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "install-commands") {
		t.Error("help output should contain 'install-commands'")
	}
	if !strings.Contains(output, "command artifacts") {
		t.Error("help output should describe command artifact installation")
	}
	if !strings.Contains(output, "Codex") {
		t.Error("help output should describe Codex skill installation")
	}
}

// TestInstallCommandsCreatesDirectory verifies the ~/.claude/commands/ directory is created
func TestInstallCommandsCreatesDirectory(t *testing.T) {
	// Create a temporary directory to act as home
	tmpHome, err := os.MkdirTemp("", "claudio-test-home-*")
	if err != nil {
		t.Fatalf("failed to create temp home: %v", err)
	}
	defer os.RemoveAll(tmpHome)

	// Call the internal function with custom home
	commandsDir := filepath.Join(tmpHome, ".claude", "commands")
	claudioMdPath := filepath.Join(commandsDir, "claudio.md")

	err = installCommandsToPath(commandsDir, claudioMdPath)
	if err != nil {
		t.Fatalf("installCommandsToPath failed: %v", err)
	}

	// Check directory was created
	if _, err := os.Stat(commandsDir); os.IsNotExist(err) {
		t.Error("commands directory should have been created")
	}

	// Check claudio.md was created
	if _, err := os.Stat(claudioMdPath); os.IsNotExist(err) {
		t.Error("claudio.md should have been created")
	}
}

// TestInstallCommandsFileContent verifies the claudio.md content is correct
func TestInstallCommandsFileContent(t *testing.T) {
	// Create a temporary directory to act as home
	tmpHome, err := os.MkdirTemp("", "claudio-test-home-*")
	if err != nil {
		t.Fatalf("failed to create temp home: %v", err)
	}
	defer os.RemoveAll(tmpHome)

	// Call the internal function with custom home
	commandsDir := filepath.Join(tmpHome, ".claude", "commands")
	claudioMdPath := filepath.Join(commandsDir, "claudio.md")

	err = installCommandsToPath(commandsDir, claudioMdPath)
	if err != nil {
		t.Fatalf("installCommandsToPath failed: %v", err)
	}

	// Read the created file
	content, err := os.ReadFile(claudioMdPath)
	if err != nil {
		t.Fatalf("failed to read claudio.md: %v", err)
	}

	contentStr := string(content)

	// Verify frontmatter
	if !strings.Contains(contentStr, "---") {
		t.Error("content should contain frontmatter delimiters")
	}
	if !strings.Contains(contentStr, "allowed-tools: Bash(claudio:*)") {
		t.Error("content should contain allowed-tools directive")
	}
	if !strings.Contains(contentStr, "argument-hint:") {
		t.Error("content should contain argument-hint")
	}
	if !strings.Contains(contentStr, "description:") {
		t.Error("content should contain description")
	}

	// Verify command documentation
	if !strings.Contains(contentStr, "volume") {
		t.Error("content should document volume command")
	}
	if !strings.Contains(contentStr, "mute") {
		t.Error("content should document mute command")
	}
	if !strings.Contains(contentStr, "unmute") {
		t.Error("content should document unmute command")
	}
	if !strings.Contains(contentStr, "status") {
		t.Error("content should document status command")
	}

	// Verify run instruction
	if !strings.Contains(contentStr, "claudio $ARGUMENTS") {
		t.Error("content should contain run instruction with $ARGUMENTS")
	}
}

// TestInstallCommandsOutputMessage verifies the success message is printed
func TestInstallCommandsOutputMessage(t *testing.T) {
	// Create a temporary directory to act as home
	tmpHome, err := os.MkdirTemp("", "claudio-test-home-*")
	if err != nil {
		t.Fatalf("failed to create temp home: %v", err)
	}
	defer os.RemoveAll(tmpHome)

	// Set HOME env var for the test
	originalHome := os.Getenv("HOME")
	originalUserProfile := os.Getenv("USERPROFILE")
	os.Setenv("HOME", tmpHome)
	os.Setenv("USERPROFILE", tmpHome)
	defer func() {
		os.Setenv("HOME", originalHome)
		os.Setenv("USERPROFILE", originalUserProfile)
	}()

	cmd := newInstallCommandsCommand()

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{})

	err = cmd.Execute()
	if err != nil {
		t.Fatalf("install-commands failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "claudio.md") {
		t.Error("success message should mention claudio.md")
	}
	if !strings.Contains(output, "/claudio") {
		t.Error("success message should show usage example")
	}
}

func TestInstallCommandsRejectsInvalidAgent(t *testing.T) {
	cmd := newInstallCommandsCommand()

	cmd.SetArgs([]string{"--agent", "gemini"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected invalid agent error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid agent") {
		t.Fatalf("expected invalid agent error, got: %v", err)
	}
}

func TestInstallCommandsCodexInstallsSkill(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("USERPROFILE", tmpHome)

	cmd := newInstallCommandsCommand()

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--agent", "codex"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("install-commands --agent codex failed: %v", err)
	}

	skillPath := filepath.Join(tmpHome, ".agents", "skills", "claudio", "SKILL.md")
	content, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("failed to read Codex skill: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "name: claudio") {
		t.Error("skill should contain claudio name metadata")
	}
	if !strings.Contains(contentStr, "description:") {
		t.Error("skill should contain description metadata")
	}
	if !strings.Contains(contentStr, "claudio volume <0.0-1.0>") {
		t.Error("skill should document claudio volume command")
	}

	output := stdout.String()
	if !strings.Contains(output, "skill for codex") {
		t.Errorf("success message should mention Codex skill, got: %s", output)
	}
	if !strings.Contains(output, "$claudio") {
		t.Errorf("success message should show Codex invocation, got: %s", output)
	}
}

// TestInstallCommandsIdempotent verifies running twice doesn't cause errors
func TestInstallCommandsIdempotent(t *testing.T) {
	// Create a temporary directory to act as home
	tmpHome, err := os.MkdirTemp("", "claudio-test-home-*")
	if err != nil {
		t.Fatalf("failed to create temp home: %v", err)
	}
	defer os.RemoveAll(tmpHome)

	commandsDir := filepath.Join(tmpHome, ".claude", "commands")
	claudioMdPath := filepath.Join(commandsDir, "claudio.md")

	// First install
	err = installCommandsToPath(commandsDir, claudioMdPath)
	if err != nil {
		t.Fatalf("first installCommandsToPath failed: %v", err)
	}

	// Second install (should succeed without error)
	err = installCommandsToPath(commandsDir, claudioMdPath)
	if err != nil {
		t.Fatalf("second installCommandsToPath should be idempotent: %v", err)
	}

	// Verify file still exists and has correct content
	content, err := os.ReadFile(claudioMdPath)
	if err != nil {
		t.Fatalf("failed to read claudio.md after second install: %v", err)
	}

	if !strings.Contains(string(content), "allowed-tools: Bash(claudio:*)") {
		t.Error("content should still be correct after second install")
	}
}

func TestUninstallCommandsCreation(t *testing.T) {
	cmd := newUninstallCommandsCommand()

	if cmd == nil {
		t.Fatal("newUninstallCommandsCommand() returned nil")
	}

	if cmd.Use != "uninstall-commands" {
		t.Errorf("expected Use 'uninstall-commands', got %s", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("expected Short description to be set")
	}
}

func TestUninstallCommandsRejectsInvalidAgent(t *testing.T) {
	cmd := newUninstallCommandsCommand()

	cmd.SetArgs([]string{"--agent", "gemini"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected invalid agent error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid agent") {
		t.Fatalf("expected invalid agent error, got: %v", err)
	}
}

func TestUninstallCommandsRemovesClaudeSlashCommand(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("USERPROFILE", tmpHome)

	commandsDir := filepath.Join(tmpHome, ".claude", "commands")
	claudioMdPath := filepath.Join(commandsDir, "claudio.md")
	if err := installCommandsToPath(commandsDir, claudioMdPath); err != nil {
		t.Fatalf("installCommandsToPath failed: %v", err)
	}

	cmd := newUninstallCommandsCommand()

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("uninstall-commands failed: %v", err)
	}

	if _, err := os.Stat(claudioMdPath); !os.IsNotExist(err) {
		t.Fatalf("expected claudio.md to be removed, stat err: %v", err)
	}
	if !strings.Contains(stdout.String(), "Removed slash command for claude") {
		t.Errorf("expected removal output, got: %s", stdout.String())
	}
}

func TestUninstallCommandsRemovesCodexSkill(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("USERPROFILE", tmpHome)

	installCmd := newInstallCommandsCommand()
	var installStdout bytes.Buffer
	installCmd.SetOut(&installStdout)
	installCmd.SetArgs([]string{"--agent", "codex"})
	if err := installCmd.Execute(); err != nil {
		t.Fatalf("install-commands --agent codex failed: %v", err)
	}

	skillPath := filepath.Join(tmpHome, ".agents", "skills", "claudio", "SKILL.md")
	if _, err := os.Stat(skillPath); err != nil {
		t.Fatalf("expected Codex skill before uninstall, stat err: %v", err)
	}

	uninstallCmd := newUninstallCommandsCommand()

	var stdout bytes.Buffer
	uninstallCmd.SetOut(&stdout)
	uninstallCmd.SetArgs([]string{"--agent", "codex"})

	err := uninstallCmd.Execute()
	if err != nil {
		t.Fatalf("uninstall-commands --agent codex failed: %v", err)
	}

	if _, err := os.Stat(skillPath); !os.IsNotExist(err) {
		t.Fatalf("expected Codex skill to be removed, stat err: %v", err)
	}
	if !strings.Contains(stdout.String(), "Removed skill for codex") {
		t.Errorf("expected Codex removal output, got: %s", stdout.String())
	}
}

func TestUninstallCommandsMissingArtifactIsIdempotent(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("USERPROFILE", tmpHome)

	cmd := newUninstallCommandsCommand()

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--agent", "codex"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("uninstall-commands should tolerate missing artifact: %v", err)
	}
	if !strings.Contains(stdout.String(), "No skill for codex found") {
		t.Errorf("expected missing artifact output, got: %s", stdout.String())
	}
}
