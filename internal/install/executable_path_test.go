package install

import (
	"strings"
	"testing"
)

func TestGetExecutablePath(t *testing.T) {
	path, err := GetExecutablePath()
	if err != nil {
		t.Errorf("GetExecutablePath failed: %v", err)
	}

	if path == "" {
		t.Error("Expected non-empty executable path")
	}
	if strings.Contains(path, `\`) {
		t.Errorf("GetExecutablePath = %q, want forward slashes only", path)
	}
}
