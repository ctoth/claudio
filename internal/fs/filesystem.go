package fs

import (
	"os"
	"path/filepath"
)

// ExecutablePath returns the current executable path with filesystem abstraction support
// This is a utility function that can be easily mocked in tests
func ExecutablePath() (string, error) {
	p, err := os.Executable()
	if err != nil {
		return "", err
	}
	// Convert to forward slashes so the path works in bash (Claude Code
	// passes hook commands through /usr/bin/bash on all platforms).
	return filepath.ToSlash(p), nil
}
