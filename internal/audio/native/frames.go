package native

import (
	"context"
	"io"
)

// DecodeFrames decodes a whole audio file through the playback decoders and
// returns every frame at the file's own sample rate, folded to stereo. It
// is what the device would be fed before resampling, so tools that measure
// or rewrite sounds see exactly what a hook would play.
func DecodeFrames(ctx context.Context, filename string, r io.Reader) ([][2]float64, int, error) {
	s, err := decodeSound(ctx, filename, r)
	if err != nil {
		return nil, 0, err
	}
	frames := make([][2]float64, 0, s.frames)
	buf := make([][2]float64, 512)
	for {
		if err := ctx.Err(); err != nil {
			return nil, 0, err
		}
		n, ok := s.Stream(buf)
		frames = append(frames, buf[:n]...)
		if !ok || n == 0 {
			break
		}
	}
	if err := s.Err(); err != nil {
		return nil, 0, err
	}
	return frames, int(s.rate), nil
}
