package config

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/spf13/afero"

	"claudio.click/internal/platform"
)

// wsl.json is not embedded: it is derived from windows.json (wsl_pack.go).
//
//go:embed windows.json darwin.json linux.json
var platformSoundpacks embed.FS

// embeddedSounds holds the synthesized default WAV tones that back the
// native-Linux platform pack. Unlike Windows/macOS — which point at system
// sound files that always exist — a bare Linux box ships no guaranteed WAVs,
// so linux.json references these by bare filename and they are extracted to
// the cache dir at load time. Regenerate with embedded_sounds/generate.go.
//
//go:embed embedded_sounds/*.wav
var embeddedSounds embed.FS

// FileLoggingConfig represents file-based logging configuration
type FileLoggingConfig struct {
	Enabled    bool   `json:"enabled"`      // Whether file logging is enabled
	Filename   string `json:"filename"`     // Log file path (empty = XDG cache path)
	MaxSizeMB  int    `json:"max_size_mb"`  // Max file size in MB before rotation
	MaxBackups int    `json:"max_backups"`  // Max number of backup files to keep
	MaxAgeDays int    `json:"max_age_days"` // Max age in days before deletion
	Compress   bool   `json:"compress"`     // Whether to compress rotated files
}

// Config represents Claudio configuration
type Config struct {
	Volume           *float64             `json:"volume,omitempty"`         // Audio volume (0.0 to 1.0), nil means use default
	DefaultSoundpack string               `json:"default_soundpack"`        // Default soundpack to use
	SoundpackPaths   []string             `json:"soundpack_paths"`          // Additional paths to search for soundpacks
	Enabled          bool                 `json:"enabled"`                  // Whether Claudio is enabled
	LogLevel         string               `json:"log_level"`                // Log level (debug, info, warn, error)
	AudioBackend     string               `json:"audio_backend"`            // Audio backend (auto, system_command, oto)
	FileLogging      *FileLoggingConfig   `json:"file_logging,omitempty"`   // File logging configuration
	SoundTracking    *SoundTrackingConfig `json:"sound_tracking,omitempty"` // Sound tracking configuration
}

// Clone returns a deep copy of c: pointer fields and slices are duplicated
// so mutating the copy never affects the original.
func (c *Config) Clone() *Config {
	clone := *c
	if c.Volume != nil {
		v := *c.Volume
		clone.Volume = &v
	}
	if c.SoundpackPaths != nil {
		clone.SoundpackPaths = append([]string(nil), c.SoundpackPaths...)
	}
	if c.FileLogging != nil {
		fl := *c.FileLogging
		clone.FileLogging = &fl
	}
	if c.SoundTracking != nil {
		st := *c.SoundTracking
		clone.SoundTracking = &st
	}
	return &clone
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
	fs  afero.Fs
}

// NewConfigManager creates a new configuration manager
func NewConfigManager() *ConfigManager {
	slog.Debug("creating new config manager")
	return &ConfigManager{
		xdg: NewXDGDirs(),
		fs:  afero.NewOsFs(), // Production uses real filesystem
	}
}

// NewConfigManagerWithFilesystem creates a new configuration manager with custom filesystem
func NewConfigManagerWithFilesystem(fs afero.Fs) *ConfigManager {
	slog.Debug("creating new config manager with custom filesystem")
	return &ConfigManager{
		xdg: NewXDGDirs(),
		fs:  fs,
	}
}

// GetDefaultConfig returns the default configuration
func (cm *ConfigManager) GetDefaultConfig() *Config {
	slog.Debug("GetDefaultConfig called - starting platform detection")
	// Use platform-specific soundpack if it exists, otherwise default.
	// getExecutableDirectoryForDefault is a method so the test-context CWD
	// recheck honors cm.fs.
	executableDir := cm.getExecutableDirectoryForDefault()
	slog.Debug("GetDefaultConfig got executable directory", "executableDir", executableDir)
	defaultSoundpack := cm.GetPlatformSoundpack(executableDir)
	slog.Debug("GetDefaultConfig platform detection result", "defaultSoundpack", defaultSoundpack)

	defaultVolume := 0.5
	defaultConfig := &Config{
		Volume:           &defaultVolume,
		DefaultSoundpack: defaultSoundpack,
		SoundpackPaths:   []string{}, // XDG paths will be used
		Enabled:          true,
		LogLevel:         "warn",
		AudioBackend:     "auto", // Default to auto-detection
		FileLogging: &FileLoggingConfig{
			Enabled:    true, // Default enabled for hook-based usage
			Filename:   "",   // Empty = XDG cache path
			MaxSizeMB:  10,
			MaxBackups: 5,
			MaxAgeDays: 30,
			Compress:   true,
		},
		SoundTracking: GetDefaultSoundTrackingConfig(),
	}

	slog.Debug("generated default config",
		"volume", *defaultConfig.Volume,
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

	// Failures are returned, not logged: callers decide whether a bad file
	// is fatal (write paths) or a warning (hook mode), and log it once.
	data, err := afero.ReadFile(cm.fs, filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// An empty file (including --config NUL or /dev/null) holds no
	// settings, so it means defaults rather than a JSON syntax error.
	if len(bytes.TrimSpace(data)) == 0 {
		slog.Debug("config file is empty; using defaults", "file_path", filePath)
		data = []byte("{}")
	}

	// Decode on top of the defaults so keys the file omits keep their default
	// values instead of Go zero values (a missing "enabled" would otherwise
	// mute Claudio). Explicit values, including false and null, still win.
	// Volume stays nil when omitted: nil already means "use the default" and
	// lets `claudio volume` report that no volume is persisted.
	config := *cm.GetDefaultConfig()
	config.Volume = nil
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config JSON: %w", err)
	}

	config.AudioBackend = migrateLegacyAudioBackend(config.AudioBackend, filePath)

	if err := cm.ValidateConfig(&config); err != nil {
		return nil, err
	}

	slog.Debug("config loaded successfully",
		"file_path", filePath,
		"volume", config.Volume, // nil-safe: slog handles pointers
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
	err = cm.fs.MkdirAll(dir, 0755)
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

	err = afero.WriteFile(cm.fs, filePath, data, 0644)
	if err != nil {
		slog.Error("failed to write config file", "file_path", filePath, "error", err)
		return fmt.Errorf("failed to write config file: %w", err)
	}

	slog.Info("config saved successfully", "file_path", filePath)
	return nil
}

// WriteConfig is an alias for SaveToFile for compatibility with tests
func (cm *ConfigManager) WriteConfig(filePath string, config *Config) error {
	return cm.SaveToFile(config, filePath)
}

// LoadConfig loads configuration using XDG path discovery
func (cm *ConfigManager) LoadConfig() (*Config, error) {
	path := cm.FindConfigFile()
	if path == "" {
		slog.Debug("no config file found, using defaults")
		return cm.GetDefaultConfig(), nil
	}
	return cm.LoadFromFile(path)
}

// FindConfigFile returns the first XDG config.json that exists, in search
// order, or "" when there is none.
func (cm *ConfigManager) FindConfigFile() string {
	for _, configPath := range cm.xdg.GetConfigPaths("config.json") {
		if _, err := cm.fs.Stat(configPath); err == nil {
			slog.Debug("found config file", "path", configPath)
			return configPath
		}
	}
	return ""
}

// ValidateConfig validates configuration values
func (cm *ConfigManager) ValidateConfig(config *Config) error {
	var errors []string

	// Validate volume (nil is valid - means use default).
	// Reject NaN/Inf before the range check to match the guards in
	// SystemCommandBackend.SetVolume and AudioPlayer.SetVolume; the float
	// comparison treats NaN as out-of-range silently, so without an explicit
	// IsNaN/IsInf guard a configured NaN volume would slip through here.
	if config.Volume != nil {
		v := float64(*config.Volume)
		if math.IsNaN(v) || math.IsInf(v, 0) {
			errors = append(errors, fmt.Sprintf("volume must be finite, got %f", *config.Volume))
		} else if v < 0.0 || v > 1.0 {
			errors = append(errors, fmt.Sprintf("volume must be between 0.0 and 1.0, got %f", *config.Volume))
		}
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
		slog.Debug("config validation failed", "errors", errMsg)
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

	// Apply overrides (only if explicitly set)
	if override.Volume != nil {
		merged.Volume = override.Volume
		slog.Debug("merged volume override", "value", *override.Volume)
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

	// Deep copy so overrides never write through into the caller's config.
	result := config.Clone()
	applyEnvVars(result, configEnvVars)

	// Apply sound tracking environment overrides
	if result.SoundTracking == nil {
		result.SoundTracking = GetDefaultSoundTrackingConfig()
	}
	result.SoundTracking = ApplySoundTrackingEnvironmentOverrides(result.SoundTracking)

	slog.Debug("environment overrides applied")
	return result
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

// GetSupportedAudioBackends returns a list of all supported audio backend types.
// "fake" is the test-only backend registered by internal/audio/fake_backend.go;
// it is listed here so cli tests can set cfg.AudioBackend = "fake" without
// tripping ConfigManager.ValidateConfig.
func (cm *ConfigManager) GetSupportedAudioBackends() []string {
	return slices.Clone(supportedAudioBackends)
}

var supportedAudioBackends = []string{"auto", "system_command", "oto", "fake"}

// isSupportedAudioBackend reports whether backend is one of the
// GetSupportedAudioBackends names (empty means auto and is accepted).
func isSupportedAudioBackend(backend string) bool {
	return backend == "" || slices.Contains(supportedAudioBackends, backend)
}

// legacyAudioBackends maps removed backend names to their replacements so a
// config written before an upgrade keeps working instead of failing every
// hook. Aliases are accepted on input only and never advertised as supported.
var legacyAudioBackends = map[string]string{"malgo": "oto"}

func migrateLegacyAudioBackend(backend, source string) string {
	replacement, ok := legacyAudioBackends[backend]
	if !ok {
		return backend
	}
	slog.Warn("deprecated audio backend in config; using its replacement",
		"configured", backend, "using", replacement, "source", source)
	return replacement
}

// IsValidAudioBackend checks if an audio backend type is supported
func (cm *ConfigManager) IsValidAudioBackend(backend string) bool {
	return isSupportedAudioBackend(backend)
}

// hasEmbeddedPlatformFile checks if an embedded platform soundpack file exists
func hasEmbeddedPlatformFile(filename string) bool {
	_, err := GetEmbeddedPlatformSoundpackData(filename)
	return err == nil
}

// GetEmbeddedPlatformSoundpackData reads embedded platform soundpack data
func GetEmbeddedPlatformSoundpackData(filename string) ([]byte, error) {
	read := platformSoundpacks.ReadFile
	if filename == wslPackFile {
		read = func(string) ([]byte, error) { return wslPackData() }
	}
	data, err := read(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded platform soundpack file '%s': %w", filename, err)
	}
	return data, nil
}

// GetEmbeddedSoundData returns the bytes of an embedded default sound by its
// bare filename (e.g. "default-success.wav"), or an error if no such sound is
// embedded. A soundpack mapping value that names an absolute system path is
// not an embedded sound, so callers can use the error to distinguish the two:
// only the native-Linux pack references these embedded tones by bare name.
func GetEmbeddedSoundData(name string) ([]byte, error) {
	data, err := embeddedSounds.ReadFile("embedded_sounds/" + name)
	if err != nil {
		return nil, fmt.Errorf("no embedded sound %q: %w", name, err)
	}
	return data, nil
}

// GetPlatformSoundpack returns platform-specific soundpack if it exists, otherwise "default".
// Enhanced version that:
//  1. Checks WSL first (prefers wsl.json over linux.json)
//  2. Looks in provided executable directory using cm.fs (the manager's
//     own filesystem; set via NewConfigManagerWithFilesystem for tests)
//  3. Returns full path to JSON file when found
//
// Pre-fix this method took an afero.Fs parameter that production code
// always satisfied with afero.NewOsFs(), defeating the cm.fs seam tests
// relied on. Drop the parameter; use cm.fs.
func (cm *ConfigManager) GetPlatformSoundpack(executableDir string) string {
	slog.Debug("detecting platform soundpack with enhanced detection",
		"executable_dir", executableDir,
		"is_wsl", platform.IsWSL(),
		"runtime_goos", runtime.GOOS)

	// WSL detection first - prefer wsl.json over linux.json when in WSL
	if platform.IsWSL() {
		if wslPath := cm.checkPlatformFile(executableDir, "wsl.json"); wslPath != "" {
			slog.Debug("WSL platform soundpack found", "path", wslPath)
			return wslPath
		}
		slog.Debug("WSL detected but wsl.json not found in executable directory", "exec_dir", executableDir)
	}

	// Regular OS-specific detection
	platformFile := runtime.GOOS + ".json"
	if platformPath := cm.checkPlatformFile(executableDir, platformFile); platformPath != "" {
		slog.Debug("platform soundpack found", "platform", runtime.GOOS, "path", platformPath)
		return platformPath
	}

	// Check embedded platform files as fallback
	var embeddedPlatformFile string
	if platform.IsWSL() {
		embeddedPlatformFile = "wsl.json"
	} else {
		embeddedPlatformFile = runtime.GOOS + ".json"
	}

	if hasEmbeddedPlatformFile(embeddedPlatformFile) {
		slog.Debug("using embedded platform soundpack",
			"platform_file", embeddedPlatformFile,
			"is_wsl", platform.IsWSL(),
			"runtime_goos", runtime.GOOS)
		return "embedded:" + embeddedPlatformFile
	}

	slog.Debug("no platform soundpack found (file or embedded), using default",
		"platform", runtime.GOOS,
		"wsl_detection", platform.IsWSL(),
		"exec_dir", executableDir,
		"embedded_file_checked", embeddedPlatformFile)
	return "default"
}

// checkPlatformFile checks if a platform JSON file exists in the specified
// directory using the manager's own filesystem (cm.fs). Returns full path if
// found, empty string if not found. Was previously a free function that took
// an afero.Fs parameter; methodified in finding #64 so it honors the seam
// established by NewConfigManagerWithFilesystem.
func (cm *ConfigManager) checkPlatformFile(dir, filename string) string {
	fullPath := filepath.Join(dir, filename)

	if info, err := cm.fs.Stat(fullPath); err == nil && !info.IsDir() {
		slog.Debug("platform file found", "path", fullPath, "size", info.Size())
		return fullPath
	}

	slog.Debug("platform file not found", "path", fullPath)
	return ""
}

// getExecutableDirectoryForDefault returns the directory containing the
// current executable for default config. Methodified in finding #64 so the
// test-context CWD recheck uses cm.fs / cm.GetPlatformSoundpack and does
// not construct a fresh ConfigManager (which would always use OsFs and
// defeat the NewConfigManagerWithFilesystem seam).
func (cm *ConfigManager) getExecutableDirectoryForDefault() string {
	executable, err := os.Executable()
	if err != nil {
		slog.Warn("failed to get executable directory for default config, using current directory", "error", err)
		return "."
	}

	execDir := filepath.Dir(executable)
	slog.Debug("executable directory detected for default config", "executable", executable, "directory", execDir)

	// If executable is in a temp build directory (e.g. /tmp/go-buildXXX on
	// POSIX, %TEMP%\go-buildNNN\... on Windows), also check current working
	// directory for platform JSON files. Detection is portable via
	// isGoTestTempExecutable, which uses os.TempDir() as the prefix and the
	// "go-build" substring as the go-test staged-binary marker.
	if isGoTestTempExecutable(executable, os.TempDir()) {
		cwd, err := os.Getwd()
		if err == nil {
			slog.Debug("executable appears to be temp build, also checking current working directory", "cwd", cwd, "temp_exec", executable)
			// Use the receiver's own filesystem so the recheck honors
			// NewConfigManagerWithFilesystem.
			cwdResult := cm.GetPlatformSoundpack(cwd)
			if cwdResult != "default" {
				slog.Debug("found platform JSON in current working directory, using that", "cwd_result", cwdResult)
				return cwd
			}
		}
	}

	return execDir
}

// isGoTestTempExecutable reports whether executablePath looks like a binary
// staged by `go test` under tmpRoot. Both inputs are cleaned via
// filepath.Clean so the comparison is portable across slash conventions:
//   - POSIX go test stages binaries under e.g. /tmp/go-buildNNN/.../pkg.test
//   - Windows go test stages binaries under e.g. %TEMP%\go-buildNNN\...\pkg.test.exe
//
// The "go-build" substring is the actual marker on every platform.
func isGoTestTempExecutable(executablePath, tmpRoot string) bool {
	if executablePath == "" || tmpRoot == "" {
		return false
	}
	cleanedExec := filepath.Clean(executablePath)
	cleanedTmp := filepath.Clean(tmpRoot)
	return strings.HasPrefix(cleanedExec, cleanedTmp) && strings.Contains(cleanedExec, "go-build")
}
