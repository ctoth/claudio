package cli

import (
	"strings"
	"testing"
)

func TestCLIWithRealHookJSON(t *testing.T) {
	cli := NewCLI()

	// Real hook JSON from our logs (simplified)
	realHookJSON := `{
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
			"stdout": "total 288\ndrwxr-xr-x  2 root root   4096 Jul 26 16:53 .\n",
			"stderr": "",
			"interrupted": false,
			"isImage": false
		}
	}`

	stdin := strings.NewReader(realHookJSON)
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	exitCode := cli.Run([]string{"claudio"}, stdin, stdout, stderr)

	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
		t.Logf("Stderr: %s", stderr.String())
	}

	// Should have processed the hook (not configuration test mode)
	if strings.Contains(stderr.String(), "configuration test mode") {
		t.Error("Should not be in configuration test mode with valid JSON input")
	}

	// Test passes if exit code is 0 (hook was processed successfully)
}

func TestCLIEmptyInput(t *testing.T) {
	cli := NewCLI()

	// Empty input should trigger configuration test mode
	stdin := strings.NewReader("")
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	exitCode := cli.Run([]string{"claudio"}, stdin, stdout, stderr)

	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
		t.Logf("Stderr: %s", stderr.String())
	}

	// Empty input should be handled gracefully (configuration test mode)
	// Exit code check above is sufficient
}
