//go:build cgo

package malgo

import (
	"bytes"
	"context"
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

	decoder := NewWavDecoder()
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(soundsDir, name))
			if err != nil {
				t.Fatalf("read embedded sound: %v", err)
			}

			audioData, err := decoder.Decode(context.Background(), bytes.NewReader(data))
			if err != nil {
				t.Fatalf("decode %s: %v", name, err)
			}
			if len(audioData.Samples) == 0 {
				t.Errorf("%s decoded to zero samples", name)
			}
			if audioData.SampleRate == 0 || audioData.Channels == 0 {
				t.Errorf("%s decoded with invalid format: rate=%d channels=%d",
					name, audioData.SampleRate, audioData.Channels)
			}
		})
	}
}
