package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"claudio.click/internal/cli/testenv"
	"claudio.click/internal/install"
)

// roundTripCase is one agent's settings shape with user-owned entries.
type roundTripCase struct {
	agent    string
	relPath  []string
	userJSON string
}

var roundTripCases = []roundTripCase{
	{
		agent:   "claude",
		relPath: []string{".claude", "settings.json"},
		userJSON: `{"theme":"dark","hooks":{
			"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"echo user-pre"}]}],
			"Stop":[{"matcher":"","hooks":[]}],
			"CustomUserEvent":[{"matcher":".*","hooks":[{"type":"command","command":"echo custom"}]}]}}`,
	},
	{
		agent:   "codex",
		relPath: []string{".codex", "hooks.json"},
		userJSON: `{"hooks":{
			"PreToolUse":[{"matcher":"*","hooks":[{"type":"command","command":"ward eval"}]}],
			"UserOnly":[{"hooks":[{"type":"command","command":"echo codex-user"}]}]}}`,
	},
	{
		agent:   "gemini",
		relPath: []string{".gemini", "settings.json"},
		userJSON: `{"model":"gemini-pro","hooks":{
			"BeforeTool":[{"matcher":"","hooks":[{"type":"command","name":"user","command":"echo gemini-user"}]}]}}`,
	},
	{
		agent:   "qwen",
		relPath: []string{".qwen", "settings.json"},
		userJSON: `{"hooks":{
			"PreToolUse":[{"matcher":".*","hooks":[{"type":"command","name":"user","command":"echo qwen-user"}]}]},"ui":{"theme":"x"}}`,
	},
	{
		agent:   "copilot",
		relPath: []string{".copilot", "settings.json"},
		userJSON: `{"hooks":{
			"PreToolUse":[{"type":"command","command":"echo copilot-user","timeoutSec":5}],
			"SessionStart":[{"type":"command","command":"echo copilot-start"}]}}`,
	},
}

// TestInstallInstallUninstallRoundTripKeepsOnlyUserEntries pins, per
// agent settings shape, that install twice then uninstall once leaves
// exactly the user's own settings: install is idempotent and uninstall
// removes everything install added and nothing else.
func TestInstallInstallUninstallRoundTripKeepsOnlyUserEntries(t *testing.T) {
	for _, tc := range roundTripCases {
		t.Run(tc.agent, func(t *testing.T) {
			root := testenv.IsolateXDG(t)
			for _, key := range []string{"CLAUDE_CONFIG_DIR", "CODEX_HOME", "COPILOT_HOME"} {
				t.Setenv(key, "")
			}
			t.Setenv("CLAUDIO_TEST_RECOGNIZE_GO_TEST", "1")

			settingsPath := filepath.Join(append([]string{root}, tc.relPath...)...)
			if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(settingsPath, []byte(tc.userJSON), 0644); err != nil {
				t.Fatal(err)
			}
			want := decodeJSONObject(t, []byte(tc.userJSON))

			runHookCommand(t, "install", tc.agent)
			first := readJSONObject(t, settingsPath)
			if names := hookNamesWithClaudio(first); len(names) == 0 {
				t.Fatalf("first install added no claudio hooks: %v", first)
			}
			runHookCommand(t, "install", tc.agent)
			second := readJSONObject(t, settingsPath)
			if !reflect.DeepEqual(first, second) {
				t.Fatalf("second install changed settings:\nfirst  %v\nsecond %v", first, second)
			}
			runHookCommand(t, "uninstall", tc.agent)
			got := readJSONObject(t, settingsPath)
			if !reflect.DeepEqual(got, want) {
				gotJSON, _ := json.Marshal(got)
				wantJSON, _ := json.Marshal(want)
				t.Fatalf("round trip did not restore user settings:\n got %s\nwant %s", gotJSON, wantJSON)
			}
		})
	}
}

func runHookCommand(t *testing.T, verb, agent string) {
	t.Helper()
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	code := NewCLI().Run([]string{"claudio", verb, "--agent", agent, "--scope", "global", "--quiet"}, strings.NewReader(""), stdout, stderr)
	if code != 0 {
		t.Fatalf("%s --agent %s exit %d; stdout=%q stderr=%q", verb, agent, code, stdout, stderr)
	}
}

func decodeJSONObject(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func readJSONObject(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return decodeJSONObject(t, data)
}

// hookNamesWithClaudio lists hook events whose value mentions a claudio
// command, using only the public recognizer.
func hookNamesWithClaudio(settings map[string]any) []string {
	hooks, _ := settings["hooks"].(map[string]any)
	var names []string
	for name, value := range hooks {
		data, _ := json.Marshal(value)
		var entries []map[string]any
		_ = json.Unmarshal(data, &entries)
		for _, entry := range entries {
			if cmd, ok := entry["command"].(string); ok && install.IsClaudioCommandString(cmd) {
				names = append(names, name)
			}
			inner, _ := entry["hooks"].([]any)
			for _, h := range inner {
				if m, ok := h.(map[string]any); ok {
					if cmd, ok := m["command"].(string); ok && install.IsClaudioCommandString(cmd) {
						names = append(names, name)
					}
				}
			}
		}
	}
	return names
}
