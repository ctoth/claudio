package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/spf13/afero"
)

// An empty config file (including --config NUL / /dev/null) means "no
// settings": it loads as defaults instead of a JSON parse error.
func TestLoadFromFile_EmptyFileIsDefaults(t *testing.T) {
	empty := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(empty, []byte(" \n\t"), 0o644); err != nil {
		t.Fatal(err)
	}
	nullDevice := "/dev/null"
	if runtime.GOOS == "windows" {
		nullDevice = "NUL"
	}

	cm := NewConfigManager()
	for _, path := range []string{empty, nullDevice} {
		cfg, err := cm.LoadFromFile(path)
		if err != nil {
			t.Fatalf("LoadFromFile(%s): %v", path, err)
		}
		def := cm.GetDefaultConfig()
		if cfg.Enabled != def.Enabled || cfg.DefaultSoundpack != def.DefaultSoundpack || cfg.LogLevel != def.LogLevel {
			t.Errorf("LoadFromFile(%s) = %+v, want defaults", path, cfg)
		}
		if cfg.Volume != nil {
			t.Errorf("LoadFromFile(%s) volume = %v, want nil (no persisted setting)", path, *cfg.Volume)
		}
	}
}

// stubConfigPaths is an XDGInterface whose config search list is fixed.
type stubConfigPaths struct {
	XDGInterface
	paths []string
}

func (s stubConfigPaths) GetConfigPaths(string) []string { return s.paths }

func TestFindConfigFile_ReturnsFirstExisting(t *testing.T) {
	fs := afero.NewMemMapFs()
	cm := NewConfigManagerWithFilesystem(fs)
	cm.xdg = stubConfigPaths{paths: []string{"/user/config.json", "/sys1/config.json", "/sys2/config.json"}}

	if got := cm.FindConfigFile(); got != "" {
		t.Fatalf("FindConfigFile() = %q with no config files", got)
	}
	for _, p := range []string{"/sys2/config.json", "/sys1/config.json"} {
		if err := afero.WriteFile(fs, p, []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if got := cm.FindConfigFile(); got != "/sys1/config.json" {
		t.Errorf("FindConfigFile() = %q, want the first existing path /sys1/config.json", got)
	}
}
