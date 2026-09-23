package hooks

import (
	"encoding/json"
	"testing"
)

// TestGetContextTypedCommandFields pins the typed Command, Subcommand and
// Phase fields the mapper builds its command levels from. They replace the
// old approach of splitting SoundHint on '-', which broke on hyphenated
// commands (docker-compose) and subcommands (kubectl port-forward).
func TestGetContextTypedCommandFields(t *testing.T) {
	t.Parallel()
	bash := func(cmd string) string {
		data, _ := json.Marshal(map[string]string{"command": cmd})
		return string(data)
	}
	tests := []struct {
		name                string
		event, tool, input  string
		response            string
		command, sub, phase string
	}{
		{name: "git commit start", event: "PreToolUse", tool: "Bash", input: bash("git commit -m x"), command: "git", sub: "commit", phase: "start"},
		{name: "docker compose up", event: "PreToolUse", tool: "Bash", input: bash("docker compose up"), command: "docker", sub: "compose", phase: "start"},
		{name: "docker-compose up", event: "PreToolUse", tool: "Bash", input: bash("docker-compose up"), command: "docker-compose", sub: "up", phase: "start"},
		{name: "kubectl port-forward", event: "PreToolUse", tool: "Bash", input: bash("kubectl port-forward svc/x 80"), command: "kubectl", sub: "port-forward", phase: "start"},
		{name: "ls no subcommand", event: "PreToolUse", tool: "Bash", input: bash("ls /tmp"), command: "ls", phase: "start"},
		{name: "bash without command", event: "PreToolUse", tool: "Bash", command: "Bash", phase: "start"},
		{name: "edit start", event: "PreToolUse", tool: "Edit", command: "Edit", phase: "start"},
		{name: "mcp start", event: "PreToolUse", tool: "mcp__github__create_issue", command: "mcp", phase: "start"},
		{name: "git push success", event: "PostToolUse", tool: "Bash", input: bash("git push"), response: `{"stderr":""}`, command: "git", sub: "push", phase: "success"},
		{name: "git push error", event: "PostToolUse", tool: "Bash", input: bash("git push"), response: `{"stderr":"boom"}`, command: "git", sub: "push", phase: "error"},
		{name: "failure", event: "PostToolUseFailure", tool: "Read", command: "Read", phase: "error"},
		{name: "lifecycle has none", event: "Stop"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			payload := map[string]any{"session_id": "s", "cwd": "/c", "hook_event_name": tc.event}
			if tc.tool != "" {
				payload["tool_name"] = tc.tool
			}
			if tc.input != "" {
				payload["tool_input"] = json.RawMessage(tc.input)
			}
			if tc.response != "" {
				payload["tool_response"] = json.RawMessage(tc.response)
			}
			data, _ := json.Marshal(payload)
			event, err := ParseHookEvent(data)
			if err != nil {
				t.Fatal(err)
			}
			ctx := event.GetContext()
			if ctx.Command != tc.command || ctx.Subcommand != tc.sub || ctx.Phase != tc.phase {
				t.Errorf("got Command=%q Subcommand=%q Phase=%q, want %q %q %q",
					ctx.Command, ctx.Subcommand, ctx.Phase, tc.command, tc.sub, tc.phase)
			}
		})
	}
}
