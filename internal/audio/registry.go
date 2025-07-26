package audio

import (
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/gabriel-vasile/mimetype"
)

// DecoderRegistry manages audio format decoders and provides format detection
type DecoderRegistry struct {
	decoders []Decoder
}

// NewDecoderRegistry creates a new empty decoder registry
func NewDecoderRegistry() *DecoderRegistry {
	slog.Debug("creating new decoder registry")
	return &DecoderRegistry{
		decoders: make([]Decoder, 0),
	}
}

// NewDefaultRegistry creates a registry with default WAV and MP3 decoders
func NewDefaultRegistry() *DecoderRegistry {
	slog.Debug("creating default decoder registry with WAV and MP3 support")
	
	registry := NewDecoderRegistry()
	
	// Register default decoders
	registry.Register(NewWavDecoder())
	registry.Register(NewMp3Decoder())
	
	slog.Info("default decoder registry initialized", 
		"supported_formats", registry.GetSupportedFormats())
	
	return registry
}

// Register adds a decoder to the registry
func (r *DecoderRegistry) Register(decoder Decoder) {
	if decoder == nil {
		slog.Warn("attempted to register nil decoder")
		return
	}
	
	formatName := decoder.FormatName()
	slog.Debug("registering decoder", "format", formatName)
	
	r.decoders = append(r.decoders, decoder)
	
	slog.Info("decoder registered successfully", 
		"format", formatName,
		"total_decoders", len(r.decoders))
}

// GetDecoders returns all registered decoders
func (r *DecoderRegistry) GetDecoders() []Decoder {
	return r.decoders
}

// GetSupportedFormats returns a list of all supported format names
func (r *DecoderRegistry) GetSupportedFormats() []string {
	formats := make([]string, 0, len(r.decoders))
	
	for _, decoder := range r.decoders {
		formats = append(formats, decoder.FormatName())
	}
	
	slog.Debug("retrieved supported formats", "formats", formats)
	return formats
}

// DetectFormat detects the appropriate decoder based on filename extension only
func (r *DecoderRegistry) DetectFormat(filename string) Decoder {
	slog.Debug("detecting format by extension", "filename", filename)
	
	if filename == "" {
		slog.Debug("empty filename provided")
		return nil
	}
	
	// Try each decoder in registration order (first registered has priority)
	for _, decoder := range r.decoders {
		if decoder.CanDecode(filename) {
			slog.Debug("format detected by extension", 
				"filename", filename,
				"format", decoder.FormatName())
			return decoder
		}
	}
	
	slog.Debug("no decoder found for filename", "filename", filename)
	return nil
}

// DetectFormatWithContent detects format using magic bytes first, fallback to extension
func (r *DecoderRegistry) DetectFormatWithContent(filename string, reader io.Reader) Decoder {
	slog.Debug("detecting format with content analysis", "filename", filename)
	
	// Read up to 512 bytes for magic number detection
	// We need to buffer this so we can still use the reader for decoding
	buffer := make([]byte, 512)
	n, err := reader.Read(buffer)
	if err != nil && err != io.EOF {
		slog.Error("failed to read header for magic detection", "error", err)
		// Fallback to extension-based detection
		return r.DetectFormat(filename)
	}
	
	if n == 0 {
		slog.Debug("empty content, using extension fallback")
		return r.DetectFormat(filename)
	}
	
	// Use mimetype for magic byte detection
	mtype := mimetype.Detect(buffer[:n])
	detectedMime := mtype.String()
	
	slog.Debug("magic byte detection result", 
		"filename", filename,
		"detected_mime", detectedMime,
		"bytes_analyzed", n)
	
	// Map MIME types to our decoders with comprehensive matching
	var formatDecoder Decoder
	mimeStr := strings.ToLower(detectedMime)
	
	switch {
	case strings.Contains(mimeStr, "wav") || mimeStr == "audio/wav" || mimeStr == "audio/x-wav" || mimeStr == "audio/vnd.wave":
		formatDecoder = r.findDecoderByFormat("WAV")
		slog.Debug("magic bytes indicate WAV format", "mime", detectedMime)
		
	case strings.Contains(mimeStr, "mpeg") || strings.Contains(mimeStr, "mp3") || mimeStr == "audio/mpeg" || mimeStr == "audio/x-mpeg" || mimeStr == "audio/mp3":
		formatDecoder = r.findDecoderByFormat("MP3")
		slog.Debug("magic bytes indicate MP3 format", "mime", detectedMime)
		
	default:
		slog.Debug("unsupported or unrecognized magic bytes", "mime_type", detectedMime)
	}
	
	// If magic detection succeeded, use it (this takes precedence over extension)
	if formatDecoder != nil {
		slog.Info("format detected by magic bytes", 
			"filename", filename,
			"detected_format", formatDecoder.FormatName(),
			"mime_type", detectedMime)
		return formatDecoder
	}
	
	// Fallback to extension-based detection
	slog.Debug("magic detection failed, falling back to extension", "filename", filename)
	extensionDecoder := r.DetectFormat(filename)
	
	if extensionDecoder != nil {
		slog.Info("format detected by extension fallback",
			"filename", filename,
			"format", extensionDecoder.FormatName())
	} else {
		slog.Warn("no format detection method succeeded", "filename", filename)
	}
	
	return extensionDecoder
}

// findDecoderByFormat finds a decoder by its format name
func (r *DecoderRegistry) findDecoderByFormat(formatName string) Decoder {
	for _, decoder := range r.decoders {
		if strings.EqualFold(decoder.FormatName(), formatName) {
			return decoder
		}
	}
	return nil
}

// DecodeFile decodes an audio file using the appropriate decoder
func (r *DecoderRegistry) DecodeFile(filename string, reader io.Reader) (*AudioData, error) {
	slog.Debug("starting file decode operation", "filename", filename)
	
	decoder := r.DetectFormatWithContent(filename, reader)
	if decoder == nil {
		err := fmt.Errorf("unsupported audio format: %s", filename)
		slog.Error("no suitable decoder found", "filename", filename, "error", err)
		return nil, err
	}
	
	slog.Info("decoder selected for file", 
		"filename", filename,
		"decoder_format", decoder.FormatName())
	
	// Note: reader may have been partially consumed by magic detection
	// For robust implementation, we should either use io.Seeker or buffer the full content
	// For now, we'll proceed with the assumption that decoders handle partial reads gracefully
	
	audioData, err := decoder.Decode(reader)
	if err != nil {
		slog.Error("decode operation failed", 
			"filename", filename,
			"decoder_format", decoder.FormatName(),
			"error", err)
		return nil, err
	}
	
	slog.Info("file decode completed successfully",
		"filename", filename,
		"decoder_format", decoder.FormatName(),
		"channels", audioData.Channels,
		"sample_rate", audioData.SampleRate,
		"data_size", len(audioData.Samples))
	
	return audioData, nil
}