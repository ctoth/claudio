package config

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// FileLoggingConfig represents file-based logging configuration
type FileLoggingConfig struct {
	Enabled    bool   `json:"enabled"`       // Whether file logging is enabled
	Filename   string `json:"filename"`      // Log file path (empty = XDG cache path)
	MaxSizeMB  int    `json:"max_size_mb"`   // Max file size in MB before rotation
	MaxBackups int    `json:"max_backups"`   // Max number of backup files to keep
	MaxAgeDays int    `json:"max_age_days"`  // Max age in days before deletion
	Compress   bool   `json:"compress"`      // Whether to compress rotated files
}

// Config represents Claudio configuration
type Config struct {
	Volume          float64             `json:"volume"`           // Audio volume (0.0 to 1.0)
	DefaultSoundpack string              `json:"default_soundpack"` // Default soundpack to use
	SoundpackPaths  []string            `json:"soundpack_paths"`  // Additional paths to search for soundpacks
	Enabled         bool                `json:"enabled"`          // Whether Claudio is enabled
	LogLevel        string              `json:"log_level"`        // Log level (debug, info, warn, error)
	AudioBackend    string              `json:"audio_backend"`    // Audio backend (auto, system_command, malgo)
	FileLogging     *FileLoggingConfig  `json:"file_logging,omitempty"` // File logging configuration
}

// XDGInterface defines the interface for XDG directory operations
type XDGInterface interface {
	GetConfigPaths(filename string) []string
	GetSoundpackPaths(soundpackID string) []string
	GetCachePath(purpose string) string
	CreateCacheDir(purpose string) error
	FindSoundFile(soundpackID, relativePath string) string
}

// ConfigManager handles loading, saving, and validating configuration
type ConfigManager struct {
	xdg XDGInterface
}

// NewConfigManager creates a new configuration manager
func NewConfigManager() *ConfigManager {
	slog.Debug("creating new config manager")
	return &ConfigManager{
		xdg: NewXDGDirs(),
	}
}

// GetDefaultConfig returns the default configuration
func (cm *ConfigManager) GetDefaultConfig() *Config {
	// Use Windows soundpack on Windows platform
	defaultSoundpack := "default"
	if cm.IsWindowsPlatform() {
		defaultSoundpack = "windows-media-native-soundpack"
	}

	defaultConfig := &Config{
		Volume:          0.5,
		DefaultSoundpack: defaultSoundpack,
		SoundpackPaths:  []string{}, // XDG paths will be used
		Enabled:         true,
		LogLevel:        "warn",
		AudioBackend:    "auto", // Default to auto-detection
		FileLogging: &FileLoggingConfig{
			Enabled:    true,  // Default enabled for hook-based usage
			Filename:   "",    // Empty = XDG cache path
			MaxSizeMB:  10,
			MaxBackups: 5,
			MaxAgeDays: 30,
			Compress:   true,
		},
	}

	slog.Debug("generated default config",
		"volume", defaultConfig.Volume,
		"default_soundpack", defaultConfig.DefaultSoundpack,
		"enabled", defaultConfig.Enabled,
		"log_level", defaultConfig.LogLevel,
		"audio_backend", defaultConfig.AudioBackend,
		"file_logging_enabled", defaultConfig.FileLogging.Enabled)

	return defaultConfig
}

// LoadFromFile loads configuration from a specific file
func (cm *ConfigManager) LoadFromFile(filePath string) (*Config, error) {
	slog.Debug("loading config from file", "file_path", filePath)

	data, err := os.ReadFile(filePath)
	if err != nil {
		slog.Error("failed to read config file", "file_path", filePath, "error", err)
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	err = json.Unmarshal(data, &config)
	if err != nil {
		slog.Error("failed to parse config JSON", "file_path", filePath, "error", err)
		return nil, fmt.Errorf("failed to parse config JSON: %w", err)
	}

	err = cm.ValidateConfig(&config)
	if err != nil {
		slog.Error("config validation failed", "file_path", filePath, "error", err)
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	slog.Debug("config loaded successfully",
		"file_path", filePath,
		"volume", config.Volume,
		"default_soundpack", config.DefaultSoundpack,
		"enabled", config.Enabled)

	return &config, nil
}

// SaveToFile saves configuration to a specific file
func (cm *ConfigManager) SaveToFile(config *Config, filePath string) error {
	slog.Debug("saving config to file", "file_path", filePath)

	err := cm.ValidateConfig(config)
	if err != nil {
		slog.Error("cannot save invalid config", "error", err)
		return fmt.Errorf("cannot save invalid config: %w", err)
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(filePath)
	err = os.MkdirAll(dir, 0755)
	if err != nil {
		slog.Error("failed to create config directory", "directory", dir, "error", err)
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Marshal with indentation for readability
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		slog.Error("failed to marshal config", "error", err)
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	err = os.WriteFile(filePath, data, 0644)
	if err != nil {
		slog.Error("failed to write config file", "file_path", filePath, "error", err)
		return fmt.Errorf("failed to write config file: %w", err)
	}

	slog.Info("config saved successfully", "file_path", filePath)
	return nil
}

// LoadConfig loads configuration using XDG path discovery
func (cm *ConfigManager) LoadConfig() (*Config, error) {
	slog.Debug("loading config using XDG path discovery")

	configPaths := cm.xdg.GetConfigPaths("config.json")

	slog.Debug("searching for config file", "paths", configPaths)

	// Try to load from each path in priority order
	for i, configPath := range configPaths {
		slog.Debug("checking config path", "path_index", i, "path", configPath)

		if _, err := os.Stat(configPath); err == nil {
			slog.Debug("found config file", "path", configPath)
			return cm.LoadFromFile(configPath)
		} else {
			slog.Debug("config file not found", "path", configPath, "error", err)
		}
	}

	slog.Debug("no config file found, using defaults")
	return cm.GetDefaultConfig(), nil
}

// ValidateConfig validates configuration values
func (cm *ConfigManager) ValidateConfig(config *Config) error {
	var errors []string

	// Validate volume
	if config.Volume < 0.0 || config.Volume > 1.0 {
		errors = append(errors, fmt.Sprintf("volume must be between 0.0 and 1.0, got %f", config.Volume))
	}

	// Validate default soundpack
	if config.DefaultSoundpack == "" {
		errors = append(errors, "default soundpack cannot be empty")
	}

	// Validate log level
	validLogLevels := []string{"debug", "info", "warn", "error"}
	if config.LogLevel != "" {
		valid := false
		for _, level := range validLogLevels {
			if config.LogLevel == level {
				valid = true
				break
			}
		}
		if !valid {
			errors = append(errors, fmt.Sprintf("invalid log level '%s', must be one of: %s", 
				config.LogLevel, strings.Join(validLogLevels, ", ")))
		}
	}

	// Validate audio backend
	if !cm.IsValidAudioBackend(config.AudioBackend) {
		supportedBackends := cm.GetSupportedAudioBackends()
		errors = append(errors, fmt.Sprintf("invalid audio backend '%s', must be one of: %s", 
			config.AudioBackend, strings.Join(supportedBackends, ", ")))
	}

	// Validate file logging configuration
	if config.FileLogging != nil {
		fileLogging := config.FileLogging
		
		if fileLogging.MaxSizeMB < 0 {
			errors = append(errors, fmt.Sprintf("file logging max_size_mb must be >= 0, got %d", fileLogging.MaxSizeMB))
		}
		
		if fileLogging.MaxBackups < 0 {
			errors = append(errors, fmt.Sprintf("file logging max_backups must be >= 0, got %d", fileLogging.MaxBackups))
		}
		
		if fileLogging.MaxAgeDays < 0 {
			errors = append(errors, fmt.Sprintf("file logging max_age_days must be >= 0, got %d", fileLogging.MaxAgeDays))
		}
	}

	if len(errors) > 0 {
		errMsg := strings.Join(errors, "; ")
		slog.Error("config validation failed", "errors", errMsg)
		return fmt.Errorf("config validation failed: %s", errMsg)
	}

	slog.Debug("config validation passed")
	return nil
}

// MergeConfigs merges two configurations, with override taking precedence
func (cm *ConfigManager) MergeConfigs(base, override *Config) *Config {
	slog.Debug("merging configurations")

	// Start with a copy of base
	merged := *base

	// Apply overrides (only non-zero values)
	if override.Volume != 0.0 {
		merged.Volume = override.Volume
		slog.Debug("merged volume override", "value", override.Volume)
	}

	if override.DefaultSoundpack != "" {
		merged.DefaultSoundpack = override.DefaultSoundpack
		slog.Debug("merged soundpack override", "value", override.DefaultSoundpack)
	}

	if len(override.SoundpackPaths) > 0 {
		merged.SoundpackPaths = override.SoundpackPaths
		slog.Debug("merged soundpack paths override", "paths", override.SoundpackPaths)
	}

	if override.LogLevel != "" {
		merged.LogLevel = override.LogLevel
		slog.Debug("merged log level override", "value", override.LogLevel)
	}

	if override.AudioBackend != "" {
		merged.AudioBackend = override.AudioBackend
		slog.Debug("merged audio backend override", "value", override.AudioBackend)
	}

	// Note: Enabled is a bool, so we need special handling
	// In JSON, explicit false would override true from base
	// This is handled naturally by the struct unmarshaling

	slog.Debug("configurations merged successfully")
	return &merged
}

// ApplyEnvironmentOverrides applies environment variable overrides to config
func (cm *ConfigManager) ApplyEnvironmentOverrides(config *Config) *Config {
	slog.Debug("applying environment variable overrides")

	// Create a copy to modify
	result := *config

	// CLAUDIO_VOLUME
	if volStr := os.Getenv("CLAUDIO_VOLUME"); volStr != "" {
		if vol, err := strconv.ParseFloat(volStr, 64); err == nil {
			result.Volume = vol
			slog.Debug("applied volume override from environment", "value", vol)
		} else {
			slog.Warn("invalid CLAUDIO_VOLUME environment variable", "value", volStr, "error", err)
		}
	}

	// CLAUDIO_SOUNDPACK
	if soundpack := os.Getenv("CLAUDIO_SOUNDPACK"); soundpack != "" {
		result.DefaultSoundpack = soundpack
		slog.Debug("applied soundpack override from environment", "value", soundpack)
	}

	// CLAUDIO_ENABLED
	if enabledStr := os.Getenv("CLAUDIO_ENABLED"); enabledStr != "" {
		if enabled, err := strconv.ParseBool(enabledStr); err == nil {
			result.Enabled = enabled
			slog.Debug("applied enabled override from environment", "value", enabled)
		} else {
			slog.Warn("invalid CLAUDIO_ENABLED environment variable", "value", enabledStr, "error", err)
		}
	}

	// CLAUDIO_LOG_LEVEL
	if logLevel := os.Getenv("CLAUDIO_LOG_LEVEL"); logLevel != "" {
		result.LogLevel = logLevel
		slog.Debug("applied log level override from environment", "value", logLevel)
	}

	// CLAUDIO_AUDIO_BACKEND
	if audioBackend := os.Getenv("CLAUDIO_AUDIO_BACKEND"); audioBackend != "" {
		// Validate the backend before applying
		if cm.IsValidAudioBackend(audioBackend) {
			result.AudioBackend = audioBackend
			slog.Debug("applied audio backend override from environment", "value", audioBackend)
		} else {
			slog.Warn("invalid CLAUDIO_AUDIO_BACKEND environment variable", "value", audioBackend)
		}
	}

	slog.Debug("environment overrides applied")
	return &result
}

// ApplyLogLevel configures slog with the specified log level
func (cm *ConfigManager) ApplyLogLevel(logLevel string) error {
	if logLevel == "" {
		slog.Debug("no log level specified, keeping current slog configuration")
		return nil
	}

	slog.Debug("applying log level configuration", "log_level", logLevel)

	// Parse log level string to slog.Level
	var level slog.Level
	switch strings.ToLower(logLevel) {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		err := fmt.Errorf("invalid log level '%s', must be one of: debug, info, warn, error", logLevel)
		slog.Error("invalid log level for slog configuration", "log_level", logLevel, "error", err)
		return err
	}

	// Create new handler with the specified level
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	})

	// Set as default slog logger
	slog.SetDefault(slog.New(handler))

	slog.Debug("slog configured successfully", "log_level", logLevel, "slog_level", level)
	return nil
}

// ResolveLogFilePath resolves the log file path using XDG cache directory when filename is empty
func (cm *ConfigManager) ResolveLogFilePath(filename string) string {
	if filename != "" {
		return filename
	}
	
	// Use XDG cache directory for log files
	return filepath.Join(cm.xdg.GetCachePath("logs"), "claudio.log")
}

// ApplyLogLevelWithWriter configures slog with the specified log level and custom writer (for testing)
func (cm *ConfigManager) ApplyLogLevelWithWriter(logLevel string, writer io.Writer) error {
	if logLevel == "" {
		slog.Debug("no log level specified, keeping current slog configuration")
		return nil
	}

	slog.Debug("applying log level configuration with custom writer", "log_level", logLevel)

	// Parse log level string to slog.Level
	var level slog.Level
	switch strings.ToLower(logLevel) {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		err := fmt.Errorf("invalid log level '%s', must be one of: debug, info, warn, error", logLevel)
		slog.Error("invalid log level for slog configuration", "log_level", logLevel, "error", err)
		return err
	}

	// Create new handler with the specified level and writer
	handler := slog.NewTextHandler(writer, &slog.HandlerOptions{
		Level: level,
	})

	// Set as default slog logger
	slog.SetDefault(slog.New(handler))

	slog.Debug("slog configured successfully with custom writer", "log_level", logLevel, "slog_level", level)
	return nil
}

// GetSupportedAudioBackends returns a list of all supported audio backend types
func (cm *ConfigManager) GetSupportedAudioBackends() []string {
	return []string{"auto", "system_command", "malgo"}
}

// IsValidAudioBackend checks if an audio backend type is supported
func (cm *ConfigManager) IsValidAudioBackend(backend string) bool {
	// Empty string is valid (defaults to auto)
	if backend == "" {
		return true
	}
	
	supported := cm.GetSupportedAudioBackends()
	for _, supportedBackend := range supported {
		if backend == supportedBackend {
			return true
		}
	}
	return false
}

// IsWindowsPlatform detects if we're running on Windows
func (cm *ConfigManager) IsWindowsPlatform() bool {
	// Check for Windows-specific environment variables
	if os.Getenv("WINDIR") != "" || os.Getenv("SYSTEMROOT") != "" {
		return true
	}
	
	// Additional check for Windows paths
	if strings.Contains(os.Getenv("PATH"), "C:\\Windows") {
		return true
	}
	
	return false
}