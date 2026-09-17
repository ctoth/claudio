package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestSoundpackInstallDirectoryAcrossWindowsVolumes(t *testing.T) {
	dataDir, _, cleanup := setupInstallTestEnv(t)
	defer cleanup()
	sourceRoot := t.TempDir()
	createDummyWAV(t, filepath.Join(sourceRoot, "cross-volume", "default.wav"))

	// SUBST exercises Windows volume semantics without requiring a second disk.
	var drive string
	for letter := 'Z'; letter >= 'D'; letter-- {
		candidate := string(letter) + ":"
		if _, err := os.Stat(candidate + `\`); os.IsNotExist(err) {
			drive = candidate
			break
		}
	}
	if drive == "" {
		t.Fatal("no unused drive letter available for cross-volume regression")
	}
	if output, err := exec.Command("subst", drive, sourceRoot).CombinedOutput(); err != nil {
		t.Fatalf("map temporary source drive: %v: %s", err, output)
	}
	t.Cleanup(func() {
		if output, err := exec.Command("subst", drive, "/D").CombinedOutput(); err != nil {
			t.Errorf("unmap temporary source drive: %v: %s", err, output)
		}
	})
	source := filepath.Join(drive+`\`, "cross-volume")
	var stdout, stderr bytes.Buffer
	if code := NewCLI().Run([]string{"claudio", "soundpack", "install", source}, nil, &stdout, &stderr); code != 0 {
		t.Fatalf("cross-volume install failed: code=%d stderr=%s", code, stderr.String())
	}
	installed := filepath.Join(dataDir, "claudio", "soundpacks", "cross-volume")
	want, err := os.ReadFile(filepath.Join(sourceRoot, "cross-volume", "default.wav"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(installed, "default.wav"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("installed audio differs from source")
	}
	if !isClaudioInstalledSoundpack(installed) {
		t.Fatal("installed directory is missing ownership marker")
	}
}
