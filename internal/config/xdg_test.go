package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestXDGDirectories(t *testing.T) {
	xdg := NewXDGDirs()

	if xdg == nil {
		t.Fatal("NewXDGDirs returned nil")
	}
}

func TestXDGSoundpackPaths(t *testing.T) {
	xdg := NewXDGDirs()

	testCases := []struct {
		name         string
		soundpackID  string
		expectedDirs []string // should check these directories exist in result
	}{
		{
			name:        "default soundpack",
			soundpackID: "default",
			expectedDirs: []string{
				"claudio/soundpacks/default",     // user data dir
				"claudio/soundpacks/default",     // system data dirs
			},
		},
		{
			name:        "custom soundpack",
			soundpackID: "mechanical-keyboard",
			expectedDirs: []string{
				"claudio/soundpacks/mechanical-keyboard",
				"claudio/soundpacks/mechanical-keyboard",
			},
		},
		{
			name:        "empty soundpack id",
			soundpackID: "",
			expectedDirs: []string{
				"claudio/soundpacks",  // fallback to base soundpacks dir
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			paths := xdg.GetSoundpackPaths(tc.soundpackID)

			if len(paths) == 0 {
				t.Error("GetSoundpackPaths returned empty slice")
				return
			}

			// Verify all paths are absolute
			for i, path := range paths {
				if !filepath.IsAbs(path) {
					t.Errorf("Path[%d] = %s is not absolute", i, path)
				}
			}

			// Check that expected directory patterns appear in results
			for _, expectedDir := range tc.expectedDirs {
				found := false
				for _, path := range paths {
					if filepath.Base(path) == filepath.Base(expectedDir) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected directory pattern %s not found in paths: %v", expectedDir, paths)
				}
			}

			// Log the actual paths for debugging
			t.Logf("Soundpack paths for %s: %v", tc.soundpackID, paths)
		})
	}
}

func TestXDGCachePaths(t *testing.T) {
	xdg := NewXDGDirs()

	testCases := []struct {
		name         string
		purpose      string
		expectedPath string // should contain this pattern
	}{
		{
			name:         "soundpack cache",
			purpose:      "soundpacks",
			expectedPath: "claudio/soundpacks",
		},
		{
			name:         "web cache",
			purpose:      "web",
			expectedPath: "claudio/web", 
		},
		{
			name:         "empty purpose",
			purpose:      "",
			expectedPath: "claudio",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			path := xdg.GetCachePath(tc.purpose)

			if path == "" {
				t.Error("GetCachePath returned empty string")
				return
			}

			if !filepath.IsAbs(path) {
				t.Errorf("Cache path %s is not absolute", path)
			}

			if !strings.HasSuffix(path, tc.expectedPath) {
				t.Errorf("Cache path %s does not end with expected pattern %s", path, tc.expectedPath)
			}

			t.Logf("Cache path for %s: %s", tc.purpose, path)
		})
	}
}

func TestXDGConfigPaths(t *testing.T) {
	xdg := NewXDGDirs()

	testCases := []struct {
		name         string
		filename     string
		expectedFile string
	}{
		{
			name:         "main config file",
			filename:     "config.yaml",
			expectedFile: "config.yaml",
		},
		{
			name:         "soundpack config",
			filename:     "soundpacks.yaml", 
			expectedFile: "soundpacks.yaml",
		},
		{
			name:         "empty filename",
			filename:     "",
			expectedFile: "", // should handle gracefully
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			paths := xdg.GetConfigPaths(tc.filename)

			if len(paths) == 0 {
				t.Error("GetConfigPaths returned empty slice")
				return
			}

			// Verify all paths are absolute
			for i, path := range paths {
				if !filepath.IsAbs(path) {
					t.Errorf("Path[%d] = %s is not absolute", i, path)
				}

				if tc.filename != "" && !strings.HasSuffix(path, tc.expectedFile) {
					t.Errorf("Path[%d] = %s does not end with expected file %s", i, path, tc.expectedFile)
				}
			}

			// All paths should contain "claudio" directory
			for i, path := range paths {
				if !strings.HasSuffix(filepath.Dir(path), "claudio") && !strings.Contains(path, "claudio") {
					t.Errorf("Path[%d] = %s does not contain 'claudio' directory", i, path)
				}
			}

			t.Logf("Config paths for %s: %v", tc.filename, paths)
		})
	}
}

func TestXDGCreateCacheDir(t *testing.T) {
	xdg := NewXDGDirs()

	// Use a test-specific subdirectory to avoid conflicts
	testCacheDir := xdg.GetCachePath("test-create")

	// Clean up before and after test
	defer os.RemoveAll(testCacheDir)
	os.RemoveAll(testCacheDir)

	// Verify directory doesn't exist initially
	if _, err := os.Stat(testCacheDir); !os.IsNotExist(err) {
		t.Fatalf("Test cache directory %s already exists", testCacheDir)
	}

	// Create the directory
	err := xdg.CreateCacheDir("test-create")
	if err != nil {
		t.Fatalf("CreateCacheDir failed: %v", err)
	}

	// Verify directory was created
	info, err := os.Stat(testCacheDir)
	if err != nil {
		t.Fatalf("Cache directory was not created: %v", err)
	}

	if !info.IsDir() {
		t.Error("Created cache path is not a directory")
	}

	// Test creating again (should not error)
	err = xdg.CreateCacheDir("test-create")
	if err != nil {
		t.Errorf("CreateCacheDir failed on existing directory: %v", err)
	}
}

func TestXDGFindSoundFile(t *testing.T) {
	xdg := NewXDGDirs()

	testCases := []struct {
		name           string
		soundpackID    string
		relativePath   string
		createFile     bool
		shouldFind     bool
	}{
		{
			name:         "existing file",
			soundpackID:  "test-pack",
			relativePath: "success/test-sound.wav",
			createFile:   true,
			shouldFind:   true,
		},
		{
			name:         "non-existing file",
			soundpackID:  "test-pack",
			relativePath: "error/missing-sound.wav", 
			createFile:   false,
			shouldFind:   false,
		},
		{
			name:         "empty soundpack",
			soundpackID:  "",
			relativePath: "default.wav",
			createFile:   false,
			shouldFind:   false,
		},
		{
			name:         "empty path",
			soundpackID:  "test-pack",
			relativePath: "",
			createFile:   false,
			shouldFind:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var testFilePath string

			if tc.createFile && tc.soundpackID != "" && tc.relativePath != "" {
				// Create a test file in the first soundpack path
				soundpackPaths := xdg.GetSoundpackPaths(tc.soundpackID)
				if len(soundpackPaths) > 0 {
					testFilePath = filepath.Join(soundpackPaths[0], tc.relativePath)
					
					// Create parent directories
					err := os.MkdirAll(filepath.Dir(testFilePath), 0755)
					if err != nil {
						t.Fatalf("Failed to create test directories: %v", err)
					}

					// Create test file
					file, err := os.Create(testFilePath)
					if err != nil {
						t.Fatalf("Failed to create test file: %v", err)
					}
					file.Close()

					// Clean up after test
					defer os.RemoveAll(soundpackPaths[0])
				}
			}

			// Test finding the file
			foundPath := xdg.FindSoundFile(tc.soundpackID, tc.relativePath)

			if tc.shouldFind {
				if foundPath == "" {
					t.Error("Expected to find sound file but got empty path")
				} else if !filepath.IsAbs(foundPath) {
					t.Errorf("Found path %s is not absolute", foundPath)
				} else {
					// Verify file actually exists
					if _, err := os.Stat(foundPath); err != nil {
						t.Errorf("Found path %s does not exist: %v", foundPath, err)
					}
				}
			} else {
				if foundPath != "" {
					t.Errorf("Expected not to find file but got: %s", foundPath)
				}
			}

			t.Logf("FindSoundFile(%s, %s) = %s", tc.soundpackID, tc.relativePath, foundPath)
		})
	}
}

func TestXDGCrossPlatform(t *testing.T) {
	xdg := NewXDGDirs()

	// These tests verify the package works across platforms
	t.Run("cache paths exist", func(t *testing.T) {
		cachePath := xdg.GetCachePath("test")
		if cachePath == "" {
			t.Error("Cache path is empty")
		}
		t.Logf("Cache path: %s", cachePath)
	})

	t.Run("config paths exist", func(t *testing.T) {
		configPaths := xdg.GetConfigPaths("test.yaml")
		if len(configPaths) == 0 {
			t.Error("No config paths returned")
		}
		t.Logf("Config paths: %v", configPaths)
	})

	t.Run("soundpack paths exist", func(t *testing.T) {
		soundpackPaths := xdg.GetSoundpackPaths("test")
		if len(soundpackPaths) == 0 {
			t.Error("No soundpack paths returned")
		}
		t.Logf("Soundpack paths: %v", soundpackPaths)
	})
}

func TestXDGErrorHandling(t *testing.T) {
	xdg := NewXDGDirs()

	t.Run("invalid characters in paths", func(t *testing.T) {
		// Test with various invalid characters
		invalidPaths := []string{
			"../../../etc/passwd",
			"test\x00null",
			"test\n\r",
			"test with spaces",  // Should be OK
			"test-with-hyphens", // Should be OK
		}

		for _, invalidPath := range invalidPaths {
			result := xdg.FindSoundFile("test", invalidPath)
			// Should handle gracefully (either find nothing or sanitize)
			t.Logf("FindSoundFile with invalid path %q: %s", invalidPath, result)
		}
	})

	t.Run("very long paths", func(t *testing.T) {
		longName := ""
		for i := 0; i < 300; i++ {
			longName += "a"
		}

		result := xdg.FindSoundFile(longName, "test.wav")
		// Should handle gracefully
		t.Logf("FindSoundFile with long name: %s", result)
	})
}