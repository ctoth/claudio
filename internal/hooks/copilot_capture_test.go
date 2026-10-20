package hooks

import "testing"

// TestCopilotCapturedPayloads scores payloads captured from GitHub Copilot
// CLI 1.0.91 on Windows (2026-10-01) with cmd/hook-logger, one hook per
// settings key. Only session_id, timestamp and paths were shortened.
//
// Copilot wraps every tool result as {result_type, text_result_for_llm}
// (camelCase keys: resultType, textResultForLlm). A shell command that
// exits nonzero still arrives as a "success" PostToolUse; the exit code is
// only in the text.
func TestCopilotCapturedPayloads(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		want    EventCategory
	}{
		{"PascalCase PostToolUse Bash exit 0",
			`{"hook_event_name":"PostToolUse","session_id":"s","timestamp":"2026-10-02T02:57:31.000Z","cwd":"C:\\run","tool_name":"Bash","tool_input":{"command":"echo hello","description":"Echo hello"},"tool_result":{"result_type":"success","text_result_for_llm":"hello\n<shellId: 0 completed with exit code 0>"}}`,
			Success},
		{"PascalCase PostToolUse Bash exit 3",
			`{"hook_event_name":"PostToolUse","session_id":"s","timestamp":"2026-10-02T02:57:34.000Z","cwd":"C:\\run","tool_name":"Bash","tool_input":{"command":"exit 3","description":"Exit with code 3"},"tool_result":{"result_type":"success","text_result_for_llm":"\n<shellId: 1 completed with exit code 3>"}}`,
			Error},
		{"PascalCase PostToolUse Read",
			`{"hook_event_name":"PostToolUse","session_id":"s","timestamp":"2026-10-02T02:57:45.000Z","cwd":"C:\\run","tool_name":"Read","tool_input":{"path":"C:\\run\\notes.txt"},"tool_result":{"result_type":"success","text_result_for_llm":"alpha"}}`,
			Success},
		{"PascalCase PostToolUse Edit",
			`{"hook_event_name":"PostToolUse","session_id":"s","timestamp":"2026-10-02T02:57:48.000Z","cwd":"C:\\run","tool_name":"Edit","tool_input":{"path":"C:\\run\\notes.txt","old_str":"alpha","new_str":"beta"},"tool_result":{"result_type":"success","text_result_for_llm":"File C:\\run\\notes.txt updated with changes."}}`,
			Success},
		{"PascalCase PostToolUseFailure Write",
			`{"hook_event_name":"PostToolUseFailure","session_id":"s","timestamp":"2026-10-02T02:57:40.075Z","cwd":"C:\\run","tool_name":"Write","tool_input":{"path":"C:\\run\\notes.txt","file_text":"alpha"},"error":"Parent directory does not exist"}`,
			Error},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, err := ParseHookEvent([]byte(tt.payload))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if got := event.GetContext(); got.Category != tt.want {
				t.Errorf("Category = %s, want %s (context %+v)", got.Category, tt.want, *got)
			}
		})
	}
}
