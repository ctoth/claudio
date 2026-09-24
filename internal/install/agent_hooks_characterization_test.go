package install

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

var updateGolden = flag.Bool("update-golden", false, "rewrite install/uninstall characterization golden files")

// characterizationCase is one starting settings file installed into and
// then uninstalled from, for every agent.
type characterizationCase struct {
	name string
	exe  string
	// settings returns the starting settings for the agent.
	settings func(agent Agent) SettingsMap
	// userOnly returns what uninstall must leave behind: the starting
	// settings minus every claudio command.
	userOnly func(agent Agent) SettingsMap
}

// agentCommand returns one command entry in the agent's hook layout: a bare
// command for Copilot, a matcher group everywhere else.
func agentCommand(agent Agent, command string) map[string]any {
	entry := map[string]any{"type": "command", "command": command}
	if agent == AgentCopilot {
		return entry
	}
	return map[string]any{"matcher": "Bash", "hooks": []any{entry}}
}

func firstEvent(agent Agent) string {
	return agent.EnabledHooks()[0].Name
}

var characterizationCases = []characterizationCase{
	{
		name:     "fresh",
		exe:      "/usr/local/bin/claudio",
		settings: func(Agent) SettingsMap { return SettingsMap{"theme": "dark"} },
		userOnly: func(Agent) SettingsMap { return SettingsMap{"theme": "dark"} },
	},
	{
		name: "user-and-stale",
		exe:  "C:/Program Files/claudio/claudio.exe",
		settings: func(agent Agent) SettingsMap {
			return SettingsMap{"theme": "dark", "hooks": map[string]any{
				firstEvent(agent): []any{
					agentCommand(agent, "echo user"),
					agentCommand(agent, "/old/bin/claudio --hook-agent "+string(agent)),
				},
			}}
		},
		userOnly: func(agent Agent) SettingsMap {
			return SettingsMap{"theme": "dark", "hooks": map[string]any{
				firstEvent(agent): []any{agentCommand(agent, "echo user")},
			}}
		},
	},
}

// canonicalJSON returns v as indented JSON with sorted keys, so it can be
// compared byte-for-byte with a golden file.
func canonicalJSON(t *testing.T, v any) []byte {
	t.Helper()
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return append(data, '\n')
}

// TestAgentHooksCharacterization pins, per agent, the exact JSON install
// writes, that reinstalling changes nothing, and that uninstall returns
// the starting settings minus claudio together with the removed events.
func TestAgentHooksCharacterization(t *testing.T) {
	for _, agent := range ConcreteAgents() {
		for _, tc := range characterizationCases {
			t.Run(string(agent)+"/"+tc.name, func(t *testing.T) {
				start := tc.settings(agent)
				pristine := tc.settings(agent)

				installed, err := InstallAgentHooks(&start, agent, tc.exe)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(start, pristine) {
					t.Fatalf("InstallAgentHooks mutated its input: %v", start)
				}

				reinstalled, err := InstallAgentHooks(installed, agent, tc.exe)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(reinstalled, installed) {
					t.Errorf("reinstall changed settings:\n got %s\nwant %s",
						canonicalJSON(t, reinstalled), canonicalJSON(t, installed))
				}

				removed, names, err := RemoveAgentHooks(installed, agent)
				if err != nil {
					t.Fatal(err)
				}
				if want := tc.userOnly(agent); !reflect.DeepEqual(*removed, want) {
					t.Errorf("after uninstall:\n got %s\nwant %s", canonicalJSON(t, removed), canonicalJSON(t, want))
				}
				if want := ClaudioHookNames(installed); !reflect.DeepEqual(names, want) || len(names) != len(agent.EnabledHooks()) {
					t.Errorf("removed names = %v, want %v (%d events)", names, want, len(agent.EnabledHooks()))
				}
				if len(ClaudioHookNames(installed)) == 0 {
					t.Error("RemoveAgentHooks mutated its input")
				}

				got := canonicalJSON(t, map[string]any{
					"installed":    installed,
					"removed":      removed,
					"removedNames": names,
				})
				golden := filepath.Join("testdata", "characterization", string(agent)+"-"+tc.name+".json")
				if *updateGolden {
					if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(golden, got, 0o644); err != nil {
						t.Fatal(err)
					}
				}
				want, err := os.ReadFile(golden)
				if err != nil {
					t.Fatalf("read golden (run with -update-golden to create): %v", err)
				}
				want = bytes.ReplaceAll(want, []byte("\r\n"), []byte("\n"))
				if !bytes.Equal(got, want) {
					t.Errorf("%s mismatch:\n got %s\nwant %s", golden, got, want)
				}
			})
		}
	}
}
