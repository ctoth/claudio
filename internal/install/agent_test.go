package install

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestParseAgentValid(t *testing.T) {
	cases := map[string]Agent{
		"auto":    AgentAuto,
		"all":     AgentAll,
		"claude":  AgentClaude,
		"codex":   AgentCodex,
		"gemini":  AgentGemini,
		"qwen":    AgentQwen,
		"copilot": AgentCopilot,
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
	if _, err := ParseAgent("bogus"); err == nil {
		t.Error("expected error for invalid agent, got nil")
	}
}

func TestConcreteAgents(t *testing.T) {
	got := ConcreteAgents()
	want := []Agent{AgentClaude, AgentCodex, AgentGemini, AgentQwen, AgentCopilot}
	if len(got) != len(want) {
		t.Fatalf("ConcreteAgents() length = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("ConcreteAgents()[%d] = %v, want %v", i, got[i], want[i])
		}
		if !got[i].IsConcrete() {
			t.Errorf("%v should be concrete", got[i])
		}
	}
	if AgentAuto.IsConcrete() {
		t.Error("auto should not be concrete")
	}
	if AgentAll.IsConcrete() {
		t.Error("all should not be concrete")
	}
}

func TestNormalizeScopePublicAndLegacy(t *testing.T) {
	cases := map[string]string{
		"global":  ScopeGlobal,
		"project": ScopeProject,
		"user":    ScopeGlobal,
	}
	for in, want := range cases {
		got, err := NormalizeScope(in)
		if err != nil {
			t.Fatalf("NormalizeScope(%q) returned error: %v", in, err)
		}
		if got != want {
			t.Errorf("NormalizeScope(%q) = %q, want %q", in, got, want)
		}
	}
	if _, err := NormalizeScope("invalid"); err == nil {
		t.Error("expected error for invalid scope")
	}
}

func TestAgentMatcher(t *testing.T) {
	if AgentClaude.Matcher() != ".*" {
		t.Errorf("claude matcher = %q, want .*", AgentClaude.Matcher())
	}
	if AgentCodex.Matcher() != "*" {
		t.Errorf("codex matcher = %q, want *", AgentCodex.Matcher())
	}
	if AgentGemini.Matcher() != "" {
		t.Errorf("gemini matcher = %q, want empty matcher", AgentGemini.Matcher())
	}
	if AgentQwen.Matcher() != ".*" {
		t.Errorf("qwen matcher = %q, want .*", AgentQwen.Matcher())
	}
	if AgentCopilot.Matcher() != "" {
		t.Errorf("copilot matcher = %q, want empty matcher", AgentCopilot.Matcher())
	}
}

func TestAgentString(t *testing.T) {
	if AgentCodex.String() != "codex" {
		t.Errorf("got %q", AgentCodex.String())
	}
}

func TestAgentRegistryAndHookNames(t *testing.T) {
	if len(AgentClaude.Registry()) == 0 {
		t.Error("claude registry should not be empty")
	}
	if len(AgentCodex.Registry()) == 0 {
		t.Error("codex registry should not be empty")
	}
	if len(AgentGemini.Registry()) == 0 {
		t.Error("gemini registry should not be empty")
	}
	if len(AgentQwen.Registry()) == 0 {
		t.Error("qwen registry should not be empty")
	}
	if len(AgentCopilot.Registry()) == 0 {
		t.Error("copilot registry should not be empty")
	}
	if AgentAuto.Registry() != nil {
		t.Error("auto should not have a concrete registry")
	}
	if Agent("bogus").Registry() != nil {
		t.Error("invalid agent should not have a registry")
	}
	if len(AgentGemini.EnabledHooks()) == 0 {
		t.Error("gemini should have default-enabled hooks")
	}
	if len(AgentGemini.HookNames()) != len(AgentGemini.Registry()) {
		t.Error("gemini hook names should match registry length")
	}
	if len(AgentQwen.HookNames()) != len(AgentQwen.Registry()) {
		t.Error("qwen hook names should match registry length")
	}
	if len(AgentCopilot.HookNames()) != len(AgentCopilot.Registry()) {
		t.Error("copilot hook names should match registry length")
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
	gp, err := AgentGemini.BestConfigPath("project")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gp, filepath.Join(".gemini", "settings.json")) {
		t.Errorf("gemini project path %q missing .gemini/settings.json", gp)
	}
	qp, err := AgentQwen.BestConfigPath("project")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(qp, filepath.Join(".qwen", "settings.json")) {
		t.Errorf("qwen project path %q missing .qwen/settings.json", qp)
	}
	copilotPath, err := AgentCopilot.BestConfigPath("project")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(copilotPath, filepath.Join(".github", "copilot", "settings.local.json")) {
		t.Errorf("copilot project path %q missing .github/copilot/settings.local.json", copilotPath)
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

func TestAgentBestConfigPathRejectsUnresolvedAgents(t *testing.T) {
	if _, err := AgentAuto.BestConfigPath(ScopeGlobal); err == nil {
		t.Error("expected auto to be resolved before selecting a config path")
	}
	if _, err := AgentAll.BestConfigPath(ScopeGlobal); err == nil {
		t.Error("expected all to be resolved before selecting a config path")
	}
	if _, err := Agent("bogus").BestConfigPath(ScopeGlobal); err == nil {
		t.Error("expected invalid agent error")
	}
}
