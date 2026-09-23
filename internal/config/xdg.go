package config

import (
	"path/filepath"

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
