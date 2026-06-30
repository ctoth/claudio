package install

import (
	"os"
	"path/filepath"
)

// FindGeminiSettingsPaths returns candidate Gemini settings.json paths for
// global or project scope, in priority order. The legacy user scope is accepted
// as an alias for global.
func FindGeminiSettingsPaths(scope string) ([]string, error) {
	normalizedScope, err := NormalizeScope(scope)
	if err != nil {
		return nil, err
	}
	if normalizedScope == ScopeGlobal {
		return findGeminiGlobalScopePaths(), nil
	}
	return []string{
		filepath.Join(".", ".gemini", "settings.json"),
		filepath.Join(".gemini", "settings.json"),
	}, nil
}

func findGeminiGlobalScopePaths() []string {
	var paths []string

	homeDir := getHomeDirectory()
	if homeDir != "" {
		paths = append(paths, filepath.Join(homeDir, ".gemini", "settings.json"))
	}

	paths = appendUserProfilePath(paths, homeDir, ".gemini", "settings.json")

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
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return paths[0], nil
}
