package install

import (
	"reflect"
	"testing"
)

// cmd returns a command entry.
func cmd(command string) map[string]any {
	return map[string]any{"type": "command", "command": command}
}

// group returns a matcher group holding entries; an empty matcher is omitted.
func group(matcher string, entries ...any) map[string]any {
	g := map[string]any{"hooks": append([]any{}, entries...)}
	if matcher != "" {
		g["matcher"] = matcher
	}
	return g
}

// TestRemoveAgentHooksShapes pins what uninstall removes and keeps across
// every hook layout: legacy strings, matcher groups, flat commands, mixed
// groups and user entries that must survive verbatim.
func TestRemoveAgentHooksShapes(t *testing.T) {
	cases := []struct {
		name      string
		input     SettingsMap
		want      SettingsMap
		wantNames []string
	}{
		{
			name: "legacy strings with any claudio path are removed, others kept",
			input: SettingsMap{"version": "1.0", "hooks": map[string]any{
				"PreToolUse":       "claudio",
				"PostToolUse":      "/usr/local/bin/claudio",
				"Stop":             "./claudio",
				"SessionStart":     "/home/user/dev/claudio/claudio",
				"UserPromptSubmit": `C:\Program Files\claudio.exe`,
				"PostCommit":       "git push",
			}},
			want: SettingsMap{"version": "1.0", "hooks": map[string]any{
				"PostCommit": "git push",
			}},
			wantNames: []string{"PostToolUse", "PreToolUse", "SessionStart", "Stop", "UserPromptSubmit"},
		},
		{
			name: "claudio-only groups drop the event and an emptied hooks section",
			input: SettingsMap{"version": "1.0", "hooks": map[string]any{
				"PreToolUse":  []any{group("", cmd("claudio"))},
				"PostToolUse": []any{group(".*", cmd("/root/code/claudio/claudio"))},
				"Stop":        "claudio.exe",
			}},
			want:      SettingsMap{"version": "1.0"},
			wantNames: []string{"PostToolUse", "PreToolUse", "Stop"},
		},
		{
			name: "mixed group keeps its non-claudio sibling and matcher",
			input: SettingsMap{"hooks": map[string]any{
				"Stop": []any{group(".*", cmd("/usr/local/bin/claudio"), cmd("other-tool"))},
			}},
			want: SettingsMap{"hooks": map[string]any{
				"Stop": []any{group(".*", cmd("other-tool"))},
			}},
			wantNames: []string{"Stop"},
		},
		{
			name: "several groups: mixed one is trimmed, claudio-only one dropped",
			input: SettingsMap{"hooks": map[string]any{
				"PreCompact": []any{
					group(".*", cmd("claudio"), cmd("keep-this")),
					group("specific", cmd("claudio")),
				},
			}},
			want: SettingsMap{"hooks": map[string]any{
				"PreCompact": []any{group(".*", cmd("keep-this"))},
			}},
			wantNames: []string{"PreCompact"},
		},
		{
			name: "flat Copilot command is removed next to a user command",
			input: SettingsMap{"hooks": map[string]any{
				"PreToolUse": []any{cmd("/usr/local/bin/claudio --hook-agent copilot"), cmd("/usr/bin/logger")},
			}},
			want: SettingsMap{"hooks": map[string]any{
				"PreToolUse": []any{cmd("/usr/bin/logger")},
			}},
			wantNames: []string{"PreToolUse"},
		},
		{
			name: "user groups, including an empty one, survive verbatim",
			input: SettingsMap{"hooks": map[string]any{
				"PostToolUse": []any{
					group("claudio-target", cmd("/usr/local/bin/claudio")),
					group("user-non-claudio", cmd("/usr/local/bin/lint")),
					group("user-empty"),
					"raw",
				},
			}},
			want: SettingsMap{"hooks": map[string]any{
				"PostToolUse": []any{
					group("user-non-claudio", cmd("/usr/local/bin/lint")),
					group("user-empty"),
					"raw",
				},
			}},
			wantNames: []string{"PostToolUse"},
		},
		{
			name: "no claudio hooks: nothing changes",
			input: SettingsMap{"hooks": map[string]any{
				"PreToolUse":  "/usr/bin/git",
				"PostToolUse": []any{group("", cmd("different-tool"))},
				"Weird":       float64(3),
			}},
			want: SettingsMap{"hooks": map[string]any{
				"PreToolUse":  "/usr/bin/git",
				"PostToolUse": []any{group("", cmd("different-tool"))},
				"Weird":       float64(3),
			}},
		},
		{
			name:  "an untouched empty hooks section is kept",
			input: SettingsMap{"hooks": map[string]any{}},
			want:  SettingsMap{"hooks": map[string]any{}},
		},
		{
			name:  "a non-object hooks section is left alone",
			input: SettingsMap{"hooks": "not-a-map"},
			want:  SettingsMap{"hooks": "not-a-map"},
		},
		{
			name:  "no hooks section",
			input: SettingsMap{"version": "1.0"},
			want:  SettingsMap{"version": "1.0"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before, err := deepCopySettings(&tc.input)
			if err != nil {
				t.Fatal(err)
			}
			got, names, err := RemoveAgentHooks(&tc.input, AgentClaude)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(*got, tc.want) {
				t.Errorf("settings:\n got %s\nwant %s", canonicalJSON(t, got), canonicalJSON(t, tc.want))
			}
			if !reflect.DeepEqual(names, tc.wantNames) {
				t.Errorf("names = %v, want %v", names, tc.wantNames)
			}
			if remaining := ClaudioHookNames(got); len(remaining) != 0 {
				t.Errorf("claudio hooks remain after removal: %v", remaining)
			}
			if !reflect.DeepEqual(canonicalJSON(t, &tc.input), canonicalJSON(t, before)) {
				t.Error("RemoveAgentHooks mutated its input")
			}
		})
	}
}

func TestRemoveAgentHooksNilSettings(t *testing.T) {
	got, names, err := RemoveAgentHooks(nil, AgentClaude)
	if err != nil || len(names) != 0 || len(*got) != 0 {
		t.Fatalf("RemoveAgentHooks(nil) = %v, %v, %v; want empty, none, nil", got, names, err)
	}
}
