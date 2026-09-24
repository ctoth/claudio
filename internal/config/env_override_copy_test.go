package config

import "testing"

// TestApplyEnvironmentOverrides_DoesNotMutateCaller guards against the
// shallow-copy bug where overriding CLAUDIO_FILE_LOGGING wrote through the
// shared *FileLoggingConfig pointer into the caller's config.
func TestApplyEnvironmentOverrides_DoesNotMutateCaller(t *testing.T) {
	t.Setenv("CLAUDIO_FILE_LOGGING", "false")
	t.Setenv("CLAUDIO_SOUND_TRACKING", "false")
	t.Setenv("CLAUDIO_VOLUME", "0.9")

	vol := 0.3
	original := &Config{
		Volume:         &vol,
		SoundpackPaths: []string{"/a"},
		FileLogging:    &FileLoggingConfig{Enabled: true, MaxSizeMB: 10},
		SoundTracking:  &SoundTrackingConfig{Enabled: true},
	}

	result := NewConfigManager().ApplyEnvironmentOverrides(original)

	if result.FileLogging.Enabled {
		t.Fatal("override not applied: result FileLogging.Enabled = true")
	}
	if !original.FileLogging.Enabled {
		t.Error("caller's FileLogging.Enabled was mutated to false")
	}
	if result.FileLogging == original.FileLogging {
		t.Error("result shares the caller's FileLogging pointer")
	}
	if result.SoundTracking == original.SoundTracking {
		t.Error("result shares the caller's SoundTracking pointer")
	}
	if !original.SoundTracking.Enabled {
		t.Error("caller's SoundTracking.Enabled was mutated")
	}
	if *original.Volume != 0.3 {
		t.Errorf("caller's Volume mutated to %v", *original.Volume)
	}
	if result.FileLogging.MaxSizeMB != 10 {
		t.Errorf("deep copy lost fields: MaxSizeMB = %d", result.FileLogging.MaxSizeMB)
	}
	result.SoundpackPaths[0] = "/changed"
	if original.SoundpackPaths[0] != "/a" {
		t.Error("result shares the caller's SoundpackPaths backing array")
	}
}
