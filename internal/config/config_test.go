package config

import (
	"encoding/json"
	"log/slog"
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

func TestLoadConfigAutoDiscovery(t *testing.T) {
	mgr := NewConfigManager()

	// Create a temporary directory to simulate XDG config directory
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, "claudio")
	err := os.MkdirAll(configDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create config directory: %v", err)
	}

	// Create a test config file
	configFile := filepath.Join(configDir, "config.json")
	testConfig := &Config{
		Volume:           0.8,
		DefaultSoundpack: "test-pack",
		SoundpackPaths:   []string{"/test/path"},
		Enabled:          true,
		LogLevel:         "debug",
	}

	// Write the config file
	configData, err := json.MarshalIndent(testConfig, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal test config: %v", err)
	}

	err = os.WriteFile(configFile, configData, 0644)
	if err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	// Mock the XDG config paths to point to our temp directory
	originalXDG := mgr.xdg
	mockXDG := &MockXDGDirs{
		configPaths: []string{configFile},
	}
	mgr.xdg = mockXDG

	// Test auto-discovery - should find and load our config
	loadedConfig, err := mgr.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}

	// Verify the config was loaded correctly
	if loadedConfig.Volume != testConfig.Volume {
		t.Errorf("Expected volume %f, got %f", testConfig.Volume, loadedConfig.Volume)
	}

	if loadedConfig.DefaultSoundpack != testConfig.DefaultSoundpack {
		t.Errorf("Expected soundpack %s, got %s", testConfig.DefaultSoundpack, loadedConfig.DefaultSoundpack)
	}

	if len(loadedConfig.SoundpackPaths) != len(testConfig.SoundpackPaths) {
		t.Errorf("Expected %d soundpack paths, got %d", len(testConfig.SoundpackPaths), len(loadedConfig.SoundpackPaths))
	}

	// Restore original XDG
	mgr.xdg = originalXDG

	t.Logf("Auto-discovery test passed: loaded config %+v", loadedConfig)
}

func TestLoadConfigRealXDGPaths(t *testing.T) {
	mgr := NewConfigManager()

	// Test with real XDG paths to see what happens
	configPaths := mgr.xdg.GetConfigPaths("config.json")

	t.Logf("Real XDG config paths: %v", configPaths)

	// Our config should be in a proper XDG config directory, not data directory
	properConfigPath := "/etc/xdg/claudio/config.json"

	// Check if the proper config path is in XDG config paths
	found := false
	for _, path := range configPaths {
		if path == properConfigPath {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Expected XDG config path %s not found in config paths: %v", properConfigPath, configPaths)
	}

	// Test actual LoadConfig behavior - should find config when placed correctly
	config, err := mgr.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}

	t.Logf("Loaded config: %+v", config)

	// This test should FAIL until we move the config file to the correct location
	if len(config.SoundpackPaths) == 0 {
		t.Error("LoadConfig() returned default config - config file should be moved to proper XDG config directory")
	}
}

// MockXDGDirs is a mock implementation for testing
type MockXDGDirs struct {
	configPaths []string
}

func (m *MockXDGDirs) GetConfigPaths(filename string) []string {
	return m.configPaths
}

func (m *MockXDGDirs) GetSoundpackPaths(soundpackID string) []string {
	return []string{}
}

func (m *MockXDGDirs) GetCachePath(purpose string) string {
	return "/tmp/test-cache"
}

func (m *MockXDGDirs) CreateCacheDir(purpose string) error {
	return nil
}

func (m *MockXDGDirs) FindSoundFile(soundpackID, relativePath string) string {
	return ""
}

func TestLoadConfigFromFile(t *testing.T) {
	mgr := NewConfigManager()

	// Create a temporary config file
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "test-config.json")

	testConfig := &Config{
		Volume:           0.75,
		DefaultSoundpack: "mechanical",
		SoundpackPaths:   []string{"/custom/path"},
		Enabled:          false,
		LogLevel:         "warn",
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
				Volume:           0.5,
				DefaultSoundpack: "default",
				SoundpackPaths:   []string{"/valid/path"},
				Enabled:          true,
				LogLevel:         "info",
			},
			shouldError: false,
		},
		{
			name: "volume too high",
			config: &Config{
				Volume:           1.5,
				DefaultSoundpack: "default",
				Enabled:          true,
			},
			shouldError: true,
			errorMsg:    "volume",
		},
		{
			name: "volume too low",
			config: &Config{
				Volume:           -0.1,
				DefaultSoundpack: "default",
				Enabled:          true,
			},
			shouldError: true,
			errorMsg:    "volume",
		},
		{
			name: "empty soundpack",
			config: &Config{
				Volume:           0.5,
				DefaultSoundpack: "",
				Enabled:          true,
			},
			shouldError: true,
			errorMsg:    "soundpack",
		},
		{
			name: "invalid log level",
			config: &Config{
				Volume:           0.5,
				DefaultSoundpack: "default",
				Enabled:          true,
				LogLevel:         "invalid",
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
		Volume:           0.8,
		DefaultSoundpack: "test-pack",
		SoundpackPaths:   []string{"/test/path1", "/test/path2"},
		Enabled:          true,
		LogLevel:         "debug",
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
		Volume:           0.5,
		DefaultSoundpack: "base",
		SoundpackPaths:   []string{"/base/path"},
		Enabled:          true,
		LogLevel:         "info",
	}

	overrideConfig := &Config{
		Volume:           0.8,
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
		Volume:           0.5,
		DefaultSoundpack: "base",
		Enabled:          true,
		LogLevel:         "info",
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

func TestLogLevelApplicationToSlog(t *testing.T) {
	// TDD RED: This test should FAIL because log level from config is not applied to slog
	// We expect that when config has log_level "warn", slog should respect it and not show DEBUG/INFO logs

	mgr := NewConfigManager()

	// Capture log output to verify level is applied
	var logBuffer strings.Builder
	originalHandler := slog.Default().Handler()
	defer slog.SetDefault(slog.New(originalHandler))

	// First, apply log level configuration with warn level
	err := mgr.ApplyLogLevelWithWriter("warn", &logBuffer)
	if err != nil {
		t.Fatalf("ApplyLogLevelWithWriter should not error for valid log level: %v", err)
	}

	// Test that DEBUG and INFO logs are filtered out when level is WARN
	slog.Debug("this debug message should not appear")
	slog.Info("this info message should not appear")
	slog.Warn("this warning should appear")
	slog.Error("this error should appear")

	logOutput := logBuffer.String()

	// CRITICAL: With warn level, DEBUG and INFO should be filtered out
	if strings.Contains(logOutput, "this debug message should not appear") {
		t.Errorf("DEBUG logs should be filtered out when log level is warn, but found debug message in output")
		t.Logf("Full log output: %s", logOutput)
	}

	if strings.Contains(logOutput, "this info message should not appear") {
		t.Errorf("INFO logs should be filtered out when log level is warn, but found info message in output")
		t.Logf("Full log output: %s", logOutput)
	}

	// WARN and ERROR should still appear
	if !strings.Contains(logOutput, "this warning should appear") {
		t.Errorf("WARN logs should appear when log level is warn, but warning message not found in output")
		t.Logf("Full log output: %s", logOutput)
	}

	if !strings.Contains(logOutput, "this error should appear") {
		t.Errorf("ERROR logs should appear when log level is warn, but error message not found in output")
		t.Logf("Full log output: %s", logOutput)
	}
}

func TestConfigLoggingLevels(t *testing.T) {
	// TDD RED: This test should FAIL because config loading operations currently use INFO logging
	// We expect routine config operations to use DEBUG level, not INFO level

	// Capture log output to verify log levels
	var logBuffer strings.Builder
	originalHandler := slog.Default().Handler()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logBuffer, &slog.HandlerOptions{
		Level: slog.LevelDebug, // Capture all logs
	})))
	defer slog.SetDefault(slog.New(originalHandler))

	mgr := NewConfigManager()

	// Test config loading from file - should be DEBUG level
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "test-config.json")

	testConfig := &Config{
		Volume:           0.8,
		DefaultSoundpack: "test-pack",
		SoundpackPaths:   []string{"/test/path"},
		Enabled:          true,
		LogLevel:         "debug",
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

	// Load config from file - should use DEBUG level for routine operation
	_, err = mgr.LoadFromFile(configFile)
	if err != nil {
		t.Fatalf("LoadFromFile should not error: %v", err)
	}

	// Test environment overrides - should be DEBUG level
	baseConfig := &Config{
		Volume:           0.5,
		DefaultSoundpack: "base",
		Enabled:          true,
		LogLevel:         "info",
	}

	_ = mgr.ApplyEnvironmentOverrides(baseConfig)

	logOutput := logBuffer.String()

	// CRITICAL: Routine operations should use DEBUG level, not INFO
	problematicInfoLogs := []string{
		"config loaded successfully",
		"environment overrides applied",
	}

	for _, logMsg := range problematicInfoLogs {
		if strings.Contains(logOutput, logMsg) {
			// Check if it appears with INFO level (bad) vs DEBUG level (good)
			if strings.Contains(logOutput, "level=INFO") && strings.Contains(logOutput, logMsg) {
				t.Errorf("Routine operation '%s' should use DEBUG level, not INFO level", logMsg)
				t.Logf("Full log output: %s", logOutput)
			}
		}
	}

	// Verify that DEBUG logs are working properly
	if !strings.Contains(logOutput, "level=DEBUG") {
		t.Error("Expected some DEBUG level logs but found none")
		t.Logf("Full log output: %s", logOutput)
	}
}


// TDD RED: Test file logging configuration fields parsing
func TestConfig_FileLoggingFields(t *testing.T) {
	mgr := NewConfigManager()

	// Test parsing config with file logging configuration
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "file-logging-config.json")

	// This test should FAIL because FileLoggingConfig doesn't exist yet
	testConfigJSON := `{
		"volume": 0.5,
		"default_soundpack": "default",
		"enabled": true,
		"log_level": "info",
		"file_logging": {
			"enabled": true,
			"filename": "/custom/path/claudio.log",
			"max_size_mb": 15,
			"max_backups": 3,
			"max_age_days": 14,
			"compress": true
		}
	}`

	err := os.WriteFile(configFile, []byte(testConfigJSON), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	// This should fail because FileLogging field doesn't exist in Config struct
	loadedConfig, err := mgr.LoadFromFile(configFile)
	if err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}

	// Verify file logging config was parsed correctly
	if loadedConfig.FileLogging == nil {
		t.Error("FileLogging should not be nil")
		return
	}

	fileConfig := loadedConfig.FileLogging
	if !fileConfig.Enabled {
		t.Error("FileLogging.Enabled should be true")
	}
	if fileConfig.Filename != "/custom/path/claudio.log" {
		t.Errorf("FileLogging.Filename = %q, expected '/custom/path/claudio.log'", fileConfig.Filename)
	}
	if fileConfig.MaxSizeMB != 15 {
		t.Errorf("FileLogging.MaxSizeMB = %d, expected 15", fileConfig.MaxSizeMB)
	}
	if fileConfig.MaxBackups != 3 {
		t.Errorf("FileLogging.MaxBackups = %d, expected 3", fileConfig.MaxBackups)
	}
	if fileConfig.MaxAgeDays != 14 {
		t.Errorf("FileLogging.MaxAgeDays = %d, expected 14", fileConfig.MaxAgeDays)
	}
	if !fileConfig.Compress {
		t.Error("FileLogging.Compress should be true")
	}
}

// TDD RED: Test default values for file logging
func TestConfig_FileLoggingDefaults(t *testing.T) {
	mgr := NewConfigManager()

	// This test should FAIL because FileLogging field doesn't exist yet
	defaultConfig := mgr.GetDefaultConfig()

	// Verify file logging defaults
	if defaultConfig.FileLogging == nil {
		t.Error("FileLogging should have default values, not be nil")
		return
	}

	fileConfig := defaultConfig.FileLogging
	if fileConfig.Enabled != true {
		t.Error("FileLogging.Enabled should default to true for hook-based usage")
	}
	if fileConfig.Filename != "" {
		t.Errorf("FileLogging.Filename should default to empty string for XDG path, got %q", fileConfig.Filename)
	}
	if fileConfig.MaxSizeMB != 10 {
		t.Errorf("FileLogging.MaxSizeMB should default to 10, got %d", fileConfig.MaxSizeMB)
	}
	if fileConfig.MaxBackups != 5 {
		t.Errorf("FileLogging.MaxBackups should default to 5, got %d", fileConfig.MaxBackups)
	}
	if fileConfig.MaxAgeDays != 30 {
		t.Errorf("FileLogging.MaxAgeDays should default to 30, got %d", fileConfig.MaxAgeDays)
	}
	if !fileConfig.Compress {
		t.Error("FileLogging.Compress should default to true")
	}
}

// TDD RED: Test validation of file logging config
func TestConfig_FileLoggingValidation(t *testing.T) {
	mgr := NewConfigManager()

	testCases := []struct {
		name        string
		config      *Config
		shouldError bool
		errorMsg    string
	}{
		{
			name: "valid file logging config",
			config: &Config{
				Volume:           0.5,
				DefaultSoundpack: "default",
				Enabled:          true,
				LogLevel:         "info",
				FileLogging: &FileLoggingConfig{
					Enabled:    true,
					Filename:   "/valid/path/claudio.log",
					MaxSizeMB:  10,
					MaxBackups: 5,
					MaxAgeDays: 30,
					Compress:   true,
				},
			},
			shouldError: false,
		},
		{
			name: "negative max size",
			config: &Config{
				Volume:           0.5,
				DefaultSoundpack: "default",
				Enabled:          true,
				FileLogging: &FileLoggingConfig{
					Enabled:    true,
					MaxSizeMB:  -1, // Invalid
					MaxBackups: 5,
					MaxAgeDays: 30,
				},
			},
			shouldError: true,
			errorMsg:    "max_size_mb",
		},
		{
			name: "negative max backups",
			config: &Config{
				Volume:           0.5,
				DefaultSoundpack: "default",
				Enabled:          true,
				FileLogging: &FileLoggingConfig{
					Enabled:    true,
					MaxSizeMB:  10,
					MaxBackups: -1, // Invalid
					MaxAgeDays: 30,
				},
			},
			shouldError: true,
			errorMsg:    "max_backups",
		},
		{
			name: "negative max age",
			config: &Config{
				Volume:           0.5,
				DefaultSoundpack: "default",
				Enabled:          true,
				FileLogging: &FileLoggingConfig{
					Enabled:    true,
					MaxSizeMB:  10,
					MaxBackups: 5,
					MaxAgeDays: -1, // Invalid
				},
			},
			shouldError: true,
			errorMsg:    "max_age_days",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// This test should FAIL because ValidateConfig doesn't handle FileLogging yet
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

// TDD RED: Test XDG log file path resolution
func TestXDG_LogPath(t *testing.T) {
	mgr := NewConfigManager()

	// Test with custom filename - should return as-is
	customPath := "/custom/path/my-claudio.log"
	resolved := mgr.ResolveLogFilePath(customPath)
	if resolved != customPath {
		t.Errorf("ResolveLogFilePath with custom path = %q, expected %q", resolved, customPath)
	}

	// Test with empty filename - should use XDG cache path
	resolved = mgr.ResolveLogFilePath("")
	expectedPath := filepath.Join(mgr.xdg.GetCachePath("logs"), "claudio.log")
	if resolved != expectedPath {
		t.Errorf("ResolveLogFilePath with empty filename = %q, expected %q", resolved, expectedPath)
	}

	// Verify the XDG path follows expected pattern
	if !strings.Contains(resolved, ".cache/claudio/logs/claudio.log") {
		t.Errorf("XDG log path should contain '.cache/claudio/logs/claudio.log', got %q", resolved)
	}

	// Test that different purposes create different cache paths
	otherCachePath := mgr.xdg.GetCachePath("other")
	logCachePath := mgr.xdg.GetCachePath("logs")
	if otherCachePath == logCachePath {
		t.Error("Different cache purposes should create different paths")
	}
}

// Helper function
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
