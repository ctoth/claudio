package uninstall

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ctoth/claudio/internal/install"
)

func TestRunUninstallWorkflow(t *testing.T) {
	// TDD RED: Test complete uninstall workflow integration
	testCases := []struct {
		name                 string
		scope               string
		existingSettings     map[string]interface{}
		existingSettingsFile bool
		expectError          bool
		expectedHooksCount   int
		expectNoHooksSection bool
	}{
		{
			name:  "uninstall from settings with claudio hooks",
			scope: "user",
			existingSettings: map[string]interface{}{
				"hooks": map[string]interface{}{
					"PreToolUse":       "claudio",
					"PostToolUse":      "claudio", 
					"UserPromptSubmit": "claudio",
					"Other":            "keep-this",
				},
				"version": "1.0",
			},
			existingSettingsFile: true,
			expectError:          false,
			expectedHooksCount:   1, // Only "Other" should remain
			expectNoHooksSection: false,
		},
		{
			name:  "uninstall from settings without claudio - no changes",
			scope: "user",
			existingSettings: map[string]interface{}{
				"hooks": map[string]interface{}{
					"Other": "different-tool",
				},
				"version": "1.0",
			},
			existingSettingsFile: true,
			expectError:          false,
			expectedHooksCount:   1, // "Other" hook preserved
			expectNoHooksSection: false,
		},
		{
			name:  "uninstall all hooks - hooks section deleted",
			scope: "project",
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
			expectedHooksCount:   0,
			expectNoHooksSection: true,
		},
		{
			name:  "uninstall from complex array hooks",
			scope: "user",
			existingSettings: map[string]interface{}{
				"hooks": map[string]interface{}{
					"Notification": []interface{}{
						map[string]interface{}{
							"hooks": []interface{}{
								map[string]interface{}{
									"command": "claudio",
									"type":    "command",
								},
								map[string]interface{}{
									"command": "other-tool",
									"type":    "command",
								},
							},
						},
					},
					"Stop": []interface{}{
						map[string]interface{}{
							"hooks": []interface{}{
								map[string]interface{}{
									"command": "claudio",
									"type":    "command",
								},
							},
						},
					},
				},
			},
			existingSettingsFile: true,
			expectError:          false,
			expectedHooksCount:   1, // Only "Notification" should remain (with other-tool)
			expectNoHooksSection: false,
		},
		{
			name:  "uninstall from mixed simple and complex hooks",
			scope: "user",
			existingSettings: map[string]interface{}{
				"hooks": map[string]interface{}{
					"PreToolUse": "claudio",
					"SubagentStop": []interface{}{
						map[string]interface{}{
							"hooks": []interface{}{
								map[string]interface{}{
									"command": "claudio",
									"type":    "command",
								},
							},
						},
					},
					"Other": "preserve-this",
				},
			},
			existingSettingsFile: true,
			expectError:          false,
			expectedHooksCount:   1, // Only "Other" should remain
			expectNoHooksSection: false,
		},
		{
			name:  "uninstall from empty settings file",
			scope: "user",
			existingSettings: map[string]interface{}{
				"version": "1.0",
			},
			existingSettingsFile: true,
			expectError:          false,
			expectedHooksCount:   0,
			expectNoHooksSection: true,
		},
		{
			name:                 "uninstall from non-existent settings file",
			scope:               "user",
			existingSettings:     nil,
			existingSettingsFile: false,
			expectError:          false,
			expectedHooksCount:   0,
			expectNoHooksSection: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create temporary directory for settings
			tempDir := t.TempDir()
			settingsDir := filepath.Join(tempDir, ".claude")
			err := os.MkdirAll(settingsDir, 0755)
			if err != nil {
				t.Fatalf("Failed to create settings directory: %v", err)
			}

			settingsFile := filepath.Join(settingsDir, "settings.json")

			// Create existing settings file if specified
			if tc.existingSettingsFile && tc.existingSettings != nil {
				settingsJSON, err := json.MarshalIndent(tc.existingSettings, "", "  ")
				if err != nil {
					t.Fatalf("Failed to marshal existing settings: %v", err)
				}
				
				err = os.WriteFile(settingsFile, settingsJSON, 0644)
				if err != nil {
					t.Fatalf("Failed to write existing settings file: %v", err)
				}
			}

			// Test the complete uninstall workflow
			err = runUninstallWorkflow(tc.scope, settingsFile)
			
			if tc.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tc.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			
			if tc.expectError {
				return // Skip verification if error was expected
			}

			// Verify uninstall results
			// 1. Settings file should exist after uninstall
			if _, err := os.Stat(settingsFile); os.IsNotExist(err) {
				if tc.existingSettingsFile {
					t.Error("Settings file should exist after uninstall")
				}
				return // File doesn't exist, which is fine for non-existent cases
			}

			// 2. Settings file should be valid JSON
			settings, err := install.ReadSettingsFile(settingsFile)
			if err != nil {
				t.Errorf("Failed to read settings after uninstall: %v", err)
			}

			// 3. Check hooks section
			if tc.expectNoHooksSection {
				if _, exists := (*settings)["hooks"]; exists {
					t.Error("Hooks section should be deleted when all hooks removed")
				}
			} else {
				if hooks, exists := (*settings)["hooks"]; exists {
					hooksMap, ok := hooks.(map[string]interface{})
					if !ok {
						t.Errorf("Hooks should be a map, got: %T", hooks)
					} else {
						if len(hooksMap) != tc.expectedHooksCount {
							t.Errorf("Expected %d hooks, got %d: %v", 
								tc.expectedHooksCount, len(hooksMap), getMapKeys(hooksMap))
						}
						
						// 4. Verify no claudio hooks remain
						claudiaHooks := detectClaudioHooks(settings)
						if len(claudiaHooks) > 0 {
							t.Errorf("Claudio hooks still present after uninstall: %v", claudiaHooks)
						}
					}
				} else if tc.expectedHooksCount > 0 {
					t.Errorf("Expected %d hooks but hooks section missing", tc.expectedHooksCount)
				}
			}

			// 5. Existing non-claudio settings should be preserved
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

			t.Logf("Uninstall workflow test passed for %s", tc.name)
		})
	}
}

func TestUninstallWorkflowErrorHandling(t *testing.T) {
	// TDD RED: Test error handling in uninstall workflow
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

			err := runUninstallWorkflow(tc.scope, settingsPath)
			
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

// Helper functions reused from hook_removal_test.go
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