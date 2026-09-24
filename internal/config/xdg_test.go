package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestXDGSoundpackPaths(t *testing.T) {
	testCases := []struct {
		name         string
		soundpackID  string
		expectedDirs []string // should check these directories exist in result
	}{
		{
			name:        "default soundpack",
			soundpackID: "default",
			expectedDirs: []string{
				"claudio/soundpacks/default", // user data dir
				"claudio/soundpacks/default", // system data dirs
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
				"claudio/soundpacks", // fallback to base soundpacks dir
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			paths := SoundpackPaths(tc.soundpackID)

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
	testCases := []struct {
		name     string
		purpose  string
		expected []string // expected path components at the end
	}{
		{
			name:     "soundpack cache",
			purpose:  "soundpacks",
			expected: []string{"claudio", "soundpacks"},
		},
		{
			name:     "web cache",
			purpose:  "web",
			expected: []string{"claudio", "web"},
		},
		{
			name:     "empty purpose",
			purpose:  "",
			expected: []string{"claudio"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			path := CachePath(tc.purpose)

			if path == "" {
				t.Error("GetCachePath returned empty string")
				return
			}

			if !filepath.IsAbs(path) {
				t.Errorf("Cache path %s is not absolute", path)
			}

			// Check path ends with expected components (OS-agnostic)
			expectedSuffix := filepath.Join(tc.expected...)
			if !strings.HasSuffix(path, expectedSuffix) {
				t.Errorf("Cache path %s does not end with expected components %v", path, tc.expected)
			}

			t.Logf("Cache path for %s: %s", tc.purpose, path)
		})
	}
}

func TestXDGConfigPaths(t *testing.T) {
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
			paths := ConfigPaths(tc.filename)

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

func TestXDGCrossPlatform(t *testing.T) {
	// These tests verify the package works across platforms
	t.Run("cache paths exist", func(t *testing.T) {
		cachePath := CachePath("test")
		if cachePath == "" {
			t.Error("Cache path is empty")
		}
		t.Logf("Cache path: %s", cachePath)
	})

	t.Run("config paths exist", func(t *testing.T) {
		configPaths := ConfigPaths("test.yaml")
		if len(configPaths) == 0 {
			t.Error("No config paths returned")
		}
		t.Logf("Config paths: %v", configPaths)
	})

	t.Run("soundpack paths exist", func(t *testing.T) {
		soundpackPaths := SoundpackPaths("test")
		if len(soundpackPaths) == 0 {
			t.Error("No soundpack paths returned")
		}
		t.Logf("Soundpack paths: %v", soundpackPaths)
	})
}
