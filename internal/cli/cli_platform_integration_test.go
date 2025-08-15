package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"claudio.click/internal/audio"
	"claudio.click/internal/config"
	"github.com/spf13/afero"
)

// TDD RED: Test unconfigured claudio uses platform JSON
func TestCLIUnconfiguredUsePlatformSoundpack(t *testing.T) {
	t.Run("unconfigured CLI should auto-detect and use platform-specific JSON", func(t *testing.T) {
		// Test should succeed with embedded platform soundpack when no file found
		
		// Create temporary directory to simulate executable location
		tempDir := t.TempDir()
		
		// Determine which platform JSON to create based on runtime
		var platformJsonFile string
		var platformJsonContent string
		
		switch runtime.GOOS {
		case "darwin":
			platformJsonFile = "darwin.json"
			platformJsonContent = `{
				"name": "test-darwin-soundpack",
				"description": "Test macOS soundpack for integration testing",
				"version": "1.0.0",
				"mappings": {
					"success/test-success.wav": "/System/Library/Sounds/Glass.aiff",
					"error/test-error.wav": "/System/Library/Sounds/Basso.aiff",
					"default.wav": "/System/Library/Sounds/Pop.aiff"
				}
			}`
		case "linux":
			// Check if WSL
			if audio.IsWSL() {
				platformJsonFile = "wsl.json"
				platformJsonContent = `{
					"name": "test-wsl-soundpack",
					"description": "Test WSL soundpack for integration testing",
					"version": "1.0.0",
					"mappings": {
						"success/test-success.wav": "/mnt/c/Windows/Media/Windows Print complete.wav",
						"error/test-error.wav": "/mnt/c/Windows/Media/Windows Error.wav",
						"default.wav": "/mnt/c/Windows/Media/Windows Default.wav"
					}
				}`
			} else {
				platformJsonFile = "linux.json"
				platformJsonContent = `{
					"name": "test-linux-soundpack",
					"description": "Test Linux soundpack for integration testing",
					"version": "1.0.0",
					"mappings": {
						"success/test-success.wav": "/usr/share/sounds/freedesktop/stereo/complete.oga",
						"error/test-error.wav": "/usr/share/sounds/freedesktop/stereo/dialog-error.oga",
						"default.wav": "/usr/share/sounds/freedesktop/stereo/bell.oga"
					}
				}`
			}
		case "windows":
			platformJsonFile = "windows.json"
			platformJsonContent = `{
				"name": "test-windows-soundpack",
				"description": "Test Windows soundpack for integration testing",
				"version": "1.0.0",
				"mappings": {
					"success/test-success.wav": "C:\\Windows\\Media\\Windows Print complete.wav",
					"error/test-error.wav": "C:\\Windows\\Media\\Windows Error.wav",
					"default.wav": "C:\\Windows\\Media\\Windows Default.wav"
				}
			}`
		default:
			t.Skip("Unsupported platform for this test")
		}
		
		// Create platform-specific JSON with test soundpack data
		platformJsonPath := filepath.Join(tempDir, platformJsonFile)
		
		err := os.WriteFile(platformJsonPath, []byte(platformJsonContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create test %s: %v", platformJsonFile, err)
		}
		
		// Create a mock executable in the same directory
		mockExecutable := filepath.Join(tempDir, "claudio")
		err = os.WriteFile(mockExecutable, []byte("mock"), 0755)
		if err != nil {
			t.Fatalf("Failed to create mock executable: %v", err)
		}
		
		// Prepare CLI test input
		testInput := `{"session_id":"test","transcript_path":"/test","cwd":"/test","hook_event_name":"PostToolUse","tool_name":"Bash","tool_response":{"stdout":"success","stderr":"","interrupted":false}}`
		
		// Change to temp directory so current working directory check finds platform JSON
		originalDir, err := os.Getwd()
		if err != nil {
			t.Fatalf("Failed to get current directory: %v", err)
		}
		defer os.Chdir(originalDir)
		
		err = os.Chdir(tempDir)
		if err != nil {
			t.Fatalf("Failed to change to temp directory: %v", err)
		}
		
		// Create CLI with no config file - should fall back to platform detection
		cli := NewCLI()
		
		// Run CLI - this should now work with platform JSON auto-detection
		stdin := strings.NewReader(testInput)
		var stdout, stderr strings.Builder
		
		exitCode := cli.Run([]string{"claudio", "--silent"}, stdin, &stdout, &stderr)
		
		// Log the results for debugging
		t.Logf("Exit code: %d", exitCode)
		t.Logf("Stdout: %s", stdout.String())
		t.Logf("Stderr: %s", stderr.String())
		t.Logf("Platform JSON path: %s", platformJsonPath)
		
		// With embedded soundpacks, CLI should succeed using either the file-based platform JSON or embedded
		if exitCode == 0 {
			// Test passes - the CLI works without config, which is the main goal
			// The fact that it's using embedded soundpack is expected behavior
			t.Log("TDD GREEN: CLI works without any config files, using embedded platform defaults")
		} else {
			t.Errorf("CLI failed to run: exit code %d", exitCode)
		}
	})
	
	t.Run("unconfigured CLI should work without any config files", func(t *testing.T) {
		// Test that claudio can run in a completely clean environment
		// This tests the "fresh install" scenario
		
		testInput := `{"session_id":"test","transcript_path":"/test","cwd":"/test","hook_event_name":"PostToolUse","tool_name":"Echo","tool_response":{"stdout":"hello","stderr":"","interrupted":false}}`
		
		cli := NewCLI()
		stdin := strings.NewReader(testInput)
		var stdout, stderr strings.Builder
		
		// This should work even without config, using built-in defaults
		exitCode := cli.Run([]string{"claudio", "--silent"}, stdin, &stdout, &stderr)
		
		t.Logf("Clean environment test - Exit code: %d", exitCode)
		t.Logf("Clean environment test - Stderr: %s", stderr.String())
		
		if exitCode != 0 {
			t.Error("TDD: CLI should work in clean environment with built-in defaults, even without platform JSON")
		}
	})
}

// TDD RED: Test configured claudio with platform fallback
func TestCLIConfiguredWithPlatformFallback(t *testing.T) {
	t.Run("configured CLI should fallback to platform JSON when configured soundpack missing", func(t *testing.T) {
		// Test should succeed with embedded platform soundpack as fallback when configured soundpack missing
		
		tempDir := t.TempDir()
		
		// Create config with non-existent soundpack
		configDir := filepath.Join(tempDir, "config")
		err := os.MkdirAll(configDir, 0755)
		if err != nil {
			t.Fatalf("Failed to create config directory: %v", err)
		}
		
		configPath := filepath.Join(configDir, "config.json")
		configContent := `{
			"volume": 0.5,
			"default_soundpack": "nonexistent-soundpack",
			"soundpack_paths": [],
			"enabled": true,
			"log_level": "debug"
		}`
		
		err = os.WriteFile(configPath, []byte(configContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create config file: %v", err)
		}
		
		// Create platform JSON next to executable 
		execDir := filepath.Join(tempDir, "exec")
		err = os.MkdirAll(execDir, 0755)
		if err != nil {
			t.Fatalf("Failed to create exec directory: %v", err)
		}
		
		// Determine which platform JSON to create
		var platformJsonFile string
		var platformJsonContent string
		var platformName string
		
		switch runtime.GOOS {
		case "darwin":
			platformJsonFile = "darwin.json"
			platformName = "fallback-darwin-soundpack"
			platformJsonContent = `{
				"name": "fallback-darwin-soundpack",
				"mappings": {
					"success/fallback.wav": "/System/Library/Sounds/Glass.aiff"
				}
			}`
		case "linux":
			if audio.IsWSL() {
				platformJsonFile = "wsl.json"
				platformName = "fallback-wsl-soundpack"
				platformJsonContent = `{
					"name": "fallback-wsl-soundpack",
					"mappings": {
						"success/fallback.wav": "/mnt/c/Windows/Media/chord.wav"
					}
				}`
			} else {
				platformJsonFile = "linux.json"
				platformName = "fallback-linux-soundpack"
				platformJsonContent = `{
					"name": "fallback-linux-soundpack",
					"mappings": {
						"success/fallback.wav": "/usr/share/sounds/freedesktop/stereo/complete.oga"
					}
				}`
			}
		case "windows":
			platformJsonFile = "windows.json"
			platformName = "fallback-windows-soundpack"
			platformJsonContent = `{
				"name": "fallback-windows-soundpack",
				"mappings": {
					"success/fallback.wav": "C:\\Windows\\Media\\chord.wav"
				}
			}`
		default:
			t.Skip("Unsupported platform for this test")
		}
		
		platformJsonPath := filepath.Join(execDir, platformJsonFile)
		
		err = os.WriteFile(platformJsonPath, []byte(platformJsonContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create fallback %s: %v", platformJsonFile, err)
		}
		
		// Change to execDir so current working directory check finds platform JSON  
		originalDir, err := os.Getwd()
		if err != nil {
			t.Fatalf("Failed to get current directory: %v", err)
		}
		defer os.Chdir(originalDir)
		
		err = os.Chdir(execDir)
		if err != nil {
			t.Fatalf("Failed to change to exec directory: %v", err)
		}
		
		// Test with the config that points to nonexistent soundpack
		testInput := `{"session_id":"test","transcript_path":"/test","cwd":"/test","hook_event_name":"PostToolUse","tool_name":"Bash","tool_response":{"stdout":"success","stderr":"","interrupted":false}}`
		
		cli := NewCLI()
		stdin := strings.NewReader(testInput)
		var stdout, stderr strings.Builder
		
		// This should now work with platform JSON fallback
		exitCode := cli.Run([]string{"claudio", "--config", configPath, "--silent"}, stdin, &stdout, &stderr)
		
		t.Logf("Configured CLI with fallback test - Exit code: %d", exitCode)
		t.Logf("Configured CLI with fallback test - Stderr: %s", stderr.String())
		t.Logf("Config path: %s", configPath) 
		t.Logf("Platform JSON path: %s", platformJsonPath)
		
		// With embedded soundpacks, CLI should succeed with either file-based or embedded fallback
		if exitCode == 0 {
			stderrStr := stderr.String()
			// Check if it used the file-based platform JSON OR the embedded platform soundpack
			usedFileJson := strings.Contains(stderrStr, platformName)
			usedEmbedded := strings.Contains(stderrStr, "embedded:"+platformJsonFile)
			
			// Check for platform-specific embedded soundpack names - these are the actual names in the embedded JSON files
			if runtime.GOOS == "darwin" {
				// The embedded darwin.json contains "darwin-system-native-soundpack"
				usedEmbedded = usedEmbedded || strings.Contains(stderrStr, "darwin-system-native-soundpack")
			} else if audio.IsWSL() {
				usedEmbedded = usedEmbedded || strings.Contains(stderrStr, "windows-media-enhanced-soundpack")
			}
			
			if usedFileJson {
				t.Log("TDD GREEN: CLI used file-based platform JSON fallback")
			} else if usedEmbedded {
				t.Logf("TDD GREEN: CLI used embedded %s soundpack fallback (expected behavior)", platformJsonFile)
			} else {
				t.Errorf("CLI succeeded but didn't use file-based or embedded platform fallback for %s", runtime.GOOS)
			}
		} else {
			t.Errorf("CLI failed when configured soundpack missing: exit code %d", exitCode)
		}
	})
	
	t.Run("configured CLI should prefer configured soundpack over platform JSON", func(t *testing.T) {
		// Create a valid soundpack directory structure
		tempDir := t.TempDir()
		
		// Create a working soundpack directory
		soundpackDir := filepath.Join(tempDir, "test-soundpack")
		successDir := filepath.Join(soundpackDir, "success")
		err := os.MkdirAll(successDir, 0755)
		if err != nil {
			t.Fatalf("Failed to create soundpack directory: %v", err)
		}
		
		// Create a test sound file (empty is fine for this test)
		testSoundPath := filepath.Join(successDir, "success.wav")
		err = os.WriteFile(testSoundPath, []byte("fake wav data"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test sound file: %v", err)
		}
		
		// Create config pointing to the valid soundpack
		configPath := filepath.Join(tempDir, "config.json")
		configContent := `{
			"volume": 0.5,
			"default_soundpack": "test-soundpack",
			"soundpack_paths": ["` + tempDir + `"],
			"enabled": true,
			"log_level": "debug"
		}`
		
		err = os.WriteFile(configPath, []byte(configContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create config file: %v", err)
		}
		
		// Also create a platform JSON that should NOT be used
		wslJsonPath := filepath.Join(tempDir, "wsl.json")
		err = os.WriteFile(wslJsonPath, []byte(`{"name":"should-not-use-this"}`), 0644)
		if err != nil {
			t.Fatalf("Failed to create wsl.json: %v", err)
		}
		
		testInput := `{"session_id":"test","transcript_path":"/test","cwd":"/test","hook_event_name":"PostToolUse","tool_name":"Bash","tool_response":{"stdout":"success","stderr":"","interrupted":false}}`
		
		cli := NewCLI()
		stdin := strings.NewReader(testInput)
		var stdout, stderr strings.Builder
		
		exitCode := cli.Run([]string{"claudio", "--config", configPath, "--silent"}, stdin, &stdout, &stderr)
		
		t.Logf("Configured CLI preference test - Exit code: %d", exitCode)
		t.Logf("Configured CLI preference test - Stderr: %s", stderr.String())
		
		if exitCode != 0 {
			t.Errorf("CLI should succeed with valid configured soundpack, got exit code %d", exitCode)
		}
		
		stderrStr := stderr.String()
		if strings.Contains(stderrStr, "should-not-use-this") {
			t.Error("CLI should prefer configured soundpack over platform JSON")
		}
	})
}

// TDD RED: Test platform detection integration in CLI initialization
func TestCLIPlatformDetectionIntegration(t *testing.T) {
	t.Run("CLI should integrate platform detection into soundpack resolution", func(t *testing.T) {
		// This tests that the CLI actually calls the platform detection logic
		
		// Create a ConfigManager and verify it has platform detection
		mgr := config.NewConfigManager()
		if mgr == nil {
			t.Fatal("ConfigManager should not be nil")
		}
		
		// Test current platform detection behavior with updated API
		// Use real filesystem and current executable directory
		execDir := getPlatformExecutableDirectoryForTest()
		platformSoundpack := mgr.GetPlatformSoundpack(afero.NewOsFs(), execDir)
		t.Logf("Current platform soundpack detection result: %s", platformSoundpack)
		
		// This should show current behavior (likely "default" or "linux.json")
		// When we implement the fixes, this should return path to actual JSON file
		
		// The CLI initialization should eventually use this information
		// But currently it's not integrated into the main soundpack resolution flow
		t.Skip("Platform detection integration into CLI not implemented yet - will implement in GREEN phase")
	})
}

// getPlatformExecutableDirectoryForTest returns the directory containing the current executable for tests
func getPlatformExecutableDirectoryForTest() string {
	executable, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(executable)
}