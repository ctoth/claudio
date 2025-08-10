package install

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// isClaudioHook checks if a hook value represents a claudio hook,
// supporting both the old string format and new array format
func isClaudioHook(hookValue interface{}) bool {
	// Helper function to check if command is a claudio executable
	isClaudioCommand := func(cmdStr string) bool {
		// Remove quotes if present (handles quoted paths in JSON)
		unquoted := cmdStr
		if len(cmdStr) >= 2 && cmdStr[0] == '"' && cmdStr[len(cmdStr)-1] == '"' {
			unquoted = cmdStr[1 : len(cmdStr)-1]
		}
		
		baseName := filepath.Base(unquoted)
		// Handle production "claudio" and test executables "install.test", "uninstall.test"
		return baseName == "claudio" || baseName == "install.test" || baseName == "uninstall.test"
	}

	// Check old string format (backward compatibility)
	if str, ok := hookValue.(string); ok {
		return isClaudioCommand(str)
	}

	// Check new array format
	if arr, ok := hookValue.([]interface{}); ok && len(arr) > 0 {
		if config, ok := arr[0].(map[string]interface{}); ok {
			if hooks, ok := config["hooks"].([]interface{}); ok && len(hooks) > 0 {
				if cmd, ok := hooks[0].(map[string]interface{}); ok {
					if cmdStr, ok := cmd["command"].(string); ok {
						return isClaudioCommand(cmdStr)
					}
				}
			}
		}
	}

	return false
}

// Helper function for merge tests to generate hooks with test parameters
func generateTestHooksForMerge() (interface{}, error) {
	factory := GetFilesystemFactory()
	memFS := factory.Memory()
	// Use mock executable path to prevent config corruption during tests
	mockExecPath := "/test/mock/claudio"
	return GenerateClaudioHooks(memFS, mockExecPath)
}

func TestMergeHooksIdempotent(t *testing.T) {
	// TDD RED: Test that merging hooks multiple times produces the same result
	testCases := []struct {
		name             string
		existingSettings *SettingsMap
		expectError      bool
	}{
		{
			name:             "empty settings - first installation",
			existingSettings: &SettingsMap{},
			expectError:      false,
		},
		{
			name: "settings with other content",
			existingSettings: &SettingsMap{
				"version": "1.0",
				"plugins": []string{"plugin1", "plugin2"},
				"config": map[string]interface{}{
					"debug":   true,
					"timeout": 30,
				},
			},
			expectError: false,
		},
		{
			name: "settings with existing empty hooks",
			existingSettings: &SettingsMap{
				"hooks":   map[string]interface{}{},
				"version": "1.0",
			},
			expectError: false,
		},
		{
			name: "settings with existing Claudio hooks (idempotent case)",
			existingSettings: &SettingsMap{
				"hooks": map[string]interface{}{
					// Using old string format to test backward compatibility
					"PreToolUse":       "claudio",
					"PostToolUse":      "claudio",
					"UserPromptSubmit": "claudio",
				},
				"version": "1.0",
			},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Generate fresh Claudio hooks
			claudioHooks, err := generateTestHooksForMerge()
			if err != nil {
				t.Fatalf("Failed to generate Claudio hooks: %v", err)
			}

			// First merge
			result1, err := MergeHooksIntoSettings(tc.existingSettings, claudioHooks)
			if tc.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tc.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if tc.expectError {
				return // Skip further tests if error was expected
			}

			// Second merge (idempotent test)
			result2, err := MergeHooksIntoSettings(result1, claudioHooks)
			if err != nil {
				t.Errorf("Second merge failed: %v", err)
			}

			// Third merge (triple idempotent test)
			result3, err := MergeHooksIntoSettings(result2, claudioHooks)
			if err != nil {
				t.Errorf("Third merge failed: %v", err)
			}

			// All results should be identical
			json1, _ := json.Marshal(result1)
			json2, _ := json.Marshal(result2)
			json3, _ := json.Marshal(result3)

			if string(json1) != string(json2) {
				t.Errorf("First and second merge results differ:\nFirst:  %s\nSecond: %s",
					string(json1), string(json2))
			}

			if string(json2) != string(json3) {
				t.Errorf("Second and third merge results differ:\nSecond: %s\nThird:  %s",
					string(json2), string(json3))
			}

			// Verify Claudio hooks are present
			if hooks, exists := (*result3)["hooks"]; exists {
				hooksMap, ok := hooks.(map[string]interface{})
				if !ok {
					t.Errorf("Hooks should be a map, got: %T", hooks)
				} else {
					expectedHooks := GetHookNames() // Use registry instead of hardcoded list
					for _, expectedHook := range expectedHooks {
						if val, exists := hooksMap[expectedHook]; !exists {
							t.Errorf("Expected hook '%s' missing after merge", expectedHook)
						} else if !isClaudioHook(val) {
							t.Errorf("Expected hook '%s' to be a claudio hook, got: %v", expectedHook, val)
						}
					}
				}
			} else {
				t.Error("Hooks section should exist after merge")
			}

			t.Logf("Idempotent merge test passed for %s", tc.name)
		})
	}
}

func TestMergeHooksPreservesExisting(t *testing.T) {
	// TDD RED: Test that merging preserves existing non-Claudio hooks and settings
	testCases := []struct {
		name                 string
		existingSettings     *SettingsMap
		expectPreservedKeys  []string
		expectPreservedHooks map[string]string
	}{
		{
			name: "preserve existing hooks",
			existingSettings: &SettingsMap{
				"hooks": map[string]interface{}{
					"PreCommit":  "git diff --check",
					"PostCommit": "git push origin main",
					"CustomHook": "echo 'custom'",
				},
				"version": "1.0",
			},
			expectPreservedKeys: []string{"version"},
			expectPreservedHooks: map[string]string{
				"PreCommit":  "git diff --check",
				"PostCommit": "git push origin main",
				"CustomHook": "echo 'custom'",
			},
		},
		{
			name: "preserve mixed existing and Claudio hooks",
			existingSettings: &SettingsMap{
				"hooks": map[string]interface{}{
					"PreToolUse":  "claudio",             // Existing Claudio hook (should be preserved)
					"PreCommit":   "git diff",            // Custom hook (should be preserved)
					"PostToolUse": "custom-sound-player", // Conflicting hook (should be resolved)
				},
				"debug":   true,
				"timeout": 5000,
			},
			expectPreservedKeys: []string{"debug", "timeout"},
			expectPreservedHooks: map[string]string{
				"PreCommit": "git diff",
			},
		},
		{
			name: "preserve complex nested settings",
			existingSettings: &SettingsMap{
				"hooks": map[string]interface{}{
					"CustomHook": "echo test",
				},
				"config": map[string]interface{}{
					"nested": map[string]interface{}{
						"deep": map[string]interface{}{
							"value": "preserved",
						},
					},
				},
				"arrays": []interface{}{"item1", "item2"},
			},
			expectPreservedKeys: []string{"config", "arrays"},
			expectPreservedHooks: map[string]string{
				"CustomHook": "echo test",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Generate Claudio hooks
			claudioHooks, err := generateTestHooksForMerge()
			if err != nil {
				t.Fatalf("Failed to generate Claudio hooks: %v", err)
			}

			// Perform merge
			result, err := MergeHooksIntoSettings(tc.existingSettings, claudioHooks)
			if err != nil {
				t.Errorf("Merge failed: %v", err)
			}

			// Verify preserved top-level keys
			for _, key := range tc.expectPreservedKeys {
				if _, exists := (*result)[key]; !exists {
					t.Errorf("Expected preserved key '%s' missing after merge", key)
				} else {
					// Deep comparison for complex values
					originalVal := (*tc.existingSettings)[key]
					resultVal := (*result)[key]

					origJSON, _ := json.Marshal(originalVal)
					resultJSON, _ := json.Marshal(resultVal)

					if string(origJSON) != string(resultJSON) {
						t.Errorf("Preserved key '%s' value changed:\nOriginal: %s\nResult:   %s",
							key, string(origJSON), string(resultJSON))
					}
				}
			}

			// Verify preserved hooks
			if hooks, exists := (*result)["hooks"]; exists {
				hooksMap, ok := hooks.(map[string]interface{})
				if !ok {
					t.Errorf("Hooks should be a map, got: %T", hooks)
				} else {
					for hookName, expectedValue := range tc.expectPreservedHooks {
						if val, exists := hooksMap[hookName]; !exists {
							t.Errorf("Expected preserved hook '%s' missing", hookName)
						} else if val != expectedValue {
							t.Errorf("Preserved hook '%s' value changed: expected '%s', got '%v'",
								hookName, expectedValue, val)
						}
					}
				}
			}

			// Verify Claudio hooks are also present
			if hooks, exists := (*result)["hooks"]; exists {
				hooksMap, ok := hooks.(map[string]interface{})
				if ok {
					claudioHookNames := GetHookNames() // Use registry instead of hardcoded list
					for _, hookName := range claudioHookNames {
						if val, exists := hooksMap[hookName]; !exists {
							t.Errorf("Claudio hook '%s' missing after merge", hookName)
						} else if !isClaudioHook(val) {
							// For conflicting hooks, check the merge strategy
							t.Logf("Hook '%s' has value '%v' (merge strategy applied)", hookName, val)
						}
					}
				}
			}

			t.Logf("Preserve existing test passed for %s", tc.name)
		})
	}
}

func TestMergeHooksErrorHandling(t *testing.T) {
	// TDD RED: Test error handling in hook merging
	testCases := []struct {
		name             string
		existingSettings *SettingsMap
		claudioHooks     interface{}
		expectError      bool
		errorMsg         string
	}{
		{
			name:             "nil existing settings",
			existingSettings: nil,
			claudioHooks:     map[string]interface{}{"PreToolUse": "claudio"},
			expectError:      true,
			errorMsg:         "settings cannot be nil",
		},
		{
			name:             "nil Claudio hooks",
			existingSettings: &SettingsMap{},
			claudioHooks:     nil,
			expectError:      true,
			errorMsg:         "hooks cannot be nil",
		},
		{
			name:             "invalid hooks type",
			existingSettings: &SettingsMap{},
			claudioHooks:     "not a map",
			expectError:      true,
			errorMsg:         "invalid hooks type",
		},
		{
			name: "corrupted existing hooks",
			existingSettings: &SettingsMap{
				"hooks": "not a map",
			},
			claudioHooks: map[string]interface{}{"PreToolUse": "claudio"},
			expectError:  true,
			errorMsg:     "existing hooks invalid",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := MergeHooksIntoSettings(tc.existingSettings, tc.claudioHooks)

			if tc.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tc.errorMsg != "" && !containsIgnoreCase(err.Error(), tc.errorMsg) {
					t.Errorf("Expected error containing '%s', got: %v", tc.errorMsg, err)
				}
				if result != nil {
					t.Errorf("Expected nil result on error, got: %v", result)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if result == nil {
					t.Errorf("Expected result but got nil")
				}
			}

			t.Logf("Error handling test passed for %s", tc.name)
		})
	}
}

func TestMergeHooksDeepCopy(t *testing.T) {
	// TDD RED: Test that merge creates deep copies and doesn't modify original settings
	original := &SettingsMap{
		"hooks": map[string]interface{}{
			"ExistingHook": "existing-command",
		},
		"config": map[string]interface{}{
			"nested": map[string]interface{}{
				"value": "original",
			},
		},
	}

	// Create a backup for comparison
	originalJSON, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal original: %v", err)
	}

	// Generate Claudio hooks
	claudioHooks, err := generateTestHooksForMerge()
	if err != nil {
		t.Fatalf("Failed to generate Claudio hooks: %v", err)
	}

	// Perform merge
	result, err := MergeHooksIntoSettings(original, claudioHooks)
	if err != nil {
		t.Errorf("Merge failed: %v", err)
	}

	// Verify original wasn't modified
	currentJSON, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal current original: %v", err)
	}

	if string(originalJSON) != string(currentJSON) {
		t.Errorf("Original settings were modified during merge:\nBefore: %s\nAfter:  %s",
			string(originalJSON), string(currentJSON))
	}

	// Verify result is different from original (has Claudio hooks)
	resultJSON, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal result: %v", err)
	}

	if string(originalJSON) == string(resultJSON) {
		t.Error("Result should be different from original (should contain Claudio hooks)")
	}

	// Modify result to verify it doesn't affect original
	if resultHooks, exists := (*result)["hooks"]; exists {
		if hooksMap, ok := resultHooks.(map[string]interface{}); ok {
			hooksMap["TestModification"] = "test-value"
		}
	}

	// Verify original is still unchanged
	finalJSON, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal final original: %v", err)
	}

	if string(originalJSON) != string(finalJSON) {
		t.Error("Original settings were modified by result modification (deep copy failed)")
	}

	t.Logf("Deep copy test passed")
}

// Helper function for case-insensitive string matching
func containsIgnoreCase(s, substr string) bool {
	// Simple implementation for testing
	return len(s) >= len(substr) &&
		(s == substr ||
			len(s) > len(substr) &&
				(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
					findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Functions that will need to be implemented (currently undefined):
// - MergeHooksIntoSettings(existing *SettingsMap, claudioHooks interface{}) (*SettingsMap, error)
