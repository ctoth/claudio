package install

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

// FindBestSettingsPath returns the first settings path where the file actually exists,
// or the first path in the list if no file exists yet (for creation).
// This prevents silent success when the primary path is wrong (e.g. MSYS path on Windows).
func FindBestSettingsPath(scope string) (string, error) {
	paths, err := FindClaudeSettingsPaths(scope)
	if err != nil {
		return "", err
	}
	if len(paths) == 0 {
		return "", fmt.Errorf("no settings paths found for scope: %s", scope)
	}

	// Return the first path where the file actually exists
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	// No existing file found — return the first path for creation
	return paths[0], nil
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
	if runtime.GOOS == "windows" {
		// On Windows, prefer USERPROFILE (canonical Windows home directory).
		// HOME may contain MSYS/Git Bash-style paths (e.g. /c/Users/Q)
		// that are not valid native Windows paths for Go's filepath operations.
		if userProfile := os.Getenv("USERPROFILE"); userProfile != "" {
			return userProfile
		}
		// Fall back to HOME with MSYS path normalization
		if home := os.Getenv("HOME"); home != "" {
			return normalizeMSYSPath(home)
		}
		// Try HOMEDRIVE + HOMEPATH combination (Windows alternative)
		if homeDrive := os.Getenv("HOMEDRIVE"); homeDrive != "" {
			if homePath := os.Getenv("HOMEPATH"); homePath != "" {
				return homeDrive + homePath
			}
		}
		return ""
	}

	// Unix/Linux/macOS: HOME is the standard
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	return ""
}

// normalizeMSYSPath converts MSYS/Git Bash-style paths (e.g. /c/Users/Q) to
// native Windows paths (e.g. C:\Users\Q). Returns the path unchanged if it
// doesn't match the MSYS pattern.
func normalizeMSYSPath(path string) string {
	// MSYS pattern: /X/... where X is a single drive letter
	if len(path) >= 3 && path[0] == '/' && path[2] == '/' &&
		((path[1] >= 'a' && path[1] <= 'z') || (path[1] >= 'A' && path[1] <= 'Z')) {
		drive := strings.ToUpper(string(path[1]))
		return drive + ":" + filepath.FromSlash(path[2:])
	}
	return path
}
