package install

import (
	"os"
	"path/filepath"
)

// FindQwenSettingsPaths returns candidate Qwen Code settings.json paths for
// global or project scope, in priority order. The legacy user scope is accepted
// as an alias for global.
func FindQwenSettingsPaths(scope string) ([]string, error) {
	normalizedScope, err := NormalizeScope(scope)
	if err != nil {
		return nil, err
	}
	if normalizedScope == ScopeGlobal {
		return findQwenGlobalScopePaths()
	}
	return []string{filepath.Join(".qwen", "settings.json")}, nil
}

func findQwenGlobalScopePaths() ([]string, error) {
	return homeScopedPaths(".qwen", "settings.json")
}

// FindBestQwenPath returns the first existing Qwen settings path, or the
// first candidate path when no settings file exists yet.
func FindBestQwenPath(scope string) (string, error) {
	paths, err := FindQwenSettingsPaths(scope)
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
