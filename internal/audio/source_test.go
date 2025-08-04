package audio

import (
	"errors"
	"io"
	"strings"
	"testing"
)

// TestAudioSourceInterface tests that AudioSource interface is properly defined
func TestAudioSourceInterface(t *testing.T) {
	// This test ensures the interface compiles and has expected methods
	var _ AudioSource = (*testAudioSource)(nil)
}

// testAudioSource is a mock implementation for testing
type testAudioSource struct {
	filePath  string
	reader    io.ReadCloser
	format    string
	fileErr   error
	readerErr error
}

func (tas *testAudioSource) AsFilePath() (string, error) {
	if tas.fileErr != nil {
		return "", tas.fileErr
	}
	return tas.filePath, nil
}

func (tas *testAudioSource) AsReader() (io.ReadCloser, string, error) {
	if tas.readerErr != nil {
		return nil, "", tas.readerErr
	}
	return tas.reader, tas.format, nil
}

// TestFileSource tests will go here but should fail initially since FileSource is not implemented
func TestFileSource_AsFilePath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
		wantErr  bool
	}{
		{
			name:     "valid file path",
			path:     "/test/sound.wav",
			expected: "/test/sound.wav",
			wantErr:  false,
		},
		{
			name:     "empty path",
			path:     "",
			expected: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := NewFileSource(tt.path, NewDefaultRegistry())
			result, err := fs.AsFilePath()

			if tt.wantErr && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestFileSource_AsReader(t *testing.T) {
	tests := []struct {
		name           string
		path           string
		expectedFormat string
		wantErr        bool
		fileExists     bool
	}{
		{
			name:           "wav file format detection",
			path:           "/test/sound.wav",
			expectedFormat: "wav",
			wantErr:        true, // File doesn't exist, expect error
			fileExists:     false,
		},
		{
			name:           "mp3 file format detection",
			path:           "/test/sound.mp3",
			expectedFormat: "mp3",
			wantErr:        true, // File doesn't exist, expect error
			fileExists:     false,
		},
		{
			name:           "unknown extension",
			path:           "/test/sound.xyz",
			expectedFormat: "",
			wantErr:        true, // Invalid format should error before file access
			fileExists:     false,
		},
		{
			name:           "empty path",
			path:           "",
			expectedFormat: "",
			wantErr:        true,
			fileExists:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := NewFileSource(tt.path, NewDefaultRegistry())
			reader, _, err := fs.AsReader()

			if tt.wantErr && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			// For non-existent files, we still want to test format detection
			// The format should be detected before file opening fails
			if tt.path != "" && !strings.Contains(tt.path, ".xyz") {
				expectedFormat := fs.DetectFormat()
				if expectedFormat != tt.expectedFormat {
					t.Errorf("format detection failed: expected %q, got %q", tt.expectedFormat, expectedFormat)
				}
			}

			if reader != nil {
				reader.Close() // Clean up
			}
		})
	}
}

// TestFileSource_FormatDetection tests format detection independently
func TestFileSource_FormatDetection(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{"wav extension", "/test/file.wav", "wav"},
		{"wave extension", "/test/file.wave", "wav"},
		{"mp3 extension", "/test/file.mp3", "mp3"},
		{"aiff extension", "/test/file.aiff", "aiff"},
		{"aif extension", "/test/file.aif", "aiff"},
		{"uppercase AIFF", "/test/file.AIFF", "aiff"},
		{"flac extension (unsupported)", "/test/file.flac", ""},
		{"ogg extension (unsupported)", "/test/file.ogg", ""},
		{"unknown extension", "/test/file.xyz", ""},
		{"no extension", "/test/file", ""},
		{"uppercase extension", "/test/file.WAV", "wav"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := NewFileSource(tt.path, NewDefaultRegistry())
			result := fs.DetectFormat()
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestFileSource_UsesRegistry tests that FileSource uses registry for format detection
func TestFileSource_UsesRegistry(t *testing.T) {
	registry := NewDefaultRegistry()
	fs := NewFileSource("/test/file.aiff", registry)
	format := fs.DetectFormat()
	if format != "aiff" {
		t.Errorf("expected 'aiff', got '%s'", format)
	}
}

func TestReaderSource_AsFilePath(t *testing.T) {
	reader := io.NopCloser(strings.NewReader("test data"))
	rs := NewReaderSource(reader, "wav")

	_, err := rs.AsFilePath()
	if !errors.Is(err, ErrNotSupported) {
		t.Errorf("expected ErrNotSupported, got %v", err)
	}
}

func TestReaderSource_AsReader(t *testing.T) {
	testData := "test audio data"
	reader := io.NopCloser(strings.NewReader(testData))
	expectedFormat := "wav"

	rs := NewReaderSource(reader, expectedFormat)

	returnedReader, format, err := rs.AsReader()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if format != expectedFormat {
		t.Errorf("expected format %q, got %q", expectedFormat, format)
	}
	if returnedReader == nil {
		t.Error("expected non-nil reader")
	}

	// Clean up
	if returnedReader != nil {
		returnedReader.Close()
	}
}

func TestErrorDefinitions(t *testing.T) {
	// Test that our error types are properly defined
	if ErrNotSupported == nil {
		t.Error("ErrNotSupported should be defined")
	}
	if ErrInvalidFormat == nil {
		t.Error("ErrInvalidFormat should be defined")
	}
	if ErrSourceClosed == nil {
		t.Error("ErrSourceClosed should be defined")
	}

	// Test error messages
	if ErrNotSupported.Error() != "operation not supported by this source" {
		t.Errorf("unexpected ErrNotSupported message: %s", ErrNotSupported.Error())
	}
}
