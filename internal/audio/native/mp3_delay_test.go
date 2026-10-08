package native

import (
	"bytes"
	"encoding/binary"
	"io"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/hajimehoshi/go-mp3"
)

// The fixtures are 0.2 s sines that start at phase zero on the first sample,
// encoded with LAME through ffmpeg. Each carries an info header frame that
// declares LAME's encoder delay and end padding. The tone frequencies are
// chosen so that playing the header frame, the delay, or both puts the
// decoded tone well out of phase with the source.

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// referenceMP3 decodes with go-mp3 directly: every frame, nothing trimmed.
func referenceMP3(t *testing.T, data []byte) [][]float64 {
	t.Helper()
	ref, err := mp3.NewDecoder(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(ref)
	if err != nil {
		t.Fatal(err)
	}
	frames := make([][]float64, len(raw)/4)
	for i := range frames {
		frames[i] = []float64{
			float64(int16(binary.LittleEndian.Uint16(raw[i*4:]))) / 32768,
			float64(int16(binary.LittleEndian.Uint16(raw[i*4+2:]))) / 32768,
		}
	}
	return frames
}

// infoEncoder returns the offset of the encoder name in the info header.
func infoEncoder(t *testing.T, data []byte) int {
	t.Helper()
	i := bytes.Index(data, []byte("Lavc"))
	if i < 0 {
		t.Fatal("fixture has no Lavc info header")
	}
	return i
}

func TestMP3InfoHeaderTrimsEncoderDelayAndPadding(t *testing.T) {
	for _, tc := range []struct {
		name, file   string
		rate, frames int
		hz           float64
		left, right  float64
	}{
		{"CBR mono, Info tag", "lame-cbr-mono-48k.mp3", 48000, 9600, 1310, 0.5, 0.5},
		{"VBR stereo, Xing tag after ID3", "lame-vbr-stereo-44k.mp3", 44100, 8820, 1210, 0.5, 0.25},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, rate, err := decodeAll(t, tc.file, readFixture(t, tc.file))
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			if rate != tc.rate {
				t.Errorf("rate = %d, want %d", rate, tc.rate)
			}
			want := make([][]float64, tc.frames)
			for i := range want {
				x := math.Sin(2 * math.Pi * tc.hz * float64(i) / float64(tc.rate))
				want[i] = []float64{tc.left * x, tc.right * x}
			}
			// Lossy, so the tolerance is loose, but one sample of
			// misalignment is already an error of 0.08.
			assertFrames(t, got, want, 0.04)
		})
	}
}

// A header frame that does not come from an encoder known to fill in the
// delay fields is still not audio: it is dropped, and nothing else is.
func TestMP3InfoHeaderWithoutDelayDropsOnlyTheHeaderFrame(t *testing.T) {
	data := bytes.Clone(readFixture(t, "lame-cbr-mono-48k.mp3"))
	copy(data[infoEncoder(t, data):], "Unkn")

	const headerFrame = 1152
	want := referenceMP3(t, data)[headerFrame:]
	got, _, err := decodeAll(t, "unknown-encoder.mp3", data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	assertFrames(t, got, want, step(16))
}

// A header that asks for more than the file holds is not believed.
func TestMP3InfoHeaderLargerThanTheStreamIsIgnored(t *testing.T) {
	const frameBytes = 192 // 64 kbit/s at 48 kHz
	data := bytes.Clone(readFixture(t, "lame-cbr-mono-48k.mp3")[:3*frameBytes])
	delay := infoEncoder(t, data) + 21
	copy(data[delay:], []byte{0xFF, 0xFF, 0xFF}) // 4095 samples at each end

	want := referenceMP3(t, data)
	got, _, err := decodeAll(t, "short.mp3", data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	assertFrames(t, got, want, step(16))
}
