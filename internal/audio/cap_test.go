//go:build cgo

package audio

import (
	"context"
	"io"
	"strings"
	"testing"

	"claudio.click/internal/safeio"
)

// zeroReader returns the same byte indefinitely. Used to exceed the size
// cap without actually allocating gigabytes of data.
type zeroReader struct {
	remaining int64
}

func (z *zeroReader) Read(p []byte) (int, error) {
	if z.remaining <= 0 {
		return 0, io.EOF
	}
	n := int64(len(p))
	if n > z.remaining {
		n = z.remaining
	}
	for i := int64(0); i < n; i++ {
		p[i] = 0
	}
	z.remaining -= n
	return int(n), nil
}

func TestDecodeWAV_RejectsOversizedInput(t *testing.T) {
	decoder := NewWavDecoder()
	// One byte over the audio cap.
	r := &zeroReader{remaining: safeio.MaxAudioFileBytes + 1}

	_, err := decoder.Decode(context.Background(), r)
	if err == nil {
		t.Fatalf("expected error for oversized WAV input")
	}
	// The decoder maps the cap error to ErrReadFailure (see wav_decoder.go).
	if err != ErrReadFailure {
		t.Errorf("got %v, want ErrReadFailure", err)
	}
}

func TestDecodeAIFF_RejectsOversizedInput(t *testing.T) {
	decoder := NewAiffDecoder()
	r := &zeroReader{remaining: safeio.MaxAudioFileBytes + 1}

	_, err := decoder.Decode(context.Background(), r)
	if err == nil {
		t.Fatalf("expected error for oversized AIFF input")
	}
	if err != ErrReadFailure {
		t.Errorf("got %v, want ErrReadFailure", err)
	}
}

func TestDecoderRegistry_DecodeFile_RejectsOversizedInput(t *testing.T) {
	reg := NewDefaultRegistry()
	r := &zeroReader{remaining: safeio.MaxAudioFileBytes + 1}

	_, err := reg.DecodeFile(context.Background(), "test.wav", r)
	if err == nil {
		t.Fatalf("expected error for oversized audio file content")
	}
	// Registry wraps the cap error in "failed to read file content: ...".
	// It should mention the kind from safeio.
	if !strings.Contains(err.Error(), "audio file") {
		t.Errorf("error %q should mention 'audio file' from safeio cap", err.Error())
	}
}
