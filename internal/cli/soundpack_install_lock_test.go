package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"claudio.click/internal/soundpack/gitpack"
)

func TestSoundpackInstallRespectsNameLock(t *testing.T) {
	dataDir, _, cleanup := setupInstallTestEnv(t)
	defer cleanup()
	source := filepath.Join(t.TempDir(), "locked-pack")
	createDummyWAV(t, filepath.Join(source, "default.wav"))
	lock, err := gitpack.LockName("locked-pack")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := lock.Unlock(); err != nil {
			t.Errorf("unlock soundpack name: %v", err)
		}
	}()
	var stdout, stderr bytes.Buffer
	code := NewCLI().Run([]string{"claudio", "soundpack", "install", source}, nil, &stdout, &stderr)
	if code == 0 || !strings.Contains(stderr.String(), "already running") {
		t.Fatalf("install bypassed name lock: code=%d stderr=%s", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(dataDir, "claudio", "soundpacks", "locked-pack")); !os.IsNotExist(err) {
		t.Fatalf("blocked install changed destination: %v", err)
	}
}
