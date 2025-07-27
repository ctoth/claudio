package cli

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestCLI(t *testing.T) {
	cli := NewCLI()

	if cli == nil {
		t.Fatal("NewCLI returned nil")
	}
}

func TestCLIBasicUsage(t *testing.T) {
	cli := NewCLI()

	// Test basic hook processing from stdin
	hookJSON := `{
		"session_id": "test",
		"transcript_path": "/test",
		"cwd": "/test",
		"hook_event_name": "PostToolUse",
		"tool_name": "Bash",
		"tool_response": {
			"stdout": "success",
			"stderr": "",
			"interrupted": false
		}
	}`

	stdin := strings.NewReader(hookJSON)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	exitCode := cli.Run([]string{"claudio"}, stdin, stdout, stderr)

	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
		t.Logf("Stderr: %s", stderr.String())
	}

	// Should process without error (exact output depends on implementation)
	if stderr.Len() > 0 {
		t.Logf("Stderr output: %s", stderr.String())
	}

	t.Logf("Stdout: %s", stdout.String())
}

func TestCLIFlags(t *testing.T) {
	cli := NewCLI()

	testCases := []struct {
		name     string
		args     []string
		exitCode int
	}{
		{
			name:     "help flag",
			args:     []string{"claudio", "--help"},
			exitCode: 0,
		},
		{
			name:     "version flag",
			args:     []string{"claudio", "--version"},
			exitCode: 0,
		},
		{
			name:     "volume flag",
			args:     []string{"claudio", "--volume", "0.8"},
			exitCode: 0,
		},
		{
			name:     "soundpack flag",
			args:     []string{"claudio", "--soundpack", "mechanical"},
			exitCode: 0,
		},
		{
			name:     "silent flag",
			args:     []string{"claudio", "--silent"},
			exitCode: 0,
		},
		{
			name:     "config flag",
			args:     []string{"claudio", "--config", "/tmp/test-config.json"},
			exitCode: 0, // Should not error even if file doesn't exist (uses defaults)
		},
		{
			name:     "invalid flag",
			args:     []string{"claudio", "--invalid-flag"},
			exitCode: 1,
		},
		{
			name:     "invalid volume",
			args:     []string{"claudio", "--volume", "invalid"},
			exitCode: 1,
		},
		{
			name:     "volume out of range",
			args:     []string{"claudio", "--volume", "2.0"},
			exitCode: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
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

			// Help and version should produce output
			if (tc.name == "help flag" || tc.name == "version flag") && stdout.Len() == 0 {
				t.Error("Expected output for help/version flag")
			}
		})
	}
}

func TestCLIJSONProcessing(t *testing.T) {
	cli := NewCLI()

	testCases := []struct {
		name     string
		input    string
		exitCode int
	}{
		{
			name: "valid PostToolUse hook",
			input: `{
				"session_id": "test",
				"transcript_path": "/test", 
				"cwd": "/test",
				"hook_event_name": "PostToolUse",
				"tool_name": "Bash",
				"tool_response": {"stdout": "success", "stderr": "", "interrupted": false}
			}`,
			exitCode: 0,
		},
		{
			name: "valid UserPromptSubmit hook",
			input: `{
				"session_id": "test",
				"transcript_path": "/test",
				"cwd": "/test", 
				"hook_event_name": "UserPromptSubmit",
				"prompt": "test message"
			}`,
			exitCode: 0,
		},
		{
			name: "valid PreToolUse hook",
			input: `{
				"session_id": "test",
				"transcript_path": "/test",
				"cwd": "/test",
				"hook_event_name": "PreToolUse",
				"tool_name": "Edit",
				"tool_input": {"file_path": "/test.go"}
			}`,
			exitCode: 0,
		},
		{
			name:     "invalid JSON",
			input:    `{invalid json}`,
			exitCode: 1,
		},
		{
			name:     "empty input",
			input:    ``,
			exitCode: 0, // Empty input triggers configuration test mode, not error
		},
		{
			name: "missing required fields",
			input: `{
				"hook_event_name": "PostToolUse"
			}`,
			exitCode: 1,
		},
		{
			name:     "not JSON object",
			input:    `"just a string"`,
			exitCode: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			stdin := strings.NewReader(tc.input)
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}

			exitCode := cli.Run([]string{"claudio"}, stdin, stdout, stderr)

			if exitCode != tc.exitCode {
				t.Errorf("Expected exit code %d, got %d", tc.exitCode, exitCode)
				t.Logf("Input: %s", tc.input)
				t.Logf("Stdout: %s", stdout.String())
				t.Logf("Stderr: %s", stderr.String())
			}

			// Error cases should have helpful error messages
			if tc.exitCode != 0 && stderr.Len() == 0 {
				t.Error("Expected error message for failed case")
			}
		})
	}
}

func TestCLIConfigOverrides(t *testing.T) {
	cli := NewCLI()

	// Create a temporary config file
	tempDir := t.TempDir()
	configFile := tempDir + "/test-config.json"

	configContent := `{
		"volume": 0.5,
		"default_soundpack": "default",
		"enabled": true,
		"log_level": "info"
	}`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	testCases := []struct {
		name string
		args []string
	}{
		{
			name: "config file override",
			args: []string{"claudio", "--config", configFile},
		},
		{
			name: "volume override",
			args: []string{"claudio", "--volume", "0.9"},
		},
		{
			name: "soundpack override", 
			args: []string{"claudio", "--soundpack", "mechanical"},
		},
		{
			name: "multiple overrides",
			args: []string{"claudio", "--config", configFile, "--volume", "0.8", "--soundpack", "test"},
		},
	}

	hookJSON := `{
		"session_id": "test",
		"transcript_path": "/test",
		"cwd": "/test",
		"hook_event_name": "PostToolUse",
		"tool_name": "Test"
	}`

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			stdin := strings.NewReader(hookJSON)
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}

			exitCode := cli.Run(tc.args, stdin, stdout, stderr)

			if exitCode != 0 {
				t.Errorf("Expected exit code 0, got %d", exitCode)
				t.Logf("Args: %v", tc.args)
				t.Logf("Stderr: %s", stderr.String())
			}
		})
	}
}

func TestCLISilentMode(t *testing.T) {
	cli := NewCLI()

	hookJSON := `{
		"session_id": "test",
		"transcript_path": "/test",
		"cwd": "/test", 
		"hook_event_name": "PostToolUse",
		"tool_name": "Bash"
	}`

	stdin := strings.NewReader(hookJSON)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	exitCode := cli.Run([]string{"claudio", "--silent"}, stdin, stdout, stderr)

	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
		t.Logf("Stderr: %s", stderr.String())
	}

	// In silent mode, should process but not play audio
	// (exact behavior depends on implementation)
	t.Logf("Silent mode output: %s", stdout.String())
}

func TestCLIEnvironmentVariables(t *testing.T) {
	cli := NewCLI()

	// Set environment variables
	os.Setenv("CLAUDIO_VOLUME", "0.6")
	os.Setenv("CLAUDIO_SOUNDPACK", "env-pack")
	defer func() {
		os.Unsetenv("CLAUDIO_VOLUME") 
		os.Unsetenv("CLAUDIO_SOUNDPACK")
	}()

	hookJSON := `{
		"session_id": "test",
		"transcript_path": "/test",
		"cwd": "/test",
		"hook_event_name": "PostToolUse", 
		"tool_name": "Test"
	}`

	stdin := strings.NewReader(hookJSON)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	exitCode := cli.Run([]string{"claudio"}, stdin, stdout, stderr)

	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
		t.Logf("Stderr: %s", stderr.String())
	}

	// Environment variables should be applied
	// (verification depends on implementation details)
}

func TestCLIErrorHandling(t *testing.T) {
	cli := NewCLI()

	testCases := []struct {
		name     string
		args     []string
		stdin    io.Reader
		exitCode int
	}{
		{
			name:     "stdin read error",
			args:     []string{"claudio"},
			stdin:    &errorReader{},
			exitCode: 1,
		},
		{
			name:     "too many arguments",
			args:     []string{"claudio", "extra", "args"},
			stdin:    strings.NewReader("{}"),
			exitCode: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}

			exitCode := cli.Run(tc.args, tc.stdin, stdout, stderr)

			if exitCode != tc.exitCode {
				t.Errorf("Expected exit code %d, got %d", tc.exitCode, exitCode)
				t.Logf("Stderr: %s", stderr.String())
			}

			// Should have helpful error messages
			if stderr.Len() == 0 {
				t.Error("Expected error message")
			}
		})
	}
}

func TestCLIVersionAndHelp(t *testing.T) {
	cli := NewCLI()

	t.Run("version output", func(t *testing.T) {
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}

		exitCode := cli.Run([]string{"claudio", "--version"}, strings.NewReader(""), stdout, stderr)

		if exitCode != 0 {
			t.Errorf("Expected exit code 0, got %d", exitCode)
		}

		output := stdout.String()
		if !strings.Contains(output, "claudio") {
			t.Error("Version output should contain 'claudio'")
		}

		if !strings.Contains(output, "version") || !strings.Contains(output, "Version") {
			t.Error("Version output should contain version information")
		}

		t.Logf("Version output: %s", output)
	})

	t.Run("help output", func(t *testing.T) {
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}

		exitCode := cli.Run([]string{"claudio", "--help"}, strings.NewReader(""), stdout, stderr)

		if exitCode != 0 {
			t.Errorf("Expected exit code 0, got %d", exitCode)
		}

		output := stdout.String()

		// Help should contain usage information
		expectedContent := []string{
			"claudio",
			"usage", "Usage",
			"--volume",
			"--soundpack",
			"--silent",
			"--config",
			"--help",
			"--version",
		}

		for _, content := range expectedContent {
			if !strings.Contains(output, content) {
				t.Errorf("Help output should contain '%s'", content)
			}
		}

		t.Logf("Help output: %s", output)
	})
}

func TestCLI_ResolvesDefaultSoundpackToPaths(t *testing.T) {
	// TDD Test: Verify CLI resolves DefaultSoundpack to actual XDG paths
	cli := NewCLI()

	// Create a temporary config that uses a default soundpack name
	tempDir := t.TempDir()
	configFile := tempDir + "/test-config.json"

	configContent := `{
		"volume": 0.5,
		"default_soundpack": "test-pack",
		"enabled": false,
		"log_level": "info"
	}`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	hookJSON := `{
		"session_id": "test",
		"transcript_path": "/test",
		"cwd": "/test",
		"hook_event_name": "PostToolUse",
		"tool_name": "Bash"
	}`

	stdin := strings.NewReader(hookJSON)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	// Run CLI with the config
	exitCode := cli.Run([]string{"claudio", "--config", configFile}, stdin, stdout, stderr)

	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
		t.Logf("Stderr: %s", stderr.String())
	}

	// Verify that the SoundLoader was initialized with XDG-resolved paths
	// This test will fail initially because CLI doesn't resolve soundpack names to paths
	if cli.soundLoader == nil {
		t.Fatal("SoundLoader should be initialized")
	}

	soundpackPaths := cli.soundLoader.GetSoundpackPaths()
	
	// Should have paths that include XDG directories for "test-pack"
	if len(soundpackPaths) == 0 {
		t.Error("SoundLoader should have resolved soundpack paths from XDG system")
	}

	// Paths should contain the soundpack name "test-pack"
	foundTestPackPath := false
	for _, path := range soundpackPaths {
		if strings.Contains(path, "test-pack") {
			foundTestPackPath = true
			break
		}
	}

	if !foundTestPackPath {
		t.Errorf("Expected soundpack paths to contain 'test-pack', got: %v", soundpackPaths)
		t.Log("This test will fail until CLI resolves DefaultSoundpack using XDG system")
	}
}

// Helper type for testing error conditions
type errorReader struct{}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, io.ErrUnexpectedEOF
}