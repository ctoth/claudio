package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"claudio.click/internal/install"
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
	cmd.SetArgs([]string{"--agent", "bogus", "--dry-run"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for invalid agent, got nil")
	}
}

func TestInstallCommandDefaultsToAutoGlobalScope(t *testing.T) {
	dir := t.TempDir()
	addFakeCliAgentBinary(t, dir, "claude")
	setIsolatedCliAgentEnv(t, dir, t.TempDir())

	cmd := newInstallCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--dry-run"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := out.String()
	if !strings.Contains(s, "global scope") {
		t.Errorf("expected global scope by default, got: %s", s)
	}
	if !strings.Contains(s, "Target agent: claude") {
		t.Errorf("expected auto-detected Claude target, got: %s", s)
	}
}

func TestInstallQuietDoesNotEmitInfoLogs(t *testing.T) {
	dir := t.TempDir()
	addFakeCliAgentBinary(t, dir, "claude")
	setIsolatedCliAgentEnv(t, dir, t.TempDir())

	cli := NewCLI()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := cli.Run(
		[]string{"claudio", "install", "--dry-run", "--quiet"},
		strings.NewReader(""),
		&stdout,
		&stderr,
	)
	if exitCode != 0 {
		t.Fatalf("exit code = %d; stderr=%q", exitCode, stderr.String())
	}
	if strings.Contains(stderr.String(), "INFO") {
		t.Fatalf("quiet install emitted info logs on stderr: %q", stderr.String())
	}
}

func TestInstallCommandGeminiDryRunUsesGeminiPath(t *testing.T) {
	cmd := newInstallCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--agent", "gemini", "--dry-run"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), filepath.Join(".gemini", "settings.json")) {
		t.Errorf("expected gemini settings path in output, got: %s", out.String())
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
	cmd.SetArgs([]string{"--agent", "claude", "--dry-run"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "settings.json") {
		t.Errorf("expected claude settings.json path by default, got: %s", out.String())
	}
}

func addFakeCliAgentBinary(t *testing.T, dir string, name string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(""), 0755); err != nil {
		t.Fatal(err)
	}
}

func setIsolatedCliAgentEnv(t *testing.T, pathDir string, home string) {
	t.Helper()
	t.Setenv("PATH", pathDir)
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")
	t.Setenv("CODEX_HOME", "")
}

// TestInstallVerifyHonorsDefaultEnabledFalse covers finding #82: the
// verify step previously iterated agent.HookNames() (the full registry)
// while the write step only wrote agent.EnabledHooks() (default-enabled
// subset). A DefaultEnabled=false hook would fail verify even though
// the write deliberately skipped it. The fix aligns both to
// EnabledHooks; this test confirms install succeeds when a hook is
// flagged DefaultEnabled=false.
func TestInstallVerifyHonorsDefaultEnabledFalse(t *testing.T) {
	// The verify path calls IsClaudioHook which checks the command
	// basename against executableRecognizer. Under `go test` the
	// executable is `<pkg>.test[.exe]`, not `claudio[.exe]`; the
	// recognizer opts in to .test/.test.exe when this env var is set.
	t.Setenv("CLAUDIO_TEST_RECOGNIZE_GO_TEST", "1")

	// Save and restore AllHooks. We mutate the package-level registry
	// for the duration of this test.
	prev := install.AllHooks
	defer func() { install.AllHooks = prev }()

	// Build a registry where one hook is DefaultEnabled=false. The write
	// step will skip it; the verify step must also skip it.
	modified := make([]install.HookDefinition, len(prev))
	copy(modified, prev)
	// Flip the first hook to disabled.
	if len(modified) == 0 {
		t.Fatal("install.AllHooks unexpectedly empty")
	}
	modified[0].DefaultEnabled = false
	install.AllHooks = modified

	// Point the install at a tempdir-scoped settings file.
	tempDir := t.TempDir()
	settingsPath := filepath.Join(tempDir, "settings.json")
	if err := os.WriteFile(settingsPath, []byte("{}"), 0644); err != nil {
		t.Fatalf("write seed settings: %v", err)
	}

	err := runInstallWorkflow(install.AgentClaude, "user", settingsPath)
	if err != nil {
		t.Fatalf("install workflow failed when one hook is DefaultEnabled=false (verify should skip it): %v", err)
	}

	// Confirm the disabled hook is NOT in the written settings.
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	if strings.Contains(string(data), modified[0].Name) {
		// The merger may still mention the disabled hook if it was
		// previously present; we wrote {} so it should be absent.
		t.Errorf("disabled hook %q unexpectedly written to settings: %s", modified[0].Name, string(data))
	}
}

func TestRunInstallWorkflowCreatesMissingSettingsDir(t *testing.T) {
	t.Setenv("CLAUDIO_TEST_RECOGNIZE_GO_TEST", "1")

	tempDir := t.TempDir()
	settingsPath := filepath.Join(tempDir, "missing", ".claude", "settings.json")

	err := runInstallWorkflow(install.AgentClaude, install.ScopeGlobal, settingsPath)
	if err != nil {
		t.Fatalf("install workflow with missing settings dir failed: %v", err)
	}
	if _, err := os.Stat(settingsPath); err != nil {
		t.Fatalf("expected settings file to be created: %v", err)
	}
}
