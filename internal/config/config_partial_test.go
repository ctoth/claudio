package config

import (
	"testing"

	"github.com/spf13/afero"
)

// loadPartial writes body to an in-memory config file and loads it.
func loadPartial(t *testing.T, body string) (*Config, *ConfigManager) {
	t.Helper()
	memFS := afero.NewMemMapFs()
	const configPath = "/cfg/config.json"
	if err := afero.WriteFile(memFS, configPath, []byte(body), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cm := NewConfigManagerWithFilesystem(memFS)
	cfg, err := cm.LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("LoadFromFile(%s) error: %v", body, err)
	}
	return cfg, cm
}

// A config file that sets only some keys must inherit defaults for the rest.
// Before this fix, omitted keys decoded as zero values: a missing "enabled"
// muted Claudio, a missing "default_soundpack" failed validation, and a
// missing "file_logging" disabled file logging.
func TestLoadFromFilePartialConfigInheritsDefaults(t *testing.T) {
	cfg, cm := loadPartial(t, `{"volume": 0.3}`)
	defaults := cm.GetDefaultConfig()

	if cfg.Volume == nil || *cfg.Volume != 0.3 {
		t.Errorf("volume = %v, want 0.3", cfg.Volume)
	}
	if !cfg.Enabled {
		t.Error("enabled = false, want true (default) when key is omitted")
	}
	if cfg.DefaultSoundpack != defaults.DefaultSoundpack {
		t.Errorf("default_soundpack = %q, want default %q", cfg.DefaultSoundpack, defaults.DefaultSoundpack)
	}
	if cfg.LogLevel != defaults.LogLevel {
		t.Errorf("log_level = %q, want default %q", cfg.LogLevel, defaults.LogLevel)
	}
	if cfg.AudioBackend != defaults.AudioBackend {
		t.Errorf("audio_backend = %q, want default %q", cfg.AudioBackend, defaults.AudioBackend)
	}
	if cfg.FileLogging == nil || !cfg.FileLogging.Enabled {
		t.Errorf("file_logging = %+v, want default enabled config", cfg.FileLogging)
	}
	if cfg.SoundTracking == nil || cfg.SoundTracking.Enabled != defaults.SoundTracking.Enabled {
		t.Errorf("sound_tracking = %+v, want default %+v", cfg.SoundTracking, defaults.SoundTracking)
	}
}

func TestLoadFromFileEmptyObjectEqualsDefaults(t *testing.T) {
	cfg, cm := loadPartial(t, `{}`)
	defaults := cm.GetDefaultConfig()
	if !cfg.Enabled || cfg.DefaultSoundpack != defaults.DefaultSoundpack {
		t.Errorf("empty config = %+v, want defaults %+v", cfg, defaults)
	}
	// Omitted volume stays nil ("use the default"), so `claudio volume` can
	// report that nothing is persisted.
	if cfg.Volume != nil {
		t.Errorf("volume = %v, want nil when omitted", *cfg.Volume)
	}
}

func TestLoadFromFileExplicitValuesOverrideDefaults(t *testing.T) {
	cfg, _ := loadPartial(t, `{"enabled": false, "file_logging": null}`)
	if cfg.Enabled {
		t.Error("explicit enabled=false was overridden by the default")
	}
	if cfg.FileLogging != nil {
		t.Errorf("explicit file_logging=null was overridden: %+v", cfg.FileLogging)
	}
}

func TestLoadFromFilePartialNestedFileLogging(t *testing.T) {
	cfg, _ := loadPartial(t, `{"file_logging": {"max_size_mb": 20}}`)
	if cfg.FileLogging == nil {
		t.Fatal("file_logging = nil")
	}
	if cfg.FileLogging.MaxSizeMB != 20 {
		t.Errorf("max_size_mb = %d, want 20", cfg.FileLogging.MaxSizeMB)
	}
	if !cfg.FileLogging.Enabled || cfg.FileLogging.MaxBackups != 5 || !cfg.FileLogging.Compress {
		t.Errorf("omitted file_logging fields lost their defaults: %+v", cfg.FileLogging)
	}
}
