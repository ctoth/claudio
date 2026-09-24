package native

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hajimehoshi/go-mp3"

	"claudio.click/internal/safeio"
	"claudio.click/internal/testutil/wavfixture"
)

// These tests pin what the decoders produce, independent of which library
// does the decoding: frame count, sample rate, and the decoded samples
// (folded to stereo) within one quantization step.

func assertFrames(t *testing.T, got [][2]float64, want [][]float64, tolerance float64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("decoded %d frames, want %d", len(got), len(want))
	}
	for i, frame := range want {
		var l, r float64
		switch len(frame) {
		case 1:
			l, r = frame[0], frame[0]
		case 2:
			l, r = frame[0], frame[1]
		default:
			for _, x := range frame {
				l += x
			}
			l /= float64(len(frame))
			r = l
		}
		if math.Abs(got[i][0]-l) > tolerance || math.Abs(got[i][1]-r) > tolerance {
			t.Fatalf("frame %d = %v, want [%v %v] within %v", i, got[i], l, r, tolerance)
		}
	}
}

func step(depth int) float64 { return 2 / float64(int64(1)<<(depth-1)) }

func TestCharacterizeWAV(t *testing.T) {
	for _, tc := range []struct {
		name            string
		tag, depth, chs int
	}{
		{"16-bit mono", wavfixture.TagPCM, 16, 1},
		{"16-bit stereo", wavfixture.TagPCM, 16, 2},
		{"8-bit stereo", wavfixture.TagPCM, 8, 2},
		{"24-bit mono", wavfixture.TagPCM, 24, 1},
		{"24-bit stereo", wavfixture.TagPCM, 24, 2},
		{"32-bit mono", wavfixture.TagPCM, 32, 1},
		{"32-bit stereo", wavfixture.TagPCM, 32, 2},
		{"float32 mono", wavfixture.TagFloat, 32, 1},
		{"float32 stereo", wavfixture.TagFloat, 32, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			want := wavfixture.SineFrames(1000, tc.chs, 0.8)
			got, rate, err := decodeAll(t, "tone.wav", wavfixture.WAV(tc.tag, tc.depth, 44100, want))
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			if rate != 44100 {
				t.Errorf("rate = %d, want 44100", rate)
			}
			tolerance := step(tc.depth)
			if tc.tag == wavfixture.TagFloat {
				tolerance = 1e-6
			}
			assertFrames(t, got, want, tolerance)
		})
	}
}

func TestCharacterizeAIFF(t *testing.T) {
	for _, tc := range []struct {
		name       string
		form       string
		depth, chs int
	}{
		{"16-bit mono", "AIFF", 16, 1},
		{"16-bit stereo", "AIFF", 16, 2},
		{"24-bit stereo", "AIFF", 24, 2},
		{"32-bit stereo", "AIFF", 32, 2},
		{"6 channels folded", "AIFF", 16, 6},
		{"AIFC NONE 16-bit stereo", "AIFC", 16, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			want := wavfixture.SineFrames(1000, tc.chs, 0.8)
			got, rate, err := decodeAll(t, "tone.aiff", wavfixture.AIFF(tc.form, tc.depth, 22050, want))
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			if rate != 22050 {
				t.Errorf("rate = %d, want 22050", rate)
			}
			assertFrames(t, got, want, step(tc.depth))
		})
	}
}

// Content, not the extension, picks the decoder.
func TestCharacterizeSniffsContentOverExtension(t *testing.T) {
	want := wavfixture.SineFrames(100, 2, 0.5)
	got, _, err := decodeAll(t, "mislabelled.mp3", wavfixture.WAV(wavfixture.TagPCM, 16, 44100, want))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	assertFrames(t, got, want, step(16))
}

func TestCharacterizeMP3(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "soundpacks", "startrek-bridge", "success", "complete.mp3"))
	if err != nil {
		t.Fatal(err)
	}
	// Reference: go-mp3 directly, 16-bit stereo.
	ref, err := mp3.NewDecoder(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(ref)
	if err != nil {
		t.Fatal(err)
	}
	want := make([][]float64, len(raw)/4)
	for i := range want {
		want[i] = []float64{
			float64(int16(binary.LittleEndian.Uint16(raw[i*4:]))) / 32768,
			float64(int16(binary.LittleEndian.Uint16(raw[i*4+2:]))) / 32768,
		}
	}
	if len(want) == 0 {
		t.Fatal("reference decode is empty")
	}

	got, rate, err := decodeAll(t, "complete.mp3", data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if rate != ref.SampleRate() {
		t.Errorf("rate = %d, want %d", rate, ref.SampleRate())
	}
	assertFrames(t, got, want, step(16))
}

func TestCharacterizeRejectsCorruptInput(t *testing.T) {
	riffGarbage := append([]byte("RIFF\x10\x00\x00\x00WAVE"), bytes.Repeat([]byte{0xAB}, 16)...)
	aiffGarbage := append([]byte("FORM\x10\x00\x00\x00AIFF"), bytes.Repeat([]byte{0xAB}, 16)...)
	for _, tc := range []struct {
		name, file string
		data       []byte
	}{
		{"empty", "x.wav", nil},
		{"garbage wav", "x.wav", bytes.Repeat([]byte{0x5A}, 64)},
		{"garbage extensionless", "x", bytes.Repeat([]byte{0x5A}, 64)},
		{"RIFF without chunks", "x.wav", riffGarbage},
		{"AIFF without chunks", "x.aiff", aiffGarbage},
		{"garbage mp3", "x.mp3", bytes.Repeat([]byte{0x5A}, 64)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := decodeAll(t, tc.file, tc.data); err == nil {
				t.Fatal("corrupt input decoded without error")
			}
		})
	}
}

// A data chunk that claims more bytes than the file holds is rejected as
// invalid rather than played short.
func TestCharacterizeTruncatedWAVIsInvalid(t *testing.T) {
	data := wavfixture.WAV(wavfixture.TagPCM, 16, 44100, wavfixture.SineFrames(1000, 2, 0.5))
	data = data[:len(data)-400*4] // drop the last 400 frames
	if _, _, err := decodeAll(t, "cut.wav", data); !errors.Is(err, ErrInvalidData) {
		t.Fatalf("truncated WAV: err = %v, want ErrInvalidData", err)
	}
}

func TestCharacterizeRejectsOversizedInput(t *testing.T) {
	_, _, err := decodeAllReader(t, "big.wav", &zeroReader{remaining: safeio.MaxAudioFileBytes + 1})
	if err == nil {
		t.Fatal("oversized input decoded")
	}
	if !strings.Contains(err.Error(), "audio file") {
		t.Errorf("error %q should name the capped audio file", err)
	}
}

// Characterization of the unsupported-format sentinel for unknown content.
func TestCharacterizeUnknownFormatIsUnsupported(t *testing.T) {
	_, _, err := decodeAll(t, "x.ogg", []byte("OggS\x00\x02 not a supported format at all"))
	if err == nil {
		t.Fatal("unknown format decoded")
	}
	if !errors.Is(err, ErrUnsupportedFormat) && !strings.Contains(err.Error(), "unsupported audio format") {
		t.Errorf("error %v does not report an unsupported format", err)
	}
}

func decodeAll(t *testing.T, name string, data []byte) ([][2]float64, int, error) {
	t.Helper()
	return decodeAllReader(t, name, bytes.NewReader(data))
}

// decodeAllReader decodes through the production path and returns every
// frame at the source rate, folded to stereo. It is the only part of these
// tests that knows the decoder API.
func decodeAllReader(t *testing.T, name string, r io.Reader) ([][2]float64, int, error) {
	t.Helper()
	ctx := context.Background()
	s, err := decodeSound(ctx, name, r)
	if err != nil {
		return nil, 0, err
	}
	var frames [][2]float64
	buf := make([][2]float64, 512)
	for {
		n, ok := s.Stream(buf)
		frames = append(frames, buf[:n]...)
		if !ok || n == 0 {
			break
		}
	}
	return frames, int(s.rate), s.Err()
}
