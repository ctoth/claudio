package native

import (
	"os"
	"path/filepath"
	"testing"
)

// TestEmbeddedDefaultSoundsDecode guards the synthesized tones that ship as
// the native-Linux default pack: they must be real, decodable 16-bit PCM, not
// just files that happen to exist. The CLI's extraction makes them resolve;
// this makes sure that what resolves is actually playable. Regenerate the
// files with internal/config/embedded_sounds/generate.go.
func TestEmbeddedDefaultSoundsDecode(t *testing.T) {
	soundsDir := filepath.Join("..", "..", "config", "embedded_sounds")
	names := []string{
		"default-success.wav",
		"default-error.wav",
		"default-loading.wav",
		"default-interactive.wav",
		"default.wav",
	}

	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(soundsDir, name))
			if err != nil {
				t.Fatalf("read embedded sound: %v", err)
			}

			frames, rate, err := decodeAll(t, name, data)
			if err != nil {
				t.Fatalf("decode %s: %v", name, err)
			}
			if len(frames) == 0 || rate == 0 {
				t.Errorf("%s decoded to %d frames at %d Hz", name, len(frames), rate)
			}
		})
	}
}
