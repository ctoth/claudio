package install

import (
	"maps"
	"reflect"
	"testing"
)

func TestRemoveClaudioHooksWithFullPaths(t *testing.T) {
	// TDD RED: Test removal of claudio hooks with full executable paths instead of just "claudio"
	testCases := []struct {
		name             string
		initialSettings  *SettingsMap
		hookNames        []string
		expectedSettings *SettingsMap
	}{
		{
			name: "remove simple string hook - full system path",
			initialSettings: &SettingsMap{
				"hooks": map[string]any{
					"PreToolUse": "/usr/local/bin/claudio",
					"PostCommit": "git push",
				},
				"version": "1.0",
			},
			hookNames: []string{"PreToolUse"},
			expectedSettings: &SettingsMap{
				"hooks": map[string]any{
					"PostCommit": "git push",
				},
				"version": "1.0",
			},
		},
		{
			name: "remove simple string hook - dev directory path",
			initialSettings: &SettingsMap{
				"hooks": map[string]any{
					"PostToolUse": "/home/user/dev/claudio/claudio",
					"Other":       "keep-this",
				},
			},
			hookNames: []string{"PostToolUse"},
			expectedSettings: &SettingsMap{
				"hooks": map[string]any{
					"Other": "keep-this",
				},
			},
		},
		{
			name: "remove simple string hook - relative path",
			initialSettings: &SettingsMap{
				"hooks": map[string]any{
					"UserPromptSubmit": "./claudio",
					"Other":            "keep",
				},
			},
			hookNames: []string{"UserPromptSubmit"},
			expectedSettings: &SettingsMap{
				"hooks": map[string]any{
					"Other": "keep",
				},
			},
		},
		{
			name: "remove complex array hook - full system path",
			initialSettings: &SettingsMap{
				"hooks": map[string]any{
					"Notification": []any{
						map[string]any{
							"matcher": ".*",
							"hooks": []any{
								map[string]any{
									"command": "/usr/local/bin/claudio",
									"type":    "command",
								},
							},
						},
					},
					"Other": "keep",
				},
			},
			hookNames: []string{"Notification"},
			expectedSettings: &SettingsMap{
				"hooks": map[string]any{
					"Other": "keep",
				},
			},
		},
		{
			name: "remove direct Copilot command hook",
			initialSettings: &SettingsMap{
				"hooks": map[string]any{
					"PreToolUse": []any{
						map[string]any{
							"command": "/usr/local/bin/claudio --hook-agent copilot",
							"type":    "command",
						},
						map[string]any{
							"command": "/usr/bin/logger",
							"type":    "command",
						},
					},
				},
			},
			hookNames: []string{"PreToolUse"},
			expectedSettings: &SettingsMap{
				"hooks": map[string]any{
					"PreToolUse": []any{
						map[string]any{
							"command": "/usr/bin/logger",
							"type":    "command",
						},
					},
				},
			},
		},
		{
			name: "remove complex array hook - dev directory path",
			initialSettings: &SettingsMap{
				"hooks": map[string]any{
					"SessionStart": []any{
						map[string]any{
							"matcher": ".*",
							"hooks": []any{
								map[string]any{
									"command": "/root/code/claudio/claudio",
									"type":    "command",
								},
							},
						},
					},
				},
			},
			hookNames:        []string{"SessionStart"},
			expectedSettings: &SettingsMap{
				// hooks section should be removed when empty
			},
		},
		{
			name: "remove claudio from mixed array with other commands - preserve others",
			initialSettings: &SettingsMap{
				"hooks": map[string]any{
					"Stop": []any{
						map[string]any{
							"matcher": ".*",
							"hooks": []any{
								map[string]any{
									"command": "/usr/local/bin/claudio",
									"type":    "command",
								},
								map[string]any{
									"command": "other-tool",
									"type":    "command",
								},
							},
						},
					},
				},
			},
			hookNames: []string{"Stop"},
			expectedSettings: &SettingsMap{
				"hooks": map[string]any{
					"Stop": []any{
						map[string]any{
							"matcher": ".*",
							"hooks": []any{
								map[string]any{
									"command": "other-tool",
									"type":    "command",
								},
							},
						},
					},
				},
			},
		},
		{
			name: "mixed full paths and backward compatibility",
			initialSettings: &SettingsMap{
				"hooks": map[string]any{
					"PreToolUse":  "claudio",                    // Old format - should be removed
					"PostToolUse": "/usr/local/bin/claudio",     // Full path - should be removed
					"Stop":        "./claudio",                  // Relative path - should be removed
					"Keep":        "/usr/bin/different-command", // Non-claudio - should be kept
				},
			},
			hookNames: []string{"PreToolUse", "PostToolUse", "Stop"},
			expectedSettings: &SettingsMap{
				"hooks": map[string]any{
					"Keep": "/usr/bin/different-command",
				},
			},
		},
		{
			name: "no claudio paths to remove - no changes",
			initialSettings: &SettingsMap{
				"hooks": map[string]any{
					"PreToolUse":  "/usr/bin/git",
					"PostToolUse": "/bin/echo",
				},
			},
			hookNames: []string{"PreToolUse", "PostToolUse"},
			expectedSettings: &SettingsMap{
				"hooks": map[string]any{
					"PreToolUse":  "/usr/bin/git",
					"PostToolUse": "/bin/echo",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Make a copy to avoid modifying the original
			settingsCopy := copySettingsForTest(tc.initialSettings)

			// Call both removal functions (since we don't know which format is used)
			removeClaudioHooks(settingsCopy)

			// Verify the result
			if !settingsEqual(settingsCopy, tc.expectedSettings) {
				t.Errorf("Settings mismatch.\nExpected: %+v\nActual:   %+v",
					tc.expectedSettings, settingsCopy)
			}

			t.Logf("Full path removal test passed for %s", tc.name)
		})
	}
}

func TestRemoveSimpleClaudioHooks(t *testing.T) {
	// TDD RED: Test removal of simple string claudio hooks
	testCases := []struct {
		name             string
		initialSettings  *SettingsMap
		hookNames        []string
		expectedSettings *SettingsMap
		expectError      bool
	}{
		{
			name: "remove single claudio hook with other hooks preserved",
			initialSettings: &SettingsMap{
				"hooks": map[string]any{
					"PreToolUse": "claudio",
					"PostCommit": "git push",
					"Other":      "keep-this",
				},
				"version": "1.0",
			},
			hookNames: []string{"PreToolUse"},
			expectedSettings: &SettingsMap{
				"hooks": map[string]any{
					"PostCommit": "git push",
					"Other":      "keep-this",
				},
				"version": "1.0",
			},
			expectError: false,
		},
		{
			name: "remove multiple claudio hooks",
			initialSettings: &SettingsMap{
				"hooks": map[string]any{
					"PreToolUse":       "claudio",
					"PostToolUse":      "claudio",
					"UserPromptSubmit": "claudio",
					"Other":            "keep",
				},
			},
			hookNames: []string{"PreToolUse", "PostToolUse", "UserPromptSubmit"},
			expectedSettings: &SettingsMap{
				"hooks": map[string]any{
					"Other": "keep",
				},
			},
			expectError: false,
		},
		{
			name: "remove non-existent hook - no changes",
			initialSettings: &SettingsMap{
				"hooks": map[string]any{
					"Other": "keep",
				},
			},
			hookNames: []string{"NonExistent"},
			expectedSettings: &SettingsMap{
				"hooks": map[string]any{
					"Other": "keep",
				},
			},
			expectError: false,
		},
		{
			name: "remove from empty settings - no changes",
			initialSettings: &SettingsMap{
				"version": "1.0",
			},
			hookNames: []string{"PreToolUse"},
			expectedSettings: &SettingsMap{
				"version": "1.0",
			},
			expectError: false,
		},
		{
			name: "remove all hooks - hooks section deleted",
			initialSettings: &SettingsMap{
				"hooks": map[string]any{
					"PreToolUse": "claudio",
				},
				"version": "1.0",
			},
			hookNames: []string{"PreToolUse"},
			expectedSettings: &SettingsMap{
				"version": "1.0",
			},
			expectError: false,
		},
		{
			name:             "nil settings - no error",
			initialSettings:  nil,
			hookNames:        []string{"PreToolUse"},
			expectedSettings: nil,
			expectError:      false,
		},
		{
			name: "remove hook that exists but is not claudio - no changes",
			initialSettings: &SettingsMap{
				"hooks": map[string]any{
					"PreToolUse": "other-command",
					"Other":      "keep",
				},
			},
			hookNames: []string{"PreToolUse"},
			expectedSettings: &SettingsMap{
				"hooks": map[string]any{
					"PreToolUse": "other-command",
					"Other":      "keep",
				},
			},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Make a copy to avoid modifying the original
			var settingsCopy *SettingsMap
			if tc.initialSettings != nil {
				settingsCopy = copySettingsForTest(tc.initialSettings)
			}

			// Call the function
			removeClaudioHooks(settingsCopy)

			// Verify the result
			if !settingsEqual(settingsCopy, tc.expectedSettings) {
				t.Errorf("Settings mismatch.\nExpected: %+v\nActual:   %+v",
					tc.expectedSettings, settingsCopy)
			}

			t.Logf("Simple hook removal test passed for %s", tc.name)
		})
	}
}

// Helper function to deep copy settings for testing
func copySettingsForTest(original *SettingsMap) *SettingsMap {
	if original == nil {
		return nil
	}

	copy := make(SettingsMap)
	for key, value := range *original {
		if key == "hooks" {
			if hooksMap, ok := value.(map[string]any); ok {
				hooksCopy := make(map[string]any)
				maps.Copy(hooksCopy, hooksMap)
				copy[key] = hooksCopy
			} else {
				copy[key] = value
			}
		} else {
			copy[key] = value
		}
	}
	return &copy
}

// Helper function to compare settings maps
func settingsEqual(a, b *SettingsMap) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	if len(*a) != len(*b) {
		return false
	}

	for key, valueA := range *a {
		valueB, exists := (*b)[key]
		if !exists {
			return false
		}

		if key == "hooks" {
			hooksA, okA := valueA.(map[string]any)
			hooksB, okB := valueB.(map[string]any)
			if okA != okB {
				return false
			}
			if okA && okB {
				if len(hooksA) != len(hooksB) {
					return false
				}
				for hookKey, hookValueA := range hooksA {
					hookValueB, exists := hooksB[hookKey]
					if !exists || !reflect.DeepEqual(hookValueA, hookValueB) {
						return false
					}
				}
			}
		} else {
			if !reflect.DeepEqual(valueA, valueB) {
				return false
			}
		}
	}

	return true
}

func TestRemoveComplexClaudioHooks(t *testing.T) {
	// TDD RED: Test removal of complex array claudio hooks
	testCases := []struct {
		name             string
		initialSettings  *SettingsMap
		hookNames        []string
		expectedSettings *SettingsMap
		expectError      bool
	}{
		{
			name: "remove claudio from array with other commands - preserve others",
			initialSettings: &SettingsMap{
				"hooks": map[string]any{
					"Notification": []any{
						map[string]any{
							"hooks": []any{
								map[string]any{
									"command": "claudio",
									"type":    "command",
								},
								map[string]any{
									"command": "other-tool",
									"type":    "command",
								},
							},
						},
					},
				},
			},
			hookNames: []string{"Notification"},
			expectedSettings: &SettingsMap{
				"hooks": map[string]any{
					"Notification": []any{
						map[string]any{
							"hooks": []any{
								map[string]any{
									"command": "other-tool",
									"type":    "command",
								},
							},
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "remove claudio from array with only claudio - delete entire hook",
			initialSettings: &SettingsMap{
				"hooks": map[string]any{
					"Stop": []any{
						map[string]any{
							"hooks": []any{
								map[string]any{
									"command": "claudio",
									"type":    "command",
								},
							},
						},
					},
					"Other": "keep-this",
				},
			},
			hookNames: []string{"Stop"},
			expectedSettings: &SettingsMap{
				"hooks": map[string]any{
					"Other": "keep-this",
				},
			},
			expectError: false,
		},
		{
			name: "remove claudio from multiple array elements",
			initialSettings: &SettingsMap{
				"hooks": map[string]any{
					"PreCompact": []any{
						map[string]any{
							"matcher": ".*",
							"hooks": []any{
								map[string]any{
									"command": "claudio",
									"type":    "command",
								},
								map[string]any{
									"command": "keep-this",
									"type":    "command",
								},
							},
						},
						map[string]any{
							"matcher": "specific",
							"hooks": []any{
								map[string]any{
									"command": "claudio",
									"type":    "command",
								},
							},
						},
					},
				},
			},
			hookNames: []string{"PreCompact"},
			expectedSettings: &SettingsMap{
				"hooks": map[string]any{
					"PreCompact": []any{
						map[string]any{
							"matcher": ".*",
							"hooks": []any{
								map[string]any{
									"command": "keep-this",
									"type":    "command",
								},
							},
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "remove from non-array hook - no changes",
			initialSettings: &SettingsMap{
				"hooks": map[string]any{
					"Simple": "not-an-array",
				},
			},
			hookNames: []string{"Simple"},
			expectedSettings: &SettingsMap{
				"hooks": map[string]any{
					"Simple": "not-an-array",
				},
			},
			expectError: false,
		},
		{
			name: "remove from array without claudio - no changes",
			initialSettings: &SettingsMap{
				"hooks": map[string]any{
					"Other": []any{
						map[string]any{
							"hooks": []any{
								map[string]any{
									"command": "different-tool",
									"type":    "command",
								},
							},
						},
					},
				},
			},
			hookNames: []string{"Other"},
			expectedSettings: &SettingsMap{
				"hooks": map[string]any{
					"Other": []any{
						map[string]any{
							"hooks": []any{
								map[string]any{
									"command": "different-tool",
									"type":    "command",
								},
							},
						},
					},
				},
			},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Make a copy to avoid modifying the original
			var settingsCopy *SettingsMap
			if tc.initialSettings != nil {
				settingsCopy = deepCopyComplexSettings(tc.initialSettings)
			}

			// Call the function
			removeClaudioHooks(settingsCopy)

			// Verify the result
			if !complexSettingsEqual(settingsCopy, tc.expectedSettings) {
				t.Errorf("Settings mismatch.\nExpected: %+v\nActual:   %+v",
					tc.expectedSettings, settingsCopy)
			}

			t.Logf("Complex hook removal test passed for %s", tc.name)
		})
	}
}

// Helper function for deep copying complex settings with arrays
func deepCopyComplexSettings(original *SettingsMap) *SettingsMap {
	if original == nil {
		return nil
	}

	copy := make(SettingsMap)
	for key, value := range *original {
		if key == "hooks" {
			if hooksMap, ok := value.(map[string]any); ok {
				hooksCopy := make(map[string]any)
				for hookKey, hookValue := range hooksMap {
					hooksCopy[hookKey] = deepCopyValue(hookValue)
				}
				copy[key] = hooksCopy
			} else {
				copy[key] = value
			}
		} else {
			copy[key] = value
		}
	}
	return &copy
}

// Helper function to recursively deep copy interface{} values
func deepCopyValue(value any) any {
	switch v := value.(type) {
	case []any:
		copiedSlice := make([]any, len(v))
		for i, item := range v {
			copiedSlice[i] = deepCopyValue(item)
		}
		return copiedSlice
	case map[string]any:
		copiedMap := make(map[string]any)
		for k, item := range v {
			copiedMap[k] = deepCopyValue(item)
		}
		return copiedMap
	default:
		return v
	}
}

// Helper function to compare complex settings with arrays
func complexSettingsEqual(a, b *SettingsMap) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	if len(*a) != len(*b) {
		return false
	}

	for key, valueA := range *a {
		valueB, exists := (*b)[key]
		if !exists {
			return false
		}

		if !deepValueEqual(valueA, valueB) {
			return false
		}
	}

	return true
}

// Helper function to recursively compare interface{} values
func deepValueEqual(a, b any) bool {
	switch va := a.(type) {
	case []any:
		vb, ok := b.([]any)
		if !ok || len(va) != len(vb) {
			return false
		}
		for i := range va {
			if !deepValueEqual(va[i], vb[i]) {
				return false
			}
		}
		return true
	case map[string]any:
		vb, ok := b.(map[string]any)
		if !ok || len(va) != len(vb) {
			return false
		}
		for k, v := range va {
			if otherV, exists := vb[k]; !exists || !deepValueEqual(v, otherV) {
				return false
			}
		}
		return true
	default:
		return a == b
	}
}

func TestRemoveNewFormatClaudioHooks(t *testing.T) {
	// TDD: Test removal of Claude Code's new array/object format hooks
	testCases := []struct {
		name             string
		initialSettings  *SettingsMap
		hookNames        []string
		expectedSettings *SettingsMap
	}{
		{
			name: "remove new format claudio hook",
			initialSettings: &SettingsMap{
				"hooks": map[string]any{
					"PreToolUse": []any{
						map[string]any{
							"hooks": []any{
								map[string]any{
									"type":    "command",
									"command": "claudio",
								},
							},
						},
					},
					"PostToolUse": "other-command", // Keep non-claudio hook
				},
			},
			hookNames: []string{"PreToolUse"},
			expectedSettings: &SettingsMap{
				"hooks": map[string]any{
					"PostToolUse": "other-command",
				},
			},
		},
		{
			name: "remove multiple new format hooks",
			initialSettings: &SettingsMap{
				"hooks": map[string]any{
					"PreToolUse": []any{
						map[string]any{
							"hooks": []any{
								map[string]any{
									"type":    "command",
									"command": "claudio",
								},
							},
						},
					},
					"PostToolUse": []any{
						map[string]any{
							"hooks": []any{
								map[string]any{
									"type":    "command",
									"command": "claudio",
								},
							},
						},
					},
					"UserPromptSubmit": []any{
						map[string]any{
							"hooks": []any{
								map[string]any{
									"type":    "command",
									"command": "claudio",
								},
							},
						},
					},
				},
				"version": "1.0",
			},
			hookNames: []string{"PreToolUse", "PostToolUse", "UserPromptSubmit"},
			expectedSettings: &SettingsMap{
				"version": "1.0",
			},
		},
		{
			name: "handle mixed old and new format hooks",
			initialSettings: &SettingsMap{
				"hooks": map[string]any{
					"PreToolUse": "claudio", // Old string format
					"PostToolUse": []any{ // New array format
						map[string]any{
							"hooks": []any{
								map[string]any{
									"type":    "command",
									"command": "claudio",
								},
							},
						},
					},
					"UserPromptSubmit": "other-command",
				},
			},
			hookNames: []string{"PreToolUse", "PostToolUse"},
			expectedSettings: &SettingsMap{
				"hooks": map[string]any{
					"UserPromptSubmit": "other-command",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Make a copy to avoid modifying the original
			settingsCopy := copySettingsForTest(tc.initialSettings)

			// The current implementation should handle both formats
			removeClaudioHooks(settingsCopy)

			// Verify the result
			if !settingsEqual(settingsCopy, tc.expectedSettings) {
				t.Errorf("Settings mismatch.\nExpected: %+v\nActual:   %+v",
					tc.expectedSettings, settingsCopy)
			}

			t.Logf("New format hook removal test passed for %s", tc.name)
		})
	}
}

// TestRemoveClaudioHooks_NoClaudioHooksRemain is the load-bearing post-
// condition that was previously enforced as a self-confirming read-back
// inside RunUninstallWorkflow (finding #92). It asserts the actual
// invariant — after running both removal primitives over a settings
// map, ClaudioHookNames returns the empty set regardless of the
// input shape (string form, array form, mixed, multiple-claudio).
func TestRemoveClaudioHooks_NoClaudioHooksRemain(t *testing.T) {
	cases := []struct {
		name  string
		input *SettingsMap
	}{
		{
			name: "simple string hooks pointing at claudio",
			input: &SettingsMap{
				"hooks": map[string]any{
					"PreToolUse":  "/usr/local/bin/claudio",
					"PostToolUse": "claudio",
					"OtherCmd":    "git push",
				},
			},
		},
		{
			name: "array form claudio hooks",
			input: &SettingsMap{
				"hooks": map[string]any{
					"PreToolUse": []any{
						map[string]any{
							"matcher": ".*",
							"hooks": []any{
								map[string]any{
									"type":    "command",
									"command": "/usr/local/bin/claudio",
								},
							},
						},
					},
					"Other": []any{
						map[string]any{
							"matcher": ".*",
							"hooks": []any{
								map[string]any{
									"type":    "command",
									"command": "/usr/bin/echo",
								},
							},
						},
					},
				},
			},
		},
		{
			name: "mixed string and array claudio hooks",
			input: &SettingsMap{
				"hooks": map[string]any{
					"PreToolUse": "claudio",
					"PostToolUse": []any{
						map[string]any{
							"matcher": ".*",
							"hooks": []any{
								map[string]any{
									"type":    "command",
									"command": "claudio.exe",
								},
							},
						},
					},
					"keep": "echo hi",
				},
			},
		},
		{
			name: "no claudio hooks at all (idempotent)",
			input: &SettingsMap{
				"hooks": map[string]any{
					"PreToolUse":  "echo hi",
					"PostToolUse": "git push",
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			removeClaudioHooks(tc.input)

			// Invariant: no claudio hooks remain after removal,
			// regardless of input shape.
			remaining := ClaudioHookNames(tc.input)
			if len(remaining) != 0 {
				t.Errorf("invariant violated: claudio hooks still detected after removal: %v\nresulting settings: %+v", remaining, *tc.input)
			}
		})
	}

}

// TestRemoveComplexClaudioHooksPreservesPreExistingEmptyItem regresses the
// drop-on-empty bug: a user's hand-edited {"matcher":"a","hooks":[]} entry
// would be silently deleted alongside the Claudio entry because the old code
// used "filteredHooks is empty" as a proxy for "we removed Claudio".
func TestRemoveComplexClaudioHooksPreservesPreExistingEmptyItem(t *testing.T) {
	preExistingEmpty := map[string]any{
		"matcher": "user-block",
		"hooks":   []any{},
	}
	claudioItem := map[string]any{
		"matcher": "claudio-target",
		"hooks": []any{
			map[string]any{
				"type":    "command",
				"command": "/usr/local/bin/claudio",
			},
		},
	}

	settings := &SettingsMap{
		"hooks": map[string]any{
			"PreToolUse": []any{
				claudioItem,
				preExistingEmpty,
			},
		},
	}

	removeClaudioHooks(settings)

	hooksMap, ok := (*settings)["hooks"].(map[string]any)
	if !ok {
		t.Fatalf("hooks section disappeared or wrong type: %T", (*settings)["hooks"])
	}

	pre, ok := hooksMap["PreToolUse"].([]any)
	if !ok {
		t.Fatalf("PreToolUse disappeared or wrong type: %T", hooksMap["PreToolUse"])
	}

	if len(pre) != 1 {
		t.Fatalf("expected exactly 1 surviving item (the pre-existing empty one); got %d: %v", len(pre), pre)
	}

	surviving, ok := pre[0].(map[string]any)
	if !ok {
		t.Fatalf("surviving item wrong type: %T", pre[0])
	}

	if surviving["matcher"] != "user-block" {
		t.Errorf("surviving item lost its matcher: got %v, want %q", surviving["matcher"], "user-block")
	}

	hooksSub, ok := surviving["hooks"].([]any)
	if !ok {
		t.Fatalf("surviving item's hooks field is wrong type (expected empty []interface{}, got %T)", surviving["hooks"])
	}
	if len(hooksSub) != 0 {
		t.Errorf("surviving item's hooks should still be empty; got %v", hooksSub)
	}
}

// TestRemoveComplexClaudioHooksItemsWithoutClaudioUnchanged: a three-item
// array where only one item has a Claudio command. The other two — a custom
// non-Claudio item and a pre-existing empty item — must both survive verbatim.
func TestRemoveComplexClaudioHooksItemsWithoutClaudioUnchanged(t *testing.T) {
	claudioItem := map[string]any{
		"matcher": "claudio-target",
		"hooks": []any{
			map[string]any{
				"type":    "command",
				"command": "/usr/local/bin/claudio",
			},
		},
	}
	userNonClaudio := map[string]any{
		"matcher": "user-non-claudio",
		"hooks": []any{
			map[string]any{
				"type":    "command",
				"command": "/usr/local/bin/lint",
			},
		},
	}
	userEmpty := map[string]any{
		"matcher": "user-empty",
		"hooks":   []any{},
	}

	settings := &SettingsMap{
		"hooks": map[string]any{
			"PostToolUse": []any{
				claudioItem,
				userNonClaudio,
				userEmpty,
			},
		},
	}

	removeClaudioHooks(settings)

	hooksMap, _ := (*settings)["hooks"].(map[string]any)
	post, ok := hooksMap["PostToolUse"].([]any)
	if !ok {
		t.Fatalf("PostToolUse disappeared or wrong type: %T", hooksMap["PostToolUse"])
	}

	if len(post) != 2 {
		t.Fatalf("expected exactly 2 surviving items; got %d: %v", len(post), post)
	}

	if !reflect.DeepEqual(post[0], userNonClaudio) {
		t.Errorf("user-non-claudio item not byte-identical to input.\nGot:  %v\nWant: %v", post[0], userNonClaudio)
	}
	if !reflect.DeepEqual(post[1], userEmpty) {
		t.Errorf("user-empty item not byte-identical to input.\nGot:  %v\nWant: %v", post[1], userEmpty)
	}
}
