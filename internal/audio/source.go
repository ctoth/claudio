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
	ErrNotSupported  = errors.New("operation not supported by this source")
	ErrInvalidFormat = errors.New("invalid audio format")
	ErrSourceClosed  = errors.New("audio source is closed")
)

// AudioSource represents a source of audio data that can be played. Every
// source can be read; backends that prefer a raw file path (e.g.
// SystemCommandBackend which exec's a player binary) can opt in to that
// fast-path via a FilePather type assertion. The previous dual API
// (AsFilePath + AsReader) was paid for by every consumer for a fast-path
// that only the exec backend used; review finding #42 collapses it.
type AudioSource interface {
	// Reader returns an io.ReadCloser over the audio bytes plus a short
	// lowercase format name (e.g. "wav", "mp3"). The caller is responsible
	// for closing the returned ReadCloser.
	Reader() (io.ReadCloser, string, error)
}

// FilePather is an optional capability for sources backed by a file on
// disk. Exec-style backends (SystemCommandBackend) prefer to pass the path
// directly to a player binary rather than read-then-write-temp; they
// type-assert to FilePather and fall back to Reader if the assertion
// fails.
type FilePather interface {
	FilePath() (string, error)
}

// FileSource represents an audio source backed by a file on disk.
type FileSource struct {
	path     string
	registry *DecoderRegistry
}

// NewFileSource creates a new FileSource for the given file path.
func NewFileSource(path string, registry *DecoderRegistry) *FileSource {
	slog.Debug("creating new FileSource", "path", path)
	return &FileSource{
		path:     path,
		registry: registry,
	}
}

// FilePath returns the file path directly. Satisfies FilePather so exec
// backends can skip the open-read-temp dance.
func (fs *FileSource) FilePath() (string, error) {
	if fs.path == "" {
		slog.Error("FileSource has empty path")
		return "", fmt.Errorf("file path is empty")
	}
	slog.Debug("FileSource providing file path", "path", fs.path)
	return fs.path, nil
}

// Reader opens the file and returns a reader with format detection.
func (fs *FileSource) Reader() (io.ReadCloser, string, error) {
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

// DetectFormat determines the audio format using the registry.
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

// ReaderSource represents an audio source backed by an io.ReadCloser. It
// is used by tests and any in-memory or streaming consumer; it
// deliberately does NOT implement FilePather so exec backends fall through
// to their write-temp-file path.
type ReaderSource struct {
	reader io.ReadCloser
	format string
}

// NewReaderSource creates a new ReaderSource with the given reader and format.
func NewReaderSource(reader io.ReadCloser, format string) *ReaderSource {
	slog.Debug("creating new ReaderSource", "format", format)
	return &ReaderSource{
		reader: reader,
		format: format,
	}
}

// Reader returns the stored reader and format.
func (rs *ReaderSource) Reader() (io.ReadCloser, string, error) {
	if rs.reader == nil {
		slog.Error("ReaderSource has nil reader")
		return nil, "", ErrSourceClosed
	}

	slog.Debug("ReaderSource providing reader", "format", rs.format)
	return rs.reader, rs.format, nil
}
