package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"claudio.click/internal/config"
	"claudio.click/internal/soundpack"
	"claudio.click/internal/testutil/wavfixture"
	"github.com/spf13/afero"
)

func TestCanonicalInstalledManifestUsesMetadataNameWithoutPhantomAlias(t *testing.T) {
	dataDir, configDir, cleanup := setupInstallTestEnv(t)
	defer cleanup()

	packDir := filepath.Join(dataDir, "claudio", "soundpacks", "portable-pack")
	wavfixture.Write(t, filepath.Join(packDir, "tone.wav"))
	manifestPath := filepath.Join(packDir, "soundpack.json")
	manifest, err := json.Marshal(soundpack.JSONSoundpackFile{
		Name:     "portable-pack",
		Mappings: map[string]string{"default.wav": "tone.wav"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, manifest, 0644); err != nil {
		t.Fatal(err)
	}
	cfg := config.NewConfigManager().GetDefaultConfig()
	cfg.SoundpackPaths = []string{manifestPath}
	if err := config.WriteConfigFile(afero.NewOsFs(), filepath.Join(configDir, "claudio", "config.json"), cfg); err != nil {
		t.Fatal(err)
	}

	if pack, ok := lookupSoundpack("portable-pack", cfg.SoundpackPaths); !ok || pack.Path != manifestPath {
		t.Fatalf("metadata name did not resolve to canonical manifest: %#v", pack)
	}
	if _, ok := lookupSoundpack("soundpack", cfg.SoundpackPaths); ok {
		t.Fatal("canonical manifest basename created phantom soundpack alias")
	}
	packs, err := discoverSoundpacks()
	if err != nil {
		t.Fatal(err)
	}
	portableCount := 0
	for _, pack := range packs {
		if pack.Name == "soundpack" {
			t.Fatalf("discovery exposed phantom soundpack alias: %#v", packs)
		}
		if pack.Name == "portable-pack" {
			portableCount++
		}
	}
	if portableCount != 1 {
		t.Fatalf("canonical pack discovered %d times, want once: %#v", portableCount, packs)
	}

	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	if code := NewCLI().Run([]string{"claudio", "soundpack", "use", "soundpack"}, nil, stdout, stderr); code == 0 {
		t.Fatalf("phantom manifest basename was accepted, stdout=%q stderr=%q", stdout, stderr)
	}
}
