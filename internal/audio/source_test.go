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
			fs := NewFileSource(tt.path)

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
			name:           "wav file extension hint",
			path:           "/test/sound.wav",
			expectedFormat: "wav",
			wantErr:        true, // File doesn't exist
		},
		{
			name:           "mp3 file extension hint",
			path:           "/test/sound.mp3",
			expectedFormat: "mp3",
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
			fs := NewFileSource(tt.path)
			reader, _, err := fs.Reader()

			if tt.wantErr && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if tt.path != "" {
				hint := fs.FormatHint()
				if hint != tt.expectedFormat {
					t.Errorf("format hint mismatch: expected %q, got %q", tt.expectedFormat, hint)
				}
			}

			if reader != nil {
				reader.Close()
			}
		})
	}
}

// TestFileSource_FormatHint tests extension-based hint mapping.
func TestFileSource_FormatHint(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{"wav extension", "/test/file.wav", "wav"},
		{"mp3 extension", "/test/file.mp3", "mp3"},
		{"aiff extension", "/test/file.aiff", "aiff"},
		{"uppercase WAV becomes wav", "/test/file.WAV", "wav"},
		{"no extension", "/test/file", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := NewFileSource(tt.path)
			result := fs.FormatHint()
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
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

	if returnedReader != nil {
		returnedReader.Close()
	}
}

func TestErrorDefinitions(t *testing.T) {
	if ErrNotSupported == nil {
		t.Error("ErrNotSupported should be defined")
	}
	if ErrInvalidFormat == nil {
		t.Error("ErrInvalidFormat should be defined")
	}
	if ErrSourceClosed == nil {
		t.Error("ErrSourceClosed should be defined")
	}

	if ErrNotSupported.Error() != "operation not supported by this source" {
		t.Errorf("unexpected ErrNotSupported message: %s", ErrNotSupported.Error())
	}
}
