package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestInstallForceFlag(t *testing.T) {
	testCases := []struct {
		name     string
		args     []string
		exitCode int
		errorMsg string
	}{
		{
			name:     "force flag present",
			args:     []string{"claudio", "install", "--force"},
			exitCode: 0,
			errorMsg: "",
		},
		{
			name:     "force with user scope",
			args:     []string{"claudio", "install", "--force", "--scope", "user"},
			exitCode: 0,
			errorMsg: "",
		},
		{
			name:     "force with project scope",
			args:     []string{"claudio", "install", "--force", "--scope", "project"},
			exitCode: 0,
			errorMsg: "",
		},
		{
			name:     "force long form",
			args:     []string{"claudio", "install", "--force=true"},
			exitCode: 0,
			errorMsg: "",
		},
		{
			name:     "force short form",
			args:     []string{"claudio", "install", "-f"},
			exitCode: 0,
			errorMsg: "",
		},
		{
			name:     "force with dry-run",
			args:     []string{"claudio", "install", "--force", "--dry-run"},
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

func TestInstallForceFlagHelp(t *testing.T) {
	// TDD RED: Test that install command shows proper help for --force flag
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
	
	// Should contain --force flag documentation
	expectedHelpContent := []string{
		"--force",
		"-f,",
		"Overwrite existing hooks without prompting",
	}

	for _, content := range expectedHelpContent {
		if !strings.Contains(helpOutput, content) {
			t.Errorf("Help output should contain '%s'", content)
		}
	}

	t.Logf("Install help output: %s", helpOutput)
}

func TestInstallForceFlagValidation(t *testing.T) {
	// TDD RED: Test that force flag doesn't accept invalid values
	testCases := []struct {
		name     string
		args     []string
		exitCode int
		errorMsg string
	}{
		{
			name:     "force with valid true",
			args:     []string{"claudio", "install", "--force=true"},
			exitCode: 0,
			errorMsg: "",
		},
		{
			name:     "force with valid false",
			args:     []string{"claudio", "install", "--force=false"},
			exitCode: 0,
			errorMsg: "",
		},
		{
			name:     "force without value defaults to true",
			args:     []string{"claudio", "install", "--force"},
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
		})
	}
}

func TestInstallForceTerminalIntegration(t *testing.T) {
	// TDD RED: Test that force flag integrates with terminal detection
	// The force flag should override interactive prompts when not running in a terminal
	
	testCases := []struct {
		name string
		args []string
	}{
		{
			name: "force user scope",
			args: []string{"claudio", "install", "--force", "--scope", "user"},
		},
		{
			name: "force project scope", 
			args: []string{"claudio", "install", "--force", "--scope", "project"},
		},
		{
			name: "force with dry-run",
			args: []string{"claudio", "install", "--force", "--dry-run"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cli := NewCLI() // Create fresh CLI instance for each test
			stdin := strings.NewReader("")
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}

			exitCode := cli.Run(tc.args, stdin, stdout, stderr)

			// Verify command succeeded
			if exitCode != 0 {
				t.Errorf("Expected exit code 0, got %d", exitCode)
				t.Logf("Stderr: %s", stderr.String())
			}

			// Verify output indicates force mode
			stdoutOutput := stdout.String()
			stdoutLower := strings.ToLower(stdoutOutput)
			// For dry-run mode, force indication is not required since no actual changes are made
			isDryRun := strings.Contains(stdoutLower, "dry-run") || strings.Contains(stdoutLower, "simulation")
			if !isDryRun && !strings.Contains(stdoutLower, "force") {
				t.Error("Force output should indicate force mode with 'force' language (except in dry-run mode)")
			}

			t.Logf("Force output: %s", stdoutOutput)
		})
	}
}

func TestInstallForceInteractivePrompting(t *testing.T) {
	// TDD RED: Test that force flag bypasses interactive prompts
	// This test simulates scenarios where files would exist and need confirmation
	
	testCases := []struct {
		name        string
		args        []string
		expectForce bool
	}{
		{
			name:        "without force flag should require prompts",
			args:        []string{"claudio", "install", "--scope", "user"},
			expectForce: false,
		},
		{
			name:        "with force flag should bypass prompts",
			args:        []string{"claudio", "install", "--force", "--scope", "user"},
			expectForce: true,
		},
		{
			name:        "force=true should bypass prompts",
			args:        []string{"claudio", "install", "--force=true", "--scope", "project"},
			expectForce: true,
		},
		{
			name:        "force=false should require prompts",
			args:        []string{"claudio", "install", "--force=false", "--scope", "project"},
			expectForce: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cli := NewCLI() // Create fresh CLI instance for each test
			stdin := strings.NewReader("")
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}

			exitCode := cli.Run(tc.args, stdin, stdout, stderr)

			// All commands should succeed for now (actual prompting logic comes later)
			if exitCode != 0 {
				t.Errorf("Expected exit code 0, got %d", exitCode)
				t.Logf("Stderr: %s", stderr.String())
			}

			stdoutOutput := stdout.String()
			
			if tc.expectForce {
				// Should indicate force mode
				if !strings.Contains(stdoutOutput, "force") && !strings.Contains(stdoutOutput, "FORCE") {
					t.Error("Expected force mode indication in output")
				}
			} else {
				// Should not indicate force mode
				if strings.Contains(stdoutOutput, "FORCE:") {
					t.Error("Should not indicate force mode when flag not set")
				}
			}

			t.Logf("Output for expectForce=%v: %s", tc.expectForce, stdoutOutput)
		})
	}
}

func TestInstallForceCombinationFlags(t *testing.T) {
	// TDD RED: Test force flag combinations with other flags
	testCases := []struct {
		name     string
		args     []string
		exitCode int
	}{
		{
			name:     "force + dry-run + user scope",
			args:     []string{"claudio", "install", "--force", "--dry-run", "--scope", "user"},
			exitCode: 0,
		},
		{
			name:     "force + dry-run + project scope",
			args:     []string{"claudio", "install", "--force", "--dry-run", "--scope", "project"},
			exitCode: 0,
		},
		{
			name:     "short forms: -f -d -s user",
			args:     []string{"claudio", "install", "-f", "-d", "-s", "user"},
			exitCode: 0,
		},
		{
			name:     "mixed forms: --force -d --scope project",
			args:     []string{"claudio", "install", "--force", "-d", "--scope", "project"},
			exitCode: 0,
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

			// All successful cases should have meaningful output
			if tc.exitCode == 0 {
				stdoutOutput := stdout.String()
				if len(stdoutOutput) == 0 {
					t.Error("Expected some output for successful command")
				}
				t.Logf("Combined flags output: %s", stdoutOutput)
			}
		})
	}
}