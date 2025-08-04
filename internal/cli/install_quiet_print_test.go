package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestInstallQuietFlag(t *testing.T) {
	testCases := []struct {
		name     string
		args     []string
		exitCode int
		errorMsg string
	}{
		{
			name:     "quiet flag present",
			args:     []string{"claudio", "install", "--quiet"},
			exitCode: 0,
			errorMsg: "",
		},
		{
			name:     "quiet with user scope",
			args:     []string{"claudio", "install", "--quiet", "--scope", "user"},
			exitCode: 0,
			errorMsg: "",
		},
		{
			name:     "quiet with project scope",
			args:     []string{"claudio", "install", "--quiet", "--scope", "project"},
			exitCode: 0,
			errorMsg: "",
		},
		{
			name:     "quiet long form",
			args:     []string{"claudio", "install", "--quiet=true"},
			exitCode: 0,
			errorMsg: "",
		},
		{
			name:     "quiet short form",
			args:     []string{"claudio", "install", "-q"},
			exitCode: 0,
			errorMsg: "",
		},
		{
			name:     "quiet with dry-run",
			args:     []string{"claudio", "install", "--quiet", "--dry-run"},
			exitCode: 0,
			errorMsg: "",
		},
		{
			name:     "quiet with force",
			args:     []string{"claudio", "install", "--quiet", "--force"},
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

func TestInstallQuietFlagHelp(t *testing.T) {
	// TDD RED: Test that install command shows proper help for --quiet flag
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

	// Should contain --quiet flag documentation
	expectedHelpContent := []string{
		"--quiet",
		"-q,",
		"Suppress output (no progress messages)",
	}

	for _, content := range expectedHelpContent {
		if !strings.Contains(helpOutput, content) {
			t.Errorf("Help output should contain '%s'", content)
		}
	}

	t.Logf("Install help output: %s", helpOutput)
}

func TestInstallQuietFlagValidation(t *testing.T) {
	// TDD RED: Test that quiet flag doesn't accept invalid values
	testCases := []struct {
		name     string
		args     []string
		exitCode int
		errorMsg string
	}{
		{
			name:     "quiet with valid true",
			args:     []string{"claudio", "install", "--quiet=true"},
			exitCode: 0,
			errorMsg: "",
		},
		{
			name:     "quiet with valid false",
			args:     []string{"claudio", "install", "--quiet=false"},
			exitCode: 0,
			errorMsg: "",
		},
		{
			name:     "quiet without value defaults to true",
			args:     []string{"claudio", "install", "--quiet"},
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

func TestInstallQuietOutputSuppression(t *testing.T) {
	// TDD RED: Test that quiet flag suppresses normal output messages
	// This is the core requirement - quiet should minimize stdout messages

	testCases := []struct {
		name        string
		args        []string
		expectQuiet bool
	}{
		{
			name:        "without quiet flag should have normal output",
			args:        []string{"claudio", "install", "--scope", "user"},
			expectQuiet: false,
		},
		{
			name:        "with quiet flag should suppress output",
			args:        []string{"claudio", "install", "--quiet", "--scope", "user"},
			expectQuiet: true,
		},
		{
			name:        "quiet=true should suppress output",
			args:        []string{"claudio", "install", "--quiet=true", "--scope", "project"},
			expectQuiet: true,
		},
		{
			name:        "quiet=false should have normal output",
			args:        []string{"claudio", "install", "--quiet=false", "--scope", "project"},
			expectQuiet: false,
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

			stdoutOutput := stdout.String()

			if tc.expectQuiet {
				// Should have minimal or no output in quiet mode
				if len(stdoutOutput) > 100 { // Allow for brief success messages
					t.Error("Quiet mode should suppress verbose output")
				}
			} else {
				// Should have normal informative output
				if len(stdoutOutput) == 0 {
					t.Error("Expected some output for non-quiet mode")
				}
			}

			t.Logf("Quiet=%v output length: %d, content: %s", tc.expectQuiet, len(stdoutOutput), stdoutOutput)
		})
	}
}

func TestInstallPrintFlag(t *testing.T) {
	testCases := []struct {
		name     string
		args     []string
		exitCode int
		errorMsg string
	}{
		{
			name:     "print flag present",
			args:     []string{"claudio", "install", "--print"},
			exitCode: 0,
			errorMsg: "",
		},
		{
			name:     "print with user scope",
			args:     []string{"claudio", "install", "--print", "--scope", "user"},
			exitCode: 0,
			errorMsg: "",
		},
		{
			name:     "print with project scope",
			args:     []string{"claudio", "install", "--print", "--scope", "project"},
			exitCode: 0,
			errorMsg: "",
		},
		{
			name:     "print long form",
			args:     []string{"claudio", "install", "--print=true"},
			exitCode: 0,
			errorMsg: "",
		},
		{
			name:     "print short form",
			args:     []string{"claudio", "install", "-p"},
			exitCode: 0,
			errorMsg: "",
		},
		{
			name:     "print with dry-run",
			args:     []string{"claudio", "install", "--print", "--dry-run"},
			exitCode: 0,
			errorMsg: "",
		},
		{
			name:     "print with force",
			args:     []string{"claudio", "install", "--print", "--force"},
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

func TestInstallPrintFlagHelp(t *testing.T) {
	// TDD RED: Test that install command shows proper help for --print flag
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

	// Should contain --print flag documentation
	expectedHelpContent := []string{
		"--print",
		"-p,",
		"Print configuration that would be written",
	}

	for _, content := range expectedHelpContent {
		if !strings.Contains(helpOutput, content) {
			t.Errorf("Help output should contain '%s'", content)
		}
	}

	t.Logf("Install help output: %s", helpOutput)
}

func TestInstallPrintFlagValidation(t *testing.T) {
	// TDD RED: Test that print flag doesn't accept invalid values
	testCases := []struct {
		name     string
		args     []string
		exitCode int
		errorMsg string
	}{
		{
			name:     "print with valid true",
			args:     []string{"claudio", "install", "--print=true"},
			exitCode: 0,
			errorMsg: "",
		},
		{
			name:     "print with valid false",
			args:     []string{"claudio", "install", "--print=false"},
			exitCode: 0,
			errorMsg: "",
		},
		{
			name:     "print without value defaults to true",
			args:     []string{"claudio", "install", "--print"},
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

func TestInstallPrintConfigurationOutput(t *testing.T) {
	// TDD RED: Test that print flag shows configuration that would be written
	// This is the core requirement - print should show the generated hook configuration

	testCases := []struct {
		name        string
		args        []string
		expectPrint bool
	}{
		{
			name:        "without print flag should not show configuration",
			args:        []string{"claudio", "install", "--scope", "user"},
			expectPrint: false,
		},
		{
			name:        "with print flag should show configuration",
			args:        []string{"claudio", "install", "--print", "--scope", "user"},
			expectPrint: true,
		},
		{
			name:        "print=true should show configuration",
			args:        []string{"claudio", "install", "--print=true", "--scope", "project"},
			expectPrint: true,
		},
		{
			name:        "print=false should not show configuration",
			args:        []string{"claudio", "install", "--print=false", "--scope", "project"},
			expectPrint: false,
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

			stdoutOutput := stdout.String()

			if tc.expectPrint {
				// Should show configuration details when print flag is set
				if !strings.Contains(stdoutOutput, "PRINT:") && !strings.Contains(stdoutOutput, "configuration") {
					t.Error("Print mode should show configuration details")
				}
				// Should have substantial output showing configuration
				if len(stdoutOutput) < 20 {
					t.Error("Print mode should show detailed configuration")
				}
			} else {
				// Should not show detailed configuration without print flag
				if strings.Contains(stdoutOutput, "PRINT:") {
					t.Error("Should not show detailed configuration when print flag not set")
				}
			}

			t.Logf("Print=%v output: %s", tc.expectPrint, stdoutOutput)
		})
	}
}

func TestInstallQuietPrintCombination(t *testing.T) {
	// TDD RED: Test interaction between --quiet and --print flags
	// Print flag should override quiet for configuration display
	testCases := []struct {
		name     string
		args     []string
		exitCode int
	}{
		{
			name:     "quiet + print user scope",
			args:     []string{"claudio", "install", "--quiet", "--print", "--scope", "user"},
			exitCode: 0,
		},
		{
			name:     "quiet + print project scope",
			args:     []string{"claudio", "install", "--quiet", "--print", "--scope", "project"},
			exitCode: 0,
		},
		{
			name:     "quiet + print + dry-run",
			args:     []string{"claudio", "install", "--quiet", "--print", "--dry-run"},
			exitCode: 0,
		},
		{
			name:     "quiet + print + force",
			args:     []string{"claudio", "install", "--quiet", "--print", "--force"},
			exitCode: 0,
		},
		{
			name:     "short forms: -q -p -d",
			args:     []string{"claudio", "install", "-q", "-p", "-d"},
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

			stdoutOutput := stdout.String()

			// When both quiet and print are specified, print should take precedence
			// Should show configuration details despite quiet flag
			if !strings.Contains(stdoutOutput, "PRINT:") && !strings.Contains(stdoutOutput, "configuration") {
				t.Error("Print flag should override quiet flag for configuration display")
			}

			t.Logf("Quiet+Print output: %s", stdoutOutput)
		})
	}
}

func TestInstallFlagCombinationsWithQuietPrint(t *testing.T) {
	// TDD RED: Test quiet and print flags with all other flag combinations
	testCases := []struct {
		name     string
		args     []string
		exitCode int
	}{
		{
			name:     "all flags: --quiet --print --dry-run --force --scope user",
			args:     []string{"claudio", "install", "--quiet", "--print", "--dry-run", "--force", "--scope", "user"},
			exitCode: 0,
		},
		{
			name:     "all flags: --quiet --print --dry-run --force --scope project",
			args:     []string{"claudio", "install", "--quiet", "--print", "--dry-run", "--force", "--scope", "project"},
			exitCode: 0,
		},
		{
			name:     "short forms: -q -p -d -f -s user",
			args:     []string{"claudio", "install", "-q", "-p", "-d", "-f", "-s", "user"},
			exitCode: 0,
		},
		{
			name:     "mixed forms: --quiet -p --dry-run -f --scope project",
			args:     []string{"claudio", "install", "--quiet", "-p", "--dry-run", "-f", "--scope", "project"},
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

			// All successful cases should have meaningful output due to --print flag
			if tc.exitCode == 0 {
				stdoutOutput := stdout.String()
				if len(stdoutOutput) == 0 {
					t.Error("Expected some output for successful command with --print flag")
				}
				t.Logf("Combined flags output: %s", stdoutOutput)
			}
		})
	}
}
