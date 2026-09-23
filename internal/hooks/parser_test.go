package hooks

import (
	"encoding/json"
	"fmt"
	"testing"
)

// Test data based on real Claude Code hook JSON
const (
	realUserPromptSubmitJSON = `{
		"session_id": "cd418646-87b6-4db2-83fa-a059baf16ccf",
		"transcript_path": "/root/.claude/projects/-root-code-claudio/cd418646-87b6-4db2-83fa-a059baf16ccf.jsonl",
		"cwd": "/root/code/claudio",
		"hook_event_name": "UserPromptSubmit",
		"prompt": "Aaaand we're back like shoulderpads"
	}`

	realPreToolUseJSON = `{
		"session_id": "cd418646-87b6-4db2-83fa-a059baf16ccf",
		"transcript_path": "/root/.claude/projects/-root-code-claudio/cd418646-87b6-4db2-83fa-a059baf16ccf.jsonl",
		"cwd": "/root/code/claudio",
		"hook_event_name": "PreToolUse",
		"tool_name": "Bash",
		"tool_input": {
			"command": "ls -la /tmp/claudio-hook-logs/",
			"description": "Check if hook logs have been created"
		}
	}`

	realPostToolUseBashJSON = `{
		"session_id": "cd418646-87b6-4db2-83fa-a059baf16ccf",
		"transcript_path": "/root/.claude/projects/-root-code-claudio/cd418646-87b6-4db2-83fa-a059baf16ccf.jsonl",
		"cwd": "/root/code/claudio",
		"hook_event_name": "PostToolUse",
		"tool_name": "Bash",
		"tool_input": {
			"command": "ls -la /tmp/claudio-hook-logs/",
			"description": "Check if hook logs have been created"
		},
		"tool_response": {
			"stdout": "total 288\ndrwxr-xr-x  2 root root   4096 Jul 26 16:53 .",
			"stderr": "",
			"interrupted": false,
			"isImage": false
		}
	}`

	realPostToolUseGrepJSON = `{
		"session_id": "cd418646-87b6-4db2-83fa-a059baf16ccf",
		"transcript_path": "/root/.claude/projects/-root-code-claudio/cd418646-87b6-4db2-83fa-a059baf16ccf.jsonl",
		"cwd": "/root/code/claudio",
		"hook_event_name": "PostToolUse",
		"tool_name": "Grep",
		"tool_input": {
			"pattern": "func.*Decode",
			"path": "/root/code/claudio.click/internal/audio",
			"output_mode": "content",
			"-n": true
		},
		"tool_response": {
			"mode": "content",
			"numFiles": 0,
			"filenames": [],
			"content": "/root/code/claudio.click/internal/audio/registry.go:18:func NewDecoderRegistry()",
			"numLines": 44
		}
	}`

	realNotificationJSON = `{
		"session_id": "cd418646-87b6-4db2-83fa-a059baf16ccf",
		"transcript_path": "/root/.claude/projects/-root-code-claudio/cd418646-87b6-4db2-83fa-a059baf16ccf.jsonl",
		"cwd": "/root/code/claudio",
		"hook_event_name": "Notification",
		"message": "Claude needs your permission to use Read"
	}`
)

func TestEventCategory_String_NewCategories(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		category EventCategory
		expected string
	}{
		{"Loading category", Loading, "loading"},
		{"Success category", Success, "success"},
		{"Error category", Error, "error"},
		{"Interactive category", Interactive, "interactive"},
		{"Completion category", Completion, "completion"},
		{"System category", System, "system"},
		{"Silent category", Silent, "silent"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.category.String()
			if result != tt.expected {
				t.Errorf("EventCategory.String() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestParseUserPromptSubmit(t *testing.T) {
	t.Parallel()
	event, err := ParseHookEvent([]byte(realUserPromptSubmitJSON))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if event == nil {
		t.Fatal("Parsed event is nil")
	}

	// Verify base fields
	if event.SessionID != "cd418646-87b6-4db2-83fa-a059baf16ccf" {
		t.Errorf("SessionID = %s, expected cd418646-87b6-4db2-83fa-a059baf16ccf", event.SessionID)
	}

	if event.EventName != "UserPromptSubmit" {
		t.Errorf("EventName = %s, expected UserPromptSubmit", event.EventName)
	}

	if event.CWD != "/root/code/claudio" {
		t.Errorf("CWD = %s, expected /root/code/claudio", event.CWD)
	}

	// Verify event-specific fields
	if event.Prompt == nil {
		t.Fatal("Prompt should not be nil for UserPromptSubmit")
	}

	if *event.Prompt != "Aaaand we're back like shoulderpads" {
		t.Errorf("Prompt = %s, expected 'Aaaand we're back like shoulderpads'", *event.Prompt)
	}

	// Fields that should be nil for this event type
	if event.ToolName != nil {
		t.Error("ToolName should be nil for UserPromptSubmit")
	}

	if event.Message != nil {
		t.Error("Message should be nil for UserPromptSubmit")
	}
}

func TestParsePreToolUse(t *testing.T) {
	t.Parallel()
	event, err := ParseHookEvent([]byte(realPreToolUseJSON))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Verify base fields
	if event.EventName != "PreToolUse" {
		t.Errorf("EventName = %s, expected PreToolUse", event.EventName)
	}

	// Verify tool-specific fields
	if event.ToolName == nil {
		t.Fatal("ToolName should not be nil for PreToolUse")
	}

	if *event.ToolName != "Bash" {
		t.Errorf("ToolName = %s, expected Bash", *event.ToolName)
	}

	if event.ToolInput == nil {
		t.Fatal("ToolInput should not be nil for PreToolUse")
	}

	// Verify tool input structure
	var toolInput map[string]interface{}
	err = json.Unmarshal(*event.ToolInput, &toolInput)
	if err != nil {
		t.Fatalf("Failed to unmarshal ToolInput: %v", err)
	}

	if command, ok := toolInput["command"].(string); !ok || command != "ls -la /tmp/claudio-hook-logs/" {
		t.Errorf("ToolInput command = %v, expected 'ls -la /tmp/claudio-hook-logs/'", toolInput["command"])
	}

	// Fields that should be nil for this event type
	if event.ToolResponse != nil {
		t.Error("ToolResponse should be nil for PreToolUse")
	}

	if event.Prompt != nil {
		t.Error("Prompt should be nil for PreToolUse")
	}
}

func TestParsePostToolUseBash(t *testing.T) {
	t.Parallel()
	event, err := ParseHookEvent([]byte(realPostToolUseBashJSON))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Verify event type
	if event.EventName != "PostToolUse" {
		t.Errorf("EventName = %s, expected PostToolUse", event.EventName)
	}

	// Verify tool name
	if event.ToolName == nil || *event.ToolName != "Bash" {
		t.Errorf("ToolName = %v, expected Bash", event.ToolName)
	}

	// Verify tool response exists
	if event.ToolResponse == nil {
		t.Fatal("ToolResponse should not be nil for PostToolUse")
	}

	// Parse tool response
	var response map[string]interface{}
	err = json.Unmarshal(*event.ToolResponse, &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal ToolResponse: %v", err)
	}

	// Verify Bash-specific response fields
	if stdout, ok := response["stdout"].(string); !ok || len(stdout) == 0 {
		t.Error("Bash response should have non-empty stdout")
	}

	if _, ok := response["stderr"].(string); !ok {
		t.Error("Bash response should have stderr field")
	}

	if interrupted, ok := response["interrupted"].(bool); !ok || interrupted {
		t.Error("Bash response interrupted should be false")
	}
}

func TestParsePostToolUseGrep(t *testing.T) {
	t.Parallel()
	event, err := ParseHookEvent([]byte(realPostToolUseGrepJSON))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Verify tool name
	if event.ToolName == nil || *event.ToolName != "Grep" {
		t.Errorf("ToolName = %v, expected Grep", event.ToolName)
	}

	// Parse tool response
	var response map[string]interface{}
	err = json.Unmarshal(*event.ToolResponse, &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal ToolResponse: %v", err)
	}

	// Verify Grep-specific response fields
	if mode, ok := response["mode"].(string); !ok || mode != "content" {
		t.Errorf("Grep response mode = %v, expected 'content'", response["mode"])
	}

	if numLines, ok := response["numLines"].(float64); !ok || numLines != 44 {
		t.Errorf("Grep response numLines = %v, expected 44", response["numLines"])
	}

	if content, ok := response["content"].(string); !ok || len(content) == 0 {
		t.Error("Grep response should have non-empty content")
	}
}

func TestParseNotification(t *testing.T) {
	t.Parallel()
	event, err := ParseHookEvent([]byte(realNotificationJSON))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Verify event type
	if event.EventName != "Notification" {
		t.Errorf("EventName = %s, expected Notification", event.EventName)
	}

	// Verify message field
	if event.Message == nil {
		t.Fatal("Message should not be nil for Notification")
	}

	if *event.Message != "Claude needs your permission to use Read" {
		t.Errorf("Message = %s, expected permission message", *event.Message)
	}

	// Fields that should be nil for this event type
	if event.ToolName != nil {
		t.Error("ToolName should be nil for Notification")
	}

	if event.Prompt != nil {
		t.Error("Prompt should be nil for Notification")
	}
}

func TestParseInvalidJSON(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name string
		json string
	}{
		{"empty", ""},
		{"invalid json", "{invalid json}"},
		{"missing required field", `{"session_id": "test"}`},
		{"wrong type", `{"session_id": 123, "hook_event_name": "test"}`},
		{"null values", `{"session_id": null, "hook_event_name": null}`},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			event, err := ParseHookEvent([]byte(tc.json))

			if err == nil {
				t.Errorf("Expected error for %s, but got none", tc.name)
			}

			if event != nil {
				t.Errorf("Expected nil event for %s, but got %v", tc.name, event)
			}
		})
	}
}

func TestExtractCommandInfo(t *testing.T) {
	t.Parallel()
	testJSON := `{
		"session_id": "test",
		"transcript_path": "/test",
		"cwd": "/test",
		"hook_event_name": "PreToolUse",
		"tool_name": "Bash",
		"tool_input": {
			"command": "git commit -m 'fix bug'",
			"description": "Test command"
		}
	}`

	event, err := ParseHookEvent([]byte(testJSON))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	commandInfo := event.extractCommandInfo()

	if commandInfo.Command != "git" {
		t.Errorf("Expected command 'git', got '%s'", commandInfo.Command)
	}

	if commandInfo.Subcommand != "commit" {
		t.Errorf("Expected subcommand 'commit', got '%s'", commandInfo.Subcommand)
	}

	if !commandInfo.HasSubcommand {
		t.Error("Expected HasSubcommand to be true")
	}
}

func TestExtractCommandInfoVariants(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name        string
		command     string
		expectedCmd string
		expectedSub string
		expectedHas bool
	}{
		{"single command", "ls -la", "ls", "", false},
		{"npm with subcommand", "npm install --save", "npm", "install", true},
		{"docker compose", "docker compose up -d", "docker", "compose", true},
		{"git with message", "git commit -m 'test message'", "git", "commit", true},
		{"empty command", "", "", "", false},
		{"flags only", "--help --verbose", "", "", false},
		{"command with flags first", "--verbose git status", "git", "status", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testJSON := fmt.Sprintf(`{
				"session_id": "test",
				"transcript_path": "/test",
				"cwd": "/test",
				"hook_event_name": "PreToolUse",
				"tool_name": "Bash",
				"tool_input": {"command": "%s"}
			}`, tc.command)

			event, err := ParseHookEvent([]byte(testJSON))
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			commandInfo := event.extractCommandInfo()

			if commandInfo.Command != tc.expectedCmd {
				t.Errorf("Command = '%s', expected '%s'", commandInfo.Command, tc.expectedCmd)
			}
			if commandInfo.Subcommand != tc.expectedSub {
				t.Errorf("Subcommand = '%s', expected '%s'", commandInfo.Subcommand, tc.expectedSub)
			}
			if commandInfo.HasSubcommand != tc.expectedHas {
				t.Errorf("HasSubcommand = %v, expected %v", commandInfo.HasSubcommand, tc.expectedHas)
			}
		})
	}
}

// Codex sends transcript_path as null or omits it entirely.
func TestParseCodexNullTranscriptPathSucceeds(t *testing.T) {
	t.Parallel()
	data := []byte(`{"session_id":"abc","cwd":"/tmp","hook_event_name":"SessionStart","transcript_path":null}`)
	event, err := ParseHookEvent(data)
	if err != nil {
		t.Fatalf("expected nil error for null transcript_path, got: %v", err)
	}
	if event.EventName != "SessionStart" {
		t.Errorf("expected SessionStart, got %q", event.EventName)
	}
}

func TestParseCodexOmittedTranscriptPathSucceeds(t *testing.T) {
	t.Parallel()
	data := []byte(`{"session_id":"abc","cwd":"/tmp","hook_event_name":"Stop"}`)
	_, err := ParseHookEvent(data)
	if err != nil {
		t.Fatalf("expected nil error for omitted transcript_path, got: %v", err)
	}
}

func TestParseStillRequiresSessionIDAndEventAndCwd(t *testing.T) {
	t.Parallel()
	cases := map[string][]byte{
		"missing session_id": []byte(`{"cwd":"/tmp","hook_event_name":"Stop"}`),
		"missing event":      []byte(`{"session_id":"a","cwd":"/tmp"}`),
		"missing cwd":        []byte(`{"session_id":"a","hook_event_name":"Stop"}`),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseHookEvent(data); err == nil {
				t.Errorf("expected error for %s, got nil", name)
			}
		})
	}
}

func TestParseCopilotResultAliases(t *testing.T) {
	t.Parallel()
	payload := []byte(`{
		"sessionId": "copilot-session",
		"hook_event_name": "PostToolUse",
		"cwd": "/tmp",
		"tool_name": "Write",
		"tool_result": {"success": true}
	}`)

	event, err := ParseHookEvent(payload)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if event.SessionID != "copilot-session" {
		t.Errorf("session id = %q, want copilot-session", event.SessionID)
	}
	if event.ToolResponse == nil {
		t.Fatal("expected tool_result alias to populate ToolResponse")
	}

	ctx := event.GetContext()
	if ctx.SoundHint != "write-success" {
		t.Errorf("sound hint = %q, want write-success", ctx.SoundHint)
	}
}

func TestParseCopilotNotificationSessionAlias(t *testing.T) {
	t.Parallel()
	payload := []byte(`{
		"sessionId": "copilot-session",
		"hook_event_name": "Notification",
		"cwd": "/tmp",
		"message": "Permission needed"
	}`)

	event, err := ParseHookEvent(payload)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	ctx := event.GetContext()
	if ctx.SoundHint != "notification-permission" {
		t.Errorf("sound hint = %q, want notification-permission", ctx.SoundHint)
	}
}

func TestParseHookEventWithDefaultNormalizesCopilotSubagentStart(t *testing.T) {
	t.Parallel()
	payload := []byte(`{
		"sessionId": "copilot-session",
		"cwd": "/tmp"
	}`)

	event, err := ParseHookEventWithDefault(payload, "subagentStart")
	if err != nil {
		t.Fatalf("ParseHookEventWithDefault returned error: %v", err)
	}
	if event.EventName != "SubagentStart" {
		t.Errorf("event name = %q, want SubagentStart", event.EventName)
	}

	ctx := event.GetContext()
	if ctx.SoundHint != "subagent-start" {
		t.Errorf("sound hint = %q, want subagent-start", ctx.SoundHint)
	}
}
