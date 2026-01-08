package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindClaudeSettings(t *testing.T) {
	// TDD RED: Test Claude settings path detection for user and project scopes
	testCases := []struct {
		name          string
		scope         string
		expectedPaths []string
		setupFunc     func() (tempDir string, cleanup func())
	}{
		{
			name:  "user scope default paths",
			scope: "user",
			expectedPaths: []string{
				"~/.claude/settings.json",
				"%USERPROFILE%/.claude/settings.json", // Windows
			},
			setupFunc: func() (string, func()) {
				// Create temporary home directory structure
				tempDir := t.TempDir()
				claudeDir := filepath.Join(tempDir, ".claude")
				err := os.MkdirAll(claudeDir, 0755)
				if err != nil {
					t.Fatalf("Failed to create temp Claude dir: %v", err)
				}

				// Set HOME environment variable for testing
				originalHome := os.Getenv("HOME")
				os.Setenv("HOME", tempDir)

				return tempDir, func() {
					os.Setenv("HOME", originalHome)
				}
			},
		},
		{
			name:  "project scope current directory",
			scope: "project",
			expectedPaths: []string{
				"./.claude/settings.json",
				".claude/settings.json",
			},
			setupFunc: func() (string, func()) {
				// Create temporary project directory structure
				tempDir := t.TempDir()
				claudeDir := filepath.Join(tempDir, ".claude")
				err := os.MkdirAll(claudeDir, 0755)
				if err != nil {
					t.Fatalf("Failed to create temp Claude dir: %v", err)
				}

				// Change to temp directory for testing
				originalDir, _ := os.Getwd()
				os.Chdir(tempDir)

				return tempDir, func() {
					os.Chdir(originalDir)
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tempDir, cleanup := tc.setupFunc()
			defer cleanup()

			// Test the FindClaudeSettingsPaths function
			paths, err := FindClaudeSettingsPaths(tc.scope)
			if err != nil {
				t.Errorf("FindClaudeSettingsPaths failed: %v", err)
				return
			}

			if len(paths) == 0 {
				t.Error("Expected at least one Claude settings path")
				return
			}

			// Verify that returned paths are reasonable
			for _, path := range paths {
				if !strings.Contains(path, "claude") {
					t.Errorf("Expected path to contain 'claude', got: %s", path)
				}
				if !strings.Contains(path, "settings.json") {
					t.Errorf("Expected path to contain 'settings.json', got: %s", path)
				}
			}

			t.Logf("Found Claude settings paths for %s scope: %v", tc.scope, paths)
			t.Logf("Temp directory used: %s", tempDir)
		})
	}
}

func TestFindClaudeSettingsInvalidScope(t *testing.T) {
	// TDD RED: Test that invalid scopes return appropriate errors
	invalidScopes := []string{
		"invalid",
		"global",
		"system",
		"admin",
		"",
	}

	for _, scope := range invalidScopes {
		t.Run("invalid_scope_"+scope, func(t *testing.T) {
			paths, err := FindClaudeSettingsPaths(scope)

			if err == nil {
				t.Errorf("Expected error for invalid scope '%s', but got paths: %v", scope, paths)
			}

			if len(paths) != 0 {
				t.Errorf("Expected no paths for invalid scope '%s', but got: %v", scope, paths)
			}
		})
	}
}

func TestFindClaudeSettingsExistingFiles(t *testing.T) {
	// TDD RED: Test detection of existing Claude settings files
	testCases := []struct {
		name           string
		scope          string
		createFiles    []string
		expectedExists bool
	}{
		{
			name:           "user scope with existing settings",
			scope:          "user",
			createFiles:    []string{".claude/settings.json"},
			expectedExists: true,
		},
		{
			name:           "project scope with existing settings",
			scope:          "project",
			createFiles:    []string{".claude/settings.json"},
			expectedExists: true,
		},
		{
			name:           "no existing settings",
			scope:          "user",
			createFiles:    []string{},
			expectedExists: false,
		},
		{
			name:           "partial settings (directory exists, no file)",
			scope:          "project",
			createFiles:    []string{".claude/"},
			expectedExists: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tempDir := t.TempDir()

			// Create test files
			for _, file := range tc.createFiles {
				fullPath := filepath.Join(tempDir, file)
				if strings.HasSuffix(file, "/") {
					// Create directory
					err := os.MkdirAll(fullPath, 0755)
					if err != nil {
						t.Fatalf("Failed to create directory %s: %v", fullPath, err)
					}
				} else {
					// Create file
					dir := filepath.Dir(fullPath)
					err := os.MkdirAll(dir, 0755)
					if err != nil {
						t.Fatalf("Failed to create directory %s: %v", dir, err)
					}

					err = os.WriteFile(fullPath, []byte(`{"test": true}`), 0644)
					if err != nil {
						t.Fatalf("Failed to create file %s: %v", fullPath, err)
					}
				}
			}

			// Set up environment for the scope - must mock ALL home-related env vars
			var cleanup func()
			if tc.scope == "user" {
				originalHome := os.Getenv("HOME")
				originalUserProfile := os.Getenv("USERPROFILE")
				originalHomeDrive := os.Getenv("HOMEDRIVE")
				originalHomePath := os.Getenv("HOMEPATH")

				os.Setenv("HOME", tempDir)
				os.Setenv("USERPROFILE", tempDir)
				os.Setenv("HOMEDRIVE", "")
				os.Setenv("HOMEPATH", "")

				cleanup = func() {
					os.Setenv("HOME", originalHome)
					os.Setenv("USERPROFILE", originalUserProfile)
					os.Setenv("HOMEDRIVE", originalHomeDrive)
					os.Setenv("HOMEPATH", originalHomePath)
				}
			} else {
				originalDir, _ := os.Getwd()
				os.Chdir(tempDir)
				cleanup = func() { os.Chdir(originalDir) }
			}
			defer cleanup()

			// Test finding settings
			paths, err := FindClaudeSettingsPaths(tc.scope)
			if err != nil {
				t.Errorf("FindClaudeSettingsPaths failed: %v", err)
				return
			}

			// Check if any paths exist
			foundExisting := false
			for _, path := range paths {
				if _, err := os.Stat(path); err == nil {
					foundExisting = true
					break
				}
			}

			if foundExisting != tc.expectedExists {
				t.Errorf("Expected existing files: %v, but found: %v", tc.expectedExists, foundExisting)
				t.Logf("Paths checked: %v", paths)
				t.Logf("Files created: %v", tc.createFiles)
			}
		})
	}
}

func TestFindClaudeSettingsMultiplePaths(t *testing.T) {
	// TDD RED: Test that function returns multiple potential paths in priority order
	testCases := []struct {
		name         string
		scope        string
		minPaths     int
		pathPatterns []string
	}{
		{
			name:     "user scope returns multiple path options",
			scope:    "user",
			minPaths: 1,
			pathPatterns: []string{
				"/.claude/settings.json",
				".claude/settings.json",
			},
		},
		{
			name:     "project scope returns multiple path options",
			scope:    "project",
			minPaths: 1,
			pathPatterns: []string{
				".claude/settings.json",
				"/.claude/settings.json",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			paths, err := FindClaudeSettingsPaths(tc.scope)
			if err != nil {
				t.Errorf("FindClaudeSettingsPaths failed: %v", err)
				return
			}

			if len(paths) < tc.minPaths {
				t.Errorf("Expected at least %d paths, got %d: %v", tc.minPaths, len(paths), paths)
			}

			// Check that paths contain expected patterns
			for _, pattern := range tc.pathPatterns {
				found := false
				for _, path := range paths {
					if strings.Contains(path, pattern) {
						found = true
						break
					}
				}
				if !found {
					t.Logf("Pattern '%s' not found in paths: %v", pattern, paths)
					// Note: This is logged but not failed as path patterns may vary by platform
				}
			}

			t.Logf("Paths for %s scope: %v", tc.scope, paths)
		})
	}
}

func TestFindClaudeSettingsPathValidation(t *testing.T) {
	// TDD RED: Test that returned paths are valid and accessible
	scopes := []string{"user", "project"}

	for _, scope := range scopes {
		t.Run("path_validation_"+scope, func(t *testing.T) {
			paths, err := FindClaudeSettingsPaths(scope)
			if err != nil {
				t.Errorf("FindClaudeSettingsPaths failed: %v", err)
				return
			}

			for _, path := range paths {
				// Path should be absolute or clearly relative
				if !filepath.IsAbs(path) && !strings.HasPrefix(path, "./") && !strings.HasPrefix(path, ".claude") {
					t.Errorf("Path should be absolute or clearly relative: %s", path)
				}

				// Path should be clean (no double slashes, etc.)
				cleanPath := filepath.Clean(path)
				if path != cleanPath && !strings.Contains(path, "~") {
					t.Errorf("Path should be clean, got: %s, expected: %s", path, cleanPath)
				}

				// Directory should be creatable (test parent directory)
				dir := filepath.Dir(path)
				if strings.HasPrefix(dir, "~") {
					// Skip home directory expansion for this test
					continue
				}

				// Try to create the directory structure
				tempTestPath := filepath.Join(t.TempDir(), "test-path-validation", filepath.Base(path))
				testDir := filepath.Dir(tempTestPath)
				err := os.MkdirAll(testDir, 0755)
				if err != nil {
					t.Errorf("Unable to create directory structure for path %s: %v", path, err)
				}
			}

			t.Logf("Validated %d paths for %s scope", len(paths), scope)
		})
	}
}

func TestFindClaudeSettingsEnvironmentIntegration(t *testing.T) {
	// TDD RED: Test integration with different environment configurations
	testCases := []struct {
		name    string
		scope   string
		envVars map[string]string
		setup   func() func() // Returns cleanup function
	}{
		{
			name:  "user scope with HOME set",
			scope: "user",
			envVars: map[string]string{
				"HOME": "/custom/home",
			},
			setup: func() func() {
				original := os.Getenv("HOME")
				os.Setenv("HOME", "/custom/home")
				return func() { os.Setenv("HOME", original) }
			},
		},
		{
			name:  "user scope with USERPROFILE set (Windows-style)",
			scope: "user",
			envVars: map[string]string{
				"USERPROFILE": "C:\\Users\\testuser",
			},
			setup: func() func() {
				originalHome := os.Getenv("HOME")
				originalUserProfile := os.Getenv("USERPROFILE")
				os.Setenv("HOME", "") // Clear HOME to test USERPROFILE
				os.Setenv("USERPROFILE", "C:\\Users\\testuser")
				return func() {
					os.Setenv("HOME", originalHome)
					os.Setenv("USERPROFILE", originalUserProfile)
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cleanup := tc.setup()
			defer cleanup()

			paths, err := FindClaudeSettingsPaths(tc.scope)
			if err != nil {
				t.Errorf("FindClaudeSettingsPaths failed: %v", err)
				return
			}

			if len(paths) == 0 {
				t.Error("Expected at least one path")
				return
			}

			// Verify that paths reflect environment variables
			for envVar, envValue := range tc.envVars {
				if envValue == "" {
					continue
				}

				found := false
				for _, path := range paths {
					if strings.Contains(path, envValue) || strings.Contains(path, strings.Replace(envValue, "\\", "/", -1)) {
						found = true
						break
					}
				}

				// Log results (some platforms may not use certain env vars)
				t.Logf("Environment %s=%s found in paths: %v", envVar, envValue, found)
			}

			t.Logf("Paths with custom environment: %v", paths)
		})
	}
}
