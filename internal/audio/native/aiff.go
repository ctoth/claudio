package native

import (
	"bytes"
	"fmt"

	"github.com/go-audio/aiff"
	"github.com/gopxl/beep/v2"
)

// decodeAIFF adapts go-audio/aiff (AIFF, and AIFC with NONE/sowt) to a
// beep streamer. go-audio returns integer samples at the source depth.
func decodeAIFF(data []byte) (sound, error) {
	d := aiff.NewDecoder(bytes.NewReader(data))
	if !d.IsValidFile() {
		// IsValidFile can fail on header fields with no decoder error set.
		if cause := d.Err(); cause != nil {
			return sound{}, fmt.Errorf("%w: not a readable AIFF file: %w", ErrInvalidData, cause)
		}
		return sound{}, fmt.Errorf("%w: not a readable AIFF file", ErrInvalidData)
	}
	depth := int(d.SampleBitDepth())
	switch depth {
	case 16, 24, 32:
	default:
		return sound{}, fmt.Errorf("%w: %d-bit AIFF", ErrUnsupportedFormat, depth)
	}
	buf, err := d.FullPCMBuffer()
	if err != nil {
		return sound{}, fmt.Errorf("%w: %w", ErrReadFailure, err)
	}
	channels := int(d.NumChans)
	frames := len(buf.Data) / channels
	scale := float64(int64(1) << (depth - 1))
	samples := buf.Data
	return sound{
		Streamer: &interleaved{
			channels: channels,
			frames:   frames,
			sample:   func(i int) float64 { return float64(samples[i]) / scale },
		},
		rate:   beep.SampleRate(d.SampleRate),
		frames: frames,
	}, nil
}
