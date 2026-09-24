package native

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/gopxl/beep/v2"

	"claudio.click/internal/safeio"
)

// MaxDecodedPCMBytes applies the 100 MiB audio budget after MP3
// decompression as well (measured as 16-bit stereo). Valid PCM WAV/AIFF
// output is bounded by the encoded size.
const MaxDecodedPCMBytes = safeio.MaxAudioFileBytes

// Decode errors. Library errors are wrapped under one of these, so callers
// can match the kind and still read the cause.
var (
	ErrInvalidData       = errors.New("invalid audio data")
	ErrReadFailure       = errors.New("failed to read audio data")
	ErrUnsupportedFormat = errors.New("unsupported audio format")
)

// sound is a decoded file ready to stream: frames of stereo float samples
// at rate. frames is the exact length, used for the playback deadline.
type sound struct {
	beep.Streamer
	rate   beep.SampleRate
	frames int
}

func (s sound) duration() time.Duration {
	return time.Duration(s.frames) * time.Second / time.Duration(s.rate)
}

// decodeSound reads the whole file once (size-capped), identifies it by its
// magic bytes and returns a streamer over the in-memory copy. Cancellation
// is checked here and again on every read of the PCM reader.
func decodeSound(ctx context.Context, filename string, r io.Reader) (sound, error) {
	if err := ctx.Err(); err != nil {
		return sound{}, err
	}
	data, err := safeio.ReadAllCapped(r, safeio.MaxAudioFileBytes, "audio file")
	if err != nil {
		return sound{}, fmt.Errorf("%w: %w", ErrReadFailure, err)
	}
	if err := ctx.Err(); err != nil {
		return sound{}, err
	}

	format := sniffFormat(data, filename)
	var s sound
	switch format {
	case formatWAV:
		s, err = decodeWAV(data)
	case formatMP3:
		s, err = decodeMP3(data)
	case formatAIFF:
		s, err = decodeAIFF(data)
	default:
		return sound{}, fmt.Errorf("%w: %s", ErrUnsupportedFormat, filename)
	}
	if err != nil {
		return sound{}, fmt.Errorf("decode %s: %w", format, err)
	}
	if s.rate <= 0 || s.frames <= 0 {
		return sound{}, fmt.Errorf("decode %s: %w: no audio frames", format, ErrInvalidData)
	}
	slog.Debug("decoded audio", "filename", filename, "format", format.String(),
		"sample_rate", int(s.rate), "frames", s.frames, "duration", s.duration())
	return s, nil
}

// clip limits a sample to full scale. Float sources may legally exceed it,
// and NaN or Inf must not reach the device.
func clip(x float64) float64 {
	switch {
	case x != x: // NaN
		return 0
	case x > 1:
		return 1
	case x < -1:
		return -1
	}
	return x
}

// interleaved streams frames from interleaved samples, folding them to
// stereo: mono is duplicated and more than two channels are averaged, since
// the device is stereo and channel layouts differ by container.
type interleaved struct {
	channels, frames, pos int
	sample                func(i int) float64 // i indexes interleaved samples
}

func (s *interleaved) Stream(dst [][2]float64) (int, bool) {
	n := min(len(dst), s.frames-s.pos)
	for i := range n {
		dst[i] = s.frame((s.pos + i) * s.channels)
	}
	s.pos += n
	return n, n > 0
}

func (s *interleaved) Err() error { return nil }

func (s *interleaved) frame(base int) [2]float64 {
	switch s.channels {
	case 1:
		x := s.sample(base)
		return [2]float64{x, x}
	case 2:
		return [2]float64{s.sample(base), s.sample(base + 1)}
	}
	var sum float64
	for ch := range s.channels {
		sum += s.sample(base + ch)
	}
	x := sum / float64(s.channels)
	return [2]float64{x, x}
}
