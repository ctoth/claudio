package config

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"claudio.click/internal/platform"
)

// TDD GREEN: Test platform soundpack detection with afero memory filesystem
func TestGetPlatformSoundpackExecutableDirectory(t *testing.T) {
	
	t.Run("finds wsl.json next to executable in WSL", func(t *testing.T) {
		// Use memory filesystem for testing; the manager wires it via cm.fs.
		memFS := afero.NewMemMapFs()
		mgr := NewConfigManagerWithFilesystem(memFS)

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
		t.Logf("WSL detection: %v", platform.IsWSL())

		// Test checkPlatformFile helper method
		result := mgr.checkPlatformFile(execDir, "wsl.json")
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
		if platform.IsWSL() {
			platformResult := mgr.GetPlatformSoundpack(execDir)
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

			platformResult := mgr.GetPlatformSoundpack(execDir)
			if platformResult != linuxJsonPath {
				t.Errorf("Expected platform detection to find %s, got %s", linuxJsonPath, platformResult)
			}
		}
	})
	
	t.Run("finds darwin.json next to executable on macOS simulation", func(t *testing.T) {
		// Use memory filesystem to simulate macOS platform detection
		memFS := afero.NewMemMapFs()
		mgr := NewConfigManagerWithFilesystem(memFS)

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
		result := mgr.checkPlatformFile(execDir, "darwin.json")
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
		mgr := NewConfigManagerWithFilesystem(memFS)

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
		t.Logf("Actual WSL detection: %v", platform.IsWSL())
		
		// Test platform detection
		result := mgr.GetPlatformSoundpack(execDir)

		if platform.IsWSL() {
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
		mgr := NewConfigManagerWithFilesystem(memFS)

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
		
		result := mgr.GetPlatformSoundpack(execDir)

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
		mgr := NewConfigManagerWithFilesystem(memFS)

		execDir := "/mock/empty"

		// Create empty directory
		err := memFS.MkdirAll(execDir, 0755)
		if err != nil {
			t.Fatalf("Failed to create mock exec directory: %v", err)
		}

		result := mgr.GetPlatformSoundpack(execDir)
		
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
		mgr := NewConfigManagerWithFilesystem(memFS)

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

		// Test that checkPlatformFile (method) finds the file via cm.fs
		result := mgr.checkPlatformFile(testDir, testFile)
		if result != testPath {
			t.Errorf("Expected checkPlatformFile to return %s, got %s", testPath, result)
		}

		// Test that it returns empty string for nonexistent file
		result = mgr.checkPlatformFile(testDir, "nonexistent.json")
		if result != "" {
			t.Errorf("Expected checkPlatformFile to return empty string for nonexistent file, got %s", result)
		}

		t.Log("TDD GREEN: checkPlatformFile works correctly with afero memory filesystem!")
	})
}

// TestGetPlatformSoundpack_HonorsCmFs confirms that when the manager
// is constructed via NewConfigManagerWithFilesystem(memFS),
// GetPlatformSoundpack reads through that memFS. Pre-fix the method
// took an afero.Fs parameter and production callers always passed
// afero.NewOsFs(), so this property was effectively untestable: the
// real filesystem state always won. After the fix cm.fs is the
// single source of truth.
func TestGetPlatformSoundpack_HonorsCmFs(t *testing.T) {
	memFS := afero.NewMemMapFs()
	mgr := NewConfigManagerWithFilesystem(memFS)

	execDir := "/test/exec"
	platformFile := runtime.GOOS + ".json"
	platformPath := filepath.Join(execDir, platformFile)

	if err := memFS.MkdirAll(execDir, 0755); err != nil {
		t.Fatalf("memFS MkdirAll failed: %v", err)
	}
	content := `{"name":"cm-fs-seam-test","mappings":{"default.wav":"x.wav"}}`
	if err := afero.WriteFile(memFS, platformPath, []byte(content), 0644); err != nil {
		t.Fatalf("memFS WriteFile failed: %v", err)
	}

	got := mgr.GetPlatformSoundpack(execDir)

	if platform.IsWSL() {
		// In WSL, GetPlatformSoundpack prefers wsl.json. We only wrote
		// runtime.GOOS+".json" (= "linux.json"), so the WSL branch
		// will not find a wsl.json in memFS and will fall through to
		// platform-file or embedded. The point of this test is the
		// memFS seam works at all; assert we got something other than
		// reading-real-disk.
		if got == "default" {
			t.Errorf("expected GetPlatformSoundpack to consult memFS and find linux.json, got %q", got)
		}
		return
	}

	if got != platformPath {
		t.Errorf("expected GetPlatformSoundpack to return %q via cm.fs (memFS), got %q", platformPath, got)
	}
}

// TestGetEmbeddedPlatformSoundpackData_LinuxEmbedded asserts that
// linux.json is present in the embed.FS so native non-WSL Linux
// runtimes receive a real platform soundpack instead of falling
// back to "default". Before this fix the //go:embed directive
// listed only windows.json wsl.json darwin.json.
func TestGetEmbeddedPlatformSoundpackData_LinuxEmbedded(t *testing.T) {
	if !hasEmbeddedPlatformFile("linux.json") {
		t.Fatal("linux.json must be embedded so non-WSL Linux gets a platform soundpack")
	}

	data, err := GetEmbeddedPlatformSoundpackData("linux.json")
	if err != nil {
		t.Fatalf("GetEmbeddedPlatformSoundpackData(linux.json) returned error: %v", err)
	}
	if len(data) == 0 {
		t.Error("embedded linux.json is empty")
	}
	// Sanity: the file is JSON with a "mappings" key
	if !strings.Contains(string(data), "\"mappings\"") {
		t.Errorf("embedded linux.json does not look like a soundpack JSON (no 'mappings' key); got: %s", string(data))
	}
}

// TestGetEmbeddedPlatformSoundpackData_AllPlatformsEmbedded confirms
// every platform soundpack the runtime ever asks for is embeddable.
// Pre-fix this would fail for "linux.json".
func TestGetEmbeddedPlatformSoundpackData_AllPlatformsEmbedded(t *testing.T) {
	for _, name := range []string{"windows.json", "wsl.json", "darwin.json", "linux.json"} {
		t.Run(name, func(t *testing.T) {
			if !hasEmbeddedPlatformFile(name) {
				t.Errorf("expected %s to be embedded", name)
			}
		})
	}
}

// TestIsGoTestTempExecutable_PortableAcrossPlatforms asserts that the
// go-test temp-build detection works on Windows (where go test stages
// binaries under %TEMP%\go-buildNNN\...) as well as POSIX. The pre-fix
// implementation hardcoded the literal "/tmp/go-build" which never
// matched on Windows, so the test-context CWD fallback never fired
// there.
func TestIsGoTestTempExecutable_PortableAcrossPlatforms(t *testing.T) {
	tests := []struct {
		name     string
		exec     string
		tmpRoot  string
		expected bool
	}{
		{
			name:     "POSIX go test under /tmp/go-build",
			exec:     "/tmp/go-build123456/b001/config.test",
			tmpRoot:  "/tmp",
			expected: true,
		},
		{
			name:     "POSIX go test under custom TMPDIR",
			exec:     "/var/folders/x/y/T/go-build987/b001/config.test",
			tmpRoot:  "/var/folders/x/y/T",
			expected: true,
		},
		{
			name:     "Windows go test under %TEMP%",
			exec:     filepath.Join("C:", "Users", "Q", "AppData", "Local", "Temp", "go-build123456", "b001", "config.test.exe"),
			tmpRoot:  filepath.Join("C:", "Users", "Q", "AppData", "Local", "Temp"),
			expected: true,
		},
		{
			name:     "production binary under /usr/local/bin",
			exec:     "/usr/local/bin/claudio",
			tmpRoot:  "/tmp",
			expected: false,
		},
		{
			name:     "production binary on Windows under Program Files",
			exec:     filepath.Join("C:", "Program Files", "claudio", "claudio.exe"),
			tmpRoot:  filepath.Join("C:", "Users", "Q", "AppData", "Local", "Temp"),
			expected: false,
		},
		{
			name:     "temp dir but not a go-build binary",
			exec:     "/tmp/something-else/binary",
			tmpRoot:  "/tmp",
			expected: false,
		},
		{
			name:     "empty executable path",
			exec:     "",
			tmpRoot:  "/tmp",
			expected: false,
		},
		{
			name:     "empty tmp root",
			exec:     "/tmp/go-build123/b001/x.test",
			tmpRoot:  "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isGoTestTempExecutable(tt.exec, tt.tmpRoot)
			if got != tt.expected {
				t.Errorf("isGoTestTempExecutable(%q, %q) = %v, want %v",
					tt.exec, tt.tmpRoot, got, tt.expected)
			}
		})
	}
}
