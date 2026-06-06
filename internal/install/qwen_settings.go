package install

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// FindQwenSettingsPaths returns candidate Qwen Code settings.json paths for
// global or project scope, in priority order. The legacy user scope is accepted
// as an alias for global.
func FindQwenSettingsPaths(scope string) ([]string, error) {
	normalizedScope, err := NormalizeScope(scope)
	if err != nil {
		return nil, err
	}
	switch normalizedScope {
	case ScopeGlobal:
		return findQwenGlobalScopePaths(), nil
	case ScopeProject:
		return []string{
			filepath.Join(".", ".qwen", "settings.json"),
			filepath.Join(".qwen", "settings.json"),
		}, nil
	default:
		return nil, fmt.Errorf("invalid scope '%s': must be 'global' or 'project'", scope)
	}
}

func findQwenGlobalScopePaths() []string {
	var paths []string

	homeDir := getHomeDirectory()
	if homeDir != "" {
		paths = append(paths, filepath.Join(homeDir, ".qwen", "settings.json"))
	}

	if runtime.GOOS == "windows" {
		userProfile := os.Getenv("USERPROFILE")
		if userProfile != "" && userProfile != homeDir {
			paths = append(paths, filepath.Join(userProfile, ".qwen", "settings.json"))
		}
	}

	if len(paths) == 0 {
		paths = append(paths, filepath.Join("~", ".qwen", "settings.json"))
	}

	return paths
}

// FindBestQwenPath returns the first existing Qwen settings path, or the
// first candidate path when no settings file exists yet.
func FindBestQwenPath(scope string) (string, error) {
	paths, err := FindQwenSettingsPaths(scope)
	if err != nil {
		return "", err
	}
	if len(paths) == 0 {
		return "", fmt.Errorf("no qwen settings paths found for scope: %s", scope)
	}
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return paths[0], nil
}
