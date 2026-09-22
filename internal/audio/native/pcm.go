package native

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math"

	"github.com/gopxl/beep/v2"
)

const outputSampleRate = 48000

// PCMFormat describes interleaved little-endian decoder output independently
// of the device library. Signed integer depths are preserved until playback.
type PCMFormat uint32

const (
	FormatUnknown PCMFormat = iota
	FormatU8
	FormatS16
	FormatS24
	FormatS32
	FormatF32
)

func getBytesPerSample(format PCMFormat) (int, error) {
	switch format {
	case FormatU8:
		return 1, nil
	case FormatS16:
		return 2, nil
	case FormatS24:
		return 3, nil
	case FormatS32, FormatF32:
		return 4, nil
	default:
		return 0, fmt.Errorf("%w: PCM format %d", ErrUnsupportedFormat, format)
	}
}

// newPCMReader converts bounded decoded PCM to Oto's process-wide format.
// Mono is duplicated and more than two channels are averaged, since the
// device is stereo and channel layouts differ by container. Resampling is streamed through fixed buffers rather than allocating another
// full sound. Beep supplies only sample-rate conversion, not device ownership.
func newPCMReader(ctx context.Context, data *AudioData) (io.Reader, error) {
	if data == nil || data.SampleRate == 0 || data.Channels < 1 {
		return nil, ErrInvalidData
	}
	width, err := getBytesPerSample(data.Format)
	if err != nil {
		return nil, err
	}
	if len(data.Samples) == 0 || len(data.Samples)%(width*int(data.Channels)) != 0 {
		return nil, fmt.Errorf("%w: incomplete PCM frames", ErrInvalidData)
	}
	var stream beep.Streamer = &pcmStream{ctx: ctx, data: data, width: width}
	if data.SampleRate != outputSampleRate {
		stream = beep.Resample(4, beep.SampleRate(data.SampleRate), outputSampleRate, stream)
	}
	return &pcmReader{ctx: ctx, stream: stream}, nil
}

type pcmStream struct {
	ctx           context.Context
	data          *AudioData
	width, offset int
	err           error
}

func (s *pcmStream) Stream(dst [][2]float64) (int, bool) {
	if s.err = s.ctx.Err(); s.err != nil {
		return 0, false
	}
	frames := min(len(dst), (len(s.data.Samples)-s.offset)/(s.width*int(s.data.Channels)))
	for i := 0; i < frames; i++ {
		dst[i] = s.frame()
	}
	return frames, frames > 0
}

func (s *pcmStream) Err() error { return s.err }

func (s *pcmStream) frame() [2]float64 {
	switch s.data.Channels {
	case 1:
		x := s.sample()
		return [2]float64{x, x}
	case 2:
		return [2]float64{s.sample(), s.sample()}
	}
	var sum float64
	for range s.data.Channels {
		sum += s.sample()
	}
	x := sum / float64(s.data.Channels)
	return [2]float64{x, x}
}

func (s *pcmStream) sample() float64 {
	b := s.data.Samples[s.offset : s.offset+s.width]
	s.offset += s.width
	switch s.data.Format {
	case FormatU8:
		return (float64(b[0]) - 128) / 128
	case FormatS16:
		return float64(int16(binary.LittleEndian.Uint16(b))) / 32768
	case FormatS24:
		x := int32(b[0]) | int32(b[1])<<8 | int32(b[2])<<16
		return float64(x<<8>>8) / 8388608
	case FormatS32:
		return float64(int32(binary.LittleEndian.Uint32(b))) / 2147483648
	case FormatF32:
		x := float64(math.Float32frombits(binary.LittleEndian.Uint32(b)))
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return 0
		}
		return x
	default:
		panic("unvalidated PCM format")
	}
}

type pcmReader struct {
	ctx     context.Context
	stream  beep.Streamer
	frames  [512][2]float64
	buffer  [512 * 8]byte
	pending []byte
}

func (r *pcmReader) Read(dst []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	if len(dst) == 0 {
		return 0, nil
	}
	if len(r.pending) == 0 {
		n, _ := r.stream.Stream(r.frames[:])
		if n == 0 {
			if err := r.stream.Err(); err != nil {
				return 0, err
			}
			return 0, io.EOF
		}
		for i, frame := range r.frames[:n] {
			for ch, sample := range frame {
				binary.LittleEndian.PutUint32(r.buffer[i*8+ch*4:], math.Float32bits(float32(sample)))
			}
		}
		r.pending = r.buffer[:n*8]
	}
	n := copy(dst, r.pending)
	r.pending = r.pending[n:]
	return n, nil
}
