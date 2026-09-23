package config

import (
	"log/slog"
)

// SoundTrackingConfig represents sound tracking configuration
type SoundTrackingConfig struct {
	Enabled      bool   `json:"enabled"`       // Whether sound tracking is enabled
	DatabasePath string `json:"database_path"` // Custom database path (empty = XDG cache path)
}

// GetDefaultSoundTrackingConfig returns the default sound tracking configuration
func GetDefaultSoundTrackingConfig() *SoundTrackingConfig {
	return &SoundTrackingConfig{
		Enabled:      true, // Default enabled to track missing sounds
		DatabasePath: "",   // Empty = XDG cache path
	}
}

// ApplySoundTrackingEnvironmentOverrides applies environment variable overrides to sound tracking config
func ApplySoundTrackingEnvironmentOverrides(config *SoundTrackingConfig) *SoundTrackingConfig {
	slog.Debug("applying sound tracking environment variable overrides")

	result := *config
	applyEnvVars(&result, soundTrackingEnvVars)

	slog.Debug("sound tracking environment overrides applied")
	return &result
}
