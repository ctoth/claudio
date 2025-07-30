package uninstall

import (
	"testing"

	"github.com/ctoth/claudio/internal/install"
)

func TestDetectClaudioHooksWithFullPaths(t *testing.T) {
	// TDD RED: Test hook detection with full executable paths instead of just "claudio"
	testCases := []struct {
		name     string
		settings *install.SettingsMap
		expected []string
	}{
		{
			name: "simple string hook - full system path",
			settings: &install.SettingsMap{
				"hooks": map[string]interface{}{
					"PreToolUse": "/usr/local/bin/claudio",
				},
			},
			expected: []string{"PreToolUse"},
		},
		{
			name: "simple string hook - dev directory path",
			settings: &install.SettingsMap{
				"hooks": map[string]interface{}{
					"PostToolUse": "/home/user/dev/claudio/claudio",
				},
			},
			expected: []string{"PostToolUse"},
		},
		{
			name: "simple string hook - relative path",
			settings: &install.SettingsMap{
				"hooks": map[string]interface{}{
					"UserPromptSubmit": "./claudio",
				},
			},
			expected: []string{"UserPromptSubmit"},
		},
		{
			name: "complex array hook - full system path",
			settings: &install.SettingsMap{
				"hooks": map[string]interface{}{
					"Notification": []interface{}{
						map[string]interface{}{
							"matcher": ".*",
							"hooks": []interface{}{
								map[string]interface{}{
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
			name: "complex array hook - dev directory path",
			settings: &install.SettingsMap{
				"hooks": map[string]interface{}{
					"SessionStart": []interface{}{
						map[string]interface{}{
							"matcher": ".*",
							"hooks": []interface{}{
								map[string]interface{}{
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
			settings: &install.SettingsMap{
				"hooks": map[string]interface{}{
					"PreToolUse":  "claudio",                      // Old format
					"PostToolUse": "/usr/local/bin/claudio",       // Full path
					"Stop":        "./claudio",                    // Relative path  
					"Other":       "/usr/bin/different-command",   // Non-claudio
				},
			},
			expected: []string{"PreToolUse", "PostToolUse", "Stop"},
		},
		{
			name: "test executable path (install.test)",
			settings: &install.SettingsMap{
				"hooks": map[string]interface{}{
					"PreToolUse": "/tmp/install.test",
				},
			},
			expected: []string{"PreToolUse"},
		},
		{
			name: "no claudio paths - different executables",
			settings: &install.SettingsMap{
				"hooks": map[string]interface{}{
					"PreToolUse":  "/usr/bin/git",
					"PostToolUse": "/bin/echo",
				},
			},
			expected: []string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := detectClaudioHooks(tc.settings)
			
			// Check length
			if len(result) != len(tc.expected) {
				t.Errorf("Expected %d claudio hooks, got %d: %v", 
					len(tc.expected), len(result), result)
				return
			}
			
			// Check each expected hook is present
			for _, expectedHook := range tc.expected {
				found := false
				for _, actualHook := range result {
					if actualHook == expectedHook {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected hook '%s' not found in result: %v", 
						expectedHook, result)
				}
			}
			
			t.Logf("Full path detection test passed for %s: found %v", tc.name, result)
		})
	}
}

func TestDetectClaudioHooks(t *testing.T) {
	// TDD RED: Test hook detection for both simple and complex formats
	testCases := []struct {
		name     string
		settings *install.SettingsMap
		expected []string
	}{
		{
			name:     "nil settings",
			settings: nil,
			expected: []string{},
		},
		{
			name: "simple string hook - claudio",
			settings: &install.SettingsMap{
				"hooks": map[string]interface{}{
					"PreToolUse": "claudio",
				},
			},
			expected: []string{"PreToolUse"},
		},
		{
			name: "complex array hook - claudio command",
			settings: &install.SettingsMap{
				"hooks": map[string]interface{}{
					"Notification": []interface{}{
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
			expected: []string{"Notification"},
		},
		{
			name: "mixed hooks - claudio and non-claudio",
			settings: &install.SettingsMap{
				"hooks": map[string]interface{}{
					"PreToolUse":  "claudio",
					"PostToolUse": "other-command",
				},
			},
			expected: []string{"PreToolUse"},
		},
		{
			name: "no claudio hooks",
			settings: &install.SettingsMap{
				"hooks": map[string]interface{}{
					"Other": "something",
				},
			},
			expected: []string{},
		},
		{
			name: "no hooks section",
			settings: &install.SettingsMap{
				"version": "1.0",
			},
			expected: []string{},
		},
		{
			name: "multiple claudio hooks",
			settings: &install.SettingsMap{
				"hooks": map[string]interface{}{
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
			result := detectClaudioHooks(tc.settings)
			
			// Check length
			if len(result) != len(tc.expected) {
				t.Errorf("Expected %d claudio hooks, got %d: %v", 
					len(tc.expected), len(result), result)
				return
			}
			
			// Check each expected hook is present
			for _, expectedHook := range tc.expected {
				found := false
				for _, actualHook := range result {
					if actualHook == expectedHook {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected hook '%s' not found in result: %v", 
						expectedHook, result)
				}
			}
			
			t.Logf("Hook detection test passed for %s: found %v", tc.name, result)
		})
	}
}