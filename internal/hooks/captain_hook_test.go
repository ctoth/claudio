package hooks

import "testing"

// These payloads were captured live (Codex 0.161.0-alpha.2, Copilot CLI
// 1.0.91) with cmd/hook-logger, then edited: ids and paths were shortened
// and the shell commands replaced ("exit 3" became "git push") so the
// expected sound names read naturally. The one payload that is not a
// capture says so. They cover what reading payloads through captain-hook
// changed.

// Codex gives hooks a shell command's output and never its exit code, so a
// finished shell command is neither a success nor an error. This one
// exited 3.
func TestCodexShellOutcomeIsUnknown(t *testing.T) {
	const payload = `{"session_id":"s","turn_id":"t","transcript_path":"C:\\home\\transcript.jsonl","cwd":"C:\\proj","hook_event_name":"PostToolUse","model":"gpt-6.1-sol","permission_mode":"bypassPermissions","tool_name":"Bash","tool_input":{"command":"git push"},"tool_response":"","tool_use_id":"exec-1"}`

	for _, agent := range []string{"codex", ""} { // named by --hook-agent, or recognized from turn_id
		event, err := ParseHookEventFrom(agent, []byte(payload), "")
		if err != nil {
			t.Fatalf("agent %q: %v", agent, err)
		}
		want := EventContext{
			Category: Success, ToolName: "git", OriginalTool: "Bash",
			SoundHint: "tool-complete", Operation: "tool-complete",
			Command: "git", Subcommand: "push", Phase: PhaseUnknown,
		}
		if got := *event.GetContext(); got != want {
			t.Errorf("agent %q:\n got %+v\nwant %+v", agent, got, want)
		}
	}
}

// Copilot sends a permission request camelCase under either settings key,
// with the event in "hookName" and the arguments in "toolInput". claudio
// used to reject it for having no event name.
func TestCopilotPermissionRequest(t *testing.T) {
	const payload = `{"hookName":"permissionRequest","sessionId":"s","timestamp":1790909851552,"cwd":"C:\\proj","toolName":"powershell","toolInput":{"command":"echo hello"},"permissionSuggestions":[]}`

	event, err := ParseHookEventFrom("copilot", []byte(payload), "")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if event.EventName != "PermissionRequest" {
		t.Errorf("EventName = %q, want PermissionRequest", event.EventName)
	}
	ctx := event.GetContext()
	if ctx.Category != Interactive || ctx.SoundHint != "permission-request" {
		t.Errorf("context = %+v, want the interactive permission-request sound", *ctx)
	}
}

// Copilot's camelCase payloads name no event; the settings key does. Its
// runtime tool names ("powershell", "view") are the shell and Read.
func TestCopilotCamelCasePayloads(t *testing.T) {
	tests := []struct {
		name, key, payload, wantHint string
		want                         EventCategory
	}{
		{"shell exit 3", "postToolUse",
			`{"sessionId":"s","timestamp":1790909857071,"cwd":"C:\\proj","toolName":"powershell","toolArgs":{"command":"git push","description":"Push"},"toolResult":{"resultType":"success","textResultForLlm":"\n<shellId: 1 completed with exit code 3>"}}`,
			"git-push-error", Error},
		{"view", "postToolUse",
			`{"sessionId":"s","timestamp":1790909867083,"cwd":"C:\\proj","toolName":"view","toolArgs":{"path":"C:\\proj\\notes.txt"},"toolResult":{"resultType":"success","textResultForLlm":"alpha"}}`,
			"read-success", Success},
		// Not a capture: GitHub's docs describe toolArgs as a string of
		// JSON, and this follows their example. 1.0.91 sent an object.
		{"toolArgs as a string", "preToolUse",
			`{"sessionId":"s","timestamp":1790909850765,"cwd":"C:\\proj","toolName":"bash","toolArgs":"{\"command\":\"git status\"}"}`,
			"git-status-start", Loading},
		{"agentStop", "agentStop",
			`{"sessionId":"s","timestamp":1790909873265,"cwd":"C:\\proj","transcriptPath":"C:\\home\\transcript.jsonl","stopReason":"end_turn","stop_hook_active":false}`,
			"agent-complete", Completion},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, err := ParseHookEventFrom("copilot", []byte(tt.payload), tt.key)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			ctx := event.GetContext()
			if ctx.Category != tt.want || ctx.SoundHint != tt.wantHint {
				t.Errorf("Category, SoundHint = %s, %q, want %s, %q", ctx.Category, ctx.SoundHint, tt.want, tt.wantHint)
			}
		})
	}
}
