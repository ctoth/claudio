package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestInstallScopeFlag(t *testing.T) {
	testCases := []struct {
		name     string
		args     []string
		exitCode int
		errorMsg string
	}{
		{
			name:     "valid user scope",
			args:     []string{"claudio", "install", "--scope", "user", "--dry-run"},
			exitCode: 0,
			errorMsg: "",
		},
		{
			name:     "valid project scope",
			args:     []string{"claudio", "install", "--scope", "project", "--dry-run"},
			exitCode: 0,
			errorMsg: "",
		},
		{
			name:     "invalid scope value",
			args:     []string{"claudio", "install", "--scope", "invalid", "--dry-run"},
			exitCode: 1,
			errorMsg: "invalid scope",
		},
		{
			name:     "missing scope value",
			args:     []string{"claudio", "install", "--scope"},
			exitCode: 1,
			errorMsg: "flag needs an argument",
		},
		{
			name:     "default scope behavior",
			args:     []string{"claudio", "install", "--dry-run"},
			exitCode: 0,
			errorMsg: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cli := NewCLI() // Create fresh CLI instance for each test
			stdin := strings.NewReader("")
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}

			exitCode := cli.Run(tc.args, stdin, stdout, stderr)

			if exitCode != tc.exitCode {
				t.Errorf("Expected exit code %d, got %d", tc.exitCode, exitCode)
				t.Logf("Args: %v", tc.args)
				t.Logf("Stdout: %s", stdout.String())
				t.Logf("Stderr: %s", stderr.String())
			}

			// Check error message if expected
			if tc.errorMsg != "" {
				stderrOutput := stderr.String()
				if !strings.Contains(stderrOutput, tc.errorMsg) {
					t.Errorf("Expected stderr to contain '%s', got: %s", tc.errorMsg, stderrOutput)
				}
			}

			// Success cases should not have error output
			if tc.exitCode == 0 && stderr.Len() > 0 {
				t.Logf("Unexpected stderr output for success case: %s", stderr.String())
			}
		})
	}
}

func TestInstallScopeFlagValidation(t *testing.T) {
	// TDD RED: Test that the install command validates scope values
	// Test scope validation with different invalid values
	invalidScopes := []string{"global", "system", "admin", "root", ""}

	for _, scope := range invalidScopes {
		t.Run("invalid_scope_"+scope, func(t *testing.T) {
			cli := NewCLI() // Create fresh CLI instance for each test
			stdin := strings.NewReader("")
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}

			args := []string{"claudio", "install", "--scope", scope, "--dry-run"}
			exitCode := cli.Run(args, stdin, stdout, stderr)

			// Should fail with exit code 1
			if exitCode != 1 {
				t.Errorf("Expected exit code 1 for invalid scope '%s', got %d", scope, exitCode)
				t.Logf("Stderr: %s", stderr.String())
			}

			// Should contain validation error message
			stderrOutput := stderr.String()
			if !strings.Contains(stderrOutput, "invalid scope") && !strings.Contains(stderrOutput, "must be") {
				t.Errorf("Expected validation error message for scope '%s', got: %s", scope, stderrOutput)
			}
		})
	}
}

func TestInstallScopeFlagHelp(t *testing.T) {
	// TDD RED: Test that install command shows proper help for --scope flag
	cli := NewCLI()

	stdin := strings.NewReader("")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	exitCode := cli.Run([]string{"claudio", "install", "--help"}, stdin, stdout, stderr)

	if exitCode != 0 {
		t.Errorf("Expected exit code 0 for help command, got %d", exitCode)
		t.Logf("Stderr: %s", stderr.String())
	}

	helpOutput := stdout.String()

	// Should contain --scope flag documentation
	expectedHelpContent := []string{
		"--scope",
		"user",
		"project",
		"Installation scope",
	}

	for _, content := range expectedHelpContent {
		if !strings.Contains(helpOutput, content) {
			t.Errorf("Help output should contain '%s'", content)
		}
	}

	t.Logf("Install help output: %s", helpOutput)
}
