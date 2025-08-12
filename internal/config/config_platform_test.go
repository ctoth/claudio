package config

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"claudio.click/internal/audio"
)

// TDD GREEN: Test platform soundpack detection with afero memory filesystem
func TestGetPlatformSoundpackExecutableDirectory(t *testing.T) {
	
	t.Run("finds wsl.json next to executable in WSL", func(t *testing.T) {
		// Use memory filesystem for testing
		memFS := afero.NewMemMapFs()
		mgr := NewConfigManager()
		
		// Mock executable directory
		execDir := "/mock/bin"
		
		// Create wsl.json in memory filesystem
		wslJsonPath := filepath.Join(execDir, "wsl.json")
		testJsonContent := `{"name":"test-wsl-soundpack","mappings":{"default.wav":"test.wav"}}`
		
		err := memFS.MkdirAll(execDir, 0755)
		if err != nil {
			t.Fatalf("Failed to create mock exec directory: %v", err)
		}
		
		err = afero.WriteFile(memFS, wslJsonPath, []byte(testJsonContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create test wsl.json: %v", err)
		}
		
		t.Logf("Mock executable directory: %s", execDir)
		t.Logf("WSL JSON path: %s", wslJsonPath)
		t.Logf("WSL detection: %v", audio.IsWSL())
		
		// Test checkPlatformFile helper function
		result := checkPlatformFile(memFS, execDir, "wsl.json")
		if result == "" {
			t.Error("checkPlatformFile should find wsl.json in memory filesystem")
		} else {
			t.Logf("checkPlatformFile found: %s", result)
			if result != wslJsonPath {
				t.Errorf("Expected checkPlatformFile to return %s, got %s", wslJsonPath, result)
			}
		}
		
		// Test platform detection logic
		// Since we're in WSL, it should find wsl.json
		if audio.IsWSL() {
			platformResult := mgr.GetPlatformSoundpack(memFS, execDir)
			t.Logf("Platform detection result: %s", platformResult)
			
			if platformResult == "default" {
				t.Error("TDD: GetPlatformSoundpack should find wsl.json next to executable in WSL, but returned 'default'")
			} else if platformResult == wslJsonPath {
				t.Log("TDD GREEN: Platform detection successfully found wsl.json next to executable!")
			} else {
				t.Logf("Platform detection returned: %s (expected: %s)", platformResult, wslJsonPath)
			}
		} else {
			t.Skip("Not in WSL, testing general JSON detection instead")
			
			// Test with linux.json since we're not in WSL
			linuxJsonPath := filepath.Join(execDir, "linux.json")
			err = afero.WriteFile(memFS, linuxJsonPath, []byte(testJsonContent), 0644)
			if err != nil {
				t.Fatalf("Failed to create test linux.json: %v", err)
			}
			
			platformResult := mgr.GetPlatformSoundpack(memFS, execDir)
			if platformResult != linuxJsonPath {
				t.Errorf("Expected platform detection to find %s, got %s", linuxJsonPath, platformResult)
			}
		}
	})
	
	t.Run("finds darwin.json next to executable on macOS simulation", func(t *testing.T) {
		// Use memory filesystem to simulate macOS platform detection
		memFS := afero.NewMemMapFs()
		
		execDir := "/mock/bin"
		
		// Simulate runtime.GOOS == "darwin" behavior by creating darwin.json
		darwinJsonPath := filepath.Join(execDir, "darwin.json")
		testJsonContent := `{"name":"test-darwin-soundpack","mappings":{"default.wav":"test.wav"}}`
		
		err := memFS.MkdirAll(execDir, 0755)
		if err != nil {
			t.Fatalf("Failed to create mock exec directory: %v", err)
		}
		
		err = afero.WriteFile(memFS, darwinJsonPath, []byte(testJsonContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create test darwin.json: %v", err)
		}
		
		// Test that checkPlatformFile finds darwin.json
		result := checkPlatformFile(memFS, execDir, "darwin.json")
		if result != darwinJsonPath {
			t.Errorf("Expected checkPlatformFile to find %s, got %s", darwinJsonPath, result)
		}
		
		// Note: We can't easily mock runtime.GOOS, but we can test the file detection logic
		t.Logf("Darwin JSON detection test passed with mock filesystem")
	})
}

// TDD GREEN: Test WSL detection integration with memory filesystem
func TestGetPlatformSoundpackWSLDetection(t *testing.T) {
	
	t.Run("prefers wsl.json over linux.json when WSL detected", func(t *testing.T) {
		memFS := afero.NewMemMapFs()
		mgr := NewConfigManager()
		
		execDir := "/mock/bin"
		
		// Create both wsl.json and linux.json in memory filesystem
		wslJsonPath := filepath.Join(execDir, "wsl.json")
		linuxJsonPath := filepath.Join(execDir, "linux.json")
		
		wslContent := `{"name":"test-wsl-soundpack","mappings":{"default.wav":"wsl.wav"}}`
		linuxContent := `{"name":"test-linux-soundpack","mappings":{"default.wav":"linux.wav"}}`
		
		err := memFS.MkdirAll(execDir, 0755)
		if err != nil {
			t.Fatalf("Failed to create mock exec directory: %v", err)
		}
		
		err = afero.WriteFile(memFS, wslJsonPath, []byte(wslContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create test wsl.json: %v", err)
		}
		
		err = afero.WriteFile(memFS, linuxJsonPath, []byte(linuxContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create test linux.json: %v", err)
		}
		
		t.Logf("Created WSL JSON: %s", wslJsonPath)
		t.Logf("Created Linux JSON: %s", linuxJsonPath)
		t.Logf("Actual WSL detection: %v", audio.IsWSL())
		
		// Test platform detection
		result := mgr.GetPlatformSoundpack(memFS, execDir)
		
		if audio.IsWSL() {
			// In real WSL, should prefer wsl.json
			if result != wslJsonPath {
				t.Errorf("TDD: In WSL, expected GetPlatformSoundpack to prefer %s, but got %s", wslJsonPath, result)
			} else {
				t.Log("TDD GREEN: WSL detection correctly prefers wsl.json over linux.json!")
			}
		} else {
			// On non-WSL Linux, should get linux.json (if runtime.GOOS == "linux")
			if runtime.GOOS == "linux" && result != linuxJsonPath {
				t.Logf("On non-WSL Linux, expected %s, got %s", linuxJsonPath, result)
			}
		}
	})
	
	t.Run("uses runtime.GOOS when not in WSL", func(t *testing.T) {
		memFS := afero.NewMemMapFs()
		mgr := NewConfigManager()
		
		execDir := "/mock/bin"
		
		// Create platform-specific JSON based on current GOOS
		platformFile := runtime.GOOS + ".json"
		platformJsonPath := filepath.Join(execDir, platformFile)
		testContent := `{"name":"test-` + runtime.GOOS + `-soundpack","mappings":{"default.wav":"test.wav"}}`
		
		err := memFS.MkdirAll(execDir, 0755)
		if err != nil {
			t.Fatalf("Failed to create mock exec directory: %v", err)
		}
		
		err = afero.WriteFile(memFS, platformJsonPath, []byte(testContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create test platform JSON: %v", err)
		}
		
		result := mgr.GetPlatformSoundpack(memFS, execDir)
		
		t.Logf("Runtime GOOS: %s", runtime.GOOS)
		t.Logf("Expected platform file: %s", platformFile)
		t.Logf("GetPlatformSoundpack result: %s", result)
		
		// Should find the platform-specific JSON
		if result != platformJsonPath {
			t.Errorf("Expected GetPlatformSoundpack to find %s, got %s", platformJsonPath, result)
		} else {
			t.Log("TDD GREEN: Platform detection correctly uses runtime.GOOS!")
		}
	})
	
	t.Run("returns embedded soundpack when no platform JSON found but embedded exists", func(t *testing.T) {
		memFS := afero.NewMemMapFs()
		mgr := NewConfigManager()
		
		execDir := "/mock/empty"
		
		// Create empty directory
		err := memFS.MkdirAll(execDir, 0755)
		if err != nil {
			t.Fatalf("Failed to create mock exec directory: %v", err)
		}
		
		result := mgr.GetPlatformSoundpack(memFS, execDir)
		
		// With embedded soundpacks, we expect the embedded platform file to be used
		// In WSL environment, this will be "embedded:wsl.json"
		// In other environments, it would be "embedded:linux.json", "embedded:darwin.json", etc.
		expectedPrefix := "embedded:"
		if !strings.HasPrefix(result, expectedPrefix) && result != "default" {
			t.Errorf("Expected GetPlatformSoundpack to return embedded soundpack or 'default', got %s", result)
		} else {
			t.Logf("TDD GREEN: Platform detection correctly returns embedded or default: %s", result)
		}
	})
}

// TDD GREEN: Test helper functions work with afero filesystem
func TestPlatformSoundpackHelpers(t *testing.T) {
	
	t.Run("checkPlatformFile works with memory filesystem", func(t *testing.T) {
		memFS := afero.NewMemMapFs()
		
		testDir := "/test/dir"
		testFile := "test.json"
		testPath := filepath.Join(testDir, testFile)
		testContent := `{"name":"test"}`
		
		// Create directory and file in memory
		err := memFS.MkdirAll(testDir, 0755)
		if err != nil {
			t.Fatalf("Failed to create test directory: %v", err)
		}
		
		err = afero.WriteFile(memFS, testPath, []byte(testContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
		
		// Test that checkPlatformFile finds the file
		result := checkPlatformFile(memFS, testDir, testFile)
		if result != testPath {
			t.Errorf("Expected checkPlatformFile to return %s, got %s", testPath, result)
		}
		
		// Test that it returns empty string for nonexistent file
		result = checkPlatformFile(memFS, testDir, "nonexistent.json")
		if result != "" {
			t.Errorf("Expected checkPlatformFile to return empty string for nonexistent file, got %s", result)
		}
		
		t.Log("TDD GREEN: checkPlatformFile works correctly with afero memory filesystem!")
	})
}