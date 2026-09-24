package native

import (
	"bytes"
	"fmt"

	"github.com/gopxl/beep/v2/mp3"
)

// bytesReadSeekCloser lets go-mp3 seek the in-memory file, which it needs
// to compute the exact length up front.
type bytesReadSeekCloser struct{ *bytes.Reader }

func (bytesReadSeekCloser) Close() error { return nil }

func decodeMP3(data []byte) (sound, error) {
	s, format, err := mp3.Decode(bytesReadSeekCloser{bytes.NewReader(data)})
	if err != nil {
		return sound{}, fmt.Errorf("%w: %w", ErrInvalidData, err)
	}
	frames := s.Len()
	if int64(frames)*4 > MaxDecodedPCMBytes {
		return sound{}, fmt.Errorf("%w: decoded MP3 exceeds %d bytes", ErrReadFailure, MaxDecodedPCMBytes)
	}
	return sound{Streamer: s, rate: format.SampleRate, frames: frames}, nil
}
