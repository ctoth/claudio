package uninstall

import (
	"testing"

	"github.com/ctoth/claudio/internal/install"
)

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