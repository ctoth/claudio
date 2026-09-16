package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSoundpackInstallRespectsNameLock(t *testing.T) {
	dataDir, _, cleanup := setupInstallTestEnv(t)
	defer cleanup()
	source := filepath.Join(t.TempDir(), "locked-pack")
	createDummyWAV(t, filepath.Join(source, "default.wav"))
	lock, err := lockSoundpackName("locked-pack")
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Unlock()
	var stdout, stderr bytes.Buffer
	code := NewCLI().Run([]string{"claudio", "soundpack", "install", source}, nil, &stdout, &stderr)
	if code == 0 || !strings.Contains(stderr.String(), "already running") {
		t.Fatalf("install bypassed name lock: code=%d stderr=%s", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(dataDir, "claudio", "soundpacks", "locked-pack")); !os.IsNotExist(err) {
		t.Fatalf("blocked install changed destination: %v", err)
	}
}

func TestSoundpackNameLockRejectsTraversal(t *testing.T) {
	_, _, cleanup := setupInstallTestEnv(t)
	defer cleanup()
	lock, err := lockSoundpackName("../outside")
	if err == nil {
		_ = lock.Unlock()
		t.Fatal("accepted traversal in lock name")
	}
}
