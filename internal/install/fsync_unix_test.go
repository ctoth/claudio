//go:build !windows

package install

import (
	"path/filepath"
	"testing"
)

func TestFsyncDirReturnsOpenError(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	if err := fsyncDir(missing); err == nil {
		t.Fatal("expected error for missing directory")
	}
}
