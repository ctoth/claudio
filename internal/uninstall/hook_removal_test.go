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