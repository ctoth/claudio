package audio

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/gen2brain/malgo"
	"github.com/go-audio/aiff"
	"github.com/go-audio/audio"
)

// AiffDecoder handles AIFF audio format decoding
type AiffDecoder struct{}

// NewAiffDecoder creates a new AIFF decoder instance
func NewAiffDecoder() *AiffDecoder {
	slog.Debug("creating new AIFF decoder instance")
	return &AiffDecoder{}
}

// FormatName returns the name of the format this decoder handles
func (d *AiffDecoder) FormatName() string {
	return "AIFF"
}

// CanDecode checks if this decoder can handle the given filename
func (d *AiffDecoder) CanDecode(filename string) bool {
	lower := strings.ToLower(filename)
	canDecode := strings.HasSuffix(lower, ".aiff") || strings.HasSuffix(lower, ".aif")

	slog.Debug("AIFF decoder file check",
		"filename", filename,
		"can_decode", canDecode)

	return canDecode
}

// Decode reads AIFF audio data from reader and returns decoded PCM data
func (d *AiffDecoder) Decode(reader io.Reader) (*AudioData, error) {
	slog.Debug("starting AIFF decode operation")

	// Read all data from reader (go-audio/aiff needs a ReadSeeker)
	data, err := io.ReadAll(reader)
	if err != nil {
		slog.Error("failed to read AIFF data", "error", err)
		return nil, ErrReadFailure
	}

	if len(data) == 0 {
		slog.Error("empty AIFF data")
		return nil, ErrInvalidData
	}

	// Create a ReadSeeker from the data
	seekReader := bytes.NewReader(data)

	// Create AIFF decoder
	decoder := aiff.NewDecoder(seekReader)

	// Read format information
	decoder.ReadInfo()

	// Validate file format
	if !decoder.IsValidFile() {
		slog.Error("invalid AIFF file format")
		return nil, ErrInvalidData
	}

	// Get format information
	format := decoder.Format()
	if format == nil {
		slog.Error("failed to get AIFF format")
		return nil, ErrInvalidData
	}

	sampleRate := uint32(decoder.SampleRate)
	channels := uint32(decoder.NumChans)
	bitDepth := decoder.SampleBitDepth()

	slog.Debug("AIFF format detected",
		"sample_rate", sampleRate,
		"channels", channels,
		"bits_per_sample", bitDepth)

	// Validate format parameters
	if channels == 0 || sampleRate == 0 || bitDepth == 0 {
		slog.Error("invalid AIFF format parameters",
			"channels", channels,
			"sample_rate", sampleRate,
			"bit_depth", bitDepth)
		return nil, ErrInvalidData
	}

	// Convert bit depth to malgo format
	var malgoFormat malgo.FormatType
	switch bitDepth {
	case 16:
		malgoFormat = malgo.FormatS16
		slog.Debug("using 16-bit signed format")
	case 24:
		malgoFormat = malgo.FormatS24
		slog.Debug("using 24-bit signed format")
	case 32:
		malgoFormat = malgo.FormatS32
		slog.Debug("using 32-bit signed format")
	default:
		slog.Error("unsupported bit depth", "bits", bitDepth)
		return nil, ErrUnsupportedFormat
	}

	// Read all audio samples into memory
	slog.Debug("reading AIFF audio samples")

	// Use FullPCMBuffer to get all samples at once
	pcmBuffer, err := decoder.FullPCMBuffer()
	if err != nil {
		slog.Error("failed to read AIFF samples", "error", err)
		return nil, ErrReadFailure
	}

	if pcmBuffer == nil || len(pcmBuffer.Data) == 0 {
		slog.Error("no audio data found in AIFF file")
		return nil, ErrInvalidData
	}

	slog.Debug("AIFF samples read successfully",
		"total_samples", len(pcmBuffer.Data),
		"buffer_format", pcmBuffer.Format.SampleRate,
		"buffer_channels", pcmBuffer.Format.NumChannels)

	// Convert PCM buffer to raw bytes based on bit depth
	rawBytes, err := d.convertPCMBufferToBytes(pcmBuffer, int(bitDepth), int(channels))
	if err != nil {
		slog.Error("failed to convert PCM buffer to bytes", "error", err)
		return nil, ErrReadFailure
	}

	audioData := &AudioData{
		Samples:    rawBytes,
		Channels:   channels,
		SampleRate: sampleRate,
		Format:     malgoFormat,
	}

	slog.Info("AIFF decode completed successfully",
		"total_bytes", len(rawBytes),
		"total_samples", len(pcmBuffer.Data),
		"channels", audioData.Channels,
		"sample_rate", audioData.SampleRate,
		"format", malgoFormat,
		"duration_estimate_ms", (len(pcmBuffer.Data)*1000)/(int(audioData.SampleRate)*int(audioData.Channels)))

	return audioData, nil
}

// convertPCMBufferToBytes converts audio.IntBuffer to raw byte array based on bit depth
func (d *AiffDecoder) convertPCMBufferToBytes(pcmBuffer *audio.IntBuffer, bitDepth, channels int) ([]byte, error) {
	slog.Debug("converting PCM buffer to bytes",
		"bit_depth", bitDepth,
		"channels", channels,
		"samples", len(pcmBuffer.Data))

	if len(pcmBuffer.Data) == 0 {
		return nil, fmt.Errorf("empty PCM buffer")
	}

	bytesPerSample := bitDepth / 8
	totalBytes := len(pcmBuffer.Data) * bytesPerSample
	rawBytes := make([]byte, totalBytes)

	// Convert each sample based on bit depth
	buf := bytes.NewBuffer(rawBytes[:0])

	for _, sample := range pcmBuffer.Data {
		switch bitDepth {
		case 16:
			// Convert to 16-bit signed integer
			val := int16(sample)
			err := binary.Write(buf, binary.LittleEndian, val)
			if err != nil {
				return nil, fmt.Errorf("failed to write 16-bit sample: %w", err)
			}

		case 24:
			// Convert to 24-bit signed integer (3 bytes)
			val := int32(sample)
			// Write as little-endian 24-bit (3 bytes)
			err := buf.WriteByte(byte(val))
			if err != nil {
				return nil, fmt.Errorf("failed to write 24-bit sample byte 1: %w", err)
			}
			err = buf.WriteByte(byte(val >> 8))
			if err != nil {
				return nil, fmt.Errorf("failed to write 24-bit sample byte 2: %w", err)
			}
			err = buf.WriteByte(byte(val >> 16))
			if err != nil {
				return nil, fmt.Errorf("failed to write 24-bit sample byte 3: %w", err)
			}

		case 32:
			// Convert to 32-bit signed integer
			val := int32(sample)
			err := binary.Write(buf, binary.LittleEndian, val)
			if err != nil {
				return nil, fmt.Errorf("failed to write 32-bit sample: %w", err)
			}

		default:
			return nil, fmt.Errorf("unsupported bit depth: %d", bitDepth)
		}
	}

	result := buf.Bytes()
	slog.Debug("PCM buffer conversion completed",
		"input_samples", len(pcmBuffer.Data),
		"output_bytes", len(result),
		"bytes_per_sample", bytesPerSample)

	return result, nil
}
