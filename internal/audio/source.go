package audio

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

// Common errors for AudioSource implementations
var (
	ErrNotSupported = errors.New("operation not supported by this source")
	ErrInvalidFormat = errors.New("invalid audio format")
	ErrSourceClosed = errors.New("audio source is closed")
)

// AudioSource represents a source of audio data that can be played
// Implementations should provide audio data either as a file path (most efficient)
// or as a reader with format information (more flexible)
type AudioSource interface {
	// AsFilePath returns a file path if the source can provide one
	// Returns ErrNotSupported if the source cannot provide a file path
	AsFilePath() (string, error)
	
	// AsReader returns a reader for the audio data along with format information
	// Format should be a string like "wav", "mp3", etc.
	// The caller is responsible for closing the returned ReadCloser
	AsReader() (io.ReadCloser, string, error)
}

// FileSource represents an audio source backed by a file on disk
type FileSource struct {
	path     string
	registry *DecoderRegistry
}

// NewFileSource creates a new FileSource for the given file path
func NewFileSource(path string, registry *DecoderRegistry) *FileSource {
	slog.Debug("creating new FileSource", "path", path)
	return &FileSource{
		path:     path, 
		registry: registry,
	}
}

// AsFilePath returns the file path directly
func (fs *FileSource) AsFilePath() (string, error) {
	if fs.path == "" {
		slog.Error("FileSource has empty path")
		return "", fmt.Errorf("file path is empty")
	}
	
	slog.Debug("FileSource providing file path", "path", fs.path)
	return fs.path, nil
}

// AsReader opens the file and returns a reader with format detection
func (fs *FileSource) AsReader() (io.ReadCloser, string, error) {
	if fs.path == "" {
		slog.Error("FileSource has empty path for reader")
		return nil, "", fmt.Errorf("file path is empty")
	}
	
	// Detect format from file extension
	format := fs.DetectFormat()
	if format == "" {
		slog.Error("unsupported audio format", "path", fs.path)
		return nil, "", ErrInvalidFormat
	}
	
	// Open the file
	file, err := os.Open(fs.path)
	if err != nil {
		slog.Error("failed to open file", "path", fs.path, "error", err)
		return nil, "", fmt.Errorf("failed to open file: %w", err)
	}
	
	slog.Debug("FileSource providing reader", "path", fs.path, "format", format)
	return file, format, nil
}

// DetectFormat determines the audio format using the registry
func (fs *FileSource) DetectFormat() string {
	if fs.registry == nil {
		slog.Warn("no registry available for format detection", "path", fs.path)
		return ""
	}
	
	decoder := fs.registry.DetectFormat(fs.path)
	if decoder != nil {
		format := strings.ToLower(decoder.FormatName())
		slog.Debug("format detected via registry", "path", fs.path, "format", format)
		return format
	}
	
	slog.Warn("unknown audio format via registry", "path", fs.path)
	return ""
}

// ReaderSource represents an audio source backed by an io.ReadCloser
// This is useful for streaming audio data or in-memory audio
type ReaderSource struct {
	reader io.ReadCloser
	format string
}

// NewReaderSource creates a new ReaderSource with the given reader and format
func NewReaderSource(reader io.ReadCloser, format string) *ReaderSource {
	slog.Debug("creating new ReaderSource", "format", format)
	return &ReaderSource{
		reader: reader,
		format: format,
	}
}

// AsFilePath returns ErrNotSupported since ReaderSource cannot provide a file path
func (rs *ReaderSource) AsFilePath() (string, error) {
	slog.Debug("ReaderSource cannot provide file path")
	return "", ErrNotSupported
}

// AsReader returns the stored reader and format
func (rs *ReaderSource) AsReader() (io.ReadCloser, string, error) {
	if rs.reader == nil {
		slog.Error("ReaderSource has nil reader")
		return nil, "", ErrSourceClosed
	}
	
	slog.Debug("ReaderSource providing reader", "format", rs.format)
	return rs.reader, rs.format, nil
}