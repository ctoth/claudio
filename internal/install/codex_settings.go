package install

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// FindCodexHooksPaths returns candidate ~/.codex/hooks.json paths for the scope, in priority order.
func FindCodexHooksPaths(scope string) ([]string, error) {
	switch scope {
	case "user":
		return findCodexUserScopePaths(), nil
	case "project":
		return []string{
			filepath.Join(".", ".codex", "hooks.json"),
			filepath.Join(".codex", "hooks.json"),
		}, nil
	default:
		return nil, fmt.Errorf("invalid scope '%s': must be 'user' or 'project'", scope)
	}
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

	if runtime.GOOS == "windows" {
		userProfile := os.Getenv("USERPROFILE")
		if userProfile != "" && userProfile != homeDir {
			paths = append(paths, filepath.Join(userProfile, ".codex", "hooks.json"))
		}
	}

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
	if len(paths) == 0 {
		return "", fmt.Errorf("no codex hooks paths found for scope: %s", scope)
	}
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return paths[0], nil
}
