package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigManager(t *testing.T) {
	mgr := NewConfigManager()

	if mgr == nil {
		t.Fatal("NewConfigManager returned nil")
	}
}

func TestLoadDefaultConfig(t *testing.T) {
	mgr := NewConfigManager()

	config := mgr.GetDefaultConfig()

	// Verify default values
	if config.Volume < 0.0 || config.Volume > 1.0 {
		t.Errorf("Default volume %f should be between 0.0 and 1.0", config.Volume)
	}

	if config.DefaultSoundpack == "" {
		t.Error("Default soundpack should not be empty")
	}

	// Note: SoundpackPaths can be empty in default config since XDG paths are used

	if !config.Enabled {
		t.Error("Default should be enabled")
	}

	t.Logf("Default config: %+v", config)
}

func TestLoadConfigFromFile(t *testing.T) {
	mgr := NewConfigManager()

	// Create a temporary config file
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "test-config.json")

	testConfig := &Config{
		Volume:          0.75,
		DefaultSoundpack: "mechanical",
		SoundpackPaths:  []string{"/custom/path"},
		Enabled:         false,
		LogLevel:        "warn",
	}

	// Write test config to file
	data, err := json.MarshalIndent(testConfig, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal test config: %v", err)
	}

	err = os.WriteFile(configFile, data, 0644)
	if err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	// Load config from file
	loadedConfig, err := mgr.LoadFromFile(configFile)
	if err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}

	// Verify loaded config matches
	if loadedConfig.Volume != testConfig.Volume {
		t.Errorf("Volume = %f, expected %f", loadedConfig.Volume, testConfig.Volume)
	}

	if loadedConfig.DefaultSoundpack != testConfig.DefaultSoundpack {
		t.Errorf("DefaultSoundpack = %s, expected %s", loadedConfig.DefaultSoundpack, testConfig.DefaultSoundpack)
	}

	if loadedConfig.Enabled != testConfig.Enabled {
		t.Errorf("Enabled = %v, expected %v", loadedConfig.Enabled, testConfig.Enabled)
	}

	if loadedConfig.LogLevel != testConfig.LogLevel {
		t.Errorf("LogLevel = %s, expected %s", loadedConfig.LogLevel, testConfig.LogLevel)
	}
}

func TestLoadConfigWithValidation(t *testing.T) {
	mgr := NewConfigManager()

	testCases := []struct {
		name        string
		config      *Config
		shouldError bool
		errorMsg    string
	}{
		{
			name: "valid config",
			config: &Config{
				Volume:          0.5,
				DefaultSoundpack: "default",
				SoundpackPaths:  []string{"/valid/path"},
				Enabled:         true,
				LogLevel:        "info",
			},
			shouldError: false,
		},
		{
			name: "volume too high",
			config: &Config{
				Volume:          1.5,
				DefaultSoundpack: "default",
				Enabled:         true,
			},
			shouldError: true,
			errorMsg:    "volume",
		},
		{
			name: "volume too low",
			config: &Config{
				Volume:          -0.1,
				DefaultSoundpack: "default",
				Enabled:         true,
			},
			shouldError: true,
			errorMsg:    "volume",
		},
		{
			name: "empty soundpack",
			config: &Config{
				Volume:          0.5,
				DefaultSoundpack: "",
				Enabled:         true,
			},
			shouldError: true,
			errorMsg:    "soundpack",
		},
		{
			name: "invalid log level",
			config: &Config{
				Volume:          0.5,
				DefaultSoundpack: "default",
				Enabled:         true,
				LogLevel:        "invalid",
			},
			shouldError: true,
			errorMsg:    "log level",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := mgr.ValidateConfig(tc.config)

			if tc.shouldError {
				if err == nil {
					t.Error("Expected validation error but got none")
				} else if tc.errorMsg != "" && !contains(err.Error(), tc.errorMsg) {
					t.Errorf("Expected error containing %q, got: %v", tc.errorMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected validation error: %v", err)
				}
			}
		})
	}
}

func TestSaveConfig(t *testing.T) {
	mgr := NewConfigManager()

	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "save-test.json")

	testConfig := &Config{
		Volume:          0.8,
		DefaultSoundpack: "test-pack",
		SoundpackPaths:  []string{"/test/path1", "/test/path2"},
		Enabled:         true,
		LogLevel:        "debug",
	}

	// Save config
	err := mgr.SaveToFile(testConfig, configFile)
	if err != nil {
		t.Fatalf("SaveToFile failed: %v", err)
	}

	// Verify file exists and is readable
	data, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatalf("Failed to read saved config file: %v", err)
	}

	// Parse saved file
	var savedConfig Config
	err = json.Unmarshal(data, &savedConfig)
	if err != nil {
		t.Fatalf("Failed to parse saved config: %v", err)
	}

	// Verify content matches
	if savedConfig.Volume != testConfig.Volume {
		t.Errorf("Saved volume = %f, expected %f", savedConfig.Volume, testConfig.Volume)
	}

	if savedConfig.DefaultSoundpack != testConfig.DefaultSoundpack {
		t.Errorf("Saved soundpack = %s, expected %s", savedConfig.DefaultSoundpack, testConfig.DefaultSoundpack)
	}

	// Verify file is properly formatted JSON
	if !json.Valid(data) {
		t.Error("Saved file is not valid JSON")
	}

	t.Logf("Saved config content: %s", string(data))
}

func TestAutoDiscoverConfig(t *testing.T) {
	mgr := NewConfigManager()

	// This should use XDG paths to find config
	config, err := mgr.LoadConfig()

	// Should not error even if no config file exists (uses defaults)
	if err != nil {
		t.Errorf("LoadConfig should not error when no config exists: %v", err)
	}

	if config == nil {
		t.Error("LoadConfig returned nil config")
	}

	// Should have reasonable defaults
	if config.Volume < 0 || config.Volume > 1 {
		t.Errorf("Auto-discovered config has invalid volume: %f", config.Volume)
	}

	t.Logf("Auto-discovered config: %+v", config)
}

func TestConfigMerging(t *testing.T) {
	mgr := NewConfigManager()

	baseConfig := &Config{
		Volume:          0.5,
		DefaultSoundpack: "base",
		SoundpackPaths:  []string{"/base/path"},
		Enabled:         true,
		LogLevel:        "info",
	}

	overrideConfig := &Config{
		Volume:          0.8,
		DefaultSoundpack: "override",
		// SoundpackPaths intentionally omitted
		// Enabled intentionally omitted
		LogLevel: "debug",
	}

	merged := mgr.MergeConfigs(baseConfig, overrideConfig)

	// Overridden values
	if merged.Volume != 0.8 {
		t.Errorf("Merged volume = %f, expected 0.8", merged.Volume)
	}

	if merged.DefaultSoundpack != "override" {
		t.Errorf("Merged soundpack = %s, expected 'override'", merged.DefaultSoundpack)
	}

	if merged.LogLevel != "debug" {
		t.Errorf("Merged log level = %s, expected 'debug'", merged.LogLevel)
	}

	// Base values preserved
	if merged.Enabled != true {
		t.Errorf("Merged enabled = %v, expected true", merged.Enabled)
	}

	if len(merged.SoundpackPaths) != 1 || merged.SoundpackPaths[0] != "/base/path" {
		t.Errorf("Merged soundpack paths = %v, expected ['/base/path']", merged.SoundpackPaths)
	}
}

func TestConfigEnvironmentOverrides(t *testing.T) {
	mgr := NewConfigManager()

	// Set environment variables
	os.Setenv("CLAUDIO_VOLUME", "0.9")
	os.Setenv("CLAUDIO_SOUNDPACK", "env-pack")
	os.Setenv("CLAUDIO_ENABLED", "false")
	defer func() {
		os.Unsetenv("CLAUDIO_VOLUME")
		os.Unsetenv("CLAUDIO_SOUNDPACK")
		os.Unsetenv("CLAUDIO_ENABLED")
	}()

	baseConfig := &Config{
		Volume:          0.5,
		DefaultSoundpack: "base",
		Enabled:         true,
		LogLevel:        "info",
	}

	finalConfig := mgr.ApplyEnvironmentOverrides(baseConfig)

	// Environment overrides should take effect
	if finalConfig.Volume != 0.9 {
		t.Errorf("Volume = %f, expected 0.9 from env", finalConfig.Volume)
	}

	if finalConfig.DefaultSoundpack != "env-pack" {
		t.Errorf("Soundpack = %s, expected 'env-pack' from env", finalConfig.DefaultSoundpack)
	}

	if finalConfig.Enabled != false {
		t.Errorf("Enabled = %v, expected false from env", finalConfig.Enabled)
	}

	// Non-overridden values should remain
	if finalConfig.LogLevel != "info" {
		t.Errorf("LogLevel = %s, expected 'info' (unchanged)", finalConfig.LogLevel)
	}
}

func TestConfigErrorHandling(t *testing.T) {
	mgr := NewConfigManager()

	t.Run("invalid JSON file", func(t *testing.T) {
		tempDir := t.TempDir()
		configFile := filepath.Join(tempDir, "invalid.json")

		// Write invalid JSON
		err := os.WriteFile(configFile, []byte("{invalid json"), 0644)
		if err != nil {
			t.Fatalf("Failed to write invalid JSON: %v", err)
		}

		_, err = mgr.LoadFromFile(configFile)
		if err == nil {
			t.Error("Expected error loading invalid JSON")
		}
	})

	t.Run("non-existent file", func(t *testing.T) {
		_, err := mgr.LoadFromFile("/non/existent/file.json")
		if err == nil {
			t.Error("Expected error loading non-existent file")
		}
	})

	t.Run("permission denied", func(t *testing.T) {
		tempDir := t.TempDir()
		configFile := filepath.Join(tempDir, "no-permission.json")

		// Create file with no read permissions
		err := os.WriteFile(configFile, []byte("{}"), 0000)
		if err != nil {
			t.Fatalf("Failed to create no-permission file: %v", err)
		}

		_, err = mgr.LoadFromFile(configFile)
		if err == nil {
			t.Error("Expected error loading file with no permissions")
		}
	})
}

// Helper function
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}