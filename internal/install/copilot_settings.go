package install

import (
	"os"
	"path/filepath"
)

// FindCopilotSettingsPaths returns candidate GitHub Copilot CLI settings paths
// for global or project scope, in priority order. The legacy user scope is
// accepted as an alias for global.
func FindCopilotSettingsPaths(scope string) ([]string, error) {
	normalizedScope, err := NormalizeScope(scope)
	if err != nil {
		return nil, err
	}
	if normalizedScope == ScopeGlobal {
		return findCopilotGlobalScopePaths()
	}
	return []string{
		filepath.Join(".github", "copilot", "settings.local.json"),
		filepath.Join(".github", "copilot", "settings.json"),
	}, nil
}

func findCopilotGlobalScopePaths() ([]string, error) {
	if copilotHome := os.Getenv("COPILOT_HOME"); copilotHome != "" {
		return []string{filepath.Join(copilotHome, "settings.json")}, nil
	}

	return homeScopedPaths(".copilot", "settings.json")
}

// FindBestCopilotPath returns the first existing Copilot settings path, or the
// first candidate path when no settings file exists yet.
func FindBestCopilotPath(scope string) (string, error) {
	paths, err := FindCopilotSettingsPaths(scope)
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
