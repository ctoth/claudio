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
			paths, err := agentConfigPaths(agent, ScopeGlobal)
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

	if _, err := ResolveAgentTargets(AgentAll, ScopeGlobal); err == nil {
		t.Fatal("expected error resolving global targets without a home directory")
	}
	if _, err := os.Stat(filepath.Join(".", "~")); !os.IsNotExist(err) {
		t.Fatalf("a literal ~ directory exists in the cwd (err=%v)", err)
	}
}
