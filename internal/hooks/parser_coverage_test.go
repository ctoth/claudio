package hooks

import (
	"encoding/json"
	"testing"
)

func TestEventCategoryStringAll(t *testing.T) {
	t.Parallel()
	cases := map[EventCategory]string{
		Loading:            "loading",
		Success:            "success",
		Error:              "error",
		Interactive:        "interactive",
		Completion:         "completion",
		System:             "system",
		EventCategory(999): "unknown",
	}
	for cat, want := range cases {
		if got := cat.String(); got != want {
			t.Errorf("EventCategory(%d).String() = %q, want %q", int(cat), got, want)
		}
	}
}

func TestExtractFileExtensionCoverage(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"/a/b/main.go":     "go",
		"/a/b/file.TXT":    "txt",
		"/a/b/archive.tmp": "",
		"/a/b/note.log":    "",
		"/a/b/old.bak":     "",
		"/a/b/x.orig":      "",
		"/a/b/noext":       "",
		"/a/b/trailing.":   "",
		"/a/b.v2/README":   "", // filepath.Ext only inspects the final element
	}
	for path, want := range cases {
		if got := extractFileExtension(path); got != want {
			t.Errorf("extractFileExtension(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestIsValidSubcommandCoverage(t *testing.T) {
	t.Parallel()
	cases := []struct {
		command, word string
		want          bool
	}{
		{"git", "commit", true},
		{"git", "notasubcommand", false},
		{"ls", "/path/to/file", false},
		{"curl", "http://example.com", false},
		{"systemctl", "start", true},
		{"weird", "has!bang", false},
		{"ls", "file.txt", false},
	}
	for _, c := range cases {
		if got := isValidSubcommand(c.command, c.word); got != c.want {
			t.Errorf("isValidSubcommand(%q,%q) = %v, want %v", c.command, c.word, got, c.want)
		}
	}
}

func TestParseCompatibilityAliasBranches(t *testing.T) {
	t.Parallel()
	_, err := ParseHookEvent([]byte(`{
		"session_id": "snake-session",
		"cwd": "/tmp",
		"hook_event_name": "Stop",
		"sessionId": 42
	}`))
	if err == nil {
		t.Fatal("expected alias parse error for non-string sessionId")
	}

	event, err := ParseHookEvent([]byte(`{
		"sessionId": "alias-session",
		"transcriptPath": "/tmp/transcript.jsonl",
		"cwd": "/tmp",
		"hookEventName": "AfterTool",
		"toolName": "run_shell_command",
		"toolArgs": {"command": "go test ./internal/hooks"},
		"toolResult": {"stdout": "ok", "stderr": ""}
	}`))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if event.SessionID != "alias-session" {
		t.Errorf("SessionID = %q, want alias-session", event.SessionID)
	}
	if event.TranscriptPath != "/tmp/transcript.jsonl" {
		t.Errorf("TranscriptPath = %q, want /tmp/transcript.jsonl", event.TranscriptPath)
	}
	if event.EventName != "AfterTool" {
		t.Errorf("EventName = %q, want AfterTool", event.EventName)
	}
	if event.ToolName == nil || *event.ToolName != "run_shell_command" {
		t.Fatalf("ToolName = %v, want run_shell_command", event.ToolName)
	}
	if event.ToolInput == nil {
		t.Fatal("expected toolArgs alias to populate ToolInput")
	}
	if event.ToolResponse == nil {
		t.Fatal("expected toolResult alias to populate ToolResponse")
	}

	ctx := event.GetContext()
	if ctx.ToolName != "go" || ctx.OriginalTool != "Bash" {
		t.Errorf("tool context = original %q tool %q, want Bash/go", ctx.OriginalTool, ctx.ToolName)
	}
	if ctx.SoundHint != "go-test-success" {
		t.Errorf("SoundHint = %q, want go-test-success", ctx.SoundHint)
	}
}

func TestNormalizeToolNameAliases(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"":                           "",
		"writefile":                  "Write",
		"create":                     "Write",
		"replace":                    "Edit",
		"multi-edit":                 "MultiEdit",
		"read-many-files":            "Read",
		"list-directory":             "LS",
		"glob":                       "Glob",
		"web-fetch":                  "WebFetch",
		"google-web-search":          "WebSearch",
		"todo-write":                 "TodoWrite",
		"read_mcp_resource":          "mcp",
		"mcp__filesystem__read_file": "mcp",
		"mcp_filesystem_read_file":   "mcp",
		"CustomTool":                 "CustomTool",
	}
	for input, want := range cases {
		if got := normalizeToolName(input); got != want {
			t.Errorf("normalizeToolName(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestParseExitCodeBranches(t *testing.T) {
	t.Parallel()
	cases := []struct {
		text string
		code int
		ok   bool
	}{
		{text: "no exit line", ok: false},
		{text: "Exit code:", ok: false},
		{text: "Exit code: nope", ok: false},
		{text: "Exit code: 0", code: 0, ok: true},
		{text: "  Exit Code: 7  ", code: 7, ok: true},
	}
	for _, tc := range cases {
		code, ok := parseExitCode(tc.text)
		if code != tc.code || ok != tc.ok {
			t.Errorf("parseExitCode(%q) = (%d, %v), want (%d, %v)", tc.text, code, ok, tc.code, tc.ok)
		}
	}
}

func TestToolInputParseFallbackBranches(t *testing.T) {
	t.Parallel()
	bad := json.RawMessage(`not json`)
	event := HookEvent{ToolInput: &bad}
	if got := event.extractFileType(); got != "" {
		t.Errorf("extractFileType with invalid JSON = %q, want empty", got)
	}
	if got := event.extractCommandInfo(); got != (CommandInfo{}) {
		t.Errorf("extractCommandInfo with invalid JSON = %+v, want zero value", got)
	}

	noCommand := json.RawMessage(`{"path":"/tmp/file.txt"}`)
	event = HookEvent{ToolInput: &noCommand}
	if got := event.extractCommandInfo(); got != (CommandInfo{}) {
		t.Errorf("extractCommandInfo without command = %+v, want zero value", got)
	}

	blankCommand := json.RawMessage(`{"command":"   "}`)
	event = HookEvent{ToolInput: &blankCommand}
	if got := event.extractCommandInfo(); got != (CommandInfo{}) {
		t.Errorf("extractCommandInfo with blank command = %+v, want zero value", got)
	}
}
