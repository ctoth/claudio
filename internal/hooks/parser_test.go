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

func TestHookEventParser(t *testing.T) {
	parser := NewHookEventParser()

	if parser == nil {
		t.Fatal("NewHookEventParser returned nil")
	}
}

func TestEventCategory_String_NewCategories(t *testing.T) {
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
	parser := NewHookEventParser()

	event, err := parser.Parse([]byte(realUserPromptSubmitJSON))
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
	parser := NewHookEventParser()

	event, err := parser.Parse([]byte(realPreToolUseJSON))
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
	parser := NewHookEventParser()

	event, err := parser.Parse([]byte(realPostToolUseBashJSON))
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
	parser := NewHookEventParser()

	event, err := parser.Parse([]byte(realPostToolUseGrepJSON))
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
	parser := NewHookEventParser()

	event, err := parser.Parse([]byte(realNotificationJSON))
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
	parser := NewHookEventParser()

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
			event, err := parser.Parse([]byte(tc.json))

			if err == nil {
				t.Errorf("Expected error for %s, but got none", tc.name)
			}

			if event != nil {
				t.Errorf("Expected nil event for %s, but got %v", tc.name, event)
			}
		})
	}
}

func TestEventContext(t *testing.T) {
	parser := NewHookEventParser()

	t.Run("UserPromptSubmit context", func(t *testing.T) {
		event, _ := parser.Parse([]byte(realUserPromptSubmitJSON))
		context := event.GetContext()

		if context.Category != Interactive {
			t.Errorf("Expected Interactive category, got %v", context.Category)
		}

		if context.SoundHint != "message-sent" {
			t.Errorf("Expected 'message-sent' sound hint, got %s", context.SoundHint)
		}
	})

	t.Run("PreToolUse context", func(t *testing.T) {
		event, _ := parser.Parse([]byte(realPreToolUseJSON))
		context := event.GetContext()

		if context.Category != Loading {
			t.Errorf("Expected Loading category, got %v", context.Category)
		}

		// Enhanced behavior: extract "ls" from "ls -la /tmp/claudio-hook-logs/"
		if context.ToolName != "ls" {
			t.Errorf("Expected ls tool name (extracted from Bash), got %s", context.ToolName)
		}

		if context.OriginalTool != "Bash" {
			t.Errorf("Expected Bash original tool, got %s", context.OriginalTool)
		}

		if context.SoundHint != "ls-start" {
			t.Errorf("Expected 'ls-start' sound hint, got %s", context.SoundHint)
		}
	})

	t.Run("PostToolUse success context", func(t *testing.T) {
		event, _ := parser.Parse([]byte(realPostToolUseBashJSON))
		context := event.GetContext()

		if context.Category != Success {
			t.Errorf("Expected Success category, got %v", context.Category)
		}

		// Enhanced behavior: extract "ls" from "ls -la /tmp/claudio-hook-logs/"
		if context.ToolName != "ls" {
			t.Errorf("Expected ls tool name (extracted from Bash), got %s", context.ToolName)
		}

		if context.OriginalTool != "Bash" {
			t.Errorf("Expected Bash original tool, got %s", context.OriginalTool)
		}

		if context.SoundHint != "ls-success" {
			t.Errorf("Expected 'ls-success' sound hint, got %s", context.SoundHint)
		}

		if !context.IsSuccess {
			t.Error("Expected IsSuccess to be true")
		}
	})

	t.Run("Notification context", func(t *testing.T) {
		event, _ := parser.Parse([]byte(realNotificationJSON))
		context := event.GetContext()

		if context.Category != Interactive {
			t.Errorf("Expected Interactive category, got %v", context.Category)
		}

		// Real notification JSON contains "permission" so should now generate specific hint
		if context.SoundHint != "notification-permission" {
			t.Errorf("Expected 'notification-permission' sound hint, got %s", context.SoundHint)
		}
	})

	t.Run("Enhanced Bash git commit success context", func(t *testing.T) {
		gitCommitJSON := `{
			"session_id": "test",
			"transcript_path": "/test",
			"cwd": "/test",
			"hook_event_name": "PostToolUse",
			"tool_name": "Bash",
			"tool_input": {"command": "git commit -m 'fix'"},
			"tool_response": {"stdout": "committed", "stderr": "", "interrupted": false}
		}`

		event, _ := parser.Parse([]byte(gitCommitJSON))
		context := event.GetContext()

		if context.ToolName != "git" {
			t.Errorf("Expected ToolName 'git', got '%s'", context.ToolName)
		}

		if context.SoundHint != "git-commit-success" {
			t.Errorf("Expected SoundHint 'git-commit-success', got '%s'", context.SoundHint)
		}

		if context.Category != Success {
			t.Errorf("Expected Success category, got %v", context.Category)
		}
	})

	t.Run("Enhanced Bash npm install thinking context", func(t *testing.T) {
		npmJSON := `{
			"session_id": "test",
			"transcript_path": "/test",
			"cwd": "/test",
			"hook_event_name": "PreToolUse",
			"tool_name": "Bash",
			"tool_input": {"command": "npm install express"}
		}`

		event, _ := parser.Parse([]byte(npmJSON))
		context := event.GetContext()

		if context.ToolName != "npm" {
			t.Errorf("Expected ToolName 'npm', got '%s'", context.ToolName)
		}

		if context.SoundHint != "npm-install-start" {
			t.Errorf("Expected SoundHint 'npm-install-start', got '%s'", context.SoundHint)
		}

		if context.Category != Loading {
			t.Errorf("Expected Loading category, got %v", context.Category)
		}
	})

	t.Run("Enhanced Bash single command context", func(t *testing.T) {
		lsJSON := `{
			"session_id": "test",
			"transcript_path": "/test",
			"cwd": "/test",
			"hook_event_name": "PostToolUse",
			"tool_name": "Bash",
			"tool_input": {"command": "ls -la"},
			"tool_response": {"stdout": "files", "stderr": "", "interrupted": false}
		}`

		event, _ := parser.Parse([]byte(lsJSON))
		context := event.GetContext()

		if context.ToolName != "ls" {
			t.Errorf("Expected ToolName 'ls', got '%s'", context.ToolName)
		}

		if context.SoundHint != "ls-success" {
			t.Errorf("Expected SoundHint 'ls-success', got '%s'", context.SoundHint)
		}
	})

	t.Run("Bash fallback when command extraction fails", func(t *testing.T) {
		emptyJSON := `{
			"session_id": "test",
			"transcript_path": "/test",
			"cwd": "/test",
			"hook_event_name": "PostToolUse",
			"tool_name": "Bash",
			"tool_input": {"command": ""},
			"tool_response": {"stdout": "", "stderr": "", "interrupted": false}
		}`

		event, _ := parser.Parse([]byte(emptyJSON))
		context := event.GetContext()

		if context.ToolName != "Bash" {
			t.Errorf("Expected ToolName 'Bash' for fallback, got '%s'", context.ToolName)
		}

		if context.SoundHint != "bash-success" {
			t.Errorf("Expected SoundHint 'bash-success' for fallback, got '%s'", context.SoundHint)
		}
	})
}

func TestEventCategorization_All7Events(t *testing.T) {
	parser := NewHookEventParser()

	tests := []struct {
		name        string
		eventName   string
		expected    EventCategory
		description string
	}{
		{"PreToolUse maps to Loading", "PreToolUse", Loading, "Tools about to start should use loading category"},
		{"PostToolUse success maps to Success", "PostToolUse", Success, "Successful tool completions should use success category"},
		{"UserPromptSubmit maps to Interactive", "UserPromptSubmit", Interactive, "User messages should use interactive category"},
		{"Notification maps to Interactive", "Notification", Interactive, "System notifications should use interactive category"},
		{"Stop maps to Completion", "Stop", Completion, "Claude finishing should use completion category (currently fails)"},
		{"SubagentStop maps to Completion", "SubagentStop", Completion, "Subagent finishing should use completion category (currently fails)"},
		{"PreCompact maps to System", "PreCompact", System, "Context compacting should use system category (currently fails)"},
		{"SessionStart maps to System", "SessionStart", System, "Session start should use system category (currently fails)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create minimal valid JSON for each event type
			var testJSON string
			switch tt.eventName {
			case "PreToolUse":
				testJSON = `{
					"session_id": "test",
					"transcript_path": "/test",
					"cwd": "/test",
					"hook_event_name": "PreToolUse",
					"tool_name": "Bash",
					"tool_input": {"command": "ls"}
				}`
			case "PostToolUse":
				testJSON = `{
					"session_id": "test",
					"transcript_path": "/test",
					"cwd": "/test",
					"hook_event_name": "PostToolUse",
					"tool_name": "Bash",
					"tool_input": {"command": "ls"},
					"tool_response": {"stdout": "success", "stderr": "", "interrupted": false}
				}`
			case "UserPromptSubmit":
				testJSON = `{
					"session_id": "test",
					"transcript_path": "/test",
					"cwd": "/test",
					"hook_event_name": "UserPromptSubmit",
					"prompt": "Hello"
				}`
			case "Notification":
				testJSON = `{
					"session_id": "test", 
					"transcript_path": "/test",
					"cwd": "/test",
					"hook_event_name": "Notification",
					"message": "Test notification"
				}`
			case "Stop":
				testJSON = `{
					"session_id": "test",
					"transcript_path": "/test", 
					"cwd": "/test",
					"hook_event_name": "Stop"
				}`
			case "SubagentStop":
				testJSON = `{
					"session_id": "test",
					"transcript_path": "/test",
					"cwd": "/test", 
					"hook_event_name": "SubagentStop"
				}`
			case "PreCompact":
				testJSON = `{
					"session_id": "test",
					"transcript_path": "/test",
					"cwd": "/test",
					"hook_event_name": "PreCompact"
				}`
			case "SessionStart":
				testJSON = `{
					"session_id": "test",
					"transcript_path": "/test",
					"cwd": "/test",
					"hook_event_name": "SessionStart"
				}`
			}

			event, err := parser.Parse([]byte(testJSON))
			if err != nil {
				t.Fatalf("Parse failed for %s: %v", tt.eventName, err)
			}

			context := event.GetContext()
			if context.Category != tt.expected {
				t.Errorf("%s: expected category %s, got %s. %s",
					tt.eventName, tt.expected.String(), context.Category.String(), tt.description)
			}
		})
	}
}

func TestExtractCommandInfo(t *testing.T) {
	parser := NewHookEventParser()

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

	event, err := parser.Parse([]byte(testJSON))
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
	parser := NewHookEventParser()

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

			event, err := parser.Parse([]byte(testJSON))
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

func TestNotificationTypeDetection(t *testing.T) {
	parser := NewHookEventParser()

	tests := []struct {
		name         string
		message      string
		expectedHint string
		description  string
	}{
		{
			"Permission notification detected",
			"Claude needs your permission to use Read",
			"notification-permission",
			"Should detect permission requests and generate specific hint",
		},
		{
			"Permission notification alternative phrasing",
			"Claude needs permission to run bash command",
			"notification-permission",
			"Should detect permission with alternative phrasing",
		},
		{
			"Idle notification detected",
			"Prompt has been idle for 60+ seconds",
			"notification-idle",
			"Should detect idle notifications and generate specific hint",
		},
		{
			"Generic notification fallback",
			"Some other notification message",
			"notification",
			"Should fallback to generic notification hint for unknown messages",
		},
		{
			"Empty message fallback",
			"",
			"notification",
			"Should handle empty messages gracefully",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testJSON := fmt.Sprintf(`{
				"session_id": "test",
				"transcript_path": "/test",
				"cwd": "/test",
				"hook_event_name": "Notification",
				"message": "%s"
			}`, tt.message)

			event, err := parser.Parse([]byte(testJSON))
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			context := event.GetContext()
			if context.SoundHint != tt.expectedHint {
				t.Errorf("%s: expected hint '%s', got '%s'. %s",
					tt.name, tt.expectedHint, context.SoundHint, tt.description)
			}

			// Verify category is still Interactive
			if context.Category != Interactive {
				t.Errorf("%s: expected Interactive category, got %s", tt.name, context.Category.String())
			}

			// Verify operation is still notification
			if context.Operation != "notification" {
				t.Errorf("%s: expected 'notification' operation, got '%s'", tt.name, context.Operation)
			}
		})
	}
}

func TestEnhancedEventContextExtraction(t *testing.T) {
	parser := NewHookEventParser()

	tests := []struct {
		name             string
		eventName        string
		expectedHint     string
		expectedCategory EventCategory
		expectedOp       string
		description      string
	}{
		{
			"Stop event context",
			"Stop",
			"agent-complete",
			Completion,
			"stop",
			"Stop events should generate agent-complete hint for Claude finishing",
		},
		{
			"SubagentStop event context",
			"SubagentStop",
			"subagent-complete",
			Completion,
			"subagent-stop",
			"SubagentStop events should generate subagent-complete hint for Task tool finishing",
		},
		{
			"PreCompact event context",
			"PreCompact",
			"compacting",
			System,
			"compact",
			"PreCompact events should generate compacting hint for context organization",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testJSON := fmt.Sprintf(`{
				"session_id": "test",
				"transcript_path": "/test",
				"cwd": "/test",
				"hook_event_name": "%s"
			}`, tt.eventName)

			event, err := parser.Parse([]byte(testJSON))
			if err != nil {
				t.Fatalf("Parse failed for %s: %v", tt.eventName, err)
			}

			context := event.GetContext()

			if context.Category != tt.expectedCategory {
				t.Errorf("%s: expected category %s, got %s",
					tt.name, tt.expectedCategory.String(), context.Category.String())
			}

			if context.SoundHint != tt.expectedHint {
				t.Errorf("%s: expected hint '%s', got '%s'. %s",
					tt.name, tt.expectedHint, context.SoundHint, tt.description)
			}

			if context.Operation != tt.expectedOp {
				t.Errorf("%s: expected operation '%s', got '%s'",
					tt.name, tt.expectedOp, context.Operation)
			}
		})
	}
}

func TestParseEdgeCases(t *testing.T) {
	parser := NewHookEventParser()

	t.Run("tool response with error", func(t *testing.T) {
		errorJSON := `{
			"session_id": "test",
			"transcript_path": "/path",
			"cwd": "/test",
			"hook_event_name": "PostToolUse",
			"tool_name": "Bash", 
			"tool_input": {"command": "false"},
			"tool_response": {
				"stdout": "",
				"stderr": "command failed",
				"interrupted": false,
				"isImage": false
			}
		}`

		event, err := parser.Parse([]byte(errorJSON))
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}

		context := event.GetContext()
		if context.Category != Error {
			t.Errorf("Expected Error category for stderr, got %v", context.Category)
		}

		if context.IsSuccess {
			t.Error("Expected IsSuccess to be false when stderr present")
		}
	})

	t.Run("interrupted tool", func(t *testing.T) {
		interruptedJSON := `{
			"session_id": "test",
			"transcript_path": "/path", 
			"cwd": "/test",
			"hook_event_name": "PostToolUse",
			"tool_name": "Bash",
			"tool_input": {"command": "sleep 10"},
			"tool_response": {
				"stdout": "",
				"stderr": "",
				"interrupted": true,
				"isImage": false
			}
		}`

		event, err := parser.Parse([]byte(interruptedJSON))
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}

		context := event.GetContext()
		if context.Category != Error {
			t.Errorf("Expected Error category for interrupted, got %v", context.Category)
		}

		if context.SoundHint != "tool-interrupted" {
			t.Errorf("Expected 'tool-interrupted' sound hint, got %s", context.SoundHint)
		}
	})
}

// TDD Phase 2.5 RED: Test PreToolUse suffix change from '-thinking' to '-start'
func TestPreToolUseStartSuffixInsteadOfThinking(t *testing.T) {
	parser := NewHookEventParser()

	tests := []struct {
		name         string
		command      string
		expectedHint string
		description  string
	}{
		{
			name:         "Simple command should use -start suffix",
			command:      "ls -la",
			expectedHint: "ls-start",
			description:  "Single commands should generate tool-start hints for semantic accuracy",
		},
		{
			name:         "Git status should use -start suffix",
			command:      "git status",
			expectedHint: "git-status-start",
			description:  "Git commands with subcommands should generate command-subcommand-start hints",
		},
		{
			name:         "Git commit should use -start suffix",
			command:      "git commit -m 'test'",
			expectedHint: "git-commit-start",
			description:  "Git commands with subcommands should generate command-subcommand-start hints",
		},
		{
			name:         "NPM install should use -start suffix",
			command:      "npm install express",
			expectedHint: "npm-install-start",
			description:  "NPM commands with subcommands should generate npm-subcommand-start hints",
		},
		{
			name:         "Docker compose should use -start suffix",
			command:      "docker compose up -d",
			expectedHint: "docker-compose-start",
			description:  "Docker compose commands should generate docker-compose-start hints",
		},
		{
			name:         "Bash fallback should use -start suffix",
			command:      "",
			expectedHint: "bash-start",
			description:  "Empty commands should fallback to bash-start for Bash tool",
		},
		{
			name:         "Non-Bash tool should use -start suffix",
			command:      "",
			expectedHint: "read-start",
			description:  "Non-Bash tools should generate tool-start hints",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var testJSON string
			if tt.name == "Non-Bash tool should use -start suffix" {
				// Test with Read tool instead of Bash
				testJSON = fmt.Sprintf(`{
					"session_id": "test",
					"transcript_path": "/test",
					"cwd": "/test",
					"hook_event_name": "PreToolUse",
					"tool_name": "Read",
					"tool_input": {"file_path": "/test/file.txt"}
				}`)
			} else {
				// Test with Bash tool
				testJSON = fmt.Sprintf(`{
					"session_id": "test",
					"transcript_path": "/test",
					"cwd": "/test",
					"hook_event_name": "PreToolUse",
					"tool_name": "Bash",
					"tool_input": {"command": "%s"}
				}`, tt.command)
			}

			event, err := parser.Parse([]byte(testJSON))
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			context := event.GetContext()

			// Test will fail initially - parser still generates '-thinking' suffix
			if context.SoundHint != tt.expectedHint {
				t.Errorf("%s: expected hint '%s', got '%s'. %s",
					tt.name, tt.expectedHint, context.SoundHint, tt.description)
			}

			// Verify still in Loading category
			if context.Category != Loading {
				t.Errorf("%s: expected Loading category, got %s", tt.name, context.Category.String())
			}

			// Verify still has tool-start operation
			if context.Operation != "tool-start" {
				t.Errorf("%s: expected 'tool-start' operation, got '%s'", tt.name, context.Operation)
			}
		})
	}
}

// TDD Phase 2.5 RED: Test that existing PostToolUse events still work correctly
func TestPostToolUseSuffixesUnchanged(t *testing.T) {
	parser := NewHookEventParser()

	tests := []struct {
		name         string
		command      string
		stderr       string
		expectedHint string
		description  string
	}{
		{
			name:         "Git commit success should use -success suffix",
			command:      "git commit -m 'test'",
			stderr:       "",
			expectedHint: "git-commit-success",
			description:  "PostToolUse success events should continue using -success suffix",
		},
		{
			name:         "Git commit error should use -error suffix",
			command:      "git commit -m 'test'",
			stderr:       "nothing to commit",
			expectedHint: "git-commit-error",
			description:  "PostToolUse error events should continue using -error suffix",
		},
		{
			name:         "NPM install success should use -success suffix",
			command:      "npm install express",
			stderr:       "",
			expectedHint: "npm-install-success",
			description:  "NPM PostToolUse success should continue using -success suffix",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testJSON := fmt.Sprintf(`{
				"session_id": "test",
				"transcript_path": "/test",
				"cwd": "/test",
				"hook_event_name": "PostToolUse",
				"tool_name": "Bash",
				"tool_input": {"command": "%s"},
				"tool_response": {"stdout": "output", "stderr": "%s", "interrupted": false}
			}`, tt.command, tt.stderr)

			event, err := parser.Parse([]byte(testJSON))
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			context := event.GetContext()

			// These should continue working correctly
			if context.SoundHint != tt.expectedHint {
				t.Errorf("%s: expected hint '%s', got '%s'. %s",
					tt.name, tt.expectedHint, context.SoundHint, tt.description)
			}

			// Verify correct category based on stderr
			expectedCategory := Success
			if tt.stderr != "" {
				expectedCategory = Error
			}
			if context.Category != expectedCategory {
				t.Errorf("%s: expected %s category, got %s",
					tt.name, expectedCategory.String(), context.Category.String())
			}
		})
	}
}
