package util

import (
	"testing"

	"claudio.click/internal/install"
)

func TestGetSettingsKeys(t *testing.T) {
	tests := []struct {
		name     string
		settings *install.SettingsMap
		want     []string
	}{
		{
			name:     "nil settings",
			settings: nil,
			want:     []string{},
		},
		{
			name:     "empty settings",
			settings: &install.SettingsMap{},
			want:     []string{},
		},
		{
			name: "single key",
			settings: &install.SettingsMap{
				"hooks": map[string]interface{}{},
			},
			want: []string{"hooks"},
		},
		{
			name: "multiple keys",
			settings: &install.SettingsMap{
				"hooks":   map[string]interface{}{},
				"version": "1.0",
				"other":   "value",
			},
			want: []string{"hooks", "version", "other"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetSettingsKeys(tt.settings)

			// Sort both slices for comparison since map iteration order is not guaranteed
			if len(got) != len(tt.want) {
				t.Errorf("GetSettingsKeys() length = %v, want %v", len(got), len(tt.want))
				return
			}

			// Check that all expected keys are present
			gotMap := make(map[string]bool)
			for _, key := range got {
				gotMap[key] = true
			}

			for _, wantKey := range tt.want {
				if !gotMap[wantKey] {
					t.Errorf("GetSettingsKeys() missing key %v, got %v", wantKey, got)
				}
			}

			// Check that no extra keys are present
			wantMap := make(map[string]bool)
			for _, key := range tt.want {
				wantMap[key] = true
			}

			for _, gotKey := range got {
				if !wantMap[gotKey] {
					t.Errorf("GetSettingsKeys() extra key %v, got %v", gotKey, got)
				}
			}
		})
	}
}

func TestGetSettingsKeysWithComplexValues(t *testing.T) {
	settings := &install.SettingsMap{
		"hooks": map[string]interface{}{
			"PostToolUse": "claudio",
			"PreToolUse": []interface{}{
				map[string]interface{}{
					"command": "claudio",
					"type":    "command",
				},
			},
		},
		"version": "2.0",
		"nested": map[string]interface{}{
			"deep": map[string]interface{}{
				"value": "test",
			},
		},
	}

	got := GetSettingsKeys(settings)
	
	if len(got) != 3 {
		t.Errorf("Expected 3 keys, got %d: %v", len(got), got)
	}

	expectedKeys := []string{"hooks", "version", "nested"}
	gotMap := make(map[string]bool)
	for _, key := range got {
		gotMap[key] = true
	}

	for _, expected := range expectedKeys {
		if !gotMap[expected] {
			t.Errorf("Expected key %s not found in result: %v", expected, got)
		}
	}
}