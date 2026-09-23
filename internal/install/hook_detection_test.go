package install

import (
	"slices"
	"testing"
)

func TestClaudioHookNamesWithFullPaths(t *testing.T) {
	// TDD RED: Test hook detection with full executable paths instead of just "claudio"
	testCases := []struct {
		name     string
		settings *SettingsMap
		expected []string
	}{
		{
			name: "simple string hook - full system path",
			settings: &SettingsMap{
				"hooks": map[string]any{
					"PreToolUse": "/usr/local/bin/claudio",
				},
			},
			expected: []string{"PreToolUse"},
		},
		{
			name: "simple string hook - dev directory path",
			settings: &SettingsMap{
				"hooks": map[string]any{
					"PostToolUse": "/home/user/dev/claudio/claudio",
				},
			},
			expected: []string{"PostToolUse"},
		},
		{
			name: "simple string hook - relative path",
			settings: &SettingsMap{
				"hooks": map[string]any{
					"UserPromptSubmit": "./claudio",
				},
			},
			expected: []string{"UserPromptSubmit"},
		},
		{
			name: "complex array hook - full system path",
			settings: &SettingsMap{
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
				},
			},
			expected: []string{"Notification"},
		},
		{
			name: "direct Copilot command hook",
			settings: &SettingsMap{
				"hooks": map[string]any{
					"PreToolUse": []any{
						map[string]any{
							"command": "/usr/local/bin/claudio --hook-agent copilot",
							"type":    "command",
						},
					},
				},
			},
			expected: []string{"PreToolUse"},
		},
		{
			name: "complex array hook - dev directory path",
			settings: &SettingsMap{
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
			expected: []string{"SessionStart"},
		},
		{
			name: "mixed full paths and backward compatibility",
			settings: &SettingsMap{
				"hooks": map[string]any{
					"PreToolUse":  "claudio",                    // Old format
					"PostToolUse": "/usr/local/bin/claudio",     // Full path
					"Stop":        "./claudio",                  // Relative path
					"Other":       "/usr/bin/different-command", // Non-claudio
				},
			},
			expected: []string{"PreToolUse", "PostToolUse", "Stop"},
		},
		{
			name: "no claudio paths - different executables",
			settings: &SettingsMap{
				"hooks": map[string]any{
					"PreToolUse":  "/usr/bin/git",
					"PostToolUse": "/bin/echo",
				},
			},
			expected: []string{},
		},
		{
			name: "user binary ending in .test is NOT a Claudio command",
			settings: &SettingsMap{
				"hooks": map[string]any{
					"PreToolUse": "/usr/local/bin/lint.test",
				},
			},
			expected: []string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ClaudioHookNames(tc.settings)

			// Check length
			if len(result) != len(tc.expected) {
				t.Errorf("Expected %d claudio hooks, got %d: %v",
					len(tc.expected), len(result), result)
				return
			}

			// Check each expected hook is present
			for _, expectedHook := range tc.expected {
				found := slices.Contains(result, expectedHook)
				if !found {
					t.Errorf("Expected hook '%s' not found in result: %v",
						expectedHook, result)
				}
			}

			t.Logf("Full path detection test passed for %s: found %v", tc.name, result)
		})
	}
}

func TestClaudioHookNames(t *testing.T) {
	// TDD RED: Test hook detection for both simple and complex formats
	testCases := []struct {
		name     string
		settings *SettingsMap
		expected []string
	}{
		{
			name:     "nil settings",
			settings: nil,
			expected: []string{},
		},
		{
			name: "simple string hook - claudio",
			settings: &SettingsMap{
				"hooks": map[string]any{
					"PreToolUse": "claudio",
				},
			},
			expected: []string{"PreToolUse"},
		},
		{
			name: "complex array hook - claudio command",
			settings: &SettingsMap{
				"hooks": map[string]any{
					"Notification": []any{
						map[string]any{
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
			expected: []string{"Notification"},
		},
		{
			name: "mixed hooks - claudio and non-claudio",
			settings: &SettingsMap{
				"hooks": map[string]any{
					"PreToolUse":  "claudio",
					"PostToolUse": "other-command",
				},
			},
			expected: []string{"PreToolUse"},
		},
		{
			name: "no claudio hooks",
			settings: &SettingsMap{
				"hooks": map[string]any{
					"Other": "something",
				},
			},
			expected: []string{},
		},
		{
			name: "no hooks section",
			settings: &SettingsMap{
				"version": "1.0",
			},
			expected: []string{},
		},
		{
			name: "multiple claudio hooks",
			settings: &SettingsMap{
				"hooks": map[string]any{
					"PreToolUse":       "claudio",
					"PostToolUse":      "claudio",
					"UserPromptSubmit": "claudio",
					"Other":            "different",
				},
			},
			expected: []string{"PreToolUse", "PostToolUse", "UserPromptSubmit"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ClaudioHookNames(tc.settings)

			// Check length
			if len(result) != len(tc.expected) {
				t.Errorf("Expected %d claudio hooks, got %d: %v",
					len(tc.expected), len(result), result)
				return
			}

			// Check each expected hook is present
			for _, expectedHook := range tc.expected {
				found := slices.Contains(result, expectedHook)
				if !found {
					t.Errorf("Expected hook '%s' not found in result: %v",
						expectedHook, result)
				}
			}

			t.Logf("Hook detection test passed for %s: found %v", tc.name, result)
		})
	}
}
