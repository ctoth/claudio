package install

import (
	"path/filepath"
	"testing"
)

// TestAgentHomeEnvIsTrimmedForEveryAgent pins that every per-agent
// config-home variable is whitespace-trimmed the same way: blank means
// unset, and surrounding spaces are ignored.
func TestAgentHomeEnvIsTrimmedForEveryAgent(t *testing.T) {
	cases := []struct {
		agent  Agent
		envVar string
		file   string
		dir    string
	}{
		{AgentClaude, "CLAUDE_CONFIG_DIR", "settings.json", ".claude"},
		{AgentCodex, "CODEX_HOME", "hooks.json", ".codex"},
		{AgentCopilot, "COPILOT_HOME", "settings.json", ".copilot"},
	}
	for _, tc := range cases {
		t.Run(string(tc.agent), func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("USERPROFILE", home)

			t.Setenv(tc.envVar, "   ")
			paths, err := tc.agent.ConfigPaths(ScopeGlobal)
			if err != nil {
				t.Fatal(err)
			}
			if want := filepath.Join(home, tc.dir, tc.file); len(paths) != 1 || paths[0] != want {
				t.Fatalf("blank %s: paths = %v, want [%s]", tc.envVar, paths, want)
			}

			custom := t.TempDir()
			t.Setenv(tc.envVar, "  "+custom+"  ")
			paths, err = tc.agent.ConfigPaths(ScopeGlobal)
			if err != nil {
				t.Fatal(err)
			}
			if want := filepath.Join(custom, tc.file); len(paths) != 1 || paths[0] != want {
				t.Fatalf("padded %s: paths = %v, want [%s]", tc.envVar, paths, want)
			}
		})
	}
}

func mustSpec(t *testing.T, a Agent) agentSpec {
	t.Helper()
	s, ok := a.spec()
	if !ok {
		t.Fatalf("no spec for agent %q", a)
	}
	return s
}

// TestAgentSpecsCoverEveryConcreteAgent pins that the descriptor table is
// the single source for agent parsing and path resolution.
func TestAgentSpecsCoverEveryConcreteAgent(t *testing.T) {
	for _, a := range []Agent{AgentClaude, AgentCodex, AgentGemini, AgentQwen, AgentCopilot} {
		s := mustSpec(t, a)
		if len(*s.registry) == 0 || s.homeDir == "" || s.globalFile == "" || len(s.projectPaths) == 0 {
			t.Errorf("incomplete spec for %s: %+v", a, s)
		}
		if parsed, err := ParseAgent(string(a)); err != nil || parsed != a {
			t.Errorf("ParseAgent(%q) = %q, %v", a, parsed, err)
		}
	}
	for _, a := range []Agent{AgentAuto, AgentAll, Agent("bogus")} {
		if _, err := a.ConfigPaths(ScopeGlobal); err == nil {
			t.Errorf("ConfigPaths(%q) should error", a)
		}
	}
}
