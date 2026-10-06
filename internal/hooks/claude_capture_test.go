package hooks

import "testing"

// TestClaudeCapturedPayloads scores payloads captured from Claude Code
// 2.1.290 on Windows (2026-10-05) with cmd/hook-logger. Only session ids,
// paths and the common fields the parser ignores were removed.
//
// Claude Code reports a failed tool as PostToolUseFailure, so a PostToolUse
// is a tool that succeeded, whatever shape its tool_response has.
func TestClaudeCapturedPayloads(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		want    EventCategory
	}{
		{"PostToolUse PowerShell",
			`{"session_id":"s","cwd":"C:\\proj","hook_event_name":"PostToolUse","tool_name":"PowerShell","tool_input":{"command":"echo hello","description":"Print hello"},"tool_response":{"stdout":"hello","stderr":"","interrupted":false,"isImage":false},"duration_ms":1228}`,
			Success},
		{"PostToolUseFailure PowerShell exit 3",
			`{"session_id":"s","cwd":"C:\\proj","hook_event_name":"PostToolUseFailure","tool_name":"PowerShell","tool_input":{"command":"exit 3","description":"Exit the shell with code 3"},"error":"Exit code 3","is_interrupt":false,"duration_ms":698}`,
			Error},
		{"PostToolUse Write",
			`{"session_id":"s","cwd":"C:\\proj","hook_event_name":"PostToolUse","tool_name":"Write","tool_input":{"file_path":"C:\\proj\\notes.txt","content":"alpha\n"},"tool_response":{"type":"create","filePath":"C:\\proj\\notes.txt","content":"alpha\n","structuredPatch":[],"originalFile":null,"userModified":false},"duration_ms":29}`,
			Success},
		{"PostToolUse Read",
			`{"session_id":"s","cwd":"C:\\proj","hook_event_name":"PostToolUse","tool_name":"Read","tool_input":{"file_path":"C:\\proj\\notes.txt"},"tool_response":{"type":"text","file":{"filePath":"C:\\proj\\notes.txt","content":"alpha\n","numLines":2,"startLine":1,"totalLines":2}},"duration_ms":14}`,
			Success},
		{"PostToolUse Edit",
			`{"session_id":"s","cwd":"C:\\proj","hook_event_name":"PostToolUse","tool_name":"Edit","tool_input":{"file_path":"C:\\proj\\notes.txt","old_string":"alpha","new_string":"beta","replace_all":false},"tool_response":{"filePath":"C:\\proj\\notes.txt","oldString":"alpha","newString":"beta","originalFile":"alpha\n","structuredPatch":[{"oldStart":1,"oldLines":1,"newStart":1,"newLines":1,"lines":["-alpha","+beta"]}],"userModified":false,"replaceAll":false},"duration_ms":26}`,
			Success},
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
