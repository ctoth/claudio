package install

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestParseAgentValid(t *testing.T) {
	cases := map[string]Agent{
		"claude": AgentClaude,
		"codex":  AgentCodex,
	}
	for in, want := range cases {
		got, err := ParseAgent(in)
		if err != nil {
			t.Fatalf("ParseAgent(%q) returned error: %v", in, err)
		}
		if got != want {
			t.Errorf("ParseAgent(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestParseAgentInvalid(t *testing.T) {
	if _, err := ParseAgent("gemini"); err == nil {
		t.Error("expected error for invalid agent, got nil")
	}
}

func TestAgentMatcher(t *testing.T) {
	if AgentClaude.Matcher() != ".*" {
		t.Errorf("claude matcher = %q, want .*", AgentClaude.Matcher())
	}
	if AgentCodex.Matcher() != "*" {
		t.Errorf("codex matcher = %q, want *", AgentCodex.Matcher())
	}
}

func TestAgentString(t *testing.T) {
	if AgentCodex.String() != "codex" {
		t.Errorf("got %q", AgentCodex.String())
	}
}

func TestAgentBestConfigPathProjectScope(t *testing.T) {
	p, err := AgentCodex.BestConfigPath("project")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(p, filepath.Join(".codex", "hooks.json")) {
		t.Errorf("codex project path %q missing .codex/hooks.json", p)
	}
	cp, err := AgentClaude.BestConfigPath("project")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(cp, filepath.Join(".claude", "settings.json")) {
		t.Errorf("claude project path %q missing .claude/settings.json", cp)
	}
}

func TestAgentBestConfigPathInvalidScope(t *testing.T) {
	if _, err := AgentCodex.BestConfigPath("bogus"); err == nil {
		t.Error("expected error for invalid codex scope")
	}
	if _, err := AgentClaude.BestConfigPath("bogus"); err == nil {
		t.Error("expected error for invalid claude scope")
	}
}
