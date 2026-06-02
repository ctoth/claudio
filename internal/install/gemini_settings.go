package install

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// FindGeminiSettingsPaths returns candidate Gemini settings.json paths for
// global or project scope, in priority order. The legacy user scope is accepted
// as an alias for global.
func FindGeminiSettingsPaths(scope string) ([]string, error) {
	normalizedScope, err := NormalizeScope(scope)
	if err != nil {
		return nil, err
	}
	switch normalizedScope {
	case ScopeGlobal:
		return findGeminiGlobalScopePaths(), nil
	case ScopeProject:
		return []string{
			filepath.Join(".", ".gemini", "settings.json"),
			filepath.Join(".gemini", "settings.json"),
		}, nil
	default:
		return nil, fmt.Errorf("invalid scope '%s': must be 'global' or 'project'", scope)
	}
}

func findGeminiGlobalScopePaths() []string {
	var paths []string

	homeDir := getHomeDirectory()
	if homeDir != "" {
		paths = append(paths, filepath.Join(homeDir, ".gemini", "settings.json"))
	}

	if runtime.GOOS == "windows" {
		userProfile := os.Getenv("USERPROFILE")
		if userProfile != "" && userProfile != homeDir {
			paths = append(paths, filepath.Join(userProfile, ".gemini", "settings.json"))
		}
	}

	if len(paths) == 0 {
		paths = append(paths, filepath.Join("~", ".gemini", "settings.json"))
	}

	return paths
}

// FindBestGeminiPath returns the first existing Gemini settings path, or the
// first candidate path when no settings file exists yet.
func FindBestGeminiPath(scope string) (string, error) {
	paths, err := FindGeminiSettingsPaths(scope)
	if err != nil {
		return "", err
	}
	if len(paths) == 0 {
		return "", fmt.Errorf("no gemini settings paths found for scope: %s", scope)
	}
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return paths[0], nil
}
