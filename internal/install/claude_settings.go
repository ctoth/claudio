package install

import (
	"os"
	"path/filepath"
	"strings"
)

// FindClaudeSettingsPaths finds potential Claude Code settings file paths based on scope.
// Returns a list of paths in priority order for global or project scope.
// The legacy user scope is accepted as an alias for global.
func FindClaudeSettingsPaths(scope string) ([]string, error) {
	normalizedScope, err := NormalizeScope(scope)
	if err != nil {
		return nil, err
	}
	if normalizedScope == ScopeGlobal {
		return findUserScopePaths()
	}
	return findProjectScopePaths()
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

	paths = appendUserProfilePath(paths, homeDir, ".claude", "settings.json")

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
