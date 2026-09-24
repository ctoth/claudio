// Package wavfixture builds small, valid WAV and AIFF files for tests.
//
// It is the one place test code in any package gets audio fixtures from, so
// the RIFF/AIFF layouts are written (and fixed) once. It is only imported by
// _test.go files and never ends up in the claudio binary.
package wavfixture

import (
	"bytes"
	"encoding/binary"
	"math"
	"math/bits"
	"os"
	"path/filepath"
	"testing"
)

// WAV format tags.
const (
	TagPCM   = 1
	TagFloat = 3
)

// SineFrames returns n frames of a sine per channel, each channel at a
// different frequency (440 Hz * channel number, at 44.1 kHz) so a channel
// swap is visible.
func SineFrames(n, channels int, amplitude float64) [][]float64 {
	frames := make([][]float64, n)
	for i := range frames {
		frames[i] = make([]float64, channels)
		for ch := range channels {
			frames[i][ch] = amplitude * math.Sin(2*math.Pi*float64(440*(ch+1))*float64(i)/44100)
		}
	}
	return frames
}

// ToneFrames returns milliseconds of a 440 Hz tone at the given rate, the
// same on every channel.
func ToneFrames(rate, channels, milliseconds int, amplitude float64) [][]float64 {
	frames := make([][]float64, rate*milliseconds/1000)
	for i := range frames {
		v := amplitude * math.Sin(2*math.Pi*440*float64(i)/float64(rate))
		frames[i] = make([]float64, channels)
		for ch := range channels {
			frames[i][ch] = v
		}
	}
	return frames
}

// Quantize encodes x as a signed integer sample of the given depth,
// clamped to the representable range.
func Quantize(x float64, depth int) int64 {
	scale := float64(int64(1) << (depth - 1))
	v := math.Round(x * scale)
	return int64(max(-scale, min(scale-1, v)))
}

// WAV encodes frames as a canonical RIFF/WAVE file. tag is TagPCM (8, 16,
// 24 or 32-bit integer) or TagFloat (32-bit IEEE float).
func WAV(tag, depth, rate int, frames [][]float64) []byte {
	channels := len(frames[0])
	width := depth / 8
	var pcm bytes.Buffer
	for _, frame := range frames {
		for _, x := range frame {
			switch {
			case tag == TagFloat:
				_ = binary.Write(&pcm, binary.LittleEndian, math.Float32bits(float32(x)))
			case depth == 8:
				pcm.WriteByte(byte(Quantize(x, 8) + 128))
			default:
				v := uint64(Quantize(x, depth))
				for b := range width {
					pcm.WriteByte(byte(v >> (8 * b)))
				}
			}
		}
	}
	var w bytes.Buffer
	w.WriteString("RIFF")
	_ = binary.Write(&w, binary.LittleEndian, uint32(36+pcm.Len()))
	w.WriteString("WAVEfmt ")
	_ = binary.Write(&w, binary.LittleEndian, uint32(16))
	_ = binary.Write(&w, binary.LittleEndian, uint16(tag))
	_ = binary.Write(&w, binary.LittleEndian, uint16(channels))
	_ = binary.Write(&w, binary.LittleEndian, uint32(rate))
	_ = binary.Write(&w, binary.LittleEndian, uint32(rate*channels*width))
	_ = binary.Write(&w, binary.LittleEndian, uint16(channels*width))
	_ = binary.Write(&w, binary.LittleEndian, uint16(depth))
	w.WriteString("data")
	_ = binary.Write(&w, binary.LittleEndian, uint32(pcm.Len()))
	w.Write(pcm.Bytes())
	return w.Bytes()
}

// ieeeExtended encodes a positive integer sample rate as the 80-bit float
// AIFF uses in its COMM chunk.
func ieeeExtended(rate int) []byte {
	out := make([]byte, 10)
	e := bits.Len64(uint64(rate)) - 1
	binary.BigEndian.PutUint16(out, uint16(16383+e))
	binary.BigEndian.PutUint64(out[2:], uint64(rate)<<(63-e))
	return out
}

// AIFF encodes frames as big-endian AIFF, or as AIFC with the NONE
// (uncompressed big-endian) encoding when form is "AIFC".
func AIFF(form string, depth, rate int, frames [][]float64) []byte {
	channels := len(frames[0])
	width := depth / 8
	var comm bytes.Buffer
	_ = binary.Write(&comm, binary.BigEndian, uint16(channels))
	_ = binary.Write(&comm, binary.BigEndian, uint32(len(frames)))
	_ = binary.Write(&comm, binary.BigEndian, uint16(depth))
	comm.Write(ieeeExtended(rate))
	if form == "AIFC" {
		comm.WriteString("NONE")
		comm.Write([]byte{0, 0}) // empty pascal name, padded to even length
	}
	var ssnd bytes.Buffer
	ssnd.Write(make([]byte, 8)) // offset, block size
	for _, frame := range frames {
		for _, x := range frame {
			v := uint64(Quantize(x, depth))
			for b := width - 1; b >= 0; b-- {
				ssnd.WriteByte(byte(v >> (8 * b)))
			}
		}
	}
	var body bytes.Buffer
	body.WriteString(form)
	if form == "AIFC" {
		body.WriteString("FVER")
		_ = binary.Write(&body, binary.BigEndian, uint32(4))
		_ = binary.Write(&body, binary.BigEndian, uint32(0xA2805140))
	}
	body.WriteString("COMM")
	_ = binary.Write(&body, binary.BigEndian, uint32(comm.Len()))
	body.Write(comm.Bytes())
	body.WriteString("SSND")
	_ = binary.Write(&body, binary.BigEndian, uint32(ssnd.Len()))
	body.Write(ssnd.Bytes())

	var out bytes.Buffer
	out.WriteString("FORM")
	_ = binary.Write(&out, binary.BigEndian, uint32(body.Len()))
	out.Write(body.Bytes())
	return out.Bytes()
}

// Minimal returns a tiny valid WAV: 16-bit mono 44.1 kHz, 4 frames.
func Minimal() []byte {
	return WAV(TagPCM, 16, 44100, [][]float64{{0}, {0.99}, {0}, {0.99}})
}

// Write creates path's parent directories and writes Minimal() to it.
func Write(t testing.TB, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("wavfixture: create dir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, Minimal(), 0o644); err != nil {
		t.Fatalf("wavfixture: write %s: %v", path, err)
	}
}
