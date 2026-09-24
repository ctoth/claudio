package native

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"testing"

	"github.com/gopxl/beep/v2"
)

// constSound is n frames of one stereo value at rate.
func constSound(rate, n int, l, r float64) sound {
	return sound{
		Streamer: &interleaved{channels: 2, frames: n, sample: func(i int) float64 {
			if i%2 == 0 {
				return l
			}
			return r
		}},
		rate:   beep.SampleRate(rate),
		frames: n,
	}
}

func readFloats(t *testing.T, r io.Reader, chunk int) []float32 {
	t.Helper()
	var out []byte
	buf := make([]byte, chunk)
	for {
		n, err := r.Read(buf)
		out = append(out, buf[:n]...)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(out)%4 != 0 {
		t.Fatalf("output %d bytes is not whole float32 samples", len(out))
	}
	floats := make([]float32, len(out)/4)
	for i := range floats {
		floats[i] = math.Float32frombits(binary.LittleEndian.Uint32(out[i*4:]))
	}
	return floats
}

// Odd read sizes must not lose partial output frames.
func TestPCMReaderOddReadSizes(t *testing.T) {
	got := readFloats(t, newPCMReader(context.Background(), constSound(48000, 3, 0.5, -0.5)), 3)
	want := []float32{0.5, -0.5, 0.5, -0.5, 0.5, -0.5}
	if len(got) != len(want) {
		t.Fatalf("got %d samples, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("sample %d = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestPCMReaderResamplesTo48kAndKeepsChannels(t *testing.T) {
	for _, rate := range []int{8000, 22050, 44100, 48000, 96000} {
		got := readFloats(t, newPCMReader(context.Background(), constSound(rate, rate/10, 0.25, -0.5)), 4096)
		if len(got) != 4800*2 {
			t.Errorf("rate %d: got %d output samples, want %d", rate, len(got), 4800*2)
		}
		for i := 0; i < len(got); i += 2 {
			if math.Abs(float64(got[i])-0.25) > 1e-5 || math.Abs(float64(got[i+1])+0.5) > 1e-5 {
				t.Fatalf("rate %d: swapped or distorted channels %v,%v", rate, got[i], got[i+1])
			}
		}
	}
}

// Whatever the source, the device never sees samples beyond full scale.
func TestPCMReaderClipsOutput(t *testing.T) {
	got := readFloats(t, newPCMReader(context.Background(), constSound(48000, 2, 3, math.Inf(-1))), 64)
	for i := 0; i < len(got); i += 2 {
		if got[i] != 1 || got[i+1] != -1 {
			t.Fatalf("frame %d = %v,%v, want 1,-1", i/2, got[i], got[i+1])
		}
	}
}

func TestPCMReaderStopsOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	r := newPCMReader(ctx, constSound(8000, 100, 0, 0))
	cancel()
	if _, err := r.Read(make([]byte, 8)); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want cancellation", err)
	}
}

func TestDecodeSoundRespectsCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	data := buildWAV(wavTagPCM, 16, 44100, sineFrames(10, 2, 0.1))
	if _, err := decodeSound(ctx, "x.wav", bytes.NewReader(data)); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want context.Canceled", err)
	}
}
