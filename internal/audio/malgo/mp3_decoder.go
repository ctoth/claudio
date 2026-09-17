//go:build cgo

package malgo

import (
	"context"
	"io"
	"log/slog"
	"strings"

	"github.com/gen2brain/malgo"
	"github.com/hajimehoshi/go-mp3"
)

// Mp3Decoder handles MP3 audio format decoding
type Mp3Decoder struct{}

// NewMp3Decoder creates a new MP3 decoder instance
func NewMp3Decoder() *Mp3Decoder {
	slog.Debug("creating new MP3 decoder instance")
	return &Mp3Decoder{}
}

// Decode reads MP3 audio data from reader and returns decoded PCM data.
// Cancellation is checked before decoding and between PCM reads. It cannot
// interrupt an underlying Read already in progress.
func (d *Mp3Decoder) Decode(ctx context.Context, reader io.Reader) (*AudioData, error) {
	slog.Debug("starting MP3 decode operation")

	// Refuse work on a pre-cancelled context.
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	decoder, err := mp3.NewDecoder(reader)
	if err != nil {
		slog.Error("failed to create MP3 decoder", "error", err)
		return nil, ErrInvalidData
	}

	sampleRate := decoder.SampleRate()
	if sampleRate <= 0 {
		slog.Error("invalid MP3 sample rate", "sample_rate", sampleRate)
		return nil, ErrInvalidData
	}

	slog.Debug("MP3 format detected",
		"sample_rate", sampleRate,
		"channels", 2) // MP3 is always decoded as stereo by go-mp3

	// Read all audio data into memory (better than malgo's streaming approach for hooks)
	slog.Debug("reading MP3 audio samples")
	samples, err := readDecodedPCM(ctx, decoder, MaxDecodedPCMBytes)
	if err != nil {
		return nil, err
	}

	if len(samples) == 0 {
		slog.Error("no audio data found in MP3 file")
		return nil, ErrInvalidData
	}

	// MP3 decoder outputs 16-bit signed PCM, always stereo
	audioData := &AudioData{
		Samples:    samples,
		Channels:   2, // go-mp3 always outputs stereo
		SampleRate: uint32(sampleRate),
		Format:     malgo.FormatS16, // go-mp3 outputs 16-bit signed
	}

	// Calculate duration estimate
	bytesPerSecond := int(audioData.Channels) * int(audioData.SampleRate) * 2 // 2 bytes per 16-bit sample
	durationMs := 0
	if bytesPerSecond > 0 {
		durationMs = (len(samples) * 1000) / bytesPerSecond
	}

	slog.Debug("MP3 decode completed successfully",
		"total_bytes", len(samples),
		"channels", audioData.Channels,
		"sample_rate", audioData.SampleRate,
		"format", malgo.FormatS16,
		"duration_estimate_ms", durationMs)

	return audioData, nil
}

// CanDecode checks if this decoder can handle the given filename
func (d *Mp3Decoder) CanDecode(filename string) bool {
	lower := strings.ToLower(filename)
	canDecode := strings.HasSuffix(lower, ".mp3") || strings.HasSuffix(lower, ".mpeg")

	slog.Debug("MP3 decoder file check",
		"filename", filename,
		"can_decode", canDecode)

	return canDecode
}

// FormatName returns the name of the format this decoder handles
func (d *Mp3Decoder) FormatName() string {
	return "MP3"
}
