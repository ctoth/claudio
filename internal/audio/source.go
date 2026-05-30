package audio

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
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
// FileSource no longer carries a *DecoderRegistry; the decoding seam lives
// in the malgo subpackage (review finding #5). FileSource derives a naive
// format hint from filepath.Ext so SystemCommandBackend's temp-file
// fallback can name the temp with the right extension.
type FileSource struct {
	path string
}

// NewFileSource creates a new FileSource for the given file path.
func NewFileSource(path string) *FileSource {
	slog.Debug("creating new FileSource", "path", path)
	return &FileSource{path: path}
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

// Reader opens the file and returns a reader. The returned format is a
// naive extension-based hint (lowercase, dot stripped, e.g. "wav"); the
// authoritative decoder selection happens inside the decoding backend
// against the full filename.
func (fs *FileSource) Reader() (io.ReadCloser, string, error) {
	if fs.path == "" {
		slog.Error("FileSource has empty path for reader")
		return nil, "", fmt.Errorf("file path is empty")
	}

	file, err := os.Open(fs.path)
	if err != nil {
		slog.Error("failed to open file", "path", fs.path, "error", err)
		return nil, "", fmt.Errorf("failed to open file: %w", err)
	}

	format := fs.FormatHint()
	slog.Debug("FileSource providing reader", "path", fs.path, "format", format)
	return file, format, nil
}

// FormatHint returns a lowercase extension-based format hint (without
// the leading dot) for the source's path. Returns empty string if the
// path has no extension. Callers needing authoritative format detection
// should pass the full path to their decoder registry — this hint is for
// naming temp files in the exec-backend fallback path.
func (fs *FileSource) FormatHint() string {
	ext := filepath.Ext(fs.path)
	if ext == "" {
		return ""
	}
	return strings.ToLower(strings.TrimPrefix(ext, "."))
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
