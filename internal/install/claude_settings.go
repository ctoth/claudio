package install

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// FindClaudeSettingsPaths finds potential Claude Code settings file paths based on scope
// Returns a list of paths in priority order for the given scope (user or project)
func FindClaudeSettingsPaths(scope string) ([]string, error) {
	switch scope {
	case "user":
		return findUserScopePaths()
	case "project":
		return findProjectScopePaths()
	default:
		return nil, fmt.Errorf("invalid scope '%s': must be 'user' or 'project'", scope)
	}
}

// findUserScopePaths returns potential user-scope Claude settings paths
func findUserScopePaths() ([]string, error) {
	var paths []string

	// Get home directory - try multiple environment variables for cross-platform support
	homeDir := getHomeDirectory()
	if homeDir != "" {
		// Primary user settings path: ~/.claude/settings.json
		userPath := filepath.Join(homeDir, ".claude", "settings.json")
		paths = append(paths, userPath)
	}

	// On Windows, also check USERPROFILE if different from HOME
	if runtime.GOOS == "windows" {
		userProfile := os.Getenv("USERPROFILE")
		if userProfile != "" && userProfile != homeDir {
			winPath := filepath.Join(userProfile, ".claude", "settings.json")
			paths = append(paths, winPath)
		}
	}

	// Ensure we have at least one path
	if len(paths) == 0 {
		// Fallback to relative path from home if no environment variables are set
		paths = append(paths, filepath.Join("~", ".claude", "settings.json"))
	}

	return paths, nil
}

// findProjectScopePaths returns potential project-scope Claude settings paths
func findProjectScopePaths() ([]string, error) {
	var paths []string

	// Project settings are relative to current working directory
	// Primary project path: ./.claude/settings.json
	paths = append(paths, filepath.Join(".", ".claude", "settings.json"))

	// Alternative project path without leading dot-slash
	paths = append(paths, filepath.Join(".claude", "settings.json"))

	return paths, nil
}

// getHomeDirectory returns the user's home directory using multiple fallback methods
func getHomeDirectory() string {
	// Try HOME first (Unix/Linux/macOS standard)
	if home := os.Getenv("HOME"); home != "" {
		return home
	}

	// Try USERPROFILE (Windows standard)
	if userProfile := os.Getenv("USERPROFILE"); userProfile != "" {
		return userProfile
	}

	// Try HOMEDRIVE + HOMEPATH combination (Windows alternative)
	if homeDrive := os.Getenv("HOMEDRIVE"); homeDrive != "" {
		if homePath := os.Getenv("HOMEPATH"); homePath != "" {
			return homeDrive + homePath
		}
	}

	// No home directory found
	return ""
}
