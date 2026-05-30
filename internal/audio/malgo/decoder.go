//go:build cgo

package malgo

import (
	"context"
	"errors"
	"io"

	"github.com/gen2brain/malgo"
)

// Common decoder errors
var (
	ErrInvalidData       = errors.New("invalid audio data")
	ErrReadFailure       = errors.New("failed to read audio data")
	ErrUnsupportedFormat = errors.New("unsupported audio format")
)

// AudioData represents decoded audio ready for playback
type AudioData struct {
	Samples    []byte           // Raw PCM data
	Channels   uint32           // Number of audio channels
	SampleRate uint32           // Sample rate in Hz
	Format     malgo.FormatType // Audio format (e.g., malgo.FormatS16)
}

// Decoder interface for audio format decoding.
//
// Decode takes a context.Context as its first argument so callers can
// cancel a long-running or stalled decode (e.g. an MP3 source whose
// underlying reader has hung). The MP3 decoder polls ctx between read
// chunks; WAV and AIFF check ctx at entry (they already buffer the whole
// input via safeio.ReadAllCapped before per-sample work begins, so the
// only meaningful cancellation point is the entry check).
//
// The interface is internal to package audio — there are no external
// importers — so adding the parameter is safe.
type Decoder interface {
	// Decode reads audio data from reader and returns decoded PCM data.
	// If ctx is cancelled, Decode returns ctx.Err().
	Decode(ctx context.Context, reader io.Reader) (*AudioData, error)

	// CanDecode checks if this decoder can handle the given filename
	CanDecode(filename string) bool

	// FormatName returns the name of the format this decoder handles
	FormatName() string
}
