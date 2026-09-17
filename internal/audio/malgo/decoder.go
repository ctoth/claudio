//go:build cgo

package malgo

import (
	"context"
	"errors"
	"io"

	"claudio.click/internal/safeio"
	"github.com/gen2brain/malgo"
)

// MaxDecodedPCMBytes applies the existing 100 MiB audio budget after MP3
// decompression as well. Valid PCM WAV/AIFF output is bounded by encoded size.
const MaxDecodedPCMBytes = safeio.MaxAudioFileBytes

type pcmContextReader struct {
	context context.Context
	reader  io.Reader
}

func (r pcmContextReader) Read(p []byte) (int, error) {
	if err := r.context.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

func readDecodedPCM(ctx context.Context, reader io.Reader, limit int64) ([]byte, error) {
	return safeio.ReadAllCapped(pcmContextReader{context: ctx, reader: reader}, limit, "decoded PCM audio")
}

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
// cancel work before decoding or between MP3 reads. Cancellation cannot
// interrupt an underlying Read that is already blocked. MP3 polls ctx between read
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
