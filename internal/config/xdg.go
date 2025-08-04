package config

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/adrg/xdg"
)

// XDGDirs provides XDG Base Directory compliant paths for Claudio
type XDGDirs struct{}

// NewXDGDirs creates a new XDG directory manager
func NewXDGDirs() *XDGDirs {
	slog.Debug("creating new XDG directory manager")
	return &XDGDirs{}
}

// GetSoundpackPaths returns prioritized paths where soundpacks can be found
// Returns paths in search order: user data dir, then system data dirs
func (x *XDGDirs) GetSoundpackPaths(soundpackID string) []string {
	var paths []string

	baseDir := "claudio/soundpacks"
	if soundpackID != "" {
		baseDir = filepath.Join(baseDir, soundpackID)
	}

	// User data directory (highest priority)
	userPath := filepath.Join(xdg.DataHome, baseDir)
	paths = append(paths, userPath)

	// System data directories (fallback)
	for _, dataDir := range xdg.DataDirs {
		systemPath := filepath.Join(dataDir, baseDir)
		paths = append(paths, systemPath)
	}

	slog.Debug("generated soundpack paths",
		"soundpack_id", soundpackID,
		"total_paths", len(paths),
		"user_path", userPath,
		"system_paths", len(xdg.DataDirs))

	return paths
}

// GetCachePath returns the cache directory path for a specific purpose
func (x *XDGDirs) GetCachePath(purpose string) string {
	baseDir := "claudio"
	if purpose != "" {
		baseDir = filepath.Join(baseDir, purpose)
	}

	cachePath := filepath.Join(xdg.CacheHome, baseDir)

	slog.Debug("generated cache path",
		"purpose", purpose,
		"cache_path", cachePath)

	return cachePath
}

// GetConfigPaths returns prioritized paths where config files can be found
// Returns paths in search order: user config dir, then system config dirs
func (x *XDGDirs) GetConfigPaths(filename string) []string {
	var paths []string

	baseDir := "claudio"

	// User config directory (highest priority)
	userConfigPath := filepath.Join(xdg.ConfigHome, baseDir)
	if filename != "" {
		userConfigPath = filepath.Join(userConfigPath, filename)
	}
	paths = append(paths, userConfigPath)

	// System config directories (fallback)
	for _, configDir := range xdg.ConfigDirs {
		systemConfigPath := filepath.Join(configDir, baseDir)
		if filename != "" {
			systemConfigPath = filepath.Join(systemConfigPath, filename)
		}
		paths = append(paths, systemConfigPath)
	}

	slog.Debug("generated config paths",
		"filename", filename,
		"total_paths", len(paths),
		"user_path", userConfigPath,
		"system_paths", len(xdg.ConfigDirs))

	return paths
}

// CreateCacheDir creates the cache directory for a specific purpose
func (x *XDGDirs) CreateCacheDir(purpose string) error {
	cachePath := x.GetCachePath(purpose)

	slog.Debug("creating cache directory", "path", cachePath)

	err := os.MkdirAll(cachePath, 0755)
	if err != nil {
		slog.Error("failed to create cache directory", "path", cachePath, "error", err)
		return err
	}

	slog.Info("cache directory created successfully", "path", cachePath)
	return nil
}

// FindSoundFile searches for a sound file in soundpack directories
// Returns the full path to the first existing file, or empty string if not found
func (x *XDGDirs) FindSoundFile(soundpackID, relativePath string) string {
	if soundpackID == "" || relativePath == "" {
		slog.Debug("empty soundpack ID or relative path", "soundpack_id", soundpackID, "relative_path", relativePath)
		return ""
	}

	// Sanitize the relative path to prevent directory traversal
	relativePath = sanitizePath(relativePath)
	if relativePath == "" {
		slog.Warn("relative path was empty after sanitization")
		return ""
	}

	soundpackPaths := x.GetSoundpackPaths(soundpackID)

	slog.Debug("searching for sound file",
		"soundpack_id", soundpackID,
		"relative_path", relativePath,
		"search_paths", len(soundpackPaths))

	for i, basePath := range soundpackPaths {
		fullPath := filepath.Join(basePath, relativePath)

		slog.Debug("checking sound file path",
			"path_index", i,
			"base_path", basePath,
			"full_path", fullPath)

		if _, err := os.Stat(fullPath); err == nil {
			slog.Info("sound file found",
				"soundpack_id", soundpackID,
				"relative_path", relativePath,
				"full_path", fullPath,
				"path_index", i)
			return fullPath
		} else {
			slog.Debug("sound file not found at path",
				"full_path", fullPath,
				"error", err)
		}
	}

	slog.Debug("sound file not found in any path",
		"soundpack_id", soundpackID,
		"relative_path", relativePath)

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
