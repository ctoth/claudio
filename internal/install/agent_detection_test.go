package install

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestResolveAgentTargetsAutoDetectsPathBinaries(t *testing.T) {
	dir := t.TempDir()
	addFakeAgentBinary(t, dir, "claude")
	addFakeAgentBinary(t, dir, "codex")
	setIsolatedAgentEnv(t, dir, dir)

	targets, err := ResolveAgentTargets(AgentAuto, ScopeGlobal)
	if err != nil {
		t.Fatalf("ResolveAgentTargets returned error: %v", err)
	}

	agents := targetAgents(targets)
	if !agents[AgentClaude] {
		t.Error("auto detection should include Claude when claude is on PATH")
	}
	if !agents[AgentCodex] {
		t.Error("auto detection should include Codex when codex is on PATH")
	}
	if agents[AgentGemini] {
		t.Error("auto detection should not include Gemini without evidence")
	}
	if agents[AgentQwen] {
		t.Error("auto detection should not include Qwen without evidence")
	}
}

func TestResolveAgentTargetsAutoDetectsGlobalConfigDir(t *testing.T) {
	home := t.TempDir()
	setIsolatedAgentEnv(t, t.TempDir(), home)
	if err := os.MkdirAll(filepath.Join(home, ".gemini"), 0755); err != nil {
		t.Fatal(err)
	}

	targets, err := ResolveAgentTargets(AgentAuto, ScopeGlobal)
	if err != nil {
		t.Fatalf("ResolveAgentTargets returned error: %v", err)
	}

	agents := targetAgents(targets)
	if !agents[AgentGemini] {
		t.Error("auto detection should include Gemini when ~/.gemini exists")
	}
}

func TestResolveAgentTargetsAutoDetectsQwenGlobalConfigDir(t *testing.T) {
	home := t.TempDir()
	setIsolatedAgentEnv(t, t.TempDir(), home)
	if err := os.MkdirAll(filepath.Join(home, ".qwen"), 0755); err != nil {
		t.Fatal(err)
	}

	targets, err := ResolveAgentTargets(AgentAuto, ScopeGlobal)
	if err != nil {
		t.Fatalf("ResolveAgentTargets returned error: %v", err)
	}

	agents := targetAgents(targets)
	if !agents[AgentQwen] {
		t.Error("auto detection should include Qwen when ~/.qwen exists")
	}
}

func TestResolveAgentTargetsAutoUsesGlobalEvidenceForProjectScope(t *testing.T) {
	dir := t.TempDir()
	addFakeAgentBinary(t, dir, "claude")
	setIsolatedAgentEnv(t, dir, t.TempDir())

	targets, err := ResolveAgentTargets(AgentAuto, ScopeProject)
	if err != nil {
		t.Fatalf("ResolveAgentTargets returned error: %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("target count = %d, want 1", len(targets))
	}
	if targets[0].Agent != AgentClaude {
		t.Errorf("agent = %v, want claude", targets[0].Agent)
	}
	if filepath.Base(filepath.Dir(targets[0].ConfigPath)) != ".claude" {
		t.Errorf("project target path should point to .claude settings, got %q", targets[0].ConfigPath)
	}
}

func TestResolveAgentTargetsAutoNoDetection(t *testing.T) {
	setIsolatedAgentEnv(t, t.TempDir(), t.TempDir())

	_, err := ResolveAgentTargets(AgentAuto, ScopeGlobal)
	if err == nil {
		t.Fatal("expected no-agents-detected error")
	}
	if err.Error() != NoSupportedAgentsDetectedMessage {
		t.Errorf("error = %q, want %q", err.Error(), NoSupportedAgentsDetectedMessage)
	}
}

func TestResolveAgentTargetsAllBypassesDetection(t *testing.T) {
	setIsolatedAgentEnv(t, t.TempDir(), t.TempDir())

	targets, err := ResolveAgentTargets(AgentAll, ScopeGlobal)
	if err != nil {
		t.Fatalf("ResolveAgentTargets returned error: %v", err)
	}
	if len(targets) != len(ConcreteAgents()) {
		t.Fatalf("target count = %d, want %d", len(targets), len(ConcreteAgents()))
	}

	agents := targetAgents(targets)
	for _, agent := range ConcreteAgents() {
		if !agents[agent] {
			t.Errorf("all target missing %s", agent)
		}
	}
}

func TestResolveAgentTargetsConcreteAgentAndInvalidAgent(t *testing.T) {
	home := t.TempDir()
	setIsolatedAgentEnv(t, t.TempDir(), home)

	targets, err := ResolveAgentTargets(AgentGemini, ScopeGlobal)
	if err != nil {
		t.Fatalf("ResolveAgentTargets concrete agent returned error: %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("target count = %d, want 1", len(targets))
	}
	wantPath := filepath.Join(home, ".gemini", "settings.json")
	if targets[0].Agent != AgentGemini || targets[0].ConfigPath != wantPath {
		t.Fatalf("target = %+v, want gemini at %q", targets[0], wantPath)
	}

	if _, err := ResolveAgentTargets(Agent("bogus"), ScopeGlobal); err == nil {
		t.Fatal("expected invalid agent error")
	}
}

func TestResolveAgentTargetsAutoDetectsExistingClaudioHooks(t *testing.T) {
	home := t.TempDir()
	setIsolatedAgentEnv(t, t.TempDir(), home)

	settingsPath := filepath.Join(home, ".codex", "hooks.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		t.Fatal(err)
	}
	content := []byte(`{"hooks":{"PreToolUse":[{"matcher":"*","hooks":[{"type":"command","command":"/usr/local/bin/claudio"}]}]}}`)
	if err := os.WriteFile(settingsPath, content, 0644); err != nil {
		t.Fatal(err)
	}

	targets, err := ResolveAgentTargets(AgentAuto, ScopeGlobal)
	if err != nil {
		t.Fatalf("ResolveAgentTargets returned error: %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("target count = %d, want 1: %+v", len(targets), targets)
	}
	if targets[0].Agent != AgentCodex {
		t.Fatalf("agent = %v, want codex", targets[0].Agent)
	}
}

func TestResolveAgentTargetsAutoIgnoresUnreadableHookEvidence(t *testing.T) {
	home := t.TempDir()
	setIsolatedAgentEnv(t, t.TempDir(), home)

	settingsPath := filepath.Join(home, ".gemini", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settingsPath, []byte(`{bad json`), 0644); err != nil {
		t.Fatal(err)
	}

	targets, err := ResolveAgentTargets(AgentAuto, ScopeGlobal)
	if err != nil {
		t.Fatalf("config directory evidence should still resolve Gemini: %v", err)
	}
	if len(targets) != 1 || targets[0].Agent != AgentGemini {
		t.Fatalf("targets = %+v, want only gemini", targets)
	}
}

func TestSettingsContainClaudioHooks(t *testing.T) {
	cases := []struct {
		name     string
		settings *SettingsMap
		want     bool
	}{
		{name: "nil settings", settings: nil, want: false},
		{name: "missing hooks", settings: &SettingsMap{}, want: false},
		{name: "hooks not map", settings: &SettingsMap{"hooks": []interface{}{}}, want: false},
		{
			name: "non claudio hook",
			settings: &SettingsMap{
				"hooks": map[string]interface{}{
					"PreToolUse": []interface{}{
						map[string]interface{}{
							"matcher": "*",
							"hooks": []interface{}{
								map[string]interface{}{"command": "/usr/bin/logger"},
							},
						},
					},
				},
			},
			want: false,
		},
		{
			name: "legacy claudio string hook",
			settings: &SettingsMap{
				"hooks": map[string]interface{}{
					"PreToolUse": "/usr/local/bin/claudio",
				},
			},
			want: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := settingsContainClaudioHooks(tc.settings); got != tc.want {
				t.Fatalf("settingsContainClaudioHooks() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestAgentDetectionPrivateHelpersRejectInvalidInputs(t *testing.T) {
	if pathExists("") {
		t.Fatal("empty path should not exist")
	}
	if hasAgentConfigEvidence(AgentAuto, ScopeGlobal) {
		t.Fatal("auto is not a concrete agent and should not have config evidence")
	}
	if hasExistingClaudioHooks(AgentAuto, ScopeGlobal) {
		t.Fatal("auto is not a concrete agent and should not have hook evidence")
	}
	if _, err := agentConfigPaths(AgentAuto, ScopeGlobal); err == nil {
		t.Fatal("expected invalid concrete agent error for auto")
	}
}

func TestHasExistingClaudioHooksHandlesMissingUnreadableAndNonClaudioSettings(t *testing.T) {
	home := t.TempDir()
	setIsolatedAgentEnv(t, t.TempDir(), home)

	if hasExistingClaudioHooks(AgentClaude, ScopeGlobal) {
		t.Fatal("missing settings file should not count as Claudio hook evidence")
	}

	settingsPath := filepath.Join(home, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settingsPath, []byte(`{bad json`), 0644); err != nil {
		t.Fatal(err)
	}
	if hasExistingClaudioHooks(AgentClaude, ScopeGlobal) {
		t.Fatal("unreadable settings should not count as Claudio hook evidence")
	}

	if err := os.WriteFile(settingsPath, []byte(`{"hooks":{"PreToolUse":"/usr/bin/logger"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	if hasExistingClaudioHooks(AgentClaude, ScopeGlobal) {
		t.Fatal("non-Claudio hooks should not count as Claudio hook evidence")
	}

	if err := os.WriteFile(settingsPath, []byte(`{"hooks":{"PreToolUse":"/usr/local/bin/claudio"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	if !hasExistingClaudioHooks(AgentClaude, ScopeGlobal) {
		t.Fatal("Claudio hook should count as hook evidence")
	}
}

func addFakeAgentBinary(t *testing.T, dir string, name string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(""), 0755); err != nil {
		t.Fatal(err)
	}
}

func setIsolatedAgentEnv(t *testing.T, pathDir string, home string) {
	t.Helper()
	t.Setenv("PATH", pathDir)
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")
	t.Setenv("CODEX_HOME", "")
}

func targetAgents(targets []AgentTarget) map[Agent]bool {
	agents := map[Agent]bool{}
	for _, target := range targets {
		agents[target.Agent] = true
	}
	return agents
}
