package cli

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestUninstallCommandRejectsInvalidAgent(t *testing.T) {
	cmd := newUninstallCommand()
	cmd.SetArgs([]string{"--agent", "gemini", "--dry-run"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for invalid agent, got nil")
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
	cmd.SetArgs([]string{"--dry-run"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "settings.json") {
		t.Errorf("expected claude settings.json path by default, got: %s", out.String())
	}
}
