package native

import (
	"context"
	"encoding/binary"
	"io"
	"math"

	"github.com/gopxl/beep/v2"
)

const outputSampleRate = 48000

// newPCMReader streams s in Oto's process-wide format: 48 kHz stereo
// float32, clipped to full scale. Resampling runs through fixed buffers
// rather than allocating another full copy of the sound. Beep supplies
// only decoding and sample-rate conversion, not device ownership.
func newPCMReader(ctx context.Context, s sound) io.Reader {
	stream := s.Streamer
	if s.rate != outputSampleRate {
		stream = beep.Resample(4, s.rate, outputSampleRate, stream)
	}
	return &pcmReader{ctx: ctx, stream: stream}
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
				binary.LittleEndian.PutUint32(r.buffer[i*8+ch*4:], math.Float32bits(float32(clip(sample))))
			}
		}
		r.pending = r.buffer[:n*8]
	}
	n := copy(dst, r.pending)
	r.pending = r.pending[n:]
	return n, nil
}
