//go:build !windows

package install

import "testing"

func TestFsyncDir_RealOsPath(t *testing.T) {
	dir := t.TempDir()
	if err := fsyncDir(dir); err != nil {
		t.Errorf("fsyncDir on real dir failed: %v", err)
	}
}
