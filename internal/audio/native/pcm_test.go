package native

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"testing"
)

func TestPCMReaderConvertsDepthAndDuplicatesMono(t *testing.T) {
	for _, tc := range []struct {
		name   string
		format PCMFormat
		input  []byte
	}{
		{"16 bit", FormatS16, []byte{0, 64, 0, 192}},
		{"24 bit", FormatS24, []byte{0, 0, 64, 0, 0, 192}},
		{"32 bit", FormatS32, []byte{0, 0, 0, 64, 0, 0, 0, 192}},
		{"float", FormatF32, []byte{0, 0, 0, 63, 0, 0, 0, 191}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, err := newPCMReader(context.Background(), &AudioData{Samples: tc.input, Channels: 1, SampleRate: 48000, Format: tc.format})
			if err != nil {
				t.Fatal(err)
			}
			// Odd read sizes must not lose partial output frames.
			var out []byte
			buf := make([]byte, 3)
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
			if len(out) != 16 {
				t.Fatalf("output bytes=%d, want 16", len(out))
			}
			for i, want := range []float32{0.5, 0.5, -0.5, -0.5} {
				got := math.Float32frombits(binary.LittleEndian.Uint32(out[i*4:]))
				if got != want {
					t.Errorf("sample %d=%v, want %v", i, got, want)
				}
			}
		})
	}
}

func TestPCMReaderPreservesRateAndStereo(t *testing.T) {
	for _, rate := range []uint32{8000, 22050, 44100, 48000, 96000} {
		data := make([]byte, int(rate)*4/10)
		for i := 0; i < len(data); i += 4 {
			binary.LittleEndian.PutUint16(data[i:], 8192)
			binary.LittleEndian.PutUint16(data[i+2:], uint16(49152))
		}
		r, err := newPCMReader(context.Background(), &AudioData{Samples: data, Channels: 2, SampleRate: rate, Format: FormatS16})
		if err != nil {
			t.Fatal(err)
		}
		out, err := io.ReadAll(r)
		if err != nil {
			t.Fatal(err)
		}
		if len(out) != 4800*8 {
			t.Errorf("rate %d: got %d output bytes, want %d", rate, len(out), 4800*8)
		}
		for i := 0; i < len(out); i += 8 {
			left := math.Float32frombits(binary.LittleEndian.Uint32(out[i:]))
			right := math.Float32frombits(binary.LittleEndian.Uint32(out[i+4:]))
			if math.Abs(float64(left)-0.25) > 1e-5 || math.Abs(float64(right)+0.5) > 1e-5 {
				t.Fatalf("rate %d: swapped or distorted channels %v,%v", rate, left, right)
			}
		}
	}
}

func TestPCMReaderRejectsInvalidFramesAndCancellation(t *testing.T) {
	for _, data := range []*AudioData{nil, {}, {Samples: []byte{1}, Channels: 1, SampleRate: 48000, Format: FormatS16}, {Samples: []byte{0, 0}, Channels: 0, SampleRate: 48000, Format: FormatS16}, {Samples: []byte{0, 0}, Channels: 1, SampleRate: 0, Format: FormatS16}} {
		if _, err := newPCMReader(context.Background(), data); err == nil {
			t.Errorf("accepted invalid data %#v", data)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	r, err := newPCMReader(ctx, &AudioData{Samples: make([]byte, 100), Channels: 1, SampleRate: 8000, Format: FormatS16})
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	if _, err = r.Read(make([]byte, 8)); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want cancellation", err)
	}
}
