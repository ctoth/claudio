package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"claudio.click/internal/config"
	"claudio.click/internal/soundpack"
)

func writeNamedJSONPack(t *testing.T, path, name, sound string) {
	t.Helper()
	data, err := json.Marshal(soundpack.JSONSoundpackFile{Name: name, Mappings: map[string]string{"default.wav": sound}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
}

// TestEverySoundpackUseAcceptsResolvesAtRuntime lays out one pack of each
// shape discovery knows about, then checks that for every name `soundpack
// use` accepts, the runtime loads that same pack rather than falling back.
func TestEverySoundpackUseAcceptsResolvesAtRuntime(t *testing.T) {
	dataDir, configDir, cleanup := setupInstallTestEnv(t)
	defer cleanup()
	packsDir := filepath.Join(dataDir, "claudio", "soundpacks")

	// XDG directory whose manifest name differs from the directory name.
	metaDir := filepath.Join(packsDir, "dir-name")
	createDummyWAV(t, filepath.Join(metaDir, "meta.wav"))
	writeNamedJSONPack(t, filepath.Join(metaDir, "soundpack.json"), "meta-name", "meta.wav")

	// Loose JSON file directly in the XDG soundpacks directory.
	createDummyWAV(t, filepath.Join(packsDir, "loose.wav"))
	writeNamedJSONPack(t, filepath.Join(packsDir, "loose-file.json"), "loose-pack", "loose.wav")

	// Loose JSON file in the parent claudio data directory.
	createDummyWAV(t, filepath.Join(dataDir, "claudio", "parent.wav"))
	writeNamedJSONPack(t, filepath.Join(dataDir, "claudio", "parent-file.json"), "parent-pack", "parent.wav")

	// Plain directory pack in XDG.
	createDummyWAV(t, filepath.Join(packsDir, "plain-dir", "default.wav"))

	// Directory and JSON packs referenced from config soundpack_paths.
	extDir := filepath.Join(t.TempDir(), "ext-dir")
	createDummyWAV(t, filepath.Join(extDir, "default.wav"))
	extJSONDir := t.TempDir()
	createDummyWAV(t, filepath.Join(extJSONDir, "ext.wav"))
	extJSON := filepath.Join(extJSONDir, "ext-file.json")
	writeNamedJSONPack(t, extJSON, "ext-json", "ext.wav")

	cm := config.NewConfigManager()
	cfg := cm.GetDefaultConfig()
	cfg.SoundpackPaths = []string{extDir, extJSON}
	configPath := filepath.Join(configDir, "claudio", "config.json")
	if err := cm.SaveToFile(cfg, configPath); err != nil {
		t.Fatal(err)
	}

	packs, err := discoverSoundpacks()
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, pack := range packs {
		if pack.Type == "embedded" {
			continue
		}
		t.Run(pack.Name, func(t *testing.T) {
			stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
			if code := NewCLI().Run([]string{"claudio", "soundpack", "use", pack.Name}, nil, stdout, stderr); code != 0 {
				t.Fatalf("use %q failed: stdout=%q stderr=%q", pack.Name, stdout, stderr)
			}
			runtimeCfg, err := cm.LoadFromFile(configPath)
			if err != nil {
				t.Fatal(err)
			}
			runtimeCfg.Enabled = false
			runtimeCLI := NewCLI()
			if err := runtimeCLI.initializeAudioSystem(runtimeCfg); err != nil {
				t.Fatalf("runtime init failed: %v", err)
			}
			resolved, err := runtimeCLI.soundpackResolver.ResolveSound("default.wav")
			if err != nil {
				t.Fatalf("runtime could not resolve default.wav for %q (listed at %s): %v", pack.Name, pack.Path, err)
			}
			packRoot := pack.Path
			if strings.EqualFold(filepath.Ext(packRoot), ".json") {
				packRoot = filepath.Dir(packRoot)
			}
			if !samePathOrWithin(resolved, packRoot) {
				t.Fatalf("runtime resolved %q to %s, outside the listed pack %s", pack.Name, resolved, pack.Path)
			}
		})
		checked++
	}
	for _, want := range []string{"meta-name", "loose-pack", "parent-pack", "plain-dir", "ext-dir", "ext-json"} {
		found := false
		for _, pack := range packs {
			found = found || pack.Name == want
		}
		if !found {
			t.Errorf("discovery did not list %q: %#v", want, packs)
		}
	}
	if checked < 6 {
		t.Fatalf("expected at least 6 non-embedded packs, checked %d", checked)
	}
}

func TestSoundpackNamePrefersListedPackOverWorkingDirectory(t *testing.T) {
	dataDir, _, cleanup := setupInstallTestEnv(t)
	defer cleanup()
	createDummyWAV(t, filepath.Join(dataDir, "claudio", "soundpacks", "shadowed", "default.wav"))

	cwd := t.TempDir()
	createDummyWAV(t, filepath.Join(cwd, "shadowed", "default.wav"))
	t.Chdir(cwd)

	cfg := config.NewConfigManager().GetDefaultConfig()
	cfg.DefaultSoundpack = "shadowed"
	cfg.Enabled = false
	cli := NewCLI()
	if err := cli.initializeAudioSystem(cfg); err != nil {
		t.Fatal(err)
	}
	resolved, err := cli.soundpackResolver.ResolveSound("default.wav")
	if err != nil {
		t.Fatal(err)
	}
	if !samePathOrWithin(resolved, filepath.Join(dataDir, "claudio", "soundpacks", "shadowed")) {
		t.Fatalf("name resolved to working-directory pack %s", resolved)
	}
}
