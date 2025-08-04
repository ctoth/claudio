package config

import (
	"log/slog"
	"os"
	"strconv"
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

	// Create a copy to modify
	result := *config

	// CLAUDIO_SOUND_TRACKING
	if trackingStr := os.Getenv("CLAUDIO_SOUND_TRACKING"); trackingStr != "" {
		if enabled, err := strconv.ParseBool(trackingStr); err == nil {
			result.Enabled = enabled
			slog.Debug("applied sound tracking override from environment", "value", enabled)
		} else {
			slog.Warn("invalid CLAUDIO_SOUND_TRACKING environment variable", "value", trackingStr, "error", err)
		}
	}

	slog.Debug("sound tracking environment overrides applied")
	return &result
}