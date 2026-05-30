package audio

import (
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
	reader    io.ReadCloser
	format    string
	readerErr error
}

func (tas *testAudioSource) Reader() (io.ReadCloser, string, error) {
	if tas.readerErr != nil {
		return nil, "", tas.readerErr
	}
	return tas.reader, tas.format, nil
}

// TestFileSource_FilePath verifies FileSource satisfies FilePather and
// returns the configured path.
func TestFileSource_FilePath(t *testing.T) {
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

			// Must satisfy FilePather.
			var _ FilePather = fs

			result, err := fs.FilePath()
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

func TestFileSource_Reader(t *testing.T) {
	tests := []struct {
		name           string
		path           string
		expectedFormat string
		wantErr        bool
	}{
		{
			name:           "wav file format detection",
			path:           "/test/sound.wav",
			expectedFormat: "wav",
			wantErr:        true, // File doesn't exist, expect error
		},
		{
			name:           "mp3 file format detection",
			path:           "/test/sound.mp3",
			expectedFormat: "mp3",
			wantErr:        true,
		},
		{
			name:           "unknown extension",
			path:           "/test/sound.xyz",
			expectedFormat: "",
			wantErr:        true,
		},
		{
			name:           "empty path",
			path:           "",
			expectedFormat: "",
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := NewFileSource(tt.path, NewDefaultRegistry())
			reader, _, err := fs.Reader()

			if tt.wantErr && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			// For non-existent files, format detection should still work
			// before file opening fails.
			if tt.path != "" && !strings.Contains(tt.path, ".xyz") {
				expectedFormat := fs.DetectFormat()
				if expectedFormat != tt.expectedFormat {
					t.Errorf("format detection failed: expected %q, got %q", tt.expectedFormat, expectedFormat)
				}
			}

			if reader != nil {
				reader.Close()
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

// TestReaderSource_NoFilePather asserts that a ReaderSource does NOT
// satisfy FilePather — exec backends rely on the assertion to decide
// whether to fast-path or write-temp.
func TestReaderSource_NoFilePather(t *testing.T) {
	reader := io.NopCloser(strings.NewReader("test data"))
	rs := NewReaderSource(reader, "wav")

	if _, ok := any(rs).(FilePather); ok {
		t.Error("ReaderSource must not satisfy FilePather; exec backends rely on the negative type assertion")
	}
}

func TestReaderSource_Reader(t *testing.T) {
	testData := "test audio data"
	reader := io.NopCloser(strings.NewReader(testData))
	expectedFormat := "wav"

	rs := NewReaderSource(reader, expectedFormat)

	returnedReader, format, err := rs.Reader()
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
