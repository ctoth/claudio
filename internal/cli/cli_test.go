package cli

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLI(t *testing.T) {
	cli := NewCLI()

	if cli == nil {
		t.Fatal("NewCLI returned nil")
	}
	
	// COMMIT 4 RED: Expect CLI to have cobra root command
	if cli.rootCmd == nil {
		t.Fatal("CLI.rootCmd is nil - expected *cobra.Command")
	}
	
	if cli.rootCmd.Use != "claudio" {
		t.Errorf("Expected rootCmd.Use to be 'claudio', got %q", cli.rootCmd.Use)
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
	// Preserve original slog configuration to avoid test interference
	originalHandler := slog.Default().Handler()
	defer slog.SetDefault(slog.New(originalHandler))
	
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
			// Create fresh CLI instance for each test to avoid state pollution
			cli := NewCLI()
			
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

	// Verify that the unified soundpack resolver was initialized
	if cli.soundpackResolver == nil {
		t.Fatal("Soundpack resolver should be initialized")
	}

	// Verify the resolver type and name
	resolverType := cli.soundpackResolver.GetType()
	resolverName := cli.soundpackResolver.GetName()
	
	slog.Info("unified soundpack resolver initialized", 
		"type", resolverType, 
		"name", resolverName)
		
	// The resolver should be functional (type should be set)
	if resolverType == "" {
		t.Error("Soundpack resolver should have a valid type")
	}
}

func TestCLI_UsesSoundLoaderForFileResolution(t *testing.T) {
	// TDD Test: Verify CLI uses SoundLoader for file resolution instead of duplicate logic
	cli := NewCLI()

	// Create temporary soundpack with test file
	tempDir := t.TempDir()
	soundpackDir := tempDir + "/test-pack/success"
	err := os.MkdirAll(soundpackDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create soundpack dir: %v", err)
	}

	// Create a simple WAV file for testing
	wavFile := soundpackDir + "/bash-success.wav"
	wavData := createMinimalWAV()
	err = os.WriteFile(wavFile, wavData, 0644)
	if err != nil {
		t.Fatalf("Failed to create test WAV file: %v", err)
	}

	// Create config that points to our test soundpack
	configFile := tempDir + "/test-config.json"
	configContent := `{
		"volume": 0.5,
		"default_soundpack": "test-pack",
		"soundpack_paths": ["` + tempDir + `"],
		"enabled": false,
		"log_level": "info"
	}`

	err = os.WriteFile(configFile, []byte(configContent), 0644)
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

	// For now, this test just verifies the CLI runs without error when a sound file exists
	// The actual test for using SoundLoader will be validated by examining the CLI code
	// TODO: This test will be more meaningful after we refactor playSound method
	t.Log("CLI should use SoundLoader.LoadSound() instead of manual file resolution in playSound()")
}

// Helper type for testing error conditions
type errorReader struct{}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, io.ErrUnexpectedEOF
}

func TestVersionFlagEarlyExit(t *testing.T) {
	// TDD RED: This test should FAIL because version flag currently initializes audio systems
	// We expect version flag to show version info without any system initialization logging
	
	cli := NewCLI()
	
	// Capture all log output to verify no system initialization occurs
	var logBuffer bytes.Buffer
	originalHandler := slog.Default().Handler()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logBuffer, &slog.HandlerOptions{
		Level: slog.LevelDebug, // Capture all logs
	})))
	defer slog.SetDefault(slog.New(originalHandler))
	
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	
	exitCode := cli.Run([]string{"claudio", "--version"}, strings.NewReader(""), stdout, stderr)
	
	// Version flag should exit successfully 
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}
	
	// Should show version info
	output := stdout.String()
	if !strings.Contains(output, "claudio version") {
		t.Errorf("Expected version output, got: %s", output)
	}
	
	// CRITICAL: Should NOT initialize any audio systems
	logOutput := logBuffer.String()
	prohibitedLogs := []string{
		"audio player created",
		"config loaded",
		"soundpack resolver initialized", 
		"configuration loaded",
		"audio context initialized",
	}
	
	for _, prohibited := range prohibitedLogs {
		if strings.Contains(logOutput, prohibited) {
			t.Errorf("Version flag should not initialize systems, but found log: %s", prohibited)
			t.Logf("Full log output: %s", logOutput)
		}
	}
	
	// Version flag should be fast - no heavy initialization
	if len(logOutput) > 100 {
		t.Errorf("Version flag should produce minimal logging, got %d chars: %s", len(logOutput), logOutput)
	}
}

func TestToolNameStringLogging(t *testing.T) {
	// TDD RED: This test should FAIL because tool names currently log as memory addresses
	// We expect tool names to appear as strings, not pointer addresses like "0xc000116480"
	
	cli := NewCLI()
	
	// Capture all log output to verify tool names are logged as strings
	var logBuffer bytes.Buffer
	originalHandler := slog.Default().Handler()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logBuffer, &slog.HandlerOptions{
		Level: slog.LevelDebug, // Capture all logs
	})))
	defer slog.SetDefault(slog.New(originalHandler))
	
	// Hook JSON with explicit tool name
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
	
	exitCode := cli.Run([]string{"claudio", "--silent"}, stdin, stdout, stderr)
	
	// Should process successfully
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
		t.Logf("Stderr: %s", stderr.String())
	}
	
	// CRITICAL: Tool name should appear as string "Bash", not memory address
	logOutput := logBuffer.String()
	
	// Should contain tool name as string
	if !strings.Contains(logOutput, "tool_name=Bash") && !strings.Contains(logOutput, `tool_name="Bash"`) {
		t.Errorf("Expected tool name to appear as string 'Bash' in logs")
		t.Logf("Full log output: %s", logOutput)
	}
	
	// Should NOT contain memory addresses (like 0xc000116480)
	if strings.Contains(logOutput, "0x") {
		t.Errorf("Tool name should not appear as memory address, but found pointer reference in logs")
		t.Logf("Full log output: %s", logOutput)
	}
	
	// Verify the specific pattern we expect to fix
	if strings.Contains(logOutput, "tool_name=0x") {
		t.Errorf("Found the exact bug: tool_name=0x... memory address instead of string")
		t.Logf("Full log output: %s", logOutput)
	}
}

func TestCLIUnifiedSoundpackIntegration(t *testing.T) {
	// TDD Test: CLI integration with new unified soundpack system

	t.Run("supports directory soundpack with unified system", func(t *testing.T) {
		cli := NewCLI()
		tempDir := t.TempDir()

		// Create directory soundpack structure
		soundpackDir := filepath.Join(tempDir, "unified-test")
		successDir := filepath.Join(soundpackDir, "success")
		err := os.MkdirAll(successDir, 0755)
		if err != nil {
			t.Fatalf("Failed to create soundpack dirs: %v", err)
		}

		// Create test sound file
		soundFile := filepath.Join(successDir, "bash.wav")
		wavData := createMinimalWAV()
		err = os.WriteFile(soundFile, wavData, 0644)
		if err != nil {
			t.Fatalf("Failed to create test WAV file: %v", err)
		}

		// Create config that points to our test soundpack
		configFile := filepath.Join(tempDir, "test-config.json")
		configContent := fmt.Sprintf(`{
			"volume": 0.5,
			"default_soundpack": "unified-test",
			"soundpack_paths": ["%s"],
			"enabled": false,
			"log_level": "warn"
		}`, tempDir)

		err = os.WriteFile(configFile, []byte(configContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create test config: %v", err)
		}

		hookJSON := `{
			"session_id": "test",
			"transcript_path": "/test",
			"cwd": "/test",
			"hook_event_name": "PostToolUse",
			"tool_name": "Bash",
			"tool_response": {"stdout": "success", "stderr": "", "interrupted": false}
		}`

		stdin := strings.NewReader(hookJSON)
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}

		// Run CLI - should use unified soundpack system
		exitCode := cli.Run([]string{"claudio", "--config", configFile}, stdin, stdout, stderr)

		if exitCode != 0 {
			t.Errorf("Expected exit code 0, got %d", exitCode)
			t.Logf("Stderr: %s", stderr.String())
		}
	})

	t.Run("supports JSON soundpack with unified system", func(t *testing.T) {
		cli := NewCLI()
		tempDir := t.TempDir()

		// Create test sound file
		soundFile := filepath.Join(tempDir, "test-sound.wav")
		wavData := createMinimalWAV()
		err := os.WriteFile(soundFile, wavData, 0644)
		if err != nil {
			t.Fatalf("Failed to create test WAV file: %v", err)
		}

		// Create JSON soundpack file
		jsonFile := filepath.Join(tempDir, "test-soundpack.json")
		jsonContent := fmt.Sprintf(`{
			"name": "json-unified-test",
			"description": "Test JSON soundpack for CLI integration",
			"mappings": {
				"success/bash.wav": "%s",
				"default.wav": "%s"
			}
		}`, soundFile, soundFile)

		err = os.WriteFile(jsonFile, []byte(jsonContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create JSON soundpack: %v", err)
		}

		// Create config that points to our JSON soundpack
		configFile := filepath.Join(tempDir, "test-config.json")
		configContent := fmt.Sprintf(`{
			"volume": 0.5,
			"default_soundpack": "%s",
			"enabled": false,
			"log_level": "warn"
		}`, jsonFile)

		err = os.WriteFile(configFile, []byte(configContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create test config: %v", err)
		}

		hookJSON := `{
			"session_id": "test",
			"transcript_path": "/test", 
			"cwd": "/test",
			"hook_event_name": "PostToolUse",
			"tool_name": "Bash",
			"tool_response": {"stdout": "success", "stderr": "", "interrupted": false}
		}`

		stdin := strings.NewReader(hookJSON)
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}

		// Run CLI - should use unified soundpack system with JSON soundpack
		exitCode := cli.Run([]string{"claudio", "--config", configFile}, stdin, stdout, stderr)

		if exitCode != 0 {
			t.Errorf("Expected exit code 0, got %d", exitCode)
			t.Logf("Stderr: %s", stderr.String())
		}
	})

	t.Run("handles missing soundpack gracefully with unified system", func(t *testing.T) {
		cli := NewCLI()
		tempDir := t.TempDir()

		// Create config that points to non-existent soundpack
		configFile := filepath.Join(tempDir, "test-config.json")
		configContent := `{
			"volume": 0.5,
			"default_soundpack": "nonexistent-soundpack",
			"enabled": false,
			"log_level": "warn"
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

		// Run CLI - should handle missing soundpack gracefully
		exitCode := cli.Run([]string{"claudio", "--config", configFile}, stdin, stdout, stderr)

		if exitCode != 0 {
			t.Errorf("Expected exit code 0, got %d", exitCode)
			t.Logf("Stderr: %s", stderr.String())
		}
	})
}

func TestCLILoggingLevels(t *testing.T) {
	// TDD RED: This test should FAIL because CLI system initialization currently uses INFO logging
	// We expect routine CLI operations to use DEBUG level, not INFO level
	
	// Capture log output to verify log levels
	var logBuffer bytes.Buffer
	originalHandler := slog.Default().Handler()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logBuffer, &slog.HandlerOptions{
		Level: slog.LevelDebug, // Capture all logs
	})))
	defer slog.SetDefault(slog.New(originalHandler))
	
	cli := NewCLI()
	defer func() {
		if err := cli.rootCmd.Context(); err == nil {
			// CLI cleanup if needed
		}
	}()
	
	// Test CLI with hook processing - triggers system initialization
	hookJSON := `{
		"session_id": "test",
		"transcript_path": "/test",
		"cwd": "/test",
		"hook_event_name": "PostToolUse",
		"tool_name": "Bash",
		"tool_response": {"stdout": "success", "stderr": "", "interrupted": false}
	}`
	
	stdin := strings.NewReader(hookJSON)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	
	// Run CLI - should trigger system initialization with DEBUG level logging
	exitCode := cli.Run([]string{"claudio", "--silent"}, stdin, stdout, stderr)
	
	if exitCode != 0 {
		t.Fatalf("CLI run should succeed, got exit code %d: %s", exitCode, stderr.String())
	}
	
	logOutput := logBuffer.String()
	
	// CRITICAL: Routine operations should use DEBUG level, not INFO
	problematicInfoLogs := []string{
		"configuration loaded",
		"soundpack resolver initialized",
	}
	
	for _, logMsg := range problematicInfoLogs {
		// Split into lines and check each line individually
		lines := strings.Split(logOutput, "\n")
		for _, line := range lines {
			if strings.Contains(line, logMsg) && strings.Contains(line, "level=INFO") {
				t.Errorf("Routine operation '%s' should use DEBUG level, not INFO level", logMsg)
				t.Logf("Problematic line: %s", line)
				t.Logf("Full log output: %s", logOutput)
			}
		}
	}
	
	// Verify that DEBUG logs are working properly
	if !strings.Contains(logOutput, "level=DEBUG") {
		t.Error("Expected some DEBUG level logs but found none")
		t.Logf("Full log output: %s", logOutput)
	}
}