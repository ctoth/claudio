//go:build cgo

package malgo

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"

	"claudio.click/internal/safeio"
	"github.com/gabriel-vasile/mimetype"
)

// mimeToFormat maps canonical MIME types to format names recognized by the
// registry. The match is exact (no substring), so e.g. audio/x-wavpack does
// not collide with the WAV decoder.
var mimeToFormat = map[string]string{
	"audio/wav":      "WAV",
	"audio/x-wav":    "WAV",
	"audio/vnd.wave": "WAV",
	"audio/wave":     "WAV",
	"audio/mpeg":     "MP3",
	"audio/x-mpeg":   "MP3",
	"audio/mp3":      "MP3",
	"audio/aiff":     "AIFF",
	"audio/x-aiff":   "AIFF",
}

// DecoderRegistry manages audio format decoders and provides format detection.
//
// All public methods are safe to call concurrently. Register takes a write
// lock; the detection / decode helpers take read locks while they iterate
// the decoders slice.
type DecoderRegistry struct {
	mu       sync.RWMutex
	decoders []Decoder
}

// NewDecoderRegistry creates a new empty decoder registry
func NewDecoderRegistry() *DecoderRegistry {
	slog.Debug("creating new decoder registry")
	return &DecoderRegistry{
		decoders: make([]Decoder, 0),
	}
}

// NewDefaultRegistry creates a registry with default WAV, MP3, and AIFF decoders
func NewDefaultRegistry() *DecoderRegistry {
	slog.Debug("creating default decoder registry with WAV, MP3, and AIFF support")

	registry := NewDecoderRegistry()

	// Register default decoders
	registry.Register(NewWavDecoder())
	registry.Register(NewMp3Decoder())
	registry.Register(NewAiffDecoder())

	slog.Debug("default decoder registry initialized",
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

	r.mu.Lock()
	r.decoders = append(r.decoders, decoder)
	total := len(r.decoders)
	r.mu.Unlock()

	slog.Debug("decoder registered successfully",
		"format", formatName,
		"total_decoders", total)
}

// GetDecoders returns a snapshot of the registered decoders. The returned
// slice is safe to iterate even if Register is called concurrently afterwards.
func (r *DecoderRegistry) GetDecoders() []Decoder {
	r.mu.RLock()
	defer r.mu.RUnlock()
	snapshot := make([]Decoder, len(r.decoders))
	copy(snapshot, r.decoders)
	return snapshot
}

// GetSupportedFormats returns a list of all supported format names
func (r *DecoderRegistry) GetSupportedFormats() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

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

	r.mu.RLock()
	defer r.mu.RUnlock()

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
	readFailed := err != nil && err != io.EOF
	if readFailed {
		slog.Error("failed to read header for magic detection", "error", err)
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	if readFailed {
		// Fallback to extension-based detection (lock-free helper)
		return r.detectFormatLocked(filename)
	}

	if n == 0 {
		slog.Debug("empty content, using extension fallback")
		return r.detectFormatLocked(filename)
	}

	// Use mimetype for magic byte detection
	mtype := mimetype.Detect(buffer[:n])
	detectedMime := mtype.String()

	slog.Debug("magic byte detection result",
		"filename", filename,
		"detected_mime", detectedMime,
		"bytes_analyzed", n)

	// Exact-match the canonical MIME set. Substring matching previously
	// allowed e.g. audio/x-wavpack to misroute to the WAV decoder.
	mimeStr := strings.ToLower(detectedMime)
	var formatDecoder Decoder
	if formatName, ok := mimeToFormat[mimeStr]; ok {
		formatDecoder = r.findDecoderByFormatLocked(formatName)
		slog.Debug("magic bytes recognized", "mime", detectedMime, "format", formatName)
	} else {
		slog.Debug("unsupported or unrecognized magic bytes", "mime_type", detectedMime)
	}

	// If magic detection succeeded, use it (this takes precedence over extension)
	if formatDecoder != nil {
		slog.Debug("format detected by magic bytes",
			"filename", filename,
			"detected_format", formatDecoder.FormatName(),
			"mime_type", detectedMime)
		return formatDecoder
	}

	// Fallback to extension-based detection (lock-free helper)
	slog.Debug("magic detection failed, falling back to extension", "filename", filename)
	extensionDecoder := r.detectFormatLocked(filename)

	if extensionDecoder != nil {
		slog.Debug("format detected by extension fallback",
			"filename", filename,
			"format", extensionDecoder.FormatName())
	} else {
		slog.Warn("no format detection method succeeded", "filename", filename)
	}

	return extensionDecoder
}

// detectFormatLocked is the lock-free extension-only detection used by
// callers that already hold r.mu (read or write).
func (r *DecoderRegistry) detectFormatLocked(filename string) Decoder {
	if filename == "" {
		return nil
	}
	for _, decoder := range r.decoders {
		if decoder.CanDecode(filename) {
			return decoder
		}
	}
	return nil
}

// findDecoderByFormatLocked finds a decoder by its format name. Caller must
// already hold r.mu (read or write).
func (r *DecoderRegistry) findDecoderByFormatLocked(formatName string) Decoder {
	for _, decoder := range r.decoders {
		if strings.EqualFold(decoder.FormatName(), formatName) {
			return decoder
		}
	}
	return nil
}

// DecodeFile decodes an audio file using the appropriate decoder. ctx is
// forwarded to the underlying decoder so a stalled decode can be cancelled.
func (r *DecoderRegistry) DecodeFile(ctx context.Context, filename string, reader io.Reader) (*AudioData, error) {
	slog.Debug("starting file decode operation", "filename", filename)

	// Buffer the entire content to avoid reader consumption issues during format detection
	fullContent, err := safeio.ReadAllCapped(reader, safeio.MaxAudioFileBytes, "audio file")
	if err != nil {
		slog.Error("failed to read file content for decode", "filename", filename, "error", err)
		return nil, fmt.Errorf("failed to read file content: %w", err)
	}

	slog.Debug("buffered file content for decode", "filename", filename, "size_bytes", len(fullContent))

	// Use buffered content for format detection
	contentReader := bytes.NewReader(fullContent)
	decoder := r.DetectFormatWithContent(filename, contentReader)
	if decoder == nil {
		err := fmt.Errorf("unsupported audio format: %s", filename)
		slog.Error("no suitable decoder found", "filename", filename, "error", err)
		return nil, err
	}

	slog.Debug("decoder selected for file",
		"filename", filename,
		"decoder_format", decoder.FormatName())

	// Create fresh reader from buffered content for decoder
	decoderReader := bytes.NewReader(fullContent)
	audioData, err := decoder.Decode(ctx, decoderReader)
	if err != nil {
		slog.Error("decode operation failed",
			"filename", filename,
			"decoder_format", decoder.FormatName(),
			"error", err)
		return nil, err
	}

	slog.Debug("file decode completed successfully",
		"filename", filename,
		"decoder_format", decoder.FormatName(),
		"channels", audioData.Channels,
		"sample_rate", audioData.SampleRate,
		"data_size", len(audioData.Samples))

	return audioData, nil
}
