package hooks

import (
	"encoding/json"
	"testing"
)

func TestEventCategoryStringAll(t *testing.T) {
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
	cases := map[string]string{
		"/a/b/main.go":     "go",
		"/a/b/file.TXT":    "txt",
		"/a/b/archive.tmp": "",
		"/a/b/note.log":    "",
		"/a/b/old.bak":     "",
		"/a/b/x.orig":      "",
		"/a/b/noext":       "",
		"/a/b/trailing.":   "",
	}
	for path, want := range cases {
		if got := extractFileExtension(path); got != want {
			t.Errorf("extractFileExtension(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestIsValidSubcommandCoverage(t *testing.T) {
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

func postToolUseContext(t *testing.T, tool string, response string) *EventContext {
	t.Helper()
	resp := json.RawMessage(response)
	e := &HookEvent{SessionID: "a", CWD: "/tmp", EventName: "PostToolUse", ToolName: &tool, ToolResponse: &resp}
	return e.GetContext()
}

func TestAnalyzeToolResponseBranches(t *testing.T) {
	// MCP-style isError on a non-Bash tool
	if ctx := postToolUseContext(t, "apply_patch", `{"isError":true}`); !ctx.HasError {
		t.Error("expected HasError for isError=true")
	}
	// interrupted
	if ctx := postToolUseContext(t, "Bash", `{"interrupted":true}`); ctx.SoundHint != "tool-interrupted" {
		t.Errorf("expected tool-interrupted, got %q", ctx.SoundHint)
	}
	// Read with content -> success
	if ctx := postToolUseContext(t, "Read", `{"content":"hello"}`); ctx.Category != Success {
		t.Errorf("expected Success for Read with content, got %v", ctx.Category)
	}
	// Read without content -> error
	if ctx := postToolUseContext(t, "Read", `{}`); ctx.Category != Error {
		t.Errorf("expected Error for Read without content, got %v", ctx.Category)
	}
	// Edit with explicit success=false -> error
	if ctx := postToolUseContext(t, "Edit", `{"success":false}`); ctx.Category != Error {
		t.Errorf("expected Error for Edit success=false, got %v", ctx.Category)
	}
	// Edit with explicit success=true -> success
	if ctx := postToolUseContext(t, "Edit", `{"success":true}`); ctx.Category != Success {
		t.Errorf("expected Success for Edit success=true, got %v", ctx.Category)
	}
	// Grep with numLines -> success
	if ctx := postToolUseContext(t, "Grep", `{"numLines":0}`); ctx.Category != Success {
		t.Errorf("expected Success for Grep numLines=0, got %v", ctx.Category)
	}
	// Unparseable tool_response -> error
	if ctx := postToolUseContext(t, "Bash", `not json`); !ctx.HasError {
		t.Error("expected HasError for unparseable tool_response")
	}
}

func TestDetectNotificationTypeCoverage(t *testing.T) {
	mk := func(msg string) *EventContext {
		m := msg
		e := &HookEvent{SessionID: "a", CWD: "/tmp", EventName: "Notification", Message: &m}
		return e.GetContext()
	}
	if mk("Claude needs your permission to run").SoundHint != "notification-permission" {
		t.Error("expected notification-permission")
	}
	if mk("Claude has been idle for 60s").SoundHint != "notification-idle" {
		t.Error("expected notification-idle")
	}
	if mk("something else entirely").SoundHint != "notification" {
		t.Error("expected generic notification")
	}
	// nil message
	e := &HookEvent{SessionID: "a", CWD: "/tmp", EventName: "Notification"}
	if e.GetContext().SoundHint != "notification" {
		t.Error("expected generic notification for nil message")
	}
}

func TestParseCompatibilityAliasBranches(t *testing.T) {
	parser := NewHookEventParser()

	_, err := parser.Parse([]byte(`{
		"session_id": "snake-session",
		"cwd": "/tmp",
		"hook_event_name": "Stop",
		"sessionId": 42
	}`))
	if err == nil {
		t.Fatal("expected alias parse error for non-string sessionId")
	}

	event, err := parser.Parse([]byte(`{
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

func TestAdditionalEventContextBranches(t *testing.T) {
	cases := []struct {
		name      string
		event     HookEvent
		category  EventCategory
		hint      string
		operation string
		hasError  bool
	}{
		{
			name:      "StopFailure",
			event:     HookEvent{SessionID: "a", CWD: "/tmp", EventName: "StopFailure"},
			category:  Error,
			hint:      "stop-failure",
			operation: "stop-failure",
			hasError:  true,
		},
		{
			name:      "unknown event",
			event:     HookEvent{SessionID: "a", CWD: "/tmp", EventName: "SomethingNew"},
			category:  Interactive,
			hint:      "default",
			operation: "unknown",
		},
		{
			name:      "PreToolUse without tool",
			event:     HookEvent{SessionID: "a", CWD: "/tmp", EventName: "PreToolUse"},
			category:  Loading,
			hint:      "tool-loading",
			operation: "tool-start",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := tc.event.GetContext()
			if ctx.Category != tc.category {
				t.Errorf("Category = %v, want %v", ctx.Category, tc.category)
			}
			if ctx.SoundHint != tc.hint {
				t.Errorf("SoundHint = %q, want %q", ctx.SoundHint, tc.hint)
			}
			if ctx.Operation != tc.operation {
				t.Errorf("Operation = %q, want %q", ctx.Operation, tc.operation)
			}
			if ctx.HasError != tc.hasError {
				t.Errorf("HasError = %v, want %v", ctx.HasError, tc.hasError)
			}
		})
	}
}

func TestPostToolFallbackHintBranches(t *testing.T) {
	t.Run("success without tool name uses generic success hint", func(t *testing.T) {
		event := &HookEvent{SessionID: "a", CWD: "/tmp", EventName: "PostToolUse"}
		ctx := event.GetContext()
		if ctx.Category != Success {
			t.Errorf("Category = %v, want Success", ctx.Category)
		}
		if ctx.SoundHint != "tool-success" {
			t.Errorf("SoundHint = %q, want tool-success", ctx.SoundHint)
		}
	})

	t.Run("error without tool name uses generic error hint", func(t *testing.T) {
		resp := json.RawMessage(`{"error":"boom"}`)
		event := &HookEvent{SessionID: "a", CWD: "/tmp", EventName: "PostToolUse", ToolResponse: &resp}
		ctx := event.GetContext()
		if ctx.Category != Error {
			t.Errorf("Category = %v, want Error", ctx.Category)
		}
		if ctx.SoundHint != "tool-error" {
			t.Errorf("SoundHint = %q, want tool-error", ctx.SoundHint)
		}
	})

	t.Run("interrupted non-MCP tool preserves interrupted hint", func(t *testing.T) {
		tool := "Read"
		resp := json.RawMessage(`{"interrupted":true}`)
		event := &HookEvent{SessionID: "a", CWD: "/tmp", EventName: "PostToolUse", ToolName: &tool, ToolResponse: &resp}
		ctx := event.GetContext()
		if ctx.SoundHint != "tool-interrupted" {
			t.Errorf("SoundHint = %q, want tool-interrupted", ctx.SoundHint)
		}
	})
}

func TestNormalizeToolNameAliases(t *testing.T) {
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

func TestAnalyzeToolResponseAdditionalBranches(t *testing.T) {
	t.Run("non-string error value is an error", func(t *testing.T) {
		ctx := postToolUseContext(t, "apply_patch", `{"error":{"message":"boom"}}`)
		if ctx.Category != Error {
			t.Errorf("Category = %v, want Error", ctx.Category)
		}
		if ctx.SoundHint != "apply_patch-error" {
			t.Errorf("SoundHint = %q, want apply_patch-error", ctx.SoundHint)
		}
	})

	t.Run("write without explicit success defaults to success", func(t *testing.T) {
		if ctx := postToolUseContext(t, "Write", `{}`); ctx.Category != Success {
			t.Errorf("Category = %v, want Success", ctx.Category)
		}
	})

	t.Run("grep without numLines defaults to success", func(t *testing.T) {
		if ctx := postToolUseContext(t, "Grep", `{}`); ctx.Category != Success {
			t.Errorf("Category = %v, want Success", ctx.Category)
		}
	})
}

func TestParseExitCodeBranches(t *testing.T) {
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
