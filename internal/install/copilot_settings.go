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
		return findCopilotGlobalScopePaths(), nil
	}
	return []string{
		filepath.Join(".", ".github", "copilot", "settings.local.json"),
		filepath.Join(".github", "copilot", "settings.local.json"),
		filepath.Join(".", ".github", "copilot", "settings.json"),
		filepath.Join(".github", "copilot", "settings.json"),
	}, nil
}

func findCopilotGlobalScopePaths() []string {
	if copilotHome := os.Getenv("COPILOT_HOME"); copilotHome != "" {
		return []string{filepath.Join(copilotHome, "settings.json")}
	}

	var paths []string
	homeDir := getHomeDirectory()
	if homeDir != "" {
		paths = append(paths, filepath.Join(homeDir, ".copilot", "settings.json"))
	}

	paths = appendUserProfilePath(paths, homeDir, ".copilot", "settings.json")

	if len(paths) == 0 {
		paths = append(paths, filepath.Join("~", ".copilot", "settings.json"))
	}

	return paths
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
