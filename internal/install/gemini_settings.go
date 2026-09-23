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
		return findGeminiGlobalScopePaths()
	}
	return []string{filepath.Join(".gemini", "settings.json")}, nil
}

func findGeminiGlobalScopePaths() ([]string, error) {
	return homeScopedPaths(".gemini", "settings.json")
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
