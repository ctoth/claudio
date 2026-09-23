package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"claudio.click/internal/testutil/wavfixture"
)

// TestLookupSoundpackDoesNotWalkPackDirectories pins that resolving a
// soundpack name (the hook hot path) never counts audio files: only
// `soundpack list` needs SoundCount.
func TestLookupSoundpackDoesNotWalkPackDirectories(t *testing.T) {
	dataDir, _, cleanup := setupInstallTestEnv(t)
	defer cleanup()

	xdgPack := filepath.Join(dataDir, "claudio", "soundpacks", "xdg-dir-pack")
	wavfixture.Write(t, filepath.Join(xdgPack, "success", "nested", "tone.wav"))
	configPack := filepath.Join(t.TempDir(), "config-dir-pack")
	wavfixture.Write(t, filepath.Join(configPack, "error", "deep", "tone.wav"))

	orig := countAudioFilesInDir
	countAudioFilesInDir = func(dir string) int {
		t.Errorf("lookup walked pack directory %s", dir)
		return 0
	}
	defer func() { countAudioFilesInDir = orig }()

	configPaths := []string{configPack}
	for _, name := range []string{"xdg-dir-pack", "config-dir-pack"} {
		pack, ok := lookupSoundpack(name, configPaths)
		if !ok || pack.Type != "directory" {
			t.Fatalf("lookupSoundpack(%q) = %#v, %v; want directory pack", name, pack, ok)
		}
	}
	if _, ok := lookupSoundpack("no-such-pack", configPaths); ok {
		t.Fatal("unknown name resolved")
	}
}

// TestSoundpackListStillCountsDirectoryPackSounds pins that list output
// keeps its SOUNDS column after lookup stopped computing counts.
func TestSoundpackListStillCountsDirectoryPackSounds(t *testing.T) {
	dataDir, _, cleanup := setupInstallTestEnv(t)
	defer cleanup()

	xdgPack := filepath.Join(dataDir, "claudio", "soundpacks", "xdg-dir-pack")
	wavfixture.Write(t, filepath.Join(xdgPack, "success", "a.wav"))
	wavfixture.Write(t, filepath.Join(xdgPack, "error", "nested", "b.wav"))

	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	if code := NewCLI().Run([]string{"claudio", "soundpack", "list"}, nil, stdout, stderr); code != 0 {
		t.Fatalf("list failed: %s", stderr)
	}
	for line := range strings.SplitSeq(stdout.String(), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 3 && fields[0] == "xdg-dir-pack" {
			if fields[2] != "2" {
				t.Fatalf("xdg-dir-pack SOUNDS = %s, want 2; line %q", fields[2], line)
			}
			return
		}
	}
	t.Fatalf("xdg-dir-pack not listed:\n%s", stdout)
}
