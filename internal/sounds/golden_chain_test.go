package sounds

import (
	"context"
	"encoding/json"
	"slices"
	"testing"

	"claudio.click/internal/hooks"
)

// goldenEvent describes one hook payload fed through the real parser and
// mapper. Empty fields are omitted from the JSON.
type goldenEvent struct {
	name     string
	event    string
	tool     string
	input    string // raw JSON for tool_input
	response string // raw JSON for tool_response
	message  string
}

func (g goldenEvent) payload(t *testing.T) []byte {
	t.Helper()
	m := map[string]any{
		"session_id":      "golden",
		"transcript_path": "/t",
		"cwd":             "/c",
		"hook_event_name": g.event,
	}
	if g.tool != "" {
		m["tool_name"] = g.tool
	}
	if g.input != "" {
		m["tool_input"] = json.RawMessage(g.input)
	}
	if g.response != "" {
		m["tool_response"] = json.RawMessage(g.response)
	}
	if g.message != "" {
		m["message"] = g.message
	}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return data
}

func bashCmd(cmd string) string {
	data, _ := json.Marshal(map[string]string{"command": cmd})
	return string(data)
}

const (
	okResp  = `{"stdout":"ok","stderr":"","interrupted":false}`
	errResp = `{"stdout":"","stderr":"boom","interrupted":false}`
	intResp = `{"stdout":"","stderr":"","interrupted":true}`
)

// goldenEvents is a broad sweep over tools, commands and lifecycle events.
var goldenEvents = []goldenEvent{
	// Bash PreToolUse
	{name: "pre bash git commit", event: "PreToolUse", tool: "Bash", input: bashCmd("git commit -m x")},
	{name: "pre bash git status", event: "PreToolUse", tool: "Bash", input: bashCmd("git status")},
	{name: "pre bash git flag first", event: "PreToolUse", tool: "Bash", input: bashCmd("git --no-pager log")},
	{name: "pre bash npm install", event: "PreToolUse", tool: "Bash", input: bashCmd("npm install lodash")},
	{name: "pre bash npm start", event: "PreToolUse", tool: "Bash", input: bashCmd("npm start")},
	{name: "pre bash ls path", event: "PreToolUse", tool: "Bash", input: bashCmd("ls -la /tmp")},
	{name: "pre bash docker compose up", event: "PreToolUse", tool: "Bash", input: bashCmd("docker compose up")},
	{name: "pre bash docker-compose up", event: "PreToolUse", tool: "Bash", input: bashCmd("docker-compose up")},
	{name: "pre bash kubectl port-forward", event: "PreToolUse", tool: "Bash", input: bashCmd("kubectl port-forward svc/x 8080:80")},
	{name: "pre bash kubectl get", event: "PreToolUse", tool: "Bash", input: bashCmd("kubectl get pods")},
	{name: "pre bash go test", event: "PreToolUse", tool: "Bash", input: bashCmd("go test ./...")},
	{name: "pre bash cargo build", event: "PreToolUse", tool: "Bash", input: bashCmd("cargo build --release")},
	{name: "pre bash bash script", event: "PreToolUse", tool: "Bash", input: bashCmd("bash script.sh")},
	{name: "pre bash echo", event: "PreToolUse", tool: "Bash", input: bashCmd("echo hello")},
	{name: "pre bash systemctl", event: "PreToolUse", tool: "Bash", input: bashCmd("systemctl restart nginx")},
	{name: "pre bash python file", event: "PreToolUse", tool: "Bash", input: bashCmd("python3 foo.py")},
	{name: "pre bash uppercase", event: "PreToolUse", tool: "Bash", input: bashCmd("Git Commit")},
	{name: "pre bash empty command", event: "PreToolUse", tool: "Bash", input: bashCmd("")},
	{name: "pre bash no input", event: "PreToolUse", tool: "Bash"},
	{name: "pre gemini shell", event: "BeforeTool", tool: "run_shell_command", input: bashCmd("git push origin main")},
	{name: "pre powershell", event: "PreToolUse", tool: "PowerShell", input: bashCmd("Get-ChildItem")},

	// Bash PostToolUse success
	{name: "post bash git commit ok", event: "PostToolUse", tool: "Bash", input: bashCmd("git commit -m x"), response: okResp},
	{name: "post bash npm start ok", event: "PostToolUse", tool: "Bash", input: bashCmd("npm start"), response: okResp},
	{name: "post bash ls ok", event: "PostToolUse", tool: "Bash", input: bashCmd("ls -la"), response: okResp},
	{name: "post bash docker compose ok", event: "PostToolUse", tool: "Bash", input: bashCmd("docker compose up"), response: okResp},
	{name: "post bash docker-compose ok", event: "PostToolUse", tool: "Bash", input: bashCmd("docker-compose up"), response: okResp},
	{name: "post bash kubectl port-forward ok", event: "PostToolUse", tool: "Bash", input: bashCmd("kubectl port-forward svc/x 80"), response: okResp},
	{name: "post bash no input ok", event: "PostToolUse", tool: "Bash", response: okResp},
	{name: "post gemini shell ok", event: "AfterTool", tool: "run_shell_command", input: bashCmd("npm test"), response: okResp},
	{name: "post bash text exit 0", event: "PostToolUse", tool: "Bash", input: bashCmd("make"), response: `"Exit code: 0\nok"`},

	// Bash PostToolUse error
	{name: "post bash git push stderr", event: "PostToolUse", tool: "Bash", input: bashCmd("git push"), response: errResp},
	{name: "post bash ls stderr", event: "PostToolUse", tool: "Bash", input: bashCmd("ls /nope"), response: errResp},
	{name: "post bash git commit interrupted", event: "PostToolUse", tool: "Bash", input: bashCmd("git commit"), response: intResp},
	{name: "post bash no input stderr", event: "PostToolUse", tool: "Bash", response: errResp},
	{name: "post bash no input interrupted", event: "PostToolUse", tool: "Bash", response: intResp},
	{name: "post bash text exit 2", event: "PostToolUse", tool: "Bash", input: bashCmd("make"), response: `"Exit code: 2\nfail"`},
	{name: "failure bash git push", event: "PostToolUseFailure", tool: "Bash", input: bashCmd("git push"), response: okResp},
	{name: "failure bash kubectl port-forward", event: "PostToolUseFailure", tool: "Bash", input: bashCmd("kubectl port-forward x 1"), response: okResp},

	// File and other tools
	{name: "pre edit", event: "PreToolUse", tool: "Edit", input: `{"file_path":"/a/b.go"}`},
	{name: "post edit ok", event: "PostToolUse", tool: "Edit", input: `{"file_path":"/a/b.go"}`, response: `{"success":true}`},
	{name: "post edit fail", event: "PostToolUse", tool: "Edit", input: `{"file_path":"/a/b.go"}`, response: `{"success":false}`},
	{name: "failure edit", event: "PostToolUseFailure", tool: "Edit"},
	{name: "pre read", event: "PreToolUse", tool: "Read", input: `{"file_path":"/a/b.md"}`},
	{name: "post read ok", event: "PostToolUse", tool: "Read", response: `{"content":"x"}`},
	{name: "post read no content", event: "PostToolUse", tool: "Read", response: `{}`},
	{name: "post read interrupted", event: "PostToolUse", tool: "Read", response: intResp},
	{name: "pre write", event: "PreToolUse", tool: "Write"},
	{name: "post write ok", event: "PostToolUse", tool: "Write", response: `{}`},
	{name: "pre multiedit", event: "PreToolUse", tool: "MultiEdit"},
	{name: "post grep ok", event: "PostToolUse", tool: "Grep", response: `{"numLines":3}`},
	{name: "post glob ok", event: "PostToolUse", tool: "Glob", response: `{"content":[]}`},
	{name: "pre ls", event: "PreToolUse", tool: "LS"},
	{name: "pre webfetch", event: "PreToolUse", tool: "WebFetch"},
	{name: "post websearch ok", event: "PostToolUse", tool: "WebSearch", response: `{}`},
	{name: "pre todowrite", event: "PreToolUse", tool: "TodoWrite"},
	{name: "pre task", event: "PreToolUse", tool: "Task"},
	{name: "post task ok", event: "PostToolUse", tool: "Task", response: `{}`},
	{name: "post task error field", event: "PostToolUse", tool: "Task", response: `{"error":"bad"}`},
	{name: "post gemini read_file ok", event: "AfterTool", tool: "read_file", response: `{"content":"x"}`},
	{name: "post no response", event: "PostToolUse", tool: "Edit"},
	{name: "post unparseable response", event: "PostToolUse", tool: "Edit", response: `[1,2]`},

	// MCP
	{name: "pre mcp github", event: "PreToolUse", tool: "mcp__github__create_issue"},
	{name: "post mcp github ok", event: "PostToolUse", tool: "mcp__github__create_issue", response: `{}`},
	{name: "post mcp github isError", event: "PostToolUse", tool: "mcp__github__create_issue", response: `{"isError":true}`},
	{name: "post mcp interrupted", event: "PostToolUse", tool: "mcp__x__y", response: intResp},
	{name: "failure mcp", event: "PostToolUseFailure", tool: "mcp_foo"},
	{name: "pre read_mcp_resource", event: "PreToolUse", tool: "read_mcp_resource"},

	// Tool events without a tool name
	{name: "pre no tool", event: "PreToolUse"},
	{name: "post no tool", event: "PostToolUse"},
	{name: "post no tool stderr", event: "PostToolUse", response: errResp},
	{name: "post no tool interrupted", event: "PostToolUse", response: intResp},

	// Lifecycle events
	{name: "UserPromptSubmit", event: "UserPromptSubmit"},
	{name: "UserPromptExpansion", event: "UserPromptExpansion"},
	{name: "Notification generic", event: "Notification", message: "hello"},
	{name: "Notification none", event: "Notification"},
	{name: "Notification permission", event: "Notification", message: "Claude needs your permission"},
	{name: "Notification idle", event: "Notification", message: "Claude has been idle"},
	{name: "PostToolBatch", event: "PostToolBatch"},
	{name: "Stop", event: "Stop"},
	{name: "SubagentStop", event: "SubagentStop"},
	{name: "SubagentStart", event: "SubagentStart"},
	{name: "subagentStart alias", event: "subagentStart"},
	{name: "PostCompact", event: "PostCompact"},
	{name: "PreCompact", event: "PreCompact"},
	{name: "PreCompress", event: "PreCompress"},
	{name: "SessionStart", event: "SessionStart"},
	{name: "PermissionRequest", event: "PermissionRequest"},
	{name: "PermissionDenied", event: "PermissionDenied"},
	{name: "SessionEnd", event: "SessionEnd"},
	{name: "BeforeAgent", event: "BeforeAgent"},
	{name: "AfterAgent", event: "AfterAgent"},
	{name: "StopFailure", event: "StopFailure"},
	{name: "ErrorOccurred", event: "ErrorOccurred"},
	{name: "Setup", event: "Setup"},
	{name: "MessageDisplay", event: "MessageDisplay"},
	{name: "TaskCreated", event: "TaskCreated"},
	{name: "TaskCompleted", event: "TaskCompleted"},
	{name: "TeammateIdle", event: "TeammateIdle"},
	{name: "InstructionsLoaded", event: "InstructionsLoaded"},
	{name: "ConfigChange", event: "ConfigChange"},
	{name: "CwdChanged", event: "CwdChanged"},
	{name: "FileChanged", event: "FileChanged"},
	{name: "WorktreeCreate", event: "WorktreeCreate"},
	{name: "WorktreeRemove", event: "WorktreeRemove"},
	{name: "Elicitation", event: "Elicitation"},
	{name: "ElicitationResult", event: "ElicitationResult"},
	{name: "TodoCreated", event: "TodoCreated"},
	{name: "TodoCompleted", event: "TodoCompleted"},
	{name: "BeforeModel", event: "BeforeModel"},
	{name: "AfterModel", event: "AfterModel"},
	{name: "BeforeToolSelection", event: "BeforeToolSelection"},
	{name: "unknown event", event: "SomethingNew"},
	{name: "unknown event with tool", event: "SomethingNew", tool: "Edit"},
}

// goldenChain is the chain type plus the exact candidate list the mapper
// produced. A nil paths slice means the event is silent.
type goldenChain struct {
	chain string
	paths []string
}

func mapGoldenEvent(t *testing.T, g goldenEvent) goldenChain {
	t.Helper()
	event, err := hooks.ParseHookEvent(g.payload(t))
	if err != nil {
		t.Fatalf("parse %s: %v", g.name, err)
	}
	result := NewSoundMapper().MapSound(context.Background(), event.GetContext())
	if result == nil {
		return goldenChain{}
	}
	return goldenChain{chain: string(result.ChainType), paths: result.AllPaths}
}

// TestMapperGoldenChains pins the exact candidate list the parser and mapper
// produce for every event above. Any refactor of GetContext or the mapper
// chains must keep this table green.
func TestMapperGoldenChains(t *testing.T) {
	if len(goldenChains) != len(goldenEvents) {
		t.Fatalf("golden table has %d entries for %d events", len(goldenChains), len(goldenEvents))
	}
	for _, g := range goldenEvents {
		t.Run(g.name, func(t *testing.T) {
			want, ok := goldenChains[g.name]
			if !ok {
				t.Fatalf("no golden entry for %q", g.name)
			}
			got := mapGoldenEvent(t, g)
			if got.chain != want.chain || !slices.Equal(got.paths, want.paths) {
				t.Errorf("chain mismatch\n got: %s %q\nwant: %s %q", got.chain, got.paths, want.chain, want.paths)
			}
		})
	}
}

// goldenChains is the recorded output. The docker-compose and kubectl
// port-forward rows carry the #8 fix: the command-subcommand level uses the
// parsed command and subcommand instead of splitting the hint on '-'.
var goldenChains = map[string]goldenChain{
	"pre bash git commit":               {"enhanced", []string{"loading/git-commit-start.wav", "loading/git-commit.wav", "loading/git-start.wav", "loading/git.wav", "loading/bash-start.wav", "loading/bash.wav", "loading/tool-start.wav", "loading/loading.wav", "default.wav"}},
	"pre bash git status":               {"enhanced", []string{"loading/git-status-start.wav", "loading/git-status.wav", "loading/git-start.wav", "loading/git.wav", "loading/bash-start.wav", "loading/bash.wav", "loading/tool-start.wav", "loading/loading.wav", "default.wav"}},
	"pre bash git flag first":           {"enhanced", []string{"loading/git-log-start.wav", "loading/git-log.wav", "loading/git-start.wav", "loading/git.wav", "loading/bash-start.wav", "loading/bash.wav", "loading/tool-start.wav", "loading/loading.wav", "default.wav"}},
	"pre bash npm install":              {"enhanced", []string{"loading/npm-install-start.wav", "loading/npm-install.wav", "loading/npm-start.wav", "loading/npm.wav", "loading/bash-start.wav", "loading/bash.wav", "loading/tool-start.wav", "loading/loading.wav", "default.wav"}},
	"pre bash npm start":                {"enhanced", []string{"loading/npm-start-start.wav", "loading/npm-start.wav", "loading/npm.wav", "loading/bash-start.wav", "loading/bash.wav", "loading/tool-start.wav", "loading/loading.wav", "default.wav"}},
	"pre bash ls path":                  {"enhanced", []string{"loading/ls-start.wav", "loading/ls.wav", "loading/bash-start.wav", "loading/bash.wav", "loading/tool-start.wav", "loading/loading.wav", "default.wav"}},
	"pre bash docker compose up":        {"enhanced", []string{"loading/docker-compose-start.wav", "loading/docker-compose.wav", "loading/docker-start.wav", "loading/docker.wav", "loading/bash-start.wav", "loading/bash.wav", "loading/tool-start.wav", "loading/loading.wav", "default.wav"}},
	"pre bash docker-compose up":        {"enhanced", []string{"loading/docker-compose-up-start.wav", "loading/docker-compose-up.wav", "loading/docker-compose-start.wav", "loading/docker-compose.wav", "loading/bash-start.wav", "loading/bash.wav", "loading/tool-start.wav", "loading/loading.wav", "default.wav"}},
	"pre bash kubectl port-forward":     {"enhanced", []string{"loading/kubectl-port-forward-start.wav", "loading/kubectl-port-forward.wav", "loading/kubectl-start.wav", "loading/kubectl.wav", "loading/bash-start.wav", "loading/bash.wav", "loading/tool-start.wav", "loading/loading.wav", "default.wav"}},
	"pre bash kubectl get":              {"enhanced", []string{"loading/kubectl-get-start.wav", "loading/kubectl-get.wav", "loading/kubectl-start.wav", "loading/kubectl.wav", "loading/bash-start.wav", "loading/bash.wav", "loading/tool-start.wav", "loading/loading.wav", "default.wav"}},
	"pre bash go test":                  {"enhanced", []string{"loading/go-test-start.wav", "loading/go-test.wav", "loading/go-start.wav", "loading/go.wav", "loading/bash-start.wav", "loading/bash.wav", "loading/tool-start.wav", "loading/loading.wav", "default.wav"}},
	"pre bash cargo build":              {"enhanced", []string{"loading/cargo-build-start.wav", "loading/cargo-build.wav", "loading/cargo-start.wav", "loading/cargo.wav", "loading/bash-start.wav", "loading/bash.wav", "loading/tool-start.wav", "loading/loading.wav", "default.wav"}},
	"pre bash bash script":              {"enhanced", []string{"loading/bash-start.wav", "loading/bash.wav", "loading/tool-start.wav", "loading/loading.wav", "default.wav"}},
	"pre bash echo":                     {"enhanced", []string{"loading/echo-hello-start.wav", "loading/echo-hello.wav", "loading/echo-start.wav", "loading/echo.wav", "loading/bash-start.wav", "loading/bash.wav", "loading/tool-start.wav", "loading/loading.wav", "default.wav"}},
	"pre bash systemctl":                {"enhanced", []string{"loading/systemctl-restart-start.wav", "loading/systemctl-restart.wav", "loading/systemctl-start.wav", "loading/systemctl.wav", "loading/bash-start.wav", "loading/bash.wav", "loading/tool-start.wav", "loading/loading.wav", "default.wav"}},
	"pre bash python file":              {"enhanced", []string{"loading/python3-start.wav", "loading/python3.wav", "loading/bash-start.wav", "loading/bash.wav", "loading/tool-start.wav", "loading/loading.wav", "default.wav"}},
	"pre bash uppercase":                {"enhanced", []string{"loading/git-commit-start.wav", "loading/git-commit.wav", "loading/git-start.wav", "loading/git.wav", "loading/bash-start.wav", "loading/bash.wav", "loading/tool-start.wav", "loading/loading.wav", "default.wav"}},
	"pre bash empty command":            {"enhanced", []string{"loading/bash-start.wav", "loading/bash.wav", "loading/tool-start.wav", "loading/loading.wav", "default.wav"}},
	"pre bash no input":                 {"enhanced", []string{"loading/bash-start.wav", "loading/bash.wav", "loading/tool-start.wav", "loading/loading.wav", "default.wav"}},
	"pre gemini shell":                  {"enhanced", []string{"loading/git-push-start.wav", "loading/git-push.wav", "loading/git-start.wav", "loading/git.wav", "loading/bash-start.wav", "loading/bash.wav", "loading/tool-start.wav", "loading/loading.wav", "default.wav"}},
	"pre powershell":                    {"enhanced", []string{"loading/get-childitem-start.wav", "loading/get-childitem.wav", "loading/bash-start.wav", "loading/bash.wav", "loading/tool-start.wav", "loading/loading.wav", "default.wav"}},
	"post bash git commit ok":           {"posttool", []string{"success/git-commit-success.wav", "success/git-success.wav", "success/bash-success.wav", "success/tool-complete.wav", "success/success.wav", "default.wav"}},
	"post bash npm start ok":            {"posttool", []string{"success/npm-start-success.wav", "success/npm-success.wav", "success/bash-success.wav", "success/tool-complete.wav", "success/success.wav", "default.wav"}},
	"post bash ls ok":                   {"posttool", []string{"success/ls-success.wav", "success/bash-success.wav", "success/tool-complete.wav", "success/success.wav", "default.wav"}},
	"post bash docker compose ok":       {"posttool", []string{"success/docker-compose-success.wav", "success/docker-success.wav", "success/bash-success.wav", "success/tool-complete.wav", "success/success.wav", "default.wav"}},
	"post bash docker-compose ok":       {"posttool", []string{"success/docker-compose-up-success.wav", "success/docker-compose-success.wav", "success/bash-success.wav", "success/tool-complete.wav", "success/success.wav", "default.wav"}},
	"post bash kubectl port-forward ok": {"posttool", []string{"success/kubectl-port-forward-success.wav", "success/kubectl-success.wav", "success/bash-success.wav", "success/tool-complete.wav", "success/success.wav", "default.wav"}},
	"post bash no input ok":             {"posttool", []string{"success/bash-success.wav", "success/tool-complete.wav", "success/success.wav", "default.wav"}},
	"post gemini shell ok":              {"posttool", []string{"success/npm-test-success.wav", "success/npm-success.wav", "success/bash-success.wav", "success/tool-complete.wav", "success/success.wav", "default.wav"}},
	"post bash text exit 0":             {"posttool", []string{"success/make-success.wav", "success/bash-success.wav", "success/tool-complete.wav", "success/success.wav", "default.wav"}},
	"post bash git push stderr":         {"posttool", []string{"error/git-push-error.wav", "error/git-error.wav", "error/bash-error.wav", "error/tool-complete.wav", "error/error.wav", "default.wav"}},
	"post bash ls stderr":               {"posttool", []string{"error/ls-error.wav", "error/bash-error.wav", "error/tool-complete.wav", "error/error.wav", "default.wav"}},
	"post bash git commit interrupted":  {"posttool", []string{"error/tool-interrupted.wav", "error/git-error.wav", "error/bash-error.wav", "error/tool-complete.wav", "error/error.wav", "default.wav"}},
	"post bash no input stderr":         {"posttool", []string{"error/bash-error.wav", "error/tool-complete.wav", "error/error.wav", "default.wav"}},
	"post bash no input interrupted":    {"posttool", []string{"error/tool-interrupted.wav", "error/bash-error.wav", "error/tool-complete.wav", "error/error.wav", "default.wav"}},
	"post bash text exit 2":             {"posttool", []string{"error/make-error.wav", "error/bash-error.wav", "error/tool-complete.wav", "error/error.wav", "default.wav"}},
	"failure bash git push":             {"posttool", []string{"error/git-push-error.wav", "error/git-error.wav", "error/bash-error.wav", "error/tool-complete.wav", "error/error.wav", "default.wav"}},
	"failure bash kubectl port-forward": {"posttool", []string{"error/kubectl-port-forward-error.wav", "error/kubectl-error.wav", "error/bash-error.wav", "error/tool-complete.wav", "error/error.wav", "default.wav"}},
	"pre edit":                          {"enhanced", []string{"loading/edit-start.wav", "loading/edit.wav", "loading/tool-start.wav", "loading/loading.wav", "default.wav"}},
	"post edit ok":                      {"posttool", []string{"success/edit-success.wav", "success/tool-complete.wav", "success/success.wav", "default.wav"}},
	"post edit fail":                    {"posttool", []string{"error/edit-error.wav", "error/tool-complete.wav", "error/error.wav", "default.wav"}},
	"failure edit":                      {"posttool", []string{"error/edit-error.wav", "error/tool-complete.wav", "error/error.wav", "default.wav"}},
	"pre read":                          {"enhanced", []string{"loading/read-start.wav", "loading/read.wav", "loading/tool-start.wav", "loading/loading.wav", "default.wav"}},
	"post read ok":                      {"posttool", []string{"success/read-success.wav", "success/tool-complete.wav", "success/success.wav", "default.wav"}},
	"post read no content":              {"posttool", []string{"error/read-error.wav", "error/tool-complete.wav", "error/error.wav", "default.wav"}},
	"post read interrupted":             {"posttool", []string{"error/tool-interrupted.wav", "error/read-error.wav", "error/tool-complete.wav", "error/error.wav", "default.wav"}},
	"pre write":                         {"enhanced", []string{"loading/write-start.wav", "loading/write.wav", "loading/tool-start.wav", "loading/loading.wav", "default.wav"}},
	"post write ok":                     {"posttool", []string{"success/write-success.wav", "success/tool-complete.wav", "success/success.wav", "default.wav"}},
	"pre multiedit":                     {"enhanced", []string{"loading/multiedit-start.wav", "loading/multiedit.wav", "loading/tool-start.wav", "loading/loading.wav", "default.wav"}},
	"post grep ok":                      {"posttool", []string{"success/grep-success.wav", "success/tool-complete.wav", "success/success.wav", "default.wav"}},
	"post glob ok":                      {"posttool", []string{"success/glob-success.wav", "success/tool-complete.wav", "success/success.wav", "default.wav"}},
	"pre ls":                            {"enhanced", []string{"loading/ls-start.wav", "loading/ls.wav", "loading/tool-start.wav", "loading/loading.wav", "default.wav"}},
	"pre webfetch":                      {"enhanced", []string{"loading/webfetch-start.wav", "loading/webfetch.wav", "loading/tool-start.wav", "loading/loading.wav", "default.wav"}},
	"post websearch ok":                 {"posttool", []string{"success/websearch-success.wav", "success/tool-complete.wav", "success/success.wav", "default.wav"}},
	"pre todowrite":                     {"enhanced", []string{"loading/todowrite-start.wav", "loading/todowrite.wav", "loading/tool-start.wav", "loading/loading.wav", "default.wav"}},
	"pre task":                          {"enhanced", []string{"loading/task-start.wav", "loading/task.wav", "loading/tool-start.wav", "loading/loading.wav", "default.wav"}},
	"post task ok":                      {"posttool", []string{"success/task-success.wav", "success/tool-complete.wav", "success/success.wav", "default.wav"}},
	"post task error field":             {"posttool", []string{"error/task-error.wav", "error/tool-complete.wav", "error/error.wav", "default.wav"}},
	"post gemini read_file ok":          {"posttool", []string{"success/read-success.wav", "success/tool-complete.wav", "success/success.wav", "default.wav"}},
	"post no response":                  {"posttool", []string{"success/edit-success.wav", "success/tool-complete.wav", "success/success.wav", "default.wav"}},
	"post unparseable response":         {"posttool", []string{"error/edit-error.wav", "error/tool-complete.wav", "error/error.wav", "default.wav"}},
	"pre mcp github":                    {"enhanced", []string{"loading/mcp-start.wav", "loading/mcp.wav", "loading/mcp-github-create-issue-start.wav", "loading/mcp-github-create-issue.wav", "loading/tool-start.wav", "loading/loading.wav", "default.wav"}},
	"post mcp github ok":                {"posttool", []string{"success/mcp-success.wav", "success/mcp-github-create-issue-success.wav", "success/tool-complete.wav", "success/success.wav", "default.wav"}},
	"post mcp github isError":           {"posttool", []string{"error/mcp-error.wav", "error/mcp-github-create-issue-error.wav", "error/tool-complete.wav", "error/error.wav", "default.wav"}},
	"post mcp interrupted":              {"posttool", []string{"error/tool-interrupted.wav", "error/mcp-error.wav", "error/mcp-x-y-error.wav", "error/tool-complete.wav", "error/error.wav", "default.wav"}},
	"failure mcp":                       {"posttool", []string{"error/mcp-error.wav", "error/mcp-foo-error.wav", "error/tool-complete.wav", "error/error.wav", "default.wav"}},
	"pre read_mcp_resource":             {"enhanced", []string{"loading/mcp-start.wav", "loading/mcp.wav", "loading/read-mcp-resource-start.wav", "loading/read-mcp-resource.wav", "loading/tool-start.wav", "loading/loading.wav", "default.wav"}},
	"pre no tool":                       {"simple", []string{"loading/tool-loading.wav", "loading/tool-start.wav", "loading/loading.wav", "default.wav"}},
	"post no tool":                      {"simple", []string{"success/tool-success.wav", "success/tool-complete.wav", "success/success.wav", "default.wav"}},
	"post no tool stderr":               {"simple", []string{"error/tool-error.wav", "error/tool-complete.wav", "error/error.wav", "default.wav"}},
	"post no tool interrupted":          {"simple", []string{"error/tool-interrupted.wav", "error/tool-complete.wav", "error/error.wav", "default.wav"}},
	"UserPromptSubmit":                  {"simple", []string{"interactive/message-sent.wav", "interactive/prompt-submit.wav", "interactive/interactive.wav", "default.wav"}},
	"UserPromptExpansion":               {"simple", []string{"interactive/prompt-expansion.wav", "interactive/interactive.wav", "default.wav"}},
	"Notification generic":              {"simple", []string{"interactive/notification.wav", "interactive/interactive.wav", "default.wav"}},
	"Notification none":                 {"simple", []string{"interactive/notification.wav", "interactive/interactive.wav", "default.wav"}},
	"Notification permission":           {"simple", []string{"interactive/notification-permission.wav", "interactive/notification.wav", "interactive/interactive.wav", "default.wav"}},
	"Notification idle":                 {"simple", []string{"interactive/notification-idle.wav", "interactive/notification.wav", "interactive/interactive.wav", "default.wav"}},
	"PostToolBatch":                     {"simple", []string{"success/tool-batch.wav", "success/success.wav", "default.wav"}},
	"Stop":                              {"simple", []string{"completion/agent-complete.wav", "completion/stop.wav", "completion/completion.wav", "default.wav"}},
	"SubagentStop":                      {"simple", []string{"completion/subagent-complete.wav", "completion/subagent-stop.wav", "completion/completion.wav", "default.wav"}},
	"SubagentStart":                     {"simple", []string{"loading/subagent-start.wav", "loading/loading.wav", "default.wav"}},
	"subagentStart alias":               {"simple", []string{"loading/subagent-start.wav", "loading/loading.wav", "default.wav"}},
	"PostCompact":                       {"simple", []string{"system/post-compact.wav", "system/system.wav", "default.wav"}},
	"PreCompact":                        {"simple", []string{"system/compacting.wav", "system/pre-compact.wav", "system/system.wav", "default.wav"}},
	"PreCompress":                       {"simple", []string{"system/compacting.wav", "system/pre-compact.wav", "system/system.wav", "default.wav"}},
	"SessionStart":                      {"simple", []string{"system/session-start.wav", "system/system.wav", "default.wav"}},
	"PermissionRequest":                 {"simple", []string{"interactive/permission-request.wav", "interactive/interactive.wav", "default.wav"}},
	"PermissionDenied":                  {"simple", []string{"error/permission-denied.wav", "error/error.wav", "default.wav"}},
	"SessionEnd":                        {"simple", []string{"interactive/session-end.wav", "interactive/interactive.wav", "default.wav"}},
	"BeforeAgent":                       {"simple", []string{"interactive/before-agent.wav", "interactive/interactive.wav", "default.wav"}},
	"AfterAgent":                        {"simple", []string{"completion/agent-complete.wav", "completion/after-agent.wav", "completion/completion.wav", "default.wav"}},
	"StopFailure":                       {"simple", []string{"error/stop-failure.wav", "error/error.wav", "default.wav"}},
	"ErrorOccurred":                     {"simple", []string{"error/error-occurred.wav", "error/error.wav", "default.wav"}},
	"Setup":                             {"simple", []string{"system/setup.wav", "system/system.wav", "default.wav"}},
	"MessageDisplay":                    {},
	"TaskCreated":                       {"simple", []string{"loading/task-created.wav", "loading/loading.wav", "default.wav"}},
	"TaskCompleted":                     {"simple", []string{"completion/task-completed.wav", "completion/completion.wav", "default.wav"}},
	"TeammateIdle":                      {"simple", []string{"interactive/teammate-idle.wav", "interactive/interactive.wav", "default.wav"}},
	"InstructionsLoaded":                {"simple", []string{"system/instructions-loaded.wav", "system/system.wav", "default.wav"}},
	"ConfigChange":                      {"simple", []string{"system/config-change.wav", "system/system.wav", "default.wav"}},
	"CwdChanged":                        {"simple", []string{"system/cwd-changed.wav", "system/system.wav", "default.wav"}},
	"FileChanged":                       {"simple", []string{"system/file-changed.wav", "system/system.wav", "default.wav"}},
	"WorktreeCreate":                    {"simple", []string{"system/worktree-create.wav", "system/system.wav", "default.wav"}},
	"WorktreeRemove":                    {"simple", []string{"system/worktree-remove.wav", "system/system.wav", "default.wav"}},
	"Elicitation":                       {"simple", []string{"interactive/elicitation.wav", "interactive/interactive.wav", "default.wav"}},
	"ElicitationResult":                 {"simple", []string{"interactive/elicitation-result.wav", "interactive/interactive.wav", "default.wav"}},
	"TodoCreated":                       {"simple", []string{"loading/todo-created.wav", "loading/loading.wav", "default.wav"}},
	"TodoCompleted":                     {"simple", []string{"completion/todo-completed.wav", "completion/completion.wav", "default.wav"}},
	"BeforeModel":                       {},
	"AfterModel":                        {},
	"BeforeToolSelection":               {},
	"unknown event":                     {"simple", []string{"interactive/default.wav", "interactive/unknown.wav", "interactive/interactive.wav", "default.wav"}},
	"unknown event with tool":           {"simple", []string{"interactive/default.wav", "interactive/unknown.wav", "interactive/interactive.wav", "default.wav"}},
}

// TestCommandSubcommandLevelUsesParsedCommand checks the #8 cases directly:
// hyphenated commands and subcommands reach the command-subcommand level.
func TestCommandSubcommandLevelUsesParsedCommand(t *testing.T) {
	tests := []struct{ cmd, want, notWant string }{
		{"docker compose up", "loading/docker-compose.wav", ""},
		{"docker-compose up", "loading/docker-compose-up.wav", ""},
		{"kubectl port-forward svc/x 80", "loading/kubectl-port-forward.wav", "loading/kubectl-port.wav"},
	}
	for _, tc := range tests {
		t.Run(tc.cmd, func(t *testing.T) {
			got := mapGoldenEvent(t, goldenEvent{name: tc.cmd, event: "PreToolUse", tool: "Bash", input: bashCmd(tc.cmd)})
			if len(got.paths) < 2 || got.paths[1] != tc.want {
				t.Errorf("level 2 = %q, want %q (all: %q)", got.paths, tc.want, got.paths)
			}
			if tc.notWant != "" && slices.Contains(got.paths, tc.notWant) {
				t.Errorf("paths contain %q: %q", tc.notWant, got.paths)
			}
		})
	}
}
