package install

import (
	"reflect"
	"testing"
)

const newClaudio = "/new/claudio"

// TestInstallAgentHooksMergesIntoExistingEvent pins what install does to
// an event that already holds hooks: user commands survive (legacy strings
// become a matcher group, or a bare entry for Copilot), stale and duplicate
// claudio commands are replaced by exactly one fresh entry appended last,
// and everything outside the event is left alone. Every case must also be
// idempotent.
func TestInstallAgentHooksMergesIntoExistingEvent(t *testing.T) {
	claudioGroup := group(".*", cmd(newClaudio))
	copilotEntry := map[string]any{
		"type": "command", "command": newClaudio + " --hook-agent copilot", "timeoutSec": 30,
	}
	cases := []struct {
		name     string
		agent    Agent
		existing any
		want     []any
	}{
		{
			name:     "foreign legacy string becomes a .* group",
			agent:    AgentClaude,
			existing: "/usr/bin/other-tool",
			want:     []any{group(".*", cmd("/usr/bin/other-tool")), claudioGroup},
		},
		{
			name:     "foreign legacy string on Copilot becomes a bare entry",
			agent:    AgentCopilot,
			existing: "/usr/bin/other-tool",
			want:     []any{cmd("/usr/bin/other-tool"), copilotEntry},
		},
		{
			name:     "claudio legacy string is replaced, not duplicated",
			agent:    AgentClaude,
			existing: "claudio",
			want:     []any{claudioGroup},
		},
		{
			name:     "user group is kept before claudio",
			agent:    AgentClaude,
			existing: []any{group(".*", cmd("existing-command"))},
			want:     []any{group(".*", cmd("existing-command")), claudioGroup},
		},
		{
			name:     "stale claudio after a user group is refreshed",
			agent:    AgentClaude,
			existing: []any{group(".*", cmd("/usr/bin/logger")), group("*", cmd("/old/claudio"))},
			want:     []any{group(".*", cmd("/usr/bin/logger")), claudioGroup},
		},
		{
			name:     "stale claudio before a user group keeps the user group",
			agent:    AgentClaude,
			existing: []any{group(".*", cmd("/old/claudio")), group("custom-matcher", cmd("/usr/local/bin/custom"))},
			want:     []any{group("custom-matcher", cmd("/usr/local/bin/custom")), claudioGroup},
		},
		{
			name:  "duplicate claudio entries collapse to one",
			agent: AgentClaude,
			existing: []any{
				group(".*", cmd("/old/claudio")),
				group(".*", cmd("/old/claudio")),
				group("custom-matcher", cmd("/usr/local/bin/custom")),
			},
			want: []any{group("custom-matcher", cmd("/usr/local/bin/custom")), claudioGroup},
		},
		{
			name:     "claudio sibling in a user group is stripped, user command kept",
			agent:    AgentClaude,
			existing: []any{group(".*", cmd("/old/claudio"), cmd("/usr/local/bin/user-lint"))},
			want:     []any{group(".*", cmd("/usr/local/bin/user-lint")), claudioGroup},
		},
		{
			name:  "unrecognized entries and commands survive verbatim",
			agent: AgentClaude,
			existing: []any{
				"raw-entry",
				map[string]any{"matcher": "*"},
				group("mixed", "raw-hook", map[string]any{"command": float64(42)}, cmd("/old/claudio"), cmd("/usr/bin/logger")),
				group("claudio-only", cmd("/old/claudio")),
			},
			want: []any{
				"raw-entry",
				map[string]any{"matcher": "*"},
				group("mixed", "raw-hook", map[string]any{"command": float64(42)}, cmd("/usr/bin/logger")),
				claudioGroup,
			},
		},
		{
			name:     "stale flat Copilot command is refreshed next to a user command",
			agent:    AgentCopilot,
			existing: []any{cmd("/usr/bin/logger"), cmd("/old/claudio --hook-agent copilot")},
			want:     []any{cmd("/usr/bin/logger"), copilotEntry},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			const event = "PreToolUse"
			settings := SettingsMap{
				"version": "1.0",
				"config":  map[string]any{"nested": map[string]any{"deep": "preserved"}},
				"hooks":   map[string]any{event: tc.existing, "PreCommit": "git diff --check"},
			}
			got, err := InstallAgentHooks(&settings, tc.agent, newClaudio)
			if err != nil {
				t.Fatal(err)
			}

			// Everything else must match a fresh install into the same
			// settings without the event under test.
			base := SettingsMap{
				"version": "1.0",
				"config":  map[string]any{"nested": map[string]any{"deep": "preserved"}},
				"hooks":   map[string]any{"PreCommit": "git diff --check"},
			}
			want, err := InstallAgentHooks(&base, tc.agent, newClaudio)
			if err != nil {
				t.Fatal(err)
			}
			(*want)["hooks"].(map[string]any)[event] = tc.want
			if !reflect.DeepEqual(canonicalJSON(t, got), canonicalJSON(t, want)) {
				t.Errorf("settings:\n got %s\nwant %s", canonicalJSON(t, got), canonicalJSON(t, want))
			}

			again, err := InstallAgentHooks(got, tc.agent, newClaudio)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(again, got) {
				t.Errorf("reinstall changed settings:\n got %s\nwant %s", canonicalJSON(t, again), canonicalJSON(t, got))
			}
		})
	}
}

// TestInstallAgentHooksResultIsIndependentOfInput pins that the returned
// settings share no maps with the input.
func TestInstallAgentHooksResultIsIndependentOfInput(t *testing.T) {
	original := SettingsMap{
		"hooks":  map[string]any{"ExistingHook": "existing-command"},
		"config": map[string]any{"nested": map[string]any{"value": "original"}},
	}
	before := canonicalJSON(t, original)
	result, err := InstallAgentHooks(&original, AgentClaude, newClaudio)
	if err != nil {
		t.Fatal(err)
	}
	(*result)["hooks"].(map[string]any)["TestModification"] = "test-value"
	(*result)["config"].(map[string]any)["nested"].(map[string]any)["value"] = "changed"
	if after := canonicalJSON(t, original); string(after) != string(before) {
		t.Errorf("input changed through the result:\nbefore %s\nafter  %s", before, after)
	}
}
