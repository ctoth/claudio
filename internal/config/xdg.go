package config

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/adrg/xdg"
)

// appDir is Claudio's subdirectory under every XDG base directory.
const appDir = "claudio"

// ConfigPaths returns where a claudio config file can be found, in search
// order: the user config dir, then each system config dir.
func ConfigPaths(filename string) []string {
	paths := []string{UserConfigPath(filename)}
	for _, dir := range xdg.ConfigDirs {
		paths = append(paths, filepath.Join(dir, appDir, filename))
	}
	return paths
}

// UserConfigPath joins elem under the user's claudio config directory
// ($XDG_CONFIG_HOME/claudio), the one place claudio writes config state.
func UserConfigPath(elem ...string) string {
	return filepath.Join(append([]string{xdg.ConfigHome, appDir}, elem...)...)
}

// SoundpackPaths returns where soundpack soundpackID can be found, in
// search order: the user data dir, then each system data dir. An empty ID
// yields the soundpacks directories themselves.
func SoundpackPaths(soundpackID string) []string {
	paths := []string{UserDataPath("soundpacks", soundpackID)}
	for _, dir := range xdg.DataDirs {
		paths = append(paths, filepath.Join(dir, appDir, "soundpacks", soundpackID))
	}
	return paths
}

// UserDataPath joins elem under the user's claudio data directory
// ($XDG_DATA_HOME/claudio), where installed and cloned soundpacks live.
func UserDataPath(elem ...string) string {
	return filepath.Join(append([]string{xdg.DataHome, appDir}, elem...)...)
}

// CachePath joins elem under claudio's cache directory
// ($XDG_CACHE_HOME/claudio): logs, the tracking database, extracted sounds.
func CachePath(elem ...string) string {
	return filepath.Join(append([]string{xdg.CacheHome, appDir}, elem...)...)
}

// CreateCacheDir creates the cache directory for a specific purpose
func CreateCacheDir(purpose string) error {
	cachePath := CachePath(purpose)
	if err := os.MkdirAll(cachePath, 0755); err != nil {
		return err
	}
	slog.Debug("cache directory ready", "path", cachePath)
	return nil
}

// FindSoundFile searches for a sound file in soundpack directories
// Returns the full path to the first existing file, or empty string if not found
func FindSoundFile(soundpackID, relativePath string) string {
	if soundpackID == "" || relativePath == "" {
		return ""
	}

	// Sanitize the relative path to prevent directory traversal
	relativePath = sanitizePath(relativePath)
	if relativePath == "" {
		slog.Warn("relative path was empty after sanitization")
		return ""
	}

	for _, basePath := range SoundpackPaths(soundpackID) {
		fullPath := filepath.Join(basePath, relativePath)
		if _, err := os.Stat(fullPath); err == nil {
			slog.Debug("sound file found", "soundpack_id", soundpackID, "full_path", fullPath)
			return fullPath
		}
	}
	return ""
}

// sanitizePath removes dangerous path components and normalizes the path
func sanitizePath(path string) string {
	// Remove null bytes and control characters
	path = strings.ReplaceAll(path, "\x00", "")
	path = strings.ReplaceAll(path, "\n", "")
	path = strings.ReplaceAll(path, "\r", "")

	// Clean the path to resolve . and .. components
	path = filepath.Clean(path)

	// Prevent directory traversal - path should not start with / or contain ..
	if strings.HasPrefix(path, "/") || strings.HasPrefix(path, "..") || strings.Contains(path, "../") {
		slog.Warn("rejecting potentially dangerous path", "path", path)
		return ""
	}

	return path
}
