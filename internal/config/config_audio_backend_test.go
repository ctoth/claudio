package config

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// TestConfigAudioBackendField tests that AudioBackend field is properly handled
func TestConfigAudioBackendField(t *testing.T) {
	mgr := NewConfigManager()

	// Test default config has audio backend
	defaultConfig := mgr.GetDefaultConfig()
	if defaultConfig.AudioBackend == "" {
		t.Error("default config should have audio backend set")
	}
	if defaultConfig.AudioBackend != "auto" {
		t.Errorf("expected default audio backend 'auto', got '%s'", defaultConfig.AudioBackend)
	}
}

func TestConfigAudioBackendValidation(t *testing.T) {
	mgr := NewConfigManager()

	tests := []struct {
		name        string
		backend     string
		expectError bool
	}{
		{
			name:        "valid auto backend",
			backend:     "auto",
			expectError: false,
		},
		{
			name:        "valid system_command backend",
			backend:     "system_command",
			expectError: false,
		},
		{
			name:        "valid malgo backend",
			backend:     "malgo",
			expectError: false,
		},
		{
			name:        "empty backend defaults to auto",
			backend:     "",
			expectError: false,
		},
		{
			name:        "invalid backend type",
			backend:     "invalid",
			expectError: true,
		},
		{
			name:        "unsupported backend type",
			backend:     "pulseaudio",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := mgr.GetDefaultConfig()
			config.AudioBackend = tt.backend

			err := mgr.ValidateConfig(config)

			if tt.expectError && err == nil {
				t.Errorf("expected validation error for backend '%s' but got none", tt.backend)
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected validation error for backend '%s': %v", tt.backend, err)
			}
		})
	}
}

func TestConfigAudioBackendJSONSerialization(t *testing.T) {
	mgr := NewConfigManager()

	// Test JSON marshaling includes audio backend
	config := mgr.GetDefaultConfig()
	config.AudioBackend = "system_command"

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal config: %v", err)
	}

	jsonStr := string(data)
	if !containsJSONField(jsonStr, "audio_backend") {
		t.Error("JSON should contain audio_backend field")
	}
	if !containsJSONValue(jsonStr, "system_command") {
		t.Error("JSON should contain audio backend value")
	}

	t.Logf("Config JSON:\n%s", jsonStr)

	// Test JSON unmarshaling reads audio backend
	var unmarshaledConfig Config
	err = json.Unmarshal(data, &unmarshaledConfig)
	if err != nil {
		t.Fatalf("failed to unmarshal config: %v", err)
	}

	if unmarshaledConfig.AudioBackend != "system_command" {
		t.Errorf("expected audio backend 'system_command', got '%s'", unmarshaledConfig.AudioBackend)
	}
}

func TestConfigAudioBackendEnvironmentOverride(t *testing.T) {
	mgr := NewConfigManager()

	tests := []struct {
		name            string
		envValue        string
		expectedBackend string
	}{
		{
			name:            "auto backend via environment",
			envValue:        "auto",
			expectedBackend: "auto",
		},
		{
			name:            "system_command backend via environment",
			envValue:        "system_command",
			expectedBackend: "system_command",
		},
		{
			name:            "malgo backend via environment",
			envValue:        "malgo",
			expectedBackend: "malgo",
		},
		{
			name:            "empty environment keeps original",
			envValue:        "",
			expectedBackend: "auto", // Should keep default
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variable
			if tt.envValue != "" {
				os.Setenv("CLAUDIO_AUDIO_BACKEND", tt.envValue)
				defer os.Unsetenv("CLAUDIO_AUDIO_BACKEND")
			}

			config := mgr.GetDefaultConfig()
			result := mgr.ApplyEnvironmentOverrides(config)

			if result.AudioBackend != tt.expectedBackend {
				t.Errorf("expected audio backend '%s', got '%s'", tt.expectedBackend, result.AudioBackend)
			}
		})
	}
}

func TestConfigAudioBackendInvalidEnvironmentOverride(t *testing.T) {
	mgr := NewConfigManager()

	// Set invalid environment variable
	os.Setenv("CLAUDIO_AUDIO_BACKEND", "invalid")
	defer os.Unsetenv("CLAUDIO_AUDIO_BACKEND")

	config := mgr.GetDefaultConfig()
	result := mgr.ApplyEnvironmentOverrides(config)

	// Should keep original value when environment is invalid
	if result.AudioBackend != "auto" {
		t.Errorf("expected audio backend to remain 'auto' with invalid env, got '%s'", result.AudioBackend)
	}
}

func TestConfigAudioBackendMerging(t *testing.T) {
	mgr := NewConfigManager()

	baseConfig := mgr.GetDefaultConfig()
	baseConfig.AudioBackend = "auto"

	overrideConfig := mgr.GetDefaultConfig()
	overrideConfig.AudioBackend = "system_command"

	merged := mgr.MergeConfigs(baseConfig, overrideConfig)

	if merged.AudioBackend != "system_command" {
		t.Errorf("expected merged audio backend 'system_command', got '%s'", merged.AudioBackend)
	}

	// Test that empty override doesn't change base
	overrideConfig.AudioBackend = ""
	merged = mgr.MergeConfigs(baseConfig, overrideConfig)

	if merged.AudioBackend != "auto" {
		t.Errorf("expected audio backend to remain 'auto' with empty override, got '%s'", merged.AudioBackend)
	}
}

func TestGetSupportedAudioBackends(t *testing.T) {
	mgr := NewConfigManager()

	supported := mgr.GetSupportedAudioBackends()

	expectedBackends := []string{"auto", "system_command", "malgo"}
	if len(supported) != len(expectedBackends) {
		t.Errorf("expected %d supported backends, got %d", len(expectedBackends), len(supported))
	}

	for _, expected := range expectedBackends {
		found := false
		for _, actual := range supported {
			if actual == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected backend '%s' not found in supported list: %v", expected, supported)
		}
	}
}

func TestIsValidAudioBackend(t *testing.T) {
	mgr := NewConfigManager()

	validBackends := []string{"auto", "system_command", "malgo", ""}
	for _, backend := range validBackends {
		if !mgr.IsValidAudioBackend(backend) {
			t.Errorf("backend '%s' should be valid", backend)
		}
	}

	invalidBackends := []string{"invalid", "pulseaudio", "alsa", "unknown"}
	for _, backend := range invalidBackends {
		if mgr.IsValidAudioBackend(backend) {
			t.Errorf("backend '%s' should be invalid", backend)
		}
	}
}

// Helper functions for JSON testing
func containsJSONField(jsonStr, field string) bool {
	return strings.Contains(jsonStr, `"`+field+`"`)
}

func containsJSONValue(jsonStr, value string) bool {
	return strings.Contains(jsonStr, `"`+value+`"`)
}