package install

import (
	"os"
	"path/filepath"
	"testing"
)

// clearHomeEnv removes every variable the home lookup consults, including
// the per-agent config-home overrides.
func clearHomeEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{"HOME", "USERPROFILE", "HOMEDRIVE", "HOMEPATH", "CLAUDE_CONFIG_DIR", "CODEX_HOME", "COPILOT_HOME"} {
		t.Setenv(key, "")
	}
}

// TestGlobalPathsErrorWhenHomeMissing pins that global scope with no home
// directory is an error, never a literal "~" path relative to the cwd.
func TestGlobalPathsErrorWhenHomeMissing(t *testing.T) {
	clearHomeEnv(t)
	for _, agent := range ConcreteAgents() {
		t.Run(string(agent), func(t *testing.T) {
			paths, err := agent.ConfigPaths(ScopeGlobal)
			if err == nil {
				t.Fatalf("expected error without a home directory, got paths %v", paths)
			}
			if _, err := agent.BestConfigPath(ScopeGlobal); err == nil {
				t.Fatal("BestConfigPath: expected error without a home directory")
			}
		})
	}
}

// TestResolveAgentTargetsWithoutHomeCreatesNoTildeDir pins that resolving
// global targets without a home fails and leaves no "~" directory behind.
func TestResolveAgentTargetsWithoutHomeCreatesNoTildeDir(t *testing.T) {
	clearHomeEnv(t)
	t.Chdir(t.TempDir())

	for _, agent := range []Agent{AgentAll, AgentClaude} {
		if _, err := ResolveAgentTargets(agent, ScopeGlobal); err == nil {
			t.Fatalf("expected error resolving global %s targets without a home directory", agent)
		}
	}
	if _, err := os.Stat(filepath.Join(".", "~")); !os.IsNotExist(err) {
		t.Fatalf("a literal ~ directory exists in the cwd (err=%v)", err)
	}
}

func TestHomeDirAndClaudeConfigDir(t *testing.T) {
	clearHomeEnv(t)
	if _, err := HomeDir(); err == nil {
		t.Error("HomeDir: expected error without a home directory")
	}
	if _, err := ClaudeConfigDir(); err == nil {
		t.Error("ClaudeConfigDir: expected error without a home directory")
	}

	custom := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", " "+custom+" ")
	if got, err := ClaudeConfigDir(); err != nil || got != custom {
		t.Errorf("ClaudeConfigDir() = %q, %v; want %q (trimmed CLAUDE_CONFIG_DIR)", got, err, custom)
	}

	home := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	if got, err := HomeDir(); err != nil || got != home {
		t.Errorf("HomeDir() = %q, %v; want %q", got, err, home)
	}
	if got, err := ClaudeConfigDir(); err != nil || got != filepath.Join(home, ".claude") {
		t.Errorf("ClaudeConfigDir() = %q, %v; want %q", got, err, filepath.Join(home, ".claude"))
	}
}
