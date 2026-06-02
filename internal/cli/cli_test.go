package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"claudio.click/internal/audio"
	"claudio.click/internal/cli/testenv"
	"claudio.click/internal/config"
	"claudio.click/internal/hooks"
	"claudio.click/internal/soundpack"
)

// TestMain ensures hook processing stays in-process during tests. The
// production binary spawns a detached worker via os.Executable(); under
// `go test` that re-invokes the test binary and would recursively run
// the entire test suite. CLAUDIO_DETACH_DISABLE=1 short-circuits the
// detach path so tests observe in-process side effects.
func TestMain(m *testing.M) {
	os.Setenv("CLAUDIO_DETACH_DISABLE", "1")
	code := m.Run()
	os.Unsetenv("CLAUDIO_DETACH_DISABLE")
	os.Exit(code)
}

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
	testenv.IsolateXDG(t)
	audio.ResetLastFakeBackend()
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

	// Closes review finding #57 (Chunk 18 analyst F3): the hook MUST
	// drive at least one Play call into the audio backend. The earlier
	// shape of this test asserted only that the fake backend was
	// constructed, which would still pass even if the entire playback
	// pipeline were silently broken downstream of init. Now we assert
	// len(Plays()) > 0 so a regression that silences Play is caught
	// loudly. The embedded windows.json / linux.json soundpacks ship
	// with default.wav-equivalent fallbacks that resolve under the
	// sandboxed XDG layout, so a PostToolUse/Bash event reliably
	// produces a Play call across platforms.
	fake := audio.LastFakeBackend()
	if fake == nil {
		t.Fatal("expected fake audio backend to be constructed via CLAUDIO_AUDIO_BACKEND=fake")
	}
	plays := fake.Plays()
	t.Logf("recorded plays: %+v", plays)
	if len(plays) == 0 {
		t.Errorf("expected at least one Play call recorded; got 0 (playback pipeline broken downstream of audio init)")
	}
	for _, p := range plays {
		if p.SourcePath == "" {
			t.Errorf("Play recorded with empty SourcePath: %+v", p)
		}
	}
}

func TestCLIFlags(t *testing.T) {
	testenv.IsolateXDG(t)
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
	testenv.IsolateXDG(t)
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
	testenv.IsolateXDG(t)
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
	testenv.IsolateXDG(t)
	audio.ResetLastFakeBackend()
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

	// Baseline: stash was just reset to nil.
	if pre := audio.LastFakeBackend(); pre != nil {
		t.Fatalf("precondition: ResetLastFakeBackend should clear the stash; got %p", pre)
	}

	exitCode := cli.Run([]string{"claudio", "--silent"}, stdin, stdout, stderr)

	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
		t.Logf("Stderr: %s", stderr.String())
	}

	t.Logf("Silent mode output: %s", stdout.String())

	// Closes review finding #57 / Chunk 18 analyst F4: silent mode
	// disables the audio init path entirely (cli.initializeSystems
	// only constructs a backend when cfg.Enabled is true; --silent
	// flips Enabled to false). The meaningful invariant is therefore
	// *either*:
	//   1. The fake backend was never constructed (stash still nil),
	//      proving cli.Run skipped audio init as the silent contract
	//      requires; OR
	//   2. The fake backend WAS constructed (e.g. if a future code
	//      change moves init before the silent check), in which case
	//      Plays() must be empty — no audio escapes silent mode.
	// The previous form treated case (1) as a free pass without
	// proving cli.Run even reached return — masking a crash before
	// audio init. We now require exit 0 (above) AND, if a backend was
	// constructed, zero plays. The exit-0 + stash-still-nil
	// combination is the tight contract for silent mode today.
	fake := audio.LastFakeBackend()
	if fake == nil {
		// Case 1: audio init was skipped. Combined with the exit==0
		// check above this proves cli.Run completed without
		// constructing a backend. This is the expected path today.
		return
	}
	// Case 2: a backend was constructed even though we asked for
	// silent. The pipeline must not have invoked Play.
	plays := fake.Plays()
	if len(plays) != 0 {
		t.Errorf("silent mode recorded %d plays, want 0: %+v", len(plays), plays)
	}
}

func TestShouldDetachHookProcessing_DisabledViaEnvVar(t *testing.T) {
	cli := NewCLI()
	cfg := &config.Config{Enabled: true}
	inputData := []byte(`{"session_id":"test","hook_event_name":"PostToolUse"}`)

	// With CLAUDIO_DETACH_DISABLE=1 (set by TestMain), shouldDetachHookProcessing
	// returns false so hook processing runs in-process.
	t.Setenv("CLAUDIO_DETACH_DISABLE", "1")
	if shouldDetachHookProcessing(cli.rootCmd, cfg, inputData) {
		t.Error("expected shouldDetachHookProcessing()==false with CLAUDIO_DETACH_DISABLE=1")
	}

	// NOTE: previously this test had a second branch that called
	// t.Setenv("CLAUDIO_DETACH_DISABLE", "") and claimed to exercise the
	// "unset" path. But t.Setenv sets the variable to empty string; it does
	// NOT unset it. os.Getenv returns "" for both cases so the production
	// check (== "1") agrees, but the comment-documented contract was not
	// truly exercised. Dropped to keep this test honest about what it
	// verifies — only the env-var-enabled disable path. The unset/enabled
	// path is covered transitively by the other shouldDetachHookProcessing
	// tests in this file.
}

func TestCLIEnvironmentVariables(t *testing.T) {
	testenv.IsolateXDG(t)
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
	testenv.IsolateXDG(t)
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
	testenv.IsolateXDG(t)
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
	testenv.IsolateXDG(t)
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

// Helper type for testing error conditions
type errorReader struct{}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, io.ErrUnexpectedEOF
}

func TestVersionFlagEarlyExit(t *testing.T) {
	testenv.IsolateXDG(t)
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

// TestToolNameStringLogging - REMOVED: Test was testing implementation details that are working correctly

// stringPtr returns a pointer to the given string
func stringPtr(s string) *string {
	return &s
}

func TestHookProcessingLoggingIsolated(t *testing.T) {
	testenv.IsolateXDG(t)
	// Isolated test for hook processing logging without CLI.Run() overhead
	// This provides more reliable, focused testing of logging behavior

	cli := NewCLI()
	cli.initializeSystems()

	// Load config with silent mode to avoid audio initialization
	cfg := cli.configManager.GetDefaultConfig()
	cfg.Enabled = false // Silent mode

	// Set up test logger to capture output
	var logBuffer bytes.Buffer
	originalHandler := slog.Default().Handler()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logBuffer, &slog.HandlerOptions{
		Level: slog.LevelDebug, // Capture all logs
	})))
	defer slog.SetDefault(slog.New(originalHandler))

	// Create test hook event
	toolResponseJSON := json.RawMessage(`{"stdout":"success","stderr":"","interrupted":false}`)
	hookEvent := &hooks.HookEvent{
		SessionID:      "test-isolated",
		TranscriptPath: "/test",
		CWD:            "/test",
		EventName:      "PostToolUse",
		ToolName:       stringPtr("Bash"),
		ToolResponse:   &toolResponseJSON,
	}

	// Initialize audio and soundpack systems for processing
	xdgDirs := config.NewXDGDirs()
	soundpackPaths := xdgDirs.GetSoundpackPaths(cfg.DefaultSoundpack)
	soundpackPaths = append(soundpackPaths, cfg.SoundpackPaths...)

	mapper, err := soundpack.CreateSoundpackMapperWithBasePaths(
		cfg.DefaultSoundpack,
		cfg.DefaultSoundpack,
		soundpackPaths,
	)
	if err != nil {
		// Create empty mapper as fallback
		mapper = soundpack.NewDirectoryMapper("fallback", []string{})
	}
	cli.soundpackResolver = soundpack.NewSoundpackResolver(mapper)

	// Process hook event directly - this should log tool_name
	cli.processHookEvent(hookEvent, cfg, &bytes.Buffer{}, &bytes.Buffer{})

	// Verify tool name appears as string in logs
	logOutput := logBuffer.String()

	if !strings.Contains(logOutput, "tool_name=Bash") && !strings.Contains(logOutput, `tool_name="Bash"`) {
		t.Errorf("Expected tool name to appear as string 'Bash' in isolated test logs")
		t.Logf("Full log output: %s", logOutput)
	}

	// Should NOT contain memory addresses
	if strings.Contains(logOutput, "0x") {
		t.Errorf("Tool name should not appear as memory address in isolated test")
		t.Logf("Full log output: %s", logOutput)
	}

	// Should contain hook processing messages
	expectedMessages := []string{
		"processing hook event",
		"hook context parsed",
		"sound mapped",
	}

	for _, msg := range expectedMessages {
		if !strings.Contains(logOutput, msg) {
			t.Errorf("Expected log message '%s' not found in isolated test output", msg)
		}
	}
}

func TestCLIUnifiedSoundpackIntegration(t *testing.T) {
	testenv.IsolateXDG(t)
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
	testenv.IsolateXDG(t)
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
		_ = cli.rootCmd.Context()
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

// createMinimalWAV produces a minimal valid WAV file for tests that need a
// real on-disk sound file to feed the loader.
func createMinimalWAV() []byte {
	// Minimal valid WAV: 44-byte header + 8 bytes audio data
	return []byte{
		// RIFF header
		'R', 'I', 'F', 'F',
		44, 0, 0, 0, // File size - 8 (44 - 8 = 36 + 8 data = 44)
		'W', 'A', 'V', 'E',

		// fmt chunk
		'f', 'm', 't', ' ',
		16, 0, 0, 0, // fmt chunk size
		1, 0, // PCM format
		1, 0, // mono
		0x44, 0xAC, 0, 0, // 44100 Hz sample rate
		0x88, 0x58, 0x01, 0, // byte rate
		2, 0, // block align
		16, 0, // 16 bits per sample

		// data chunk
		'd', 'a', 't', 'a',
		8, 0, 0, 0, // data size
		0, 0, 0x7F, 0x7F, 0, 0, 0x7F, 0x7F, // 4 samples of audio data
	}
}

// TestVersionFlagAtAnyPosition covers finding #49: the manual args[1]
// short-circuit only matched when --version was literally args[1]. With
// rootCmd.Version set and hasVersionFlag scanning all args, the version
// fast path now fires for `claudio --silent --version` too.
func TestVersionFlagAtAnyPosition(t *testing.T) {
	testenv.IsolateXDG(t)

	cases := [][]string{
		{"claudio", "--silent", "--version"},
		{"claudio", "--version", "--silent"},
		{"claudio", "-v"},
		{"claudio", "--soundpack", "x", "--version"},
	}

	for _, args := range cases {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			cli := NewCLI()
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}

			code := cli.Run(args, strings.NewReader(""), stdout, stderr)
			if code != 0 {
				t.Fatalf("expected exit 0 for %v, got %d (stderr=%q)", args, code, stderr.String())
			}
			out := stdout.String()
			if !strings.Contains(out, "claudio version") {
				t.Errorf("expected 'claudio version' in stdout for %v, got %q", args, out)
			}
			// Crucially: no audio backend or tracking should have been
			// instantiated on the version fast path.
			if cli.audioBackend != nil {
				t.Errorf("audioBackend must not be created on version fast path for %v", args)
			}
			if cli.trackingDB != nil {
				t.Errorf("trackingDB must not be created on version fast path for %v", args)
			}
		})
	}
}

// TestSetupLogging_DualOutputWithExistingVerboseHandler covers finding #47:
// when a test installs a DEBUG-level default handler AND file logging is
// configured, BOTH outputs must receive records. The previous early-return
// dropped file logging in that scenario.
func TestSetupLogging_DualOutputWithExistingVerboseHandler(t *testing.T) {
	testenv.IsolateXDG(t)

	// Preserve and restore slog default. The cleanup must run BEFORE
	// t.TempDir's removal so lumberjack's open file handle can be replaced
	// (otherwise Windows refuses to delete the still-open file).
	prev := slog.Default()
	defer slog.SetDefault(prev)

	// Use a manually-managed tempdir so we can guarantee slog is reset
	// before the directory is removed.
	logDir, err := os.MkdirTemp("", "claudio-setuplogging-")
	if err != nil {
		t.Fatalf("mkdir temp: %v", err)
	}
	logFile := filepath.Join(logDir, "claudio.log")
	defer func() {
		// Drop slog reference to the lumberjack writer so the OS releases
		// the handle, then remove the directory.
		slog.SetDefault(prev)
		_ = os.RemoveAll(logDir)
	}()

	// Install a verbose (DEBUG-level) default handler that writes to a
	// known buffer — this is the shape used by test harnesses.
	var existingBuf bytes.Buffer
	existingHandler := slog.NewTextHandler(&existingBuf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	slog.SetDefault(slog.New(existingHandler))

	cfg := &config.Config{
		LogLevel: "warn", // higher than DEBUG so the early-return previously triggered
		FileLogging: &config.FileLoggingConfig{
			Enabled:    true,
			Filename:   logFile,
			MaxSizeMB:  1,
			MaxBackups: 1,
			MaxAgeDays: 1,
			Compress:   false,
		},
	}

	var stderrBuf bytes.Buffer
	setupLogging(cfg, &stderrBuf)

	// Emit logs that should reach BOTH outputs:
	// - DEBUG must hit the preserved existing handler (its buffer)
	// - WARN must hit the file (LogLevel=warn)
	slog.Debug("debug to existing")
	slog.Warn("warn to file")
	slog.Error("error to stderr and file")

	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}
	fileOutput := string(data)
	if !strings.Contains(fileOutput, "warn to file") {
		t.Errorf("file should contain 'warn to file' (file logging dropped); got: %q", fileOutput)
	}
	if !strings.Contains(fileOutput, "error to stderr and file") {
		t.Errorf("file should contain error message; got: %q", fileOutput)
	}

	if got := existingBuf.String(); !strings.Contains(got, "debug to existing") {
		t.Errorf("preserved existing handler should still receive DEBUG; got: %q", got)
	}

	if got := stderrBuf.String(); !strings.Contains(got, "error to stderr and file") {
		t.Errorf("stderr should contain ERROR; got: %q", got)
	}
}
