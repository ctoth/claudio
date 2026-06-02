package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/afero"
)

// TestWriteConfigFile_RoundTrip verifies that WriteConfigFile writes a
// Config and that the file on disk round-trips back through JSON unmarshal
// with all fields intact.
func TestWriteConfigFile_RoundTrip(t *testing.T) {
	memFS := afero.NewMemMapFs()
	path := "/test/config.json"

	vol := 0.7
	cfg := &Config{
		Volume:           &vol,
		DefaultSoundpack: "test-pack",
		SoundpackPaths:   []string{"/a", "/b"},
		Enabled:          true,
		LogLevel:         "info",
		AudioBackend:     "auto",
	}

	if err := WriteConfigFile(memFS, path, cfg); err != nil {
		t.Fatalf("WriteConfigFile: %v", err)
	}

	exists, err := afero.Exists(memFS, path)
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if !exists {
		t.Fatal("expected config file to exist after WriteConfigFile")
	}

	data, err := afero.ReadFile(memFS, path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var got Config
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal written config: %v", err)
	}

	if got.Volume == nil || *got.Volume != 0.7 {
		t.Errorf("Volume round-trip: got %v, want 0.7", got.Volume)
	}
	if got.DefaultSoundpack != "test-pack" {
		t.Errorf("DefaultSoundpack: got %q, want %q", got.DefaultSoundpack, "test-pack")
	}
	if !got.Enabled {
		t.Errorf("Enabled: got %v, want true", got.Enabled)
	}
	if got.LogLevel != "info" {
		t.Errorf("LogLevel: got %q, want %q", got.LogLevel, "info")
	}
}

// TestWriteConfigFile_CreatesParentDir verifies WriteConfigFile creates
// the parent directory if missing.
func TestWriteConfigFile_CreatesParentDir(t *testing.T) {
	memFS := afero.NewMemMapFs()
	path := "/deeply/nested/config/config.json"

	cfg := &Config{DefaultSoundpack: "x", Enabled: true, AudioBackend: "auto"}

	if err := WriteConfigFile(memFS, path, cfg); err != nil {
		t.Fatalf("WriteConfigFile: %v", err)
	}

	exists, err := afero.Exists(memFS, path)
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if !exists {
		t.Fatal("expected config file to exist in newly-created dir")
	}
}

// TestWriteConfigFile_AtomicWriteAndBackup verifies that a second write
// produces a .bak file whose content equals the FIRST write's content.
func TestWriteConfigFile_AtomicWriteAndBackup(t *testing.T) {
	memFS := afero.NewMemMapFs()
	path := "/backup-test/config.json"

	vol1 := 0.3
	first := &Config{Volume: &vol1, DefaultSoundpack: "v1", Enabled: true, AudioBackend: "auto"}
	vol2 := 0.8
	second := &Config{Volume: &vol2, DefaultSoundpack: "v2", Enabled: false, AudioBackend: "auto"}

	if err := WriteConfigFile(memFS, path, first); err != nil {
		t.Fatalf("first write: %v", err)
	}

	firstBytes, err := afero.ReadFile(memFS, path)
	if err != nil {
		t.Fatalf("read after first write: %v", err)
	}

	if err := WriteConfigFile(memFS, path, second); err != nil {
		t.Fatalf("second write: %v", err)
	}

	bakPath := path + ".bak"
	exists, err := afero.Exists(memFS, bakPath)
	if err != nil {
		t.Fatalf("check .bak: %v", err)
	}
	if !exists {
		t.Fatal("expected .bak after second write")
	}

	bakBytes, err := afero.ReadFile(memFS, bakPath)
	if err != nil {
		t.Fatalf("read .bak: %v", err)
	}
	if string(bakBytes) != string(firstBytes) {
		t.Errorf(".bak does not match first write\n  bak:  %s\n  want: %s", bakBytes, firstBytes)
	}

	// Assert no temp-file residue remains.
	residue := false
	_ = afero.Walk(memFS, filepath.Dir(path), func(p string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		base := filepath.Base(p)
		if strings.HasPrefix(base, ".config-") && strings.HasSuffix(base, ".tmp") {
			residue = true
		}
		if strings.HasPrefix(base, ".config-bak-") && strings.HasSuffix(base, ".tmp") {
			residue = true
		}
		return nil
	})
	if residue {
		t.Error("temp file residue remained after WriteConfigFile")
	}
}

// TestWriteConfigFile_NoBackupOnFirstWrite verifies writing to a fresh
// path does not create a .bak file (nothing to back up).
func TestWriteConfigFile_NoBackupOnFirstWrite(t *testing.T) {
	memFS := afero.NewMemMapFs()
	path := "/first-write/config.json"

	cfg := &Config{DefaultSoundpack: "x", Enabled: true, AudioBackend: "auto"}

	if err := WriteConfigFile(memFS, path, cfg); err != nil {
		t.Fatalf("write: %v", err)
	}

	exists, err := afero.Exists(memFS, path+".bak")
	if err != nil {
		t.Fatalf("check .bak: %v", err)
	}
	if exists {
		t.Error("expected no .bak after first write to fresh path")
	}
}

// TestWriteConfigFile_RefusesBackupOfCorruptJSON verifies that when the
// target file is not valid JSON, BackupConfigFile refuses to overwrite
// any existing .bak — preserving the last-known-good copy.
func TestWriteConfigFile_RefusesBackupOfCorruptJSON(t *testing.T) {
	memFS := afero.NewMemMapFs()
	path := "/corrupt-test/config.json"
	bakPath := path + ".bak"

	if err := memFS.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Pre-existing valid .bak that MUST be preserved.
	knownGood := []byte(`{"default_soundpack":"keep-me","enabled":true}`)
	if err := afero.WriteFile(memFS, bakPath, knownGood, 0644); err != nil {
		t.Fatalf("seed .bak: %v", err)
	}

	// Corrupt target file (not valid JSON).
	if err := afero.WriteFile(memFS, path, []byte(`{this is not valid json`), 0644); err != nil {
		t.Fatalf("seed corrupt config: %v", err)
	}

	newCfg := &Config{DefaultSoundpack: "new", Enabled: true, AudioBackend: "auto"}
	if err := WriteConfigFile(memFS, path, newCfg); err != nil {
		t.Fatalf("write should succeed even with corrupt prior config: %v", err)
	}

	gotBak, err := afero.ReadFile(memFS, bakPath)
	if err != nil {
		t.Fatalf("read .bak: %v", err)
	}
	if string(gotBak) != string(knownGood) {
		t.Errorf(".bak was overwritten with corrupt content\n  got:  %s\n  want: %s",
			gotBak, knownGood)
	}
}

// TestBackupConfigFile_NoOpWhenSourceMissing verifies BackupConfigFile
// silently returns when the source file does not exist.
func TestBackupConfigFile_NoOpWhenSourceMissing(t *testing.T) {
	memFS := afero.NewMemMapFs()
	path := "/missing/config.json"

	// Should not panic, should not error, should not create .bak.
	BackupConfigFile(memFS, path)

	exists, _ := afero.Exists(memFS, path+".bak")
	if exists {
		t.Error("expected no .bak when source does not exist")
	}
}
