package hooks

import (
	"encoding/json"
	"testing"
)

// getContextCase is one GetContext input and the full EventContext it must
// produce. Empty input/response strings mean the field is absent; a nil
// message means no "message" key.
type getContextCase struct {
	name            string
	event, tool     string
	input, response string
	message         *string
	want            EventContext
}

func ptr(s string) *string { return &s }

func (tc getContextCase) hookEvent() *HookEvent {
	event := &HookEvent{SessionID: "s", CWD: "/c", EventName: tc.event, Message: tc.message}
	if tc.tool != "" {
		event.ToolName = ptr(tc.tool)
	}
	if tc.input != "" {
		raw := json.RawMessage(tc.input)
		event.ToolInput = &raw
	}
	if tc.response != "" {
		raw := json.RawMessage(tc.response)
		event.ToolResponse = &raw
	}
	return event
}

// TestGetContext is the single table for HookEvent.GetContext: every
// lifecycle event, the tool-event shapes of each supported agent (Claude,
// Codex, Gemini, Qwen, Copilot), MCP normalization, tool_response
// success/error analysis and the typed Command/Subcommand/Phase fields.
// Each row pins the whole EventContext.
func TestGetContext(t *testing.T) {
	t.Parallel()
	tests := []getContextCase{
		{name: "Gemini BeforeModel is silent", event: "BeforeModel",
			want: EventContext{Category: Silent, Operation: "beforemodel"}},
		{name: "Gemini AfterModel is silent", event: "AfterModel",
			want: EventContext{Category: Silent, Operation: "aftermodel"}},
		{name: "Gemini BeforeToolSelection is silent", event: "BeforeToolSelection",
			want: EventContext{Category: Silent, Operation: "beforetoolselection"}},
		{name: "UserPromptSubmit", event: "UserPromptSubmit",
			want: EventContext{Category: Interactive, SoundHint: "message-sent", Operation: "prompt"}},
		{name: "PreToolUse Bash ls with path", event: "PreToolUse", tool: "Bash", input: `{"command":"ls -la /tmp/claudio-hook-logs/","description":"Check if hook logs have been created"}`,
			want: EventContext{Category: Loading, ToolName: "ls", OriginalTool: "Bash", SoundHint: "ls-start", Operation: "tool-start", Command: "ls", Phase: "start"}},
		{name: "PostToolUse Bash ls with path success", event: "PostToolUse", tool: "Bash", input: `{"command":"ls -la /tmp/claudio-hook-logs/","description":"Check if hook logs have been created"}`, response: `{"stdout":"total 288","stderr":"","interrupted":false,"isImage":false}`,
			want: EventContext{Category: Success, ToolName: "ls", OriginalTool: "Bash", SoundHint: "ls-success", Operation: "tool-complete", Command: "ls", Phase: "success", IsSuccess: true}},
		{name: "Notification permission", event: "Notification", message: ptr("Claude needs your permission to use Read"),
			want: EventContext{Category: Interactive, SoundHint: "notification-permission", Operation: "notification"}},
		{name: "Notification permission alternative phrasing", event: "Notification", message: ptr("Claude needs permission to run bash command"),
			want: EventContext{Category: Interactive, SoundHint: "notification-permission", Operation: "notification"}},
		{name: "Notification idle", event: "Notification", message: ptr("Prompt has been idle for 60+ seconds"),
			want: EventContext{Category: Interactive, SoundHint: "notification-idle", Operation: "notification"}},
		{name: "Notification generic", event: "Notification", message: ptr("Some other notification message"),
			want: EventContext{Category: Interactive, SoundHint: "notification", Operation: "notification"}},
		{name: "Notification empty message", event: "Notification", message: ptr(""),
			want: EventContext{Category: Interactive, SoundHint: "notification", Operation: "notification"}},
		{name: "Notification nil message", event: "Notification",
			want: EventContext{Category: Interactive, SoundHint: "notification", Operation: "notification"}},
		{name: "Notification has been idle", event: "Notification", message: ptr("Claude has been idle for 60s"),
			want: EventContext{Category: Interactive, SoundHint: "notification-idle", Operation: "notification"}},
		{name: "PostToolUse Bash git commit success", event: "PostToolUse", tool: "Bash", input: `{"command":"git commit -m 'fix'"}`, response: `{"stdout":"committed","stderr":"","interrupted":false}`,
			want: EventContext{Category: Success, ToolName: "git", OriginalTool: "Bash", SoundHint: "git-commit-success", Operation: "tool-complete", Command: "git", Subcommand: "commit", Phase: "success", IsSuccess: true}},
		{name: "PostToolUse Bash git commit error", event: "PostToolUse", tool: "Bash", input: `{"command":"git commit -m 'test'"}`, response: `{"stdout":"output","stderr":"nothing to commit","interrupted":false}`,
			want: EventContext{Category: Error, ToolName: "git", OriginalTool: "Bash", SoundHint: "git-commit-error", Operation: "tool-complete", Command: "git", Subcommand: "commit", Phase: "error", HasError: true}},
		{name: "PostToolUse Bash npm install success", event: "PostToolUse", tool: "Bash", input: `{"command":"npm install express"}`, response: `{"stdout":"output","stderr":"","interrupted":false}`,
			want: EventContext{Category: Success, ToolName: "npm", OriginalTool: "Bash", SoundHint: "npm-install-success", Operation: "tool-complete", Command: "npm", Subcommand: "install", Phase: "success", IsSuccess: true}},
		{name: "PreToolUse Bash npm install", event: "PreToolUse", tool: "Bash", input: `{"command":"npm install express"}`,
			want: EventContext{Category: Loading, ToolName: "npm", OriginalTool: "Bash", SoundHint: "npm-install-start", Operation: "tool-start", Command: "npm", Subcommand: "install", Phase: "start"}},
		{name: "PostToolUse Bash ls success", event: "PostToolUse", tool: "Bash", input: `{"command":"ls -la"}`, response: `{"stdout":"files","stderr":"","interrupted":false}`,
			want: EventContext{Category: Success, ToolName: "ls", OriginalTool: "Bash", SoundHint: "ls-success", Operation: "tool-complete", Command: "ls", Phase: "success", IsSuccess: true}},
		{name: "PostToolUse Bash empty command falls back to Bash", event: "PostToolUse", tool: "Bash", input: `{"command":""}`, response: `{"stdout":"","stderr":"","interrupted":false}`,
			want: EventContext{Category: Success, ToolName: "Bash", SoundHint: "bash-success", Operation: "tool-complete", Command: "Bash", Phase: "success", IsSuccess: true}},
		{name: "PreToolUse Bash ls", event: "PreToolUse", tool: "Bash", input: `{"command":"ls -la"}`,
			want: EventContext{Category: Loading, ToolName: "ls", OriginalTool: "Bash", SoundHint: "ls-start", Operation: "tool-start", Command: "ls", Phase: "start"}},
		{name: "PreToolUse Bash empty command falls back to Bash", event: "PreToolUse", tool: "Bash", input: `{"command":""}`,
			want: EventContext{Category: Loading, ToolName: "Bash", SoundHint: "bash-start", Operation: "tool-start", Command: "Bash", Phase: "start"}},
		{name: "PreToolUse Bash git status", event: "PreToolUse", tool: "Bash", input: `{"command":"git status"}`,
			want: EventContext{Category: Loading, ToolName: "git", OriginalTool: "Bash", SoundHint: "git-status-start", Operation: "tool-start", Command: "git", Subcommand: "status", Phase: "start"}},
		{name: "PreToolUse Bash git commit", event: "PreToolUse", tool: "Bash", input: `{"command":"git commit -m x"}`,
			want: EventContext{Category: Loading, ToolName: "git", OriginalTool: "Bash", SoundHint: "git-commit-start", Operation: "tool-start", Command: "git", Subcommand: "commit", Phase: "start"}},
		{name: "PreToolUse Bash docker compose up", event: "PreToolUse", tool: "Bash", input: `{"command":"docker compose up -d"}`,
			want: EventContext{Category: Loading, ToolName: "docker", OriginalTool: "Bash", SoundHint: "docker-compose-start", Operation: "tool-start", Command: "docker", Subcommand: "compose", Phase: "start"}},
		{name: "PreToolUse Bash docker-compose up", event: "PreToolUse", tool: "Bash", input: `{"command":"docker-compose up"}`,
			want: EventContext{Category: Loading, ToolName: "docker-compose", OriginalTool: "Bash", SoundHint: "docker-compose-up-start", Operation: "tool-start", Command: "docker-compose", Subcommand: "up", Phase: "start"}},
		{name: "PreToolUse Bash kubectl port-forward", event: "PreToolUse", tool: "Bash", input: `{"command":"kubectl port-forward svc/x 80"}`,
			want: EventContext{Category: Loading, ToolName: "kubectl", OriginalTool: "Bash", SoundHint: "kubectl-port-forward-start", Operation: "tool-start", Command: "kubectl", Subcommand: "port-forward", Phase: "start"}},
		{name: "PreToolUse Bash without tool_input", event: "PreToolUse", tool: "Bash",
			want: EventContext{Category: Loading, ToolName: "Bash", SoundHint: "bash-start", Operation: "tool-start", Command: "Bash", Phase: "start"}},
		{name: "PreToolUse Read", event: "PreToolUse", tool: "Read", input: `{"file_path":"/test/file.txt"}`,
			want: EventContext{Category: Loading, ToolName: "Read", SoundHint: "read-start", FileType: "txt", Operation: "tool-start", Command: "Read", Phase: "start"}},
		{name: "PreToolUse Edit", event: "PreToolUse", tool: "Edit",
			want: EventContext{Category: Loading, ToolName: "Edit", SoundHint: "edit-start", Operation: "tool-start", Command: "Edit", Phase: "start"}},
		{name: "PostToolUse Bash stderr is an error", event: "PostToolUse", tool: "Bash", input: `{"command":"false"}`, response: `{"stdout":"","stderr":"command failed","interrupted":false,"isImage":false}`,
			want: EventContext{Category: Error, ToolName: "false", OriginalTool: "Bash", SoundHint: "false-error", Operation: "tool-complete", Command: "false", Phase: "error", HasError: true}},
		{name: "PostToolUse Bash interrupted", event: "PostToolUse", tool: "Bash", input: `{"command":"sleep 10"}`, response: `{"stdout":"","stderr":"","interrupted":true,"isImage":false}`,
			want: EventContext{Category: Error, ToolName: "sleep", OriginalTool: "Bash", SoundHint: "tool-interrupted", Operation: "tool-complete", Command: "sleep", Subcommand: "10", Phase: "error", HasError: true}},
		{name: "PostToolUse Bash git push success", event: "PostToolUse", tool: "Bash", input: `{"command":"git push"}`, response: `{"stderr":""}`,
			want: EventContext{Category: Success, ToolName: "git", OriginalTool: "Bash", SoundHint: "git-push-success", Operation: "tool-complete", Command: "git", Subcommand: "push", Phase: "success", IsSuccess: true}},
		{name: "PostToolUse Bash git push error", event: "PostToolUse", tool: "Bash", input: `{"command":"git push"}`, response: `{"stderr":"boom"}`,
			want: EventContext{Category: Error, ToolName: "git", OriginalTool: "Bash", SoundHint: "git-push-error", Operation: "tool-complete", Command: "git", Subcommand: "push", Phase: "error", HasError: true}},
		{name: "Stop", event: "Stop",
			want: EventContext{Category: Completion, SoundHint: "agent-complete", Operation: "stop"}},
		{name: "SubagentStop", event: "SubagentStop",
			want: EventContext{Category: Completion, SoundHint: "subagent-complete", Operation: "subagent-stop"}},
		{name: "PreCompact", event: "PreCompact",
			want: EventContext{Category: System, SoundHint: "compacting", Operation: "compact"}},
		{name: "SessionStart", event: "SessionStart",
			want: EventContext{Category: System, SoundHint: "session-start", Operation: "session-start"}},
		{name: "PermissionRequest", event: "PermissionRequest",
			want: EventContext{Category: Interactive, SoundHint: "permission-request", Operation: "permission-request"}},
		{name: "SessionEnd", event: "SessionEnd",
			want: EventContext{Category: Interactive, SoundHint: "session-end", Operation: "session-end"}},
		{name: "SubagentStart", event: "SubagentStart",
			want: EventContext{Category: Loading, SoundHint: "subagent-start", Operation: "subagent-start"}},
		{name: "PostCompact", event: "PostCompact",
			want: EventContext{Category: System, SoundHint: "post-compact", Operation: "post-compact"}},
		{name: "MCP PreToolUse context7", event: "PreToolUse", tool: "mcp__context7__query-docs", input: "{}",
			want: EventContext{Category: Loading, ToolName: "mcp", OriginalTool: "mcp__context7__query-docs", SoundHint: "mcp-start", Operation: "tool-start", Command: "mcp", Phase: "start"}},
		{name: "MCP PreToolUse filesystem", event: "PreToolUse", tool: "mcp__filesystem__read_file",
			want: EventContext{Category: Loading, ToolName: "mcp", OriginalTool: "mcp__filesystem__read_file", SoundHint: "mcp-start", Operation: "tool-start", Command: "mcp", Phase: "start"}},
		{name: "MCP PreToolUse github", event: "PreToolUse", tool: "mcp__github__create_issue",
			want: EventContext{Category: Loading, ToolName: "mcp", OriginalTool: "mcp__github__create_issue", SoundHint: "mcp-start", Operation: "tool-start", Command: "mcp", Phase: "start"}},
		{name: "MCP Gemini BeforeTool", event: "BeforeTool", tool: "mcp_filesystem_read_file", input: "{}",
			want: EventContext{Category: Loading, ToolName: "mcp", OriginalTool: "mcp_filesystem_read_file", SoundHint: "mcp-start", Operation: "tool-start", Command: "mcp", Phase: "start"}},
		{name: "MCP PostToolUse success", event: "PostToolUse", tool: "mcp__github__create_issue", input: "{}", response: `{"content":"issue created","isError":false}`,
			want: EventContext{Category: Success, ToolName: "mcp", OriginalTool: "mcp__github__create_issue", SoundHint: "mcp-success", Operation: "tool-complete", Command: "mcp", Phase: "success", IsSuccess: true}},
		{name: "MCP Gemini AfterTool success", event: "AfterTool", tool: "mcp_filesystem_read_file", input: "{}", response: `{"content":"file contents","isError":false}`,
			want: EventContext{Category: Success, ToolName: "mcp", OriginalTool: "mcp_filesystem_read_file", SoundHint: "mcp-success", Operation: "tool-complete", Command: "mcp", Phase: "success", IsSuccess: true}},
		{name: "MCP PostToolUse error", event: "PostToolUse", tool: "mcp__slack__send_message", input: "{}", response: `{"content":"rate limited","isError":true}`,
			want: EventContext{Category: Error, ToolName: "mcp", OriginalTool: "mcp__slack__send_message", SoundHint: "mcp-error", Operation: "tool-complete", Command: "mcp", Phase: "error", HasError: true}},
		{name: "MCP PostToolUse interrupted", event: "PostToolUse", tool: "mcp__github__create_issue", input: "{}", response: `{"content":"","interrupted":true,"isError":true}`,
			want: EventContext{Category: Error, ToolName: "mcp", OriginalTool: "mcp__github__create_issue", SoundHint: "tool-interrupted", Operation: "tool-complete", Command: "mcp", Phase: "error", HasError: true}},
		{name: "PreToolUse Read without input", event: "PreToolUse", tool: "Read", input: "{}",
			want: EventContext{Category: Loading, ToolName: "Read", SoundHint: "read-start", Operation: "tool-start", Command: "Read", Phase: "start"}},
		{name: "Codex apply_patch PreToolUse", event: "PreToolUse", tool: "apply_patch",
			want: EventContext{Category: Loading, ToolName: "apply_patch", SoundHint: "apply_patch-start", Operation: "tool-start", Command: "apply_patch", Phase: "start"}},
		{name: "Codex apply_patch PostToolUse success", event: "PostToolUse", tool: "apply_patch", response: `{"output":"done"}`,
			want: EventContext{Category: Success, ToolName: "apply_patch", SoundHint: "apply_patch-success", Operation: "tool-complete", Command: "apply_patch", Phase: "success", IsSuccess: true}},
		{name: "Codex string tool_response exit 0", event: "PostToolUse", tool: "Bash", input: `{"command":"git status --short"}`, response: `"Exit code: 0\nWall time: 0.1 seconds\nOutput:\nclean"`,
			want: EventContext{Category: Success, ToolName: "git", OriginalTool: "Bash", SoundHint: "git-status-success", Operation: "tool-complete", Command: "git", Subcommand: "status", Phase: "success", IsSuccess: true}},
		{name: "Codex string tool_response exit 1", event: "PostToolUse", tool: "Bash", input: `{"command":"git status --short"}`, response: `"Exit code: 1\nWall time: 0.1 seconds\nOutput:\nfatal: not a git repository"`,
			want: EventContext{Category: Error, ToolName: "git", OriginalTool: "Bash", SoundHint: "git-status-error", Operation: "tool-complete", Command: "git", Subcommand: "status", Phase: "error", HasError: true}},
		{name: "Gemini BeforeTool run_shell_command", event: "BeforeTool", tool: "run_shell_command", input: `{"command":"git status --short"}`,
			want: EventContext{Category: Loading, ToolName: "git", OriginalTool: "Bash", SoundHint: "git-status-start", Operation: "tool-start", Command: "git", Subcommand: "status", Phase: "start"}},
		{name: "Gemini AfterTool write_file success", event: "AfterTool", tool: "write_file", response: `{"success":true}`,
			want: EventContext{Category: Success, ToolName: "Write", SoundHint: "write-success", Operation: "tool-complete", Command: "Write", Phase: "success", IsSuccess: true}},
		{name: "Gemini BeforeAgent", event: "BeforeAgent",
			want: EventContext{Category: Interactive, SoundHint: "before-agent", Operation: "before-agent"}},
		{name: "Gemini AfterAgent", event: "AfterAgent",
			want: EventContext{Category: Completion, SoundHint: "agent-complete", Operation: "after-agent"}},
		{name: "Gemini PreCompress", event: "PreCompress",
			want: EventContext{Category: System, SoundHint: "compacting", Operation: "compact"}},
		{name: "Qwen PostToolUseFailure run_shell_command", event: "PostToolUseFailure", tool: "run_shell_command", input: `{"command":"git status"}`, response: `{"stderr":"fatal: not a git repository"}`,
			want: EventContext{Category: Error, ToolName: "git", OriginalTool: "Bash", SoundHint: "git-status-error", Operation: "tool-complete", Command: "git", Subcommand: "status", Phase: "error", HasError: true}},
		{name: "PostToolUseFailure Read", event: "PostToolUseFailure", tool: "Read",
			want: EventContext{Category: Error, ToolName: "Read", SoundHint: "read-error", Operation: "tool-complete", Command: "Read", Phase: "error", HasError: true}},
		{name: "ErrorOccurred", event: "ErrorOccurred",
			want: EventContext{Category: Error, SoundHint: "error-occurred", Operation: "error-occurred", HasError: true}},
		{name: "Copilot powershell", event: "PreToolUse", tool: "powershell", input: `{"command":"git status"}`,
			want: EventContext{Category: Loading, ToolName: "git", OriginalTool: "Bash", SoundHint: "git-status-start", Operation: "tool-start", Command: "git", Subcommand: "status", Phase: "start"}},
		{name: "Command Code shell_command with args", event: "PreToolUse", tool: "shell_command", input: `{"command":"git","args":["commit","-m","x"]}`,
			want: EventContext{Category: Loading, ToolName: "git", OriginalTool: "Bash", SoundHint: "git-commit-start", Operation: "tool-start", Command: "git", Subcommand: "commit", Phase: "start"}},
		{name: "OpenCode bash exit 2", event: "PostToolUse", tool: "bash", input: `{"command":"go test ./..."}`, response: `"Exit code: 2"`,
			want: EventContext{Category: Error, ToolName: "go", OriginalTool: "Bash", SoundHint: "go-test-error", Operation: "tool-complete", Command: "go", Subcommand: "test", Phase: "error", HasError: true}},
		{name: "Copilot view", event: "PreToolUse", tool: "view",
			want: EventContext{Category: Loading, ToolName: "Read", SoundHint: "read-start", Operation: "tool-start", Command: "Read", Phase: "start"}},
		{name: "apply_patch isError", event: "PostToolUse", tool: "apply_patch", response: `{"isError":true}`,
			want: EventContext{Category: Error, ToolName: "apply_patch", SoundHint: "apply_patch-error", Operation: "tool-complete", Command: "apply_patch", Phase: "error", HasError: true}},
		{name: "Bash interrupted without input", event: "PostToolUse", tool: "Bash", response: `{"interrupted":true}`,
			want: EventContext{Category: Error, ToolName: "Bash", SoundHint: "tool-interrupted", Operation: "tool-complete", Command: "Bash", Phase: "error", HasError: true}},
		{name: "Read with content succeeds", event: "PostToolUse", tool: "Read", response: `{"content":"hello"}`,
			want: EventContext{Category: Success, ToolName: "Read", SoundHint: "read-success", Operation: "tool-complete", Command: "Read", Phase: "success", IsSuccess: true}},
		{name: "Read without content fails", event: "PostToolUse", tool: "Read", response: "{}",
			want: EventContext{Category: Error, ToolName: "Read", SoundHint: "read-error", Operation: "tool-complete", Command: "Read", Phase: "error", HasError: true}},
		{name: "Edit success false fails", event: "PostToolUse", tool: "Edit", response: `{"success":false}`,
			want: EventContext{Category: Error, ToolName: "Edit", SoundHint: "edit-error", Operation: "tool-complete", Command: "Edit", Phase: "error", HasError: true}},
		{name: "Edit success true succeeds", event: "PostToolUse", tool: "Edit", response: `{"success":true}`,
			want: EventContext{Category: Success, ToolName: "Edit", SoundHint: "edit-success", Operation: "tool-complete", Command: "Edit", Phase: "success", IsSuccess: true}},
		{name: "Grep numLines 0 succeeds", event: "PostToolUse", tool: "Grep", response: `{"numLines":0}`,
			want: EventContext{Category: Success, ToolName: "Grep", SoundHint: "grep-success", Operation: "tool-complete", Command: "Grep", Phase: "success", IsSuccess: true}},
		{name: "Grep without numLines succeeds", event: "PostToolUse", tool: "Grep", response: "{}",
			want: EventContext{Category: Success, ToolName: "Grep", SoundHint: "grep-success", Operation: "tool-complete", Command: "Grep", Phase: "success", IsSuccess: true}},
		{name: "unparseable tool_response is an error", event: "PostToolUse", tool: "Bash", response: "not json",
			want: EventContext{Category: Error, ToolName: "Bash", SoundHint: "bash-error", Operation: "tool-complete", Command: "Bash", Phase: "error", HasError: true}},
		{name: "unknown event", event: "SomethingNew",
			want: EventContext{Category: Interactive, SoundHint: "default", Operation: "unknown"}},
		{name: "PreToolUse without tool", event: "PreToolUse",
			want: EventContext{Category: Loading, SoundHint: "tool-loading", Operation: "tool-start", Phase: "start"}},
		{name: "PostToolUse without tool succeeds", event: "PostToolUse",
			want: EventContext{Category: Success, SoundHint: "tool-success", Operation: "tool-complete", Phase: "success", IsSuccess: true}},
		{name: "PostToolUse without tool errors", event: "PostToolUse", response: `{"error":"boom"}`,
			want: EventContext{Category: Error, SoundHint: "tool-error", Operation: "tool-complete", Phase: "error", HasError: true}},
		{name: "Read interrupted", event: "PostToolUse", tool: "Read", response: `{"interrupted":true}`,
			want: EventContext{Category: Error, ToolName: "Read", SoundHint: "tool-interrupted", Operation: "tool-complete", Command: "Read", Phase: "error", HasError: true}},
		{name: "apply_patch non-string error", event: "PostToolUse", tool: "apply_patch", response: `{"error":{"message":"boom"}}`,
			want: EventContext{Category: Error, ToolName: "apply_patch", SoundHint: "apply_patch-error", Operation: "tool-complete", Command: "apply_patch", Phase: "error", HasError: true}},
		{name: "Write without explicit success", event: "PostToolUse", tool: "Write", response: "{}",
			want: EventContext{Category: Success, ToolName: "Write", SoundHint: "write-success", Operation: "tool-complete", Command: "Write", Phase: "success", IsSuccess: true}},
		{name: "Setup", event: "Setup",
			want: EventContext{Category: System, SoundHint: "setup", Operation: "setup"}},
		{name: "UserPromptExpansion", event: "UserPromptExpansion",
			want: EventContext{Category: Interactive, SoundHint: "prompt-expansion", Operation: "prompt-expansion"}},
		{name: "PermissionDenied", event: "PermissionDenied",
			want: EventContext{Category: Error, SoundHint: "permission-denied", Operation: "permission-denied", HasError: true}},
		{name: "PostToolBatch", event: "PostToolBatch",
			want: EventContext{Category: Success, SoundHint: "tool-batch", Operation: "tool-batch"}},
		{name: "MessageDisplay", event: "MessageDisplay",
			want: EventContext{Category: Silent, Operation: "message-display"}},
		{name: "TaskCreated", event: "TaskCreated",
			want: EventContext{Category: Loading, SoundHint: "task-created", Operation: "task-created"}},
		{name: "TaskCompleted", event: "TaskCompleted",
			want: EventContext{Category: Completion, SoundHint: "task-completed", Operation: "task-completed"}},
		{name: "StopFailure", event: "StopFailure",
			want: EventContext{Category: Error, SoundHint: "stop-failure", Operation: "stop-failure", HasError: true}},
		{name: "TeammateIdle", event: "TeammateIdle",
			want: EventContext{Category: Interactive, SoundHint: "teammate-idle", Operation: "teammate-idle"}},
		{name: "InstructionsLoaded", event: "InstructionsLoaded",
			want: EventContext{Category: System, SoundHint: "instructions-loaded", Operation: "instructions-loaded"}},
		{name: "ConfigChange", event: "ConfigChange",
			want: EventContext{Category: System, SoundHint: "config-change", Operation: "config-change"}},
		{name: "CwdChanged", event: "CwdChanged",
			want: EventContext{Category: System, SoundHint: "cwd-changed", Operation: "cwd-changed"}},
		{name: "FileChanged", event: "FileChanged",
			want: EventContext{Category: System, SoundHint: "file-changed", Operation: "file-changed"}},
		{name: "WorktreeCreate", event: "WorktreeCreate",
			want: EventContext{Category: System, SoundHint: "worktree-create", Operation: "worktree-create"}},
		{name: "WorktreeRemove", event: "WorktreeRemove",
			want: EventContext{Category: System, SoundHint: "worktree-remove", Operation: "worktree-remove"}},
		{name: "Elicitation", event: "Elicitation",
			want: EventContext{Category: Interactive, SoundHint: "elicitation", Operation: "elicitation"}},
		{name: "ElicitationResult", event: "ElicitationResult",
			want: EventContext{Category: Interactive, SoundHint: "elicitation-result", Operation: "elicitation-result"}},
		{name: "TodoCreated", event: "TodoCreated",
			want: EventContext{Category: Loading, SoundHint: "todo-created", Operation: "todo-created"}},
		{name: "TodoCompleted", event: "TodoCompleted",
			want: EventContext{Category: Completion, SoundHint: "todo-completed", Operation: "todo-completed"}},
		{name: "PostToolUseFailure without tool", event: "PostToolUseFailure",
			want: EventContext{Category: Error, SoundHint: "tool-error", Operation: "tool-complete", Phase: "error", HasError: true}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := *tc.hookEvent().GetContext(); got != tc.want {
				t.Errorf("GetContext()\n got %+v\nwant %+v", got, tc.want)
			}
		})
	}
}
