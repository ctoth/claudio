package hooks

import (
	"encoding/json"
	"testing"
)

func TestEventCategoryStringAll(t *testing.T) {
	cases := map[EventCategory]string{
		Loading:     "loading",
		Success:     "success",
		Error:       "error",
		Interactive: "interactive",
		Completion:  "completion",
		System:      "system",
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
		"/a/b/main.go":   "go",
		"/a/b/file.TXT":  "txt",
		"/a/b/archive.tmp": "",
		"/a/b/note.log":  "",
		"/a/b/old.bak":   "",
		"/a/b/x.orig":    "",
		"/a/b/noext":     "",
		"/a/b/trailing.": "",
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
