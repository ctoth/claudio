package config

import (
	"testing"

	"github.com/spf13/afero"
)

// "fake" is a test double that lives in internal/audio/audiotest, outside
// the production binary. A real config naming it must fail validation, and
// the env override must ignore it, instead of failing at playback.
func TestFakeAudioBackendRejected(t *testing.T) {
	fs := afero.NewMemMapFs()
	if err := afero.WriteFile(fs, "/c/config.json", []byte(`{"audio_backend":"fake"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	mgr := NewConfigManagerWithFilesystem(fs)
	cfg, err := mgr.LoadFromFile("/c/config.json")
	if err == nil {
		err = mgr.ValidateConfig(cfg)
	}
	if err == nil {
		t.Fatal(`config with "audio_backend":"fake" passed validation`)
	}

	t.Setenv("CLAUDIO_AUDIO_BACKEND", "fake")
	if got := mgr.ApplyEnvironmentOverrides(mgr.GetDefaultConfig()).AudioBackend; got != "auto" {
		t.Errorf("CLAUDIO_AUDIO_BACKEND=fake applied: audio_backend = %q, want auto", got)
	}
}
