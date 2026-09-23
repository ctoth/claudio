package install

import (
	"os"
	"path/filepath"
	"strings"
)

// FindCodexHooksPaths returns candidate ~/.codex/hooks.json paths for the scope, in priority order.
func FindCodexHooksPaths(scope string) ([]string, error) {
	normalizedScope, err := NormalizeScope(scope)
	if err != nil {
		return nil, err
	}
	if normalizedScope == ScopeGlobal {
		return findCodexUserScopePaths()
	}
	return []string{filepath.Join(".codex", "hooks.json")}, nil
}

func findCodexUserScopePaths() ([]string, error) {
	// Codex reads its configuration only from CODEX_HOME when it is set, so
	// that is the only candidate: falling back to ~/.codex would write hooks
	// Codex never loads (mirrors CLAUDE_CONFIG_DIR handling for Claude).
	if codexHome := strings.TrimSpace(os.Getenv("CODEX_HOME")); codexHome != "" {
		return []string{filepath.Join(codexHome, "hooks.json")}, nil
	}

	return homeScopedPaths(".codex", "hooks.json")
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
