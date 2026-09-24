package install

import (
	"reflect"
	"testing"
)

func TestInstallAgentHooksEveryAgentAddsClaudioWithoutMutatingInput(t *testing.T) {
	for _, agent := range ConcreteAgents() {
		t.Run(string(agent), func(t *testing.T) {
			settings := SettingsMap{"theme": "dark"}
			before := SettingsMap{"theme": "dark"}
			got, err := InstallAgentHooks(&settings, agent, "/usr/local/bin/claudio")
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(settings, before) {
				t.Errorf("input settings mutated: %v", settings)
			}
			hooks, ok := (*got)["hooks"].(map[string]interface{})
			if !ok || len(hooks) != len(agent.EnabledHooks()) {
				t.Fatalf("installed hooks = %v, want %d events", (*got)["hooks"], len(agent.EnabledHooks()))
			}
			for name, value := range hooks {
				if !IsClaudioHook(value) {
					t.Errorf("hook %s is not a claudio hook: %v", name, value)
				}
			}
		})
	}
}

func TestInstallAgentHooksRejectsNonConcreteAgent(t *testing.T) {
	for _, agent := range []Agent{AgentAuto, AgentAll, Agent("bogus")} {
		if _, err := InstallAgentHooks(&SettingsMap{}, agent, "/usr/local/bin/claudio"); err == nil {
			t.Errorf("InstallAgentHooks(%q) should error", agent)
		}
		if _, err := GenerateClaudioHooksForAgent("/usr/local/bin/claudio", agent); err == nil {
			t.Errorf("GenerateClaudioHooksForAgent(%q) should error", agent)
		}
	}
}

func TestInstallAgentHooksReportsMergeErrors(t *testing.T) {
	settings := SettingsMap{"hooks": "not-a-map"}
	if _, err := InstallAgentHooks(&settings, AgentClaude, "/usr/local/bin/claudio"); err == nil {
		t.Fatal("expected merge error for a non-map hooks section")
	}
}

func TestAgentSpecAccessors(t *testing.T) {
	if !AgentCodex.UsesCaptainHook() || AgentClaude.UsesCaptainHook() {
		t.Error("only codex should use captain-hook")
	}
	if AgentCodex.TrustHint() == "" || AgentClaude.TrustHint() != "" {
		t.Error("only codex should have a trust hint")
	}
	if got := AgentAuto.Matcher(); got != ".*" {
		t.Errorf("non-concrete matcher = %q, want .*", got)
	}
	if got := Agent("bogus").Registry(); got != nil {
		t.Errorf("unknown agent registry = %v, want nil", got)
	}
}
