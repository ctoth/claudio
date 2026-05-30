package install

import (
	"encoding/json"
	"testing"
)

// Helper function for merge tests to generate hooks with test parameters
func generateTestHooksForMerge() (interface{}, error) {
	// Use mock executable path to prevent config corruption during tests
	mockExecPath := "/test/mock/claudio"
	return GenerateClaudioHooks(mockExecPath)
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
						} else if !IsClaudioHook(val) {
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
						} else if !IsClaudioHook(val) {
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

func TestMergeHookValues(t *testing.T) {
	// TDD RED: Test the mergeHookValues function in isolation
	testCases := []struct {
		name           string
		existingValue  interface{}
		claudioValue   interface{}
		expectedCount  int // expected number of commands in result
		expectError    bool
	}{
		{
			name:          "merge string hook with claudio array",
			existingValue: "git add .",
			claudioValue: []interface{}{
				map[string]interface{}{
					"matcher": ".*",
					"hooks": []interface{}{
						map[string]interface{}{
							"type":    "command",
							"command": "claudio",
						},
					},
				},
			},
			expectedCount: 2, // existing + claudio
			expectError:   false,
		},
		{
			name: "merge array hook with claudio array",
			existingValue: []interface{}{
				map[string]interface{}{
					"matcher": ".*",
					"hooks": []interface{}{
						map[string]interface{}{
							"type":    "command",
							"command": "existing-cmd",
						},
					},
				},
			},
			claudioValue: []interface{}{
				map[string]interface{}{
					"matcher": ".*",
					"hooks": []interface{}{
						map[string]interface{}{
							"type":    "command",
							"command": "claudio",
						},
					},
				},
			},
			expectedCount: 2, // existing + claudio
			expectError:   false,
		},
		{
			name:          "claudio hook with claudio hook should not duplicate",
			existingValue: "claudio",
			claudioValue: []interface{}{
				map[string]interface{}{
					"matcher": ".*",
					"hooks": []interface{}{
						map[string]interface{}{
							"type":    "command",
							"command": "claudio",
						},
					},
				},
			},
			expectedCount: 1, // just claudio (no duplication)
			expectError:   false,
		},
		{
			name: "complex existing array with multiple commands",
			existingValue: []interface{}{
				map[string]interface{}{
					"matcher": ".*",
					"hooks": []interface{}{
						map[string]interface{}{
							"type":    "command",
							"command": "cmd1",
						},
						map[string]interface{}{
							"type":    "command",
							"command": "cmd2",
						},
					},
				},
			},
			claudioValue: []interface{}{
				map[string]interface{}{
					"matcher": ".*",
					"hooks": []interface{}{
						map[string]interface{}{
							"type":    "command",
							"command": "claudio",
						},
					},
				},
			},
			expectedCount: 3, // cmd1 + cmd2 + claudio
			expectError:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Call the function we're about to implement
			result := mergeHookValues(tc.existingValue, tc.claudioValue)

			// Result should always be in array format
			resultArray, ok := result.([]interface{})
			if !ok {
				t.Errorf("mergeHookValues should return array format, got: %T", result)
				return
			}

			// Count total commands in the result
			totalCommands := 0
			for _, item := range resultArray {
				if config, ok := item.(map[string]interface{}); ok {
					if hooksInItem, ok := config["hooks"].([]interface{}); ok {
						totalCommands += len(hooksInItem)
					}
				}
			}

			if totalCommands != tc.expectedCount {
				t.Errorf("Expected %d commands, got %d. Result: %v", 
					tc.expectedCount, totalCommands, result)
			}

			t.Logf("mergeHookValues test passed for %s", tc.name)
		})
	}
}

func TestMergeHooksWithExistingNonClaudioHooks(t *testing.T) {
	// TDD RED: Test that existing non-Claudio hooks are merged with Claudio hooks in array format
	testCases := []struct {
		name             string
		existingSettings *SettingsMap
		expectMerged     map[string]int // hook name -> expected number of commands
		expectError      bool
	}{
		{
			name: "merge single existing string hook with claudio",
			existingSettings: &SettingsMap{
				"hooks": map[string]interface{}{
					"PostToolUse": "git add .",
				},
			},
			expectMerged: map[string]int{
				"PostToolUse": 2, // existing + claudio command
			},
			expectError: false,
		},
		{
			name: "merge multiple existing hooks with claudio",
			existingSettings: &SettingsMap{
				"hooks": map[string]interface{}{
					"PostToolUse": "custom-script.sh",
					"PreToolUse":  "echo 'starting'",
				},
			},
			expectMerged: map[string]int{
				"PostToolUse": 2, // existing + claudio
				"PreToolUse":  2, // existing + claudio
			},
			expectError: false,
		},
		{
			name: "merge existing array hook with claudio",
			existingSettings: &SettingsMap{
				"hooks": map[string]interface{}{
					"PostToolUse": []interface{}{
						map[string]interface{}{
							"matcher": ".*",
							"hooks": []interface{}{
								map[string]interface{}{
									"type":    "command",
									"command": "existing-command",
								},
							},
						},
					},
				},
			},
			expectMerged: map[string]int{
				"PostToolUse": 2, // existing array + claudio command
			},
			expectError: false,
		},
		{
			name: "existing claudio hook should not duplicate",
			existingSettings: &SettingsMap{
				"hooks": map[string]interface{}{
					"PostToolUse": "claudio",
				},
			},
			expectMerged: map[string]int{
				"PostToolUse": 1, // just claudio (no duplication)
			},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Generate mock Claudio hooks with known structure
			mockClaudioHooks := map[string]interface{}{
				"PostToolUse": []interface{}{
					map[string]interface{}{
						"matcher": ".*",
						"hooks": []interface{}{
							map[string]interface{}{
								"type":    "command",
								"command": "/test/mock/claudio",
							},
						},
					},
				},
				"PreToolUse": []interface{}{
					map[string]interface{}{
						"matcher": ".*",
						"hooks": []interface{}{
							map[string]interface{}{
								"type":    "command",
								"command": "/test/mock/claudio",
							},
						},
					},
				},
			}

			// Perform merge
			result, err := MergeHooksIntoSettings(tc.existingSettings, mockClaudioHooks)

			if tc.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if result == nil {
				t.Errorf("Expected result but got nil")
				return
			}

			// Verify merged hooks structure
			hooks, exists := (*result)["hooks"]
			if !exists {
				t.Errorf("No hooks section in result")
				return
			}

			hooksMap, ok := hooks.(map[string]interface{})
			if !ok {
				t.Errorf("Hooks should be a map, got: %T", hooks)
				return
			}

			// Check each expected merged hook
			for hookName, expectedCount := range tc.expectMerged {
				hookValue, exists := hooksMap[hookName]
				if !exists {
					t.Errorf("Expected hook '%s' missing from result", hookName)
					continue
				}

				// Hook should be in array format after merging
				hookArray, ok := hookValue.([]interface{})
				if !ok {
					t.Errorf("Hook '%s' should be array format after merge, got: %T", hookName, hookValue)
					continue
				}

				// Count total commands in the array
				totalCommands := 0
				for _, item := range hookArray {
					if config, ok := item.(map[string]interface{}); ok {
						if hooksInItem, ok := config["hooks"].([]interface{}); ok {
							totalCommands += len(hooksInItem)
						}
					}
				}

				if totalCommands != expectedCount {
					t.Errorf("Hook '%s' expected %d commands, got %d. Hook value: %v", 
						hookName, expectedCount, totalCommands, hookValue)
				}
			}

			t.Logf("Merge test passed for %s", tc.name)
		})
	}
}

// Functions that will need to be implemented (currently undefined):
// - MergeHooksIntoSettings(existing *SettingsMap, claudioHooks interface{}) (*SettingsMap, error)

// claudioArrayEntry returns a hook array element with a single claudio command.
func claudioArrayEntry(cmd string) map[string]interface{} {
	return map[string]interface{}{
		"matcher": ".*",
		"hooks": []interface{}{
			map[string]interface{}{
				"type":    "command",
				"command": cmd,
			},
		},
	}
}

// customArrayEntry returns a hook array element with a single non-claudio
// command. The matcher value is parametrised so callers can distinguish their
// entries when asserting preservation.
func customArrayEntry(matcher, cmd string) map[string]interface{} {
	return map[string]interface{}{
		"matcher": matcher,
		"hooks": []interface{}{
			map[string]interface{}{
				"type":    "command",
				"command": cmd,
			},
		},
	}
}

func countCommandsInHookValue(t *testing.T, v interface{}) int {
	t.Helper()
	arr, ok := v.([]interface{})
	if !ok {
		t.Fatalf("expected []interface{} hook value, got %T", v)
	}
	count := 0
	for _, item := range arr {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		hooks, ok := m["hooks"].([]interface{})
		if !ok {
			continue
		}
		count += len(hooks)
	}
	return count
}

// TestIsClaudioHookMultiMatcherArray verifies that IsClaudioHook returns true
// for an array whose claudio entry is NOT in position 0. The old arr[0]-only
// scan would have returned false here, and the merge path would have appended
// a second claudio block.
func TestIsClaudioHookMultiMatcherArray(t *testing.T) {
	value := []interface{}{
		customArrayEntry(".*", "/usr/local/bin/custom"),
		claudioArrayEntry("/usr/local/bin/claudio"),
	}
	if !IsClaudioHook(value) {
		t.Errorf("IsClaudioHook should detect claudio in any array position, got false for %v", value)
	}
}

// TestIsClaudioHookMultiHookInsideMatcher verifies the predicate scans every
// inner hook inside a matcher block, not just hooks[0].
func TestIsClaudioHookMultiHookInsideMatcher(t *testing.T) {
	value := []interface{}{
		map[string]interface{}{
			"matcher": ".*",
			"hooks": []interface{}{
				map[string]interface{}{"type": "command", "command": "/usr/local/bin/custom"},
				map[string]interface{}{"type": "command", "command": "/usr/local/bin/claudio"},
			},
		},
	}
	if !IsClaudioHook(value) {
		t.Errorf("IsClaudioHook should detect claudio in any hooks-sub-array position, got false")
	}
}

// TestMergeHooksIdempotent_CustomThenClaudio: starting with a pre-existing
// [custom, claudio] array, run two merges. After the first merge the array
// should still have exactly two elements (claudio is replaced, not duplicated).
// After the second merge it must remain two elements (idempotency).
func TestMergeHooksIdempotent_CustomThenClaudio(t *testing.T) {
	const claudioCmd = "/test/mock/claudio"

	existing := &SettingsMap{
		"hooks": map[string]interface{}{
			"PostToolUse": []interface{}{
				customArrayEntry(".*", "/usr/local/bin/custom"),
				claudioArrayEntry(claudioCmd),
			},
		},
	}

	claudioHooks := map[string]interface{}{
		"PostToolUse": []interface{}{claudioArrayEntry(claudioCmd)},
	}

	first, err := MergeHooksIntoSettings(existing, claudioHooks)
	if err != nil {
		t.Fatalf("first merge failed: %v", err)
	}

	firstHooks, _ := (*first)["hooks"].(map[string]interface{})
	firstArr, ok := firstHooks["PostToolUse"].([]interface{})
	if !ok {
		t.Fatalf("expected PostToolUse to be []interface{}, got %T", firstHooks["PostToolUse"])
	}
	if got, want := len(firstArr), 2; got != want {
		t.Errorf("after first merge: expected %d array elements, got %d (%v)", want, got, firstArr)
	}

	// Second merge — must be byte-identical to first.
	second, err := MergeHooksIntoSettings(first, claudioHooks)
	if err != nil {
		t.Fatalf("second merge failed: %v", err)
	}

	firstJSON, _ := json.Marshal(first)
	secondJSON, _ := json.Marshal(second)
	if string(firstJSON) != string(secondJSON) {
		t.Errorf("merge not idempotent:\nfirst:  %s\nsecond: %s", firstJSON, secondJSON)
	}
}

// TestMergeHooksIdempotent_ClaudioThenCustom asserts the data-loss bug is
// fixed: starting with [claudio, custom], the custom entry is preserved
// across a merge. Under the old code IsClaudioHook(arr[0]) returned true
// and the idempotent-update branch replaced the whole array with the
// claudio-only value, deleting the user's custom hook.
func TestMergeHooksIdempotent_ClaudioThenCustom(t *testing.T) {
	const claudioCmd = "/test/mock/claudio"
	const customCmd = "/usr/local/bin/custom"

	existing := &SettingsMap{
		"hooks": map[string]interface{}{
			"PostToolUse": []interface{}{
				claudioArrayEntry(claudioCmd),
				customArrayEntry("custom-matcher", customCmd),
			},
		},
	}

	claudioHooks := map[string]interface{}{
		"PostToolUse": []interface{}{claudioArrayEntry(claudioCmd)},
	}

	merged, err := MergeHooksIntoSettings(existing, claudioHooks)
	if err != nil {
		t.Fatalf("merge failed: %v", err)
	}

	mergedHooks, _ := (*merged)["hooks"].(map[string]interface{})
	arr, ok := mergedHooks["PostToolUse"].([]interface{})
	if !ok {
		t.Fatalf("expected PostToolUse to be []interface{}, got %T", mergedHooks["PostToolUse"])
	}

	if got, want := len(arr), 2; got != want {
		t.Errorf("expected %d array elements after merge, got %d (%v)", want, got, arr)
	}

	// Verify the custom entry survived.
	foundCustom := false
	for _, item := range arr {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		hooks, _ := m["hooks"].([]interface{})
		for _, h := range hooks {
			hm, ok := h.(map[string]interface{})
			if !ok {
				continue
			}
			if cmd, _ := hm["command"].(string); cmd == customCmd {
				foundCustom = true
			}
		}
	}
	if !foundCustom {
		t.Errorf("custom hook %q was silently deleted by merge — data-loss bug regressed. Got: %v", customCmd, arr)
	}

	// Exactly one claudio entry.
	if claudioCount := countCommandsInHookValue(t, arr) - 1; claudioCount != 1 {
		t.Errorf("expected exactly 1 claudio command after merge, got %d (1 custom + claudio commands)", claudioCount)
	}
}

// TestMergeHooksIdempotent_MultipleClaudio: a pre-corrupted state with two
// claudio entries plus one custom entry. One merge must collapse to exactly
// one claudio plus the custom — self-healing for any settings file already
// corrupted by the pre-fix code.
func TestMergeHooksIdempotent_MultipleClaudio(t *testing.T) {
	const claudioCmd = "/test/mock/claudio"
	const customCmd = "/usr/local/bin/custom"

	existing := &SettingsMap{
		"hooks": map[string]interface{}{
			"PostToolUse": []interface{}{
				claudioArrayEntry(claudioCmd),
				claudioArrayEntry(claudioCmd),
				customArrayEntry("custom-matcher", customCmd),
			},
		},
	}

	claudioHooks := map[string]interface{}{
		"PostToolUse": []interface{}{claudioArrayEntry(claudioCmd)},
	}

	merged, err := MergeHooksIntoSettings(existing, claudioHooks)
	if err != nil {
		t.Fatalf("merge failed: %v", err)
	}

	mergedHooks, _ := (*merged)["hooks"].(map[string]interface{})
	arr, ok := mergedHooks["PostToolUse"].([]interface{})
	if !ok {
		t.Fatalf("expected PostToolUse to be []interface{}, got %T", mergedHooks["PostToolUse"])
	}

	if got, want := len(arr), 2; got != want {
		t.Errorf("expected exactly 2 array elements (1 custom + 1 claudio) after collapsing duplicates, got %d (%v)", got, arr)
	}

	// Count claudio commands across the array.
	claudioCommands := 0
	customFound := false
	for _, item := range arr {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		hooks, _ := m["hooks"].([]interface{})
		for _, h := range hooks {
			hm, ok := h.(map[string]interface{})
			if !ok {
				continue
			}
			cmd, _ := hm["command"].(string)
			if cmd == claudioCmd {
				claudioCommands++
			}
			if cmd == customCmd {
				customFound = true
			}
		}
	}
	if claudioCommands != 1 {
		t.Errorf("expected exactly 1 claudio command after collapse, got %d", claudioCommands)
	}
	if !customFound {
		t.Errorf("custom command %q was lost during multi-claudio collapse", customCmd)
	}
}

// TestMergeHooksIdempotent_ClaudioSiblingNonClaudio asserts that when a
// matcher's hooks sub-array contains BOTH a Claudio command AND a user
// non-Claudio command, the merge filters only the Claudio command and
// preserves the user command in the same matcher block.
//
// Regression for Chunk 5 analyst Finding 1: the strip-and-replace previously
// dropped the entire array item if it contained ANY Claudio command, losing
// the user's sibling command in the same hooks sub-array.
func TestMergeHooksIdempotent_ClaudioSiblingNonClaudio(t *testing.T) {
	const claudioCmd = "/test/mock/claudio"
	const userCmd = "/usr/local/bin/user-lint"

	existing := &SettingsMap{
		"hooks": map[string]interface{}{
			"PostToolUse": []interface{}{
				map[string]interface{}{
					"matcher": ".*",
					"hooks": []interface{}{
						map[string]interface{}{"type": "command", "command": claudioCmd},
						map[string]interface{}{"type": "command", "command": userCmd},
					},
				},
			},
		},
	}

	claudioHooks := map[string]interface{}{
		"PostToolUse": []interface{}{claudioArrayEntry(claudioCmd)},
	}

	merged, err := MergeHooksIntoSettings(existing, claudioHooks)
	if err != nil {
		t.Fatalf("first merge failed: %v", err)
	}

	// Inspect the merged result: the user-lint command must survive somewhere
	// in the PostToolUse array. The Claudio command must appear exactly once.
	mergedHooks, _ := (*merged)["hooks"].(map[string]interface{})
	arr, ok := mergedHooks["PostToolUse"].([]interface{})
	if !ok {
		t.Fatalf("expected PostToolUse to be []interface{}, got %T", mergedHooks["PostToolUse"])
	}

	userFound := false
	claudioCount := 0
	for _, item := range arr {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		hooks, _ := m["hooks"].([]interface{})
		for _, h := range hooks {
			hm, ok := h.(map[string]interface{})
			if !ok {
				continue
			}
			cmd, _ := hm["command"].(string)
			switch cmd {
			case userCmd:
				userFound = true
			case claudioCmd:
				claudioCount++
			}
		}
	}
	if !userFound {
		t.Errorf("user command %q was silently deleted by merge — Finding 1 regressed. Got: %v", userCmd, arr)
	}
	if claudioCount != 1 {
		t.Errorf("expected exactly 1 claudio command after merge, got %d", claudioCount)
	}

	// Second merge — must be byte-identical (idempotency).
	second, err := MergeHooksIntoSettings(merged, claudioHooks)
	if err != nil {
		t.Fatalf("second merge failed: %v", err)
	}
	firstJSON, _ := json.Marshal(merged)
	secondJSON, _ := json.Marshal(second)
	if string(firstJSON) != string(secondJSON) {
		t.Errorf("merge not idempotent:\nfirst:  %s\nsecond: %s", firstJSON, secondJSON)
	}
}
