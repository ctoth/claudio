package uninstall

import (
	"testing"

	"github.com/ctoth/claudio/internal/install"
)

func TestRemoveSimpleClaudioHooks(t *testing.T) {
	// TDD RED: Test removal of simple string claudio hooks
	testCases := []struct {
		name           string
		initialSettings *install.SettingsMap
		hookNames      []string
		expectedSettings *install.SettingsMap
		expectError    bool
	}{
		{
			name: "remove single claudio hook with other hooks preserved",
			initialSettings: &install.SettingsMap{
				"hooks": map[string]interface{}{
					"PreToolUse":  "claudio",
					"PostCommit": "git push",
					"Other":      "keep-this",
				},
				"version": "1.0",
			},
			hookNames: []string{"PreToolUse"},
			expectedSettings: &install.SettingsMap{
				"hooks": map[string]interface{}{
					"PostCommit": "git push",
					"Other":      "keep-this",
				},
				"version": "1.0",
			},
			expectError: false,
		},
		{
			name: "remove multiple claudio hooks",
			initialSettings: &install.SettingsMap{
				"hooks": map[string]interface{}{
					"PreToolUse":       "claudio",
					"PostToolUse":      "claudio",
					"UserPromptSubmit": "claudio",
					"Other":            "keep",
				},
			},
			hookNames: []string{"PreToolUse", "PostToolUse", "UserPromptSubmit"},
			expectedSettings: &install.SettingsMap{
				"hooks": map[string]interface{}{
					"Other": "keep",
				},
			},
			expectError: false,
		},
		{
			name: "remove non-existent hook - no changes",
			initialSettings: &install.SettingsMap{
				"hooks": map[string]interface{}{
					"Other": "keep",
				},
			},
			hookNames: []string{"NonExistent"},
			expectedSettings: &install.SettingsMap{
				"hooks": map[string]interface{}{
					"Other": "keep",
				},
			},
			expectError: false,
		},
		{
			name: "remove from empty settings - no changes",
			initialSettings: &install.SettingsMap{
				"version": "1.0",
			},
			hookNames: []string{"PreToolUse"},
			expectedSettings: &install.SettingsMap{
				"version": "1.0",
			},
			expectError: false,
		},
		{
			name: "remove all hooks - hooks section deleted",
			initialSettings: &install.SettingsMap{
				"hooks": map[string]interface{}{
					"PreToolUse": "claudio",
				},
				"version": "1.0",
			},
			hookNames: []string{"PreToolUse"},
			expectedSettings: &install.SettingsMap{
				"version": "1.0",
			},
			expectError: false,
		},
		{
			name: "nil settings - no error",
			initialSettings: nil,
			hookNames: []string{"PreToolUse"},
			expectedSettings: nil,
			expectError: false,
		},
		{
			name: "remove hook that exists but is not claudio - no changes",
			initialSettings: &install.SettingsMap{
				"hooks": map[string]interface{}{
					"PreToolUse": "other-command",
					"Other":      "keep",
				},
			},
			hookNames: []string{"PreToolUse"},
			expectedSettings: &install.SettingsMap{
				"hooks": map[string]interface{}{
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
			var settingsCopy *install.SettingsMap
			if tc.initialSettings != nil {
				settingsCopy = deepCopySettings(tc.initialSettings)
			}

			// Call the function
			removeSimpleClaudioHooks(settingsCopy, tc.hookNames)

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
func deepCopySettings(original *install.SettingsMap) *install.SettingsMap {
	if original == nil {
		return nil
	}
	
	copy := make(install.SettingsMap)
	for key, value := range *original {
		if key == "hooks" {
			if hooksMap, ok := value.(map[string]interface{}); ok {
				hooksCopy := make(map[string]interface{})
				for hookKey, hookValue := range hooksMap {
					hooksCopy[hookKey] = hookValue
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

// Helper function to compare settings maps
func settingsEqual(a, b *install.SettingsMap) bool {
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
			hooksA, okA := valueA.(map[string]interface{})
			hooksB, okB := valueB.(map[string]interface{})
			if okA != okB {
				return false
			}
			if okA && okB {
				if len(hooksA) != len(hooksB) {
					return false
				}
				for hookKey, hookValueA := range hooksA {
					hookValueB, exists := hooksB[hookKey]
					if !exists || hookValueA != hookValueB {
						return false
					}
				}
			}
		} else {
			if valueA != valueB {
				return false
			}
		}
	}
	
	return true
}

func TestRemoveComplexClaudioHooks(t *testing.T) {
	// TDD RED: Test removal of complex array claudio hooks
	testCases := []struct {
		name           string
		initialSettings *install.SettingsMap
		hookNames      []string
		expectedSettings *install.SettingsMap
		expectError    bool
	}{
		{
			name: "remove claudio from array with other commands - preserve others",
			initialSettings: &install.SettingsMap{
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
				},
			},
			hookNames: []string{"Notification"},
			expectedSettings: &install.SettingsMap{
				"hooks": map[string]interface{}{
					"Notification": []interface{}{
						map[string]interface{}{
							"hooks": []interface{}{
								map[string]interface{}{
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
			initialSettings: &install.SettingsMap{
				"hooks": map[string]interface{}{
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
					"Other": "keep-this",
				},
			},
			hookNames: []string{"Stop"},
			expectedSettings: &install.SettingsMap{
				"hooks": map[string]interface{}{
					"Other": "keep-this",
				},
			},
			expectError: false,
		},
		{
			name: "remove claudio from multiple array elements",
			initialSettings: &install.SettingsMap{
				"hooks": map[string]interface{}{
					"PreCompact": []interface{}{
						map[string]interface{}{
							"matcher": ".*",
							"hooks": []interface{}{
								map[string]interface{}{
									"command": "claudio",
									"type":    "command",
								},
								map[string]interface{}{
									"command": "keep-this",
									"type":    "command",
								},
							},
						},
						map[string]interface{}{
							"matcher": "specific",
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
			hookNames: []string{"PreCompact"},
			expectedSettings: &install.SettingsMap{
				"hooks": map[string]interface{}{
					"PreCompact": []interface{}{
						map[string]interface{}{
							"matcher": ".*",
							"hooks": []interface{}{
								map[string]interface{}{
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
			initialSettings: &install.SettingsMap{
				"hooks": map[string]interface{}{
					"Simple": "not-an-array",
				},
			},
			hookNames: []string{"Simple"},
			expectedSettings: &install.SettingsMap{
				"hooks": map[string]interface{}{
					"Simple": "not-an-array",
				},
			},
			expectError: false,
		},
		{
			name: "remove from array without claudio - no changes",
			initialSettings: &install.SettingsMap{
				"hooks": map[string]interface{}{
					"Other": []interface{}{
						map[string]interface{}{
							"hooks": []interface{}{
								map[string]interface{}{
									"command": "different-tool",
									"type":    "command",
								},
							},
						},
					},
				},
			},
			hookNames: []string{"Other"},
			expectedSettings: &install.SettingsMap{
				"hooks": map[string]interface{}{
					"Other": []interface{}{
						map[string]interface{}{
							"hooks": []interface{}{
								map[string]interface{}{
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
			var settingsCopy *install.SettingsMap
			if tc.initialSettings != nil {
				settingsCopy = deepCopyComplexSettings(tc.initialSettings)
			}

			// Call the function
			removeComplexClaudioHooks(settingsCopy, tc.hookNames)

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
func deepCopyComplexSettings(original *install.SettingsMap) *install.SettingsMap {
	if original == nil {
		return nil
	}
	
	copy := make(install.SettingsMap)
	for key, value := range *original {
		if key == "hooks" {
			if hooksMap, ok := value.(map[string]interface{}); ok {
				hooksCopy := make(map[string]interface{})
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
func deepCopyValue(value interface{}) interface{} {
	switch v := value.(type) {
	case []interface{}:
		copiedSlice := make([]interface{}, len(v))
		for i, item := range v {
			copiedSlice[i] = deepCopyValue(item)
		}
		return copiedSlice
	case map[string]interface{}:
		copiedMap := make(map[string]interface{})
		for k, item := range v {
			copiedMap[k] = deepCopyValue(item)
		}
		return copiedMap
	default:
		return v
	}
}

// Helper function to compare complex settings with arrays
func complexSettingsEqual(a, b *install.SettingsMap) bool {
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
func deepValueEqual(a, b interface{}) bool {
	switch va := a.(type) {
	case []interface{}:
		vb, ok := b.([]interface{})
		if !ok || len(va) != len(vb) {
			return false
		}
		for i := range va {
			if !deepValueEqual(va[i], vb[i]) {
				return false
			}
		}
		return true
	case map[string]interface{}:
		vb, ok := b.(map[string]interface{})
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
		initialSettings  *install.SettingsMap
		hookNames        []string
		expectedSettings *install.SettingsMap
	}{
		{
			name: "remove new format claudio hook",
			initialSettings: &install.SettingsMap{
				"hooks": map[string]interface{}{
					"PreToolUse": []interface{}{
						map[string]interface{}{
							"hooks": []interface{}{
								map[string]interface{}{
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
			expectedSettings: &install.SettingsMap{
				"hooks": map[string]interface{}{
					"PostToolUse": "other-command",
				},
			},
		},
		{
			name: "remove multiple new format hooks",
			initialSettings: &install.SettingsMap{
				"hooks": map[string]interface{}{
					"PreToolUse": []interface{}{
						map[string]interface{}{
							"hooks": []interface{}{
								map[string]interface{}{
									"type":    "command",
									"command": "claudio",
								},
							},
						},
					},
					"PostToolUse": []interface{}{
						map[string]interface{}{
							"hooks": []interface{}{
								map[string]interface{}{
									"type":    "command",
									"command": "claudio",
								},
							},
						},
					},
					"UserPromptSubmit": []interface{}{
						map[string]interface{}{
							"hooks": []interface{}{
								map[string]interface{}{
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
			expectedSettings: &install.SettingsMap{
				"version": "1.0",
			},
		},
		{
			name: "handle mixed old and new format hooks",
			initialSettings: &install.SettingsMap{
				"hooks": map[string]interface{}{
					"PreToolUse": "claudio", // Old string format
					"PostToolUse": []interface{}{ // New array format
						map[string]interface{}{
							"hooks": []interface{}{
								map[string]interface{}{
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
			expectedSettings: &install.SettingsMap{
				"hooks": map[string]interface{}{
					"UserPromptSubmit": "other-command",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Make a copy to avoid modifying the original
			settingsCopy := deepCopySettings(tc.initialSettings)

			// The current implementation should handle both formats
			removeSimpleClaudioHooks(settingsCopy, tc.hookNames)

			// Verify the result
			if !settingsEqual(settingsCopy, tc.expectedSettings) {
				t.Errorf("Settings mismatch.\nExpected: %+v\nActual:   %+v",
					tc.expectedSettings, settingsCopy)
			}

			t.Logf("New format hook removal test passed for %s", tc.name)
		})
	}
}