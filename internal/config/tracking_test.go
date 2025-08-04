package config

import (
	"os"
	"testing"
)

func TestSoundTrackingConfig_DefaultValues(t *testing.T) {
	config := GetDefaultSoundTrackingConfig()

	if !config.Enabled {
		t.Errorf("Expected default sound tracking to be enabled, got %v", config.Enabled)
	}

	if config.DatabasePath != "" {
		t.Errorf("Expected default database path to be empty (XDG cache), got %s", config.DatabasePath)
	}
}

func TestApplySoundTrackingEnvironmentOverrides_EnabledTrue(t *testing.T) {
	os.Setenv("CLAUDIO_SOUND_TRACKING", "true")
	defer os.Unsetenv("CLAUDIO_SOUND_TRACKING")

	config := &SoundTrackingConfig{
		Enabled:      false,
		DatabasePath: "",
	}

	result := ApplySoundTrackingEnvironmentOverrides(config)

	if !result.Enabled {
		t.Errorf("Expected tracking to be enabled, got %v", result.Enabled)
	}
}

func TestApplySoundTrackingEnvironmentOverrides_EnabledFalse(t *testing.T) {
	os.Setenv("CLAUDIO_SOUND_TRACKING", "false")
	defer os.Unsetenv("CLAUDIO_SOUND_TRACKING")

	config := &SoundTrackingConfig{
		Enabled:      true,
		DatabasePath: "",
	}

	result := ApplySoundTrackingEnvironmentOverrides(config)

	if result.Enabled {
		t.Errorf("Expected tracking to be disabled, got %v", result.Enabled)
	}
}

func TestApplySoundTrackingEnvironmentOverrides_Enabled1(t *testing.T) {
	os.Setenv("CLAUDIO_SOUND_TRACKING", "1")
	defer os.Unsetenv("CLAUDIO_SOUND_TRACKING")

	config := &SoundTrackingConfig{
		Enabled:      false,
		DatabasePath: "",
	}

	result := ApplySoundTrackingEnvironmentOverrides(config)

	if !result.Enabled {
		t.Errorf("Expected tracking to be enabled with '1', got %v", result.Enabled)
	}
}

func TestApplySoundTrackingEnvironmentOverrides_Enabled0(t *testing.T) {
	os.Setenv("CLAUDIO_SOUND_TRACKING", "0")
	defer os.Unsetenv("CLAUDIO_SOUND_TRACKING")

	config := &SoundTrackingConfig{
		Enabled:      true,
		DatabasePath: "",
	}

	result := ApplySoundTrackingEnvironmentOverrides(config)

	if result.Enabled {
		t.Errorf("Expected tracking to be disabled with '0', got %v", result.Enabled)
	}
}

func TestApplySoundTrackingEnvironmentOverrides_InvalidValue(t *testing.T) {
	os.Setenv("CLAUDIO_SOUND_TRACKING", "maybe")
	defer os.Unsetenv("CLAUDIO_SOUND_TRACKING")

	config := &SoundTrackingConfig{
		Enabled:      true,
		DatabasePath: "",
	}

	result := ApplySoundTrackingEnvironmentOverrides(config)

	// Should remain unchanged for invalid values
	if !result.Enabled {
		t.Errorf("Expected tracking to remain enabled with invalid value, got %v", result.Enabled)
	}
}

func TestApplySoundTrackingEnvironmentOverrides_EmptyValue(t *testing.T) {
	os.Setenv("CLAUDIO_SOUND_TRACKING", "")
	defer os.Unsetenv("CLAUDIO_SOUND_TRACKING")

	config := &SoundTrackingConfig{
		Enabled:      true,
		DatabasePath: "",
	}

	result := ApplySoundTrackingEnvironmentOverrides(config)

	// Should remain unchanged for empty values
	if !result.Enabled {
		t.Errorf("Expected tracking to remain enabled with empty value, got %v", result.Enabled)
	}
}

func TestApplySoundTrackingEnvironmentOverrides_NoEnvironmentVariable(t *testing.T) {
	// Ensure the environment variable is not set
	os.Unsetenv("CLAUDIO_SOUND_TRACKING")

	config := &SoundTrackingConfig{
		Enabled:      true,
		DatabasePath: "/custom/path.db",
	}

	result := ApplySoundTrackingEnvironmentOverrides(config)

	// Should remain unchanged when no environment variable is set
	if !result.Enabled {
		t.Errorf("Expected tracking to remain enabled without env var, got %v", result.Enabled)
	}

	if result.DatabasePath != "/custom/path.db" {
		t.Errorf("Expected database path to remain unchanged, got %s", result.DatabasePath)
	}
}

func TestConfigWithSoundTracking_Integration(t *testing.T) {
	// Create a config manager and get default config
	cm := NewConfigManager()
	config := cm.GetDefaultConfig()

	// Verify that the main Config struct has SoundTracking field
	if config.SoundTracking == nil {
		t.Error("Expected main Config to have SoundTracking field")
	}

	if !config.SoundTracking.Enabled {
		t.Errorf("Expected default sound tracking to be enabled, got %v", config.SoundTracking.Enabled)
	}
}

func TestApplyEnvironmentOverrides_IncludesSoundTracking(t *testing.T) {
	os.Setenv("CLAUDIO_SOUND_TRACKING", "false")
	defer os.Unsetenv("CLAUDIO_SOUND_TRACKING")

	cm := NewConfigManager()
	config := cm.GetDefaultConfig()

	// Apply environment overrides
	result := cm.ApplyEnvironmentOverrides(config)

	// Verify that sound tracking was affected by environment override
	if result.SoundTracking.Enabled {
		t.Errorf("Expected sound tracking to be disabled by environment override, got %v", result.SoundTracking.Enabled)
	}
}