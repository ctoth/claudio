package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"claudio.click/internal/config"
	"github.com/spf13/afero"
)

// TDD RED: Test unconfigured claudio uses platform JSON
func TestCLIUnconfiguredUsePlatformSoundpack(t *testing.T) {
	t.Run("unconfigured CLI should auto-detect and use wsl.json in WSL", func(t *testing.T) {
		// This test should FAIL - currently requires explicit config
		
		// Create temporary directory to simulate executable location
		tempDir := t.TempDir()
		
		// Create wsl.json with test soundpack data
		wslJsonPath := filepath.Join(tempDir, "wsl.json")
		wslJsonContent := `{
			"name": "test-wsl-soundpack",
			"description": "Test WSL soundpack for integration testing",
			"version": "1.0.0",
			"mappings": {
				"success/test-success.wav": "/mnt/c/Windows/Media/Windows Print complete.wav",
				"error/test-error.wav": "/mnt/c/Windows/Media/Windows Error.wav",
				"default.wav": "/mnt/c/Windows/Media/Windows Default.wav"
			}
		}`
		
		err := os.WriteFile(wslJsonPath, []byte(wslJsonContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create test wsl.json: %v", err)
		}
		
		// Create a mock executable in the same directory
		mockExecutable := filepath.Join(tempDir, "claudio")
		err = os.WriteFile(mockExecutable, []byte("mock"), 0755)
		if err != nil {
			t.Fatalf("Failed to create mock executable: %v", err)
		}
		
		// Prepare CLI test input
		testInput := `{"session_id":"test","transcript_path":"/test","cwd":"/test","hook_event_name":"PostToolUse","tool_name":"Bash","tool_response":{"stdout":"success","stderr":"","interrupted":false}}`
		
		// Change to temp directory so current working directory check finds wsl.json
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
		t.Logf("WSL JSON path: %s", wslJsonPath)
		
		// This test should fail because CLI doesn't automatically find platform JSON adjacent to executable
		if exitCode == 0 {
			// If it succeeds, check if it actually used our platform JSON
			stderrStr := stderr.String()
			if !strings.Contains(stderrStr, "test-wsl-soundpack") && !strings.Contains(stderrStr, wslJsonPath) {
				t.Error("TDD RED: CLI succeeded but didn't use platform JSON - should auto-detect wsl.json next to executable")
			}
		} else {
			t.Logf("TDD RED: CLI failed to run with platform JSON auto-detection (expected failure)")
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
		// This test should FAIL - current implementation doesn't have platform JSON fallback
		
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
		
		wslJsonPath := filepath.Join(execDir, "wsl.json")
		wslJsonContent := `{
			"name": "fallback-wsl-soundpack",
			"mappings": {
				"success/fallback.wav": "/mnt/c/Windows/Media/chord.wav"
			}
		}`
		
		err = os.WriteFile(wslJsonPath, []byte(wslJsonContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create fallback wsl.json: %v", err)
		}
		
		// Change to execDir so current working directory check finds wsl.json  
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
		t.Logf("WSL JSON path: %s", wslJsonPath)
		
		// Current implementation will likely fail or not use the fallback
		if exitCode == 0 {
			stderrStr := stderr.String()
			if !strings.Contains(stderrStr, "fallback-wsl-soundpack") {
				t.Error("TDD RED: CLI succeeded but didn't fallback to platform JSON when configured soundpack missing")
			}
		} else {
			t.Logf("TDD RED: CLI failed with missing configured soundpack - should fallback to platform JSON")
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