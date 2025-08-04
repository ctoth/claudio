package audio

import (
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

// Decoder interface for audio format decoding
type Decoder interface {
	// Decode reads audio data from reader and returns decoded PCM data
	Decode(reader io.Reader) (*AudioData, error)

	// CanDecode checks if this decoder can handle the given filename
	CanDecode(filename string) bool

	// FormatName returns the name of the format this decoder handles
	FormatName() string
}
