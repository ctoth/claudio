package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ctoth/claudio/internal/install"
)

func TestInstallWorkflowUser(t *testing.T) {
	// TDD RED: Test complete user-scope installation workflow
	testCases := []struct {
		name                 string
		existingSettings     map[string]interface{}
		existingSettingsFile bool
		expectError          bool
		expectHooksCount     int
	}{
		{
			name:                 "fresh user installation - no existing settings",
			existingSettings:     nil,
			existingSettingsFile: false,
			expectError:          false,
			expectHooksCount:     3, // PreToolUse, PostToolUse, UserPromptSubmit
		},
		{
			name: "user installation with existing settings",
			existingSettings: map[string]interface{}{
				"version": "1.0",
				"plugins": []string{"plugin1", "plugin2"},
				"config": map[string]interface{}{
					"debug": true,
				},
			},
			existingSettingsFile: true,
			expectError:          false,
			expectHooksCount:     3,
		},
		{
			name: "user installation with existing hooks",
			existingSettings: map[string]interface{}{
				"hooks": map[string]interface{}{
					"PreCommit":  "git diff --check",
					"PostCommit": "git push",
				},
				"version": "1.0",
			},
			existingSettingsFile: true,
			expectError:          false,
			expectHooksCount:     5, // 2 existing + 3 Claudio
		},
		{
			name: "user installation idempotent - Claudio already installed",
			existingSettings: map[string]interface{}{
				"hooks": map[string]interface{}{
					"PreToolUse":       "claudio",
					"PostToolUse":      "claudio",
					"UserPromptSubmit": "claudio",
				},
				"version": "1.0",
			},
			existingSettingsFile: true,
			expectError:          false,
			expectHooksCount:     3, // Should remain the same
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create temporary directory for user settings
			tempDir := t.TempDir()
			userSettingsDir := filepath.Join(tempDir, ".claude")
			err := os.MkdirAll(userSettingsDir, 0755)
			if err != nil {
				t.Fatalf("Failed to create user settings directory: %v", err)
			}

			userSettingsFile := filepath.Join(userSettingsDir, "settings.json")

			// Create existing settings file if specified
			if tc.existingSettingsFile && tc.existingSettings != nil {
				settingsJSON, err := json.MarshalIndent(tc.existingSettings, "", "  ")
				if err != nil {
					t.Fatalf("Failed to marshal existing settings: %v", err)
				}
				
				err = os.WriteFile(userSettingsFile, settingsJSON, 0644)
				if err != nil {
					t.Fatalf("Failed to write existing settings file: %v", err)
				}
			}

			// Test the complete installation workflow
			err = runInstallWorkflow("user", userSettingsFile)
			
			if tc.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tc.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			
			if tc.expectError {
				return // Skip verification if error was expected
			}

			// Verify installation results
			// 1. Settings file should exist
			if _, err := os.Stat(userSettingsFile); os.IsNotExist(err) {
				t.Error("Settings file should exist after installation")
			}

			// 2. Settings file should be valid JSON
			settings, err := install.ReadSettingsFile(userSettingsFile)
			if err != nil {
				t.Errorf("Failed to read installed settings: %v", err)
			}

			// 3. Hooks section should exist and have expected number of hooks
			if hooks, exists := (*settings)["hooks"]; exists {
				hooksMap, ok := hooks.(map[string]interface{})
				if !ok {
					t.Errorf("Hooks should be a map, got: %T", hooks)
				} else {
					if len(hooksMap) != tc.expectHooksCount {
						t.Errorf("Expected %d hooks, got %d: %v", 
							tc.expectHooksCount, len(hooksMap), getMapKeys(hooksMap))
					}
					
					// 4. Claudio hooks should be present
					expectedClaudiaHooks := []string{"PreToolUse", "PostToolUse", "UserPromptSubmit"}
					for _, hookName := range expectedClaudiaHooks {
						if val, exists := hooksMap[hookName]; !exists {
							t.Errorf("Claudio hook '%s' missing after installation", hookName)
						} else if val != "claudio" {
							t.Errorf("Claudio hook '%s' should be 'claudio', got: %v", hookName, val)
						}
					}
				}
			} else {
				t.Error("Hooks section should exist after installation")
			}

			// 5. Existing settings should be preserved
			if tc.existingSettings != nil {
				for key, expectedValue := range tc.existingSettings {
					if key == "hooks" {
						continue // Already tested above
					}
					
					if actualValue, exists := (*settings)[key]; !exists {
						t.Errorf("Existing setting '%s' was not preserved", key)
					} else {
						// Deep comparison using JSON
						expectedJSON, _ := json.Marshal(expectedValue)
						actualJSON, _ := json.Marshal(actualValue)
						if string(expectedJSON) != string(actualJSON) {
							t.Errorf("Existing setting '%s' was modified:\nExpected: %s\nActual:   %s", 
								key, string(expectedJSON), string(actualJSON))
						}
					}
				}
			}

			t.Logf("User installation workflow test passed for %s", tc.name)
		})
	}
}

func TestInstallWorkflowProject(t *testing.T) {
	// TDD RED: Test complete project-scope installation workflow
	testCases := []struct {
		name                 string
		existingSettings     map[string]interface{}
		existingSettingsFile bool
		expectError          bool
		expectHooksCount     int
	}{
		{
			name:                 "fresh project installation - no existing settings",
			existingSettings:     nil,
			existingSettingsFile: false,
			expectError:          false,
			expectHooksCount:     3,
		},
		{
			name: "project installation with team settings",
			existingSettings: map[string]interface{}{
				"hooks": map[string]interface{}{
					"PreCommit":  "npm run lint",
					"PostCommit": "npm run test",
					"PrePush":    "npm run build",
				},
				"team": map[string]interface{}{
					"style_guide": "airbnb",
					"auto_format": true,
				},
			},
			existingSettingsFile: true,
			expectError:          false,
			expectHooksCount:     6, // 3 existing + 3 Claudio
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create temporary directory for project settings
			tempDir := t.TempDir()
			projectSettingsDir := filepath.Join(tempDir, ".claude")
			err := os.MkdirAll(projectSettingsDir, 0755)
			if err != nil {
				t.Fatalf("Failed to create project settings directory: %v", err)
			}

			projectSettingsFile := filepath.Join(projectSettingsDir, "settings.json")

			// Create existing settings file if specified
			if tc.existingSettingsFile && tc.existingSettings != nil {
				settingsJSON, err := json.MarshalIndent(tc.existingSettings, "", "  ")
				if err != nil {
					t.Fatalf("Failed to marshal existing settings: %v", err)
				}
				
				err = os.WriteFile(projectSettingsFile, settingsJSON, 0644)
				if err != nil {
					t.Fatalf("Failed to write existing settings file: %v", err)
				}
			}

			// Test the complete installation workflow
			err = runInstallWorkflow("project", projectSettingsFile)
			
			if tc.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tc.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			
			if tc.expectError {
				return
			}

			// Verify installation results (similar to user tests)
			settings, err := install.ReadSettingsFile(projectSettingsFile)
			if err != nil {
				t.Errorf("Failed to read installed settings: %v", err)
			}

			if hooks, exists := (*settings)["hooks"]; exists {
				hooksMap, ok := hooks.(map[string]interface{})
				if !ok {
					t.Errorf("Hooks should be a map, got: %T", hooks)
				} else if len(hooksMap) != tc.expectHooksCount {
					t.Errorf("Expected %d hooks, got %d", tc.expectHooksCount, len(hooksMap))
				}
			} else {
				t.Error("Hooks section should exist after installation")
			}

			t.Logf("Project installation workflow test passed for %s", tc.name)
		})
	}
}

func TestInstallWorkflowErrorHandling(t *testing.T) {
	// TDD RED: Test error handling in installation workflow
	testCases := []struct {
		name        string
		scope       string
		setupFunc   func() (settingsPath string, cleanup func())
		expectError bool
		errorMsg    string
	}{
		{
			name:  "invalid scope",
			scope: "invalid",
			setupFunc: func() (string, func()) {
				tempDir := t.TempDir()
				return filepath.Join(tempDir, "settings.json"), func() {}
			},
			expectError: true,
			errorMsg:    "invalid scope",
		},
		{
			name:  "permission denied directory",
			scope: "user",
			setupFunc: func() (string, func()) {
				tempDir := t.TempDir()
				settingsDir := filepath.Join(tempDir, "restricted")
				os.MkdirAll(settingsDir, 0000) // No permissions
				
				return filepath.Join(settingsDir, "settings.json"), func() {
					os.Chmod(settingsDir, 0755) // Restore permissions for cleanup
				}
			},
			expectError: true,
			errorMsg:    "permission",
		},
		{
			name:  "corrupted existing settings file",
			scope: "user", 
			setupFunc: func() (string, func()) {
				tempDir := t.TempDir()
				settingsDir := filepath.Join(tempDir, ".claude")
				os.MkdirAll(settingsDir, 0755)
				
				settingsFile := filepath.Join(settingsDir, "settings.json")
				// Write invalid JSON
				os.WriteFile(settingsFile, []byte("{invalid json"), 0644)
				
				return settingsFile, func() {}
			},
			expectError: true,
			errorMsg:    "invalid JSON",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			settingsPath, cleanup := tc.setupFunc()
			defer cleanup()

			err := runInstallWorkflow(tc.scope, settingsPath)
			
			if tc.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tc.errorMsg != "" && !containsString(err.Error(), tc.errorMsg) {
					t.Errorf("Expected error containing '%s', got: %v", tc.errorMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}

			t.Logf("Error handling test passed for %s", tc.name)
		})
	}
}

func TestInstallWorkflowConcurrency(t *testing.T) {
	// TDD RED: Test that concurrent installations work safely
	tempDir := t.TempDir()
	settingsDir := filepath.Join(tempDir, ".claude")
	err := os.MkdirAll(settingsDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create settings directory: %v", err)
	}

	settingsFile := filepath.Join(settingsDir, "settings.json")
	
	// Create initial settings
	initialSettings := map[string]interface{}{
		"version": "1.0",
		"hooks": map[string]interface{}{
			"PreCommit": "git diff",
		},
	}
	initialJSON, _ := json.MarshalIndent(initialSettings, "", "  ")
	err = os.WriteFile(settingsFile, initialJSON, 0644)
	if err != nil {
		t.Fatalf("Failed to write initial settings: %v", err)
	}

	// Run multiple concurrent installations
	const numConcurrent = 5
	errors := make(chan error, numConcurrent)
	
	for i := 0; i < numConcurrent; i++ {
		go func() {
			err := runInstallWorkflow("user", settingsFile)
			errors <- err
		}()
	}

	// Collect results
	for i := 0; i < numConcurrent; i++ {
		if err := <-errors; err != nil {
			t.Errorf("Concurrent installation %d failed: %v", i, err)
		}
	}

	// Verify final state is consistent
	finalSettings, err := install.ReadSettingsFile(settingsFile)
	if err != nil {
		t.Fatalf("Failed to read final settings: %v", err)
	}

	// Should have original hook + Claudio hooks
	if hooks, exists := (*finalSettings)["hooks"]; exists {
		hooksMap, ok := hooks.(map[string]interface{})
		if !ok {
			t.Errorf("Hooks should be a map, got: %T", hooks)
		} else {
			expectedHooks := 4 // PreCommit + 3 Claudio hooks
			if len(hooksMap) != expectedHooks {
				t.Errorf("Expected %d hooks after concurrent installation, got %d", 
					expectedHooks, len(hooksMap))
			}
		}
	}

	t.Logf("Concurrent installation test passed")
}

// Helper functions
func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && 
		   (s == substr || findSubstringSimple(s, substr))
}

func findSubstringSimple(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Functions that will need to be implemented (currently undefined):
// - runInstallWorkflow(scope string, settingsPath string) error