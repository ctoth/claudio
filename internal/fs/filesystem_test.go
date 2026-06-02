package fs

import (
	"testing"
)

func TestExecutablePathFunction(t *testing.T) {
	path, err := ExecutablePath()
	if err != nil {
		t.Errorf("ExecutablePath failed: %v", err)
	}

	if path == "" {
		t.Error("Expected non-empty executable path")
	}
}
