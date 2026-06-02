package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestUninstallCommandRejectsInvalidAgent(t *testing.T) {
	cmd := newUninstallCommand()
	cmd.SetArgs([]string{"--agent", "bogus", "--dry-run"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for invalid agent, got nil")
	}
}

func TestUninstallCommandDefaultsToAutoGlobalScope(t *testing.T) {
	dir := t.TempDir()
	addFakeUninstallAgentBinary(t, dir, "claude")
	setIsolatedUninstallAgentEnv(t, dir, t.TempDir())

	cmd := newUninstallCommand()
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

func TestUninstallCommandGeminiDryRunUsesGeminiPath(t *testing.T) {
	cmd := newUninstallCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--agent", "gemini", "--dry-run"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), filepath.Join(".gemini", "settings.json")) {
		t.Errorf("expected gemini settings path, got: %s", out.String())
	}
}

func TestUninstallCommandCodexDryRunUsesCodexPath(t *testing.T) {
	cmd := newUninstallCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--agent", "codex", "--dry-run"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "hooks.json") {
		t.Errorf("expected codex hooks.json path, got: %s", out.String())
	}
}

func TestUninstallCommandDefaultsToClaude(t *testing.T) {
	cmd := newUninstallCommand()
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

func addFakeUninstallAgentBinary(t *testing.T, dir string, name string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(""), 0755); err != nil {
		t.Fatal(err)
	}
}

func setIsolatedUninstallAgentEnv(t *testing.T, pathDir string, home string) {
	t.Helper()
	t.Setenv("PATH", pathDir)
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")
	t.Setenv("CODEX_HOME", "")
}
