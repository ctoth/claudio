package install

import (
	"os"
	"path/filepath"
)

// FindCodexHooksPaths returns candidate ~/.codex/hooks.json paths for the scope, in priority order.
func FindCodexHooksPaths(scope string) ([]string, error) {
	normalizedScope, err := NormalizeScope(scope)
	if err != nil {
		return nil, err
	}
	if normalizedScope == ScopeGlobal {
		return findCodexUserScopePaths(), nil
	}
	return []string{
		filepath.Join(".", ".codex", "hooks.json"),
		filepath.Join(".codex", "hooks.json"),
	}, nil
}

func findCodexUserScopePaths() []string {
	var paths []string

	codexHome := os.Getenv("CODEX_HOME")
	if codexHome != "" {
		paths = append(paths, filepath.Join(codexHome, "hooks.json"))
	}

	homeDir := getHomeDirectory()
	if homeDir != "" {
		paths = append(paths, filepath.Join(homeDir, ".codex", "hooks.json"))
	}

	paths = appendUserProfilePath(paths, homeDir, ".codex", "hooks.json")

	if len(paths) == 0 {
		paths = append(paths, filepath.Join("~", ".codex", "hooks.json"))
	}

	return paths
}

// FindBestCodexPath returns the first existing Codex hooks path, or the first candidate for creation.
func FindBestCodexPath(scope string) (string, error) {
	paths, err := FindCodexHooksPaths(scope)
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
