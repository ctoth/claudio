package native

import (
	"bytes"
	"errors"
	"testing"

	"claudio.click/internal/safeio"
	"claudio.click/internal/testutil/wavfixture"
)

// Float WAV samples outside [-1, 1] are legal and common after mastering.
// They must clip to full scale, never wrap around to the opposite sign.
func TestFloatWAVClipsInsteadOfOverflowing(t *testing.T) {
	src := [][]float64{{2.0, -3.0}, {1.5, -1.0000001}, {0.5, -0.5}}
	got, _, err := decodeAll(t, "hot.wav", wavfixture.WAV(wavfixture.TagFloat, 32, 44100, src))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	assertFrames(t, got, [][]float64{{1, -1}, {1, -1}, {0.5, -0.5}}, 1e-6)
}

// The sentinels still match, and the library's reason is kept.
func TestDecodeErrorsWrapSentinelAndCause(t *testing.T) {
	for _, tc := range []struct {
		name, file string
		data       []byte
		sentinel   error
	}{
		{"garbage mp3", "x.mp3", bytes.Repeat([]byte{0x5A}, 64), ErrInvalidData},
		{"RIFF without chunks", "x.wav", []byte("RIFF\x10\x00\x00\x00WAVE\xab\xab\xab\xab"), ErrInvalidData},
		{"AIFF without chunks", "x.aiff", []byte("FORM\x10\x00\x00\x00AIFF\xab\xab\xab\xab"), ErrInvalidData},
		{"unknown", "x.ogg", []byte("OggS\x00\x02 not supported"), ErrUnsupportedFormat},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := decodeAll(t, tc.file, tc.data)
			if !errors.Is(err, tc.sentinel) {
				t.Fatalf("err = %v, want %v", err, tc.sentinel)
			}
			if err.Error() == tc.sentinel.Error() {
				t.Errorf("err %q carries no cause", err)
			}
		})
	}
}

func TestOversizedInputIsReadFailure(t *testing.T) {
	_, _, err := decodeAllReader(t, "big.wav", &zeroReader{remaining: safeio.MaxAudioFileBytes + 1})
	if !errors.Is(err, ErrReadFailure) {
		t.Fatalf("err = %v, want ErrReadFailure", err)
	}
}
