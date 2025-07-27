package audio

import (
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/gen2brain/malgo"
	"github.com/hajimehoshi/go-mp3"
	"github.com/youpy/go-wav"
)

// StreamingAudioData represents audio that can be streamed
type StreamingAudioData struct {
	Reader     io.Reader         // The audio data reader
	Channels   uint32            // Number of audio channels
	SampleRate uint32            // Sample rate in Hz
	Format     malgo.FormatType  // Audio format (e.g., malgo.FormatS16)
}

// StreamingDecoder interface for streaming audio format decoding
type StreamingDecoder interface {
	// DecodeStreaming returns a reader and audio properties for streaming playback
	DecodeStreaming(data []byte) (*StreamingAudioData, error)
	
	// CanDecode checks if this decoder can handle the given filename
	CanDecode(filename string) bool
	
	// FormatName returns the name of the format this decoder handles
	FormatName() string
}

// StreamingWavDecoder handles WAV audio format for streaming
type StreamingWavDecoder struct{}

// NewStreamingWavDecoder creates a new streaming WAV decoder
func NewStreamingWavDecoder() *StreamingWavDecoder {
	slog.Debug("creating new streaming WAV decoder")
	return &StreamingWavDecoder{}
}

// DecodeStreaming returns a reader for streaming WAV data
func (d *StreamingWavDecoder) DecodeStreaming(data []byte) (*StreamingAudioData, error) {
	slog.Debug("starting streaming WAV decode")
	
	// Create a reader from the data
	reader := strings.NewReader(string(data))
	wavReader := wav.NewReader(reader)
	
	format, err := wavReader.Format()
	if err != nil {
		slog.Error("failed to read WAV format", "error", err)
		return nil, fmt.Errorf("failed to read WAV format: %w", err)
	}
	
	slog.Debug("WAV format detected",
		"sample_rate", format.SampleRate,
		"channels", format.NumChannels,
		"bits_per_sample", format.BitsPerSample)
	
	// Convert bit depth to malgo format
	var malgoFormat malgo.FormatType
	switch format.BitsPerSample {
	case 16:
		malgoFormat = malgo.FormatS16
	case 24:
		malgoFormat = malgo.FormatS24
	case 32:
		malgoFormat = malgo.FormatS32
	default:
		return nil, fmt.Errorf("unsupported bit depth: %d", format.BitsPerSample)
	}
	
	return &StreamingAudioData{
		Reader:     wavReader,
		Channels:   uint32(format.NumChannels),
		SampleRate: uint32(format.SampleRate),
		Format:     malgoFormat,
	}, nil
}

// CanDecode checks if this decoder can handle the given filename
func (d *StreamingWavDecoder) CanDecode(filename string) bool {
	lower := strings.ToLower(filename)
	return strings.HasSuffix(lower, ".wav") || strings.HasSuffix(lower, ".wave")
}

// FormatName returns the name of the format this decoder handles
func (d *StreamingWavDecoder) FormatName() string {
	return "WAV"
}

// StreamingMp3Decoder handles MP3 audio format for streaming
type StreamingMp3Decoder struct{}

// NewStreamingMp3Decoder creates a new streaming MP3 decoder
func NewStreamingMp3Decoder() *StreamingMp3Decoder {
	slog.Debug("creating new streaming MP3 decoder")
	return &StreamingMp3Decoder{}
}

// DecodeStreaming returns a reader for streaming MP3 data
func (d *StreamingMp3Decoder) DecodeStreaming(data []byte) (*StreamingAudioData, error) {
	slog.Debug("starting streaming MP3 decode")
	
	// Create a reader from the data
	reader := strings.NewReader(string(data))
	
	decoder, err := mp3.NewDecoder(reader)
	if err != nil {
		slog.Error("failed to create MP3 decoder", "error", err)
		return nil, fmt.Errorf("failed to create MP3 decoder: %w", err)
	}
	
	slog.Debug("MP3 format detected",
		"sample_rate", decoder.SampleRate())
	
	return &StreamingAudioData{
		Reader:     decoder,
		Channels:   2, // MP3 decoder always outputs stereo
		SampleRate: uint32(decoder.SampleRate()),
		Format:     malgo.FormatS16,
	}, nil
}

// CanDecode checks if this decoder can handle the given filename
func (d *StreamingMp3Decoder) CanDecode(filename string) bool {
	lower := strings.ToLower(filename)
	return strings.HasSuffix(lower, ".mp3")
}

// FormatName returns the name of the format this decoder handles
func (d *StreamingMp3Decoder) FormatName() string {
	return "MP3"
}