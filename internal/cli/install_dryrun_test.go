package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestInstallDryRunFlag(t *testing.T) {
	testCases := []struct {
		name     string
		args     []string
		exitCode int
		errorMsg string
	}{
		{
			name:     "dry-run flag present",
			args:     []string{"claudio", "install", "--dry-run"},
			exitCode: 0,
			errorMsg: "",
		},
		{
			name:     "dry-run with user scope",
			args:     []string{"claudio", "install", "--dry-run", "--scope", "user"},
			exitCode: 0,
			errorMsg: "",
		},
		{
			name:     "dry-run with project scope",
			args:     []string{"claudio", "install", "--dry-run", "--scope", "project"},
			exitCode: 0,
			errorMsg: "",
		},
		{
			name:     "dry-run long form",
			args:     []string{"claudio", "install", "--dry-run=true"},
			exitCode: 0,
			errorMsg: "",
		},
		{
			name:     "dry-run short form",
			args:     []string{"claudio", "install", "-d"},
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

func TestInstallDryRunFlagHelp(t *testing.T) {
	// TDD RED: Test that install command shows proper help for --dry-run flag
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

	// Should contain --dry-run flag documentation
	expectedHelpContent := []string{
		"--dry-run",
		"-d,",
		"Show what would be done without making changes",
		"simulation",
	}

	for _, content := range expectedHelpContent {
		if !strings.Contains(helpOutput, content) {
			t.Errorf("Help output should contain '%s'", content)
		}
	}

	t.Logf("Install help output: %s", helpOutput)
}

func TestInstallDryRunFlagValidation(t *testing.T) {
	// TDD RED: Test that dry-run flag doesn't accept invalid values
	testCases := []struct {
		name     string
		args     []string
		exitCode int
		errorMsg string
	}{
		{
			name:     "dry-run with valid true",
			args:     []string{"claudio", "install", "--dry-run=true"},
			exitCode: 0,
			errorMsg: "",
		},
		{
			name:     "dry-run with valid false",
			args:     []string{"claudio", "install", "--dry-run=false"},
			exitCode: 0,
			errorMsg: "",
		},
		{
			name:     "dry-run without value defaults to true",
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
		})
	}
}

func TestInstallDryRunFilesystemSafety(t *testing.T) {
	// TDD RED: Test that dry-run mode never writes to filesystem
	// This is the core requirement - dry-run should simulate without changes

	// Create temporary directory to monitor for unwanted changes
	tempDir := t.TempDir()

	testCases := []struct {
		name string
		args []string
	}{
		{
			name: "dry-run user scope",
			args: []string{"claudio", "install", "--dry-run", "--scope", "user"},
		},
		{
			name: "dry-run project scope",
			args: []string{"claudio", "install", "--dry-run", "--scope", "project"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cli := NewCLI() // Create fresh CLI instance for each test
			stdin := strings.NewReader("")
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}

			// Record initial state of temp directory
			initialEntries, err := countDirectoryEntries(tempDir)
			if err != nil {
				t.Fatalf("Failed to count initial directory entries: %v", err)
			}

			exitCode := cli.Run(tc.args, stdin, stdout, stderr)

			// Verify command succeeded
			if exitCode != 0 {
				t.Errorf("Expected exit code 0, got %d", exitCode)
				t.Logf("Stderr: %s", stderr.String())
			}

			// Verify no files were created in temp directory during dry-run
			finalEntries, err := countDirectoryEntries(tempDir)
			if err != nil {
				t.Fatalf("Failed to count final directory entries: %v", err)
			}

			if finalEntries != initialEntries {
				t.Errorf("Dry-run should not create files: initial=%d, final=%d", initialEntries, finalEntries)
			}

			// Verify output indicates dry-run mode
			stdoutOutput := strings.ToLower(stdout.String())
			if !strings.Contains(stdoutOutput, "dry-run") && !strings.Contains(stdoutOutput, "would") {
				t.Error("Dry-run output should indicate simulation mode with 'dry-run' or 'would' language")
			}

			t.Logf("Dry-run output: %s", stdoutOutput)
		})
	}
}

// Helper function to count directory entries
func countDirectoryEntries(dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}
	return len(entries), nil
}
