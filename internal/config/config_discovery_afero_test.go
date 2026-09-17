package config

import (
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
)

func TestLoadConfigDiscoversConfigOnInjectedFilesystem(t *testing.T) {
	memFS := afero.NewMemMapFs()
	configPath := "/memory/config.json"
	data := []byte(`{
		"volume": 0.73,
		"default_soundpack": "memory-discovery",
		"enabled": true,
		"log_level": "info",
		"audio_backend": "auto"
	}`)
	if err := memFS.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := afero.WriteFile(memFS, configPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	cm := NewConfigManagerWithFilesystem(memFS)
	cm.xdg = &MockXDGDirs{configPaths: []string{configPath}}
	got, err := cm.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if got.DefaultSoundpack != "memory-discovery" || got.Volume == nil || *got.Volume != 0.73 {
		t.Fatalf("LoadConfig ignored injected filesystem config: %+v", got)
	}
}
