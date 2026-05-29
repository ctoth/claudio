//go:build windows

package install

import "testing"

func TestFsyncDir_AlwaysNoOpOnWindows(t *testing.T) {
	if err := fsyncDir("nonexistent-path-does-not-matter"); err != nil {
		t.Errorf("fsyncDir should no-op cleanly on Windows, got: %v", err)
	}
}
