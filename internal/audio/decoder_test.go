package audio

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/gen2brain/malgo"
)

// MockDecoder for testing
type MockDecoder struct {
	formatName string
	extensions []string
	shouldFail bool
	returnData *AudioData
}

func (m *MockDecoder) Decode(reader io.Reader) (*AudioData, error) {
	if m.shouldFail {
		return nil, ErrUnsupportedFormat
	}
	if m.returnData != nil {
		return m.returnData, nil
	}

	// Return default test data
	return &AudioData{
		Samples:    []byte{0x00, 0x01, 0x02, 0x03}, // Test PCM data
		Channels:   2,                              // Stereo
		SampleRate: 44100,                          // Standard rate
		Format:     malgo.FormatS16,                // 16-bit signed
	}, nil
}

func (m *MockDecoder) CanDecode(filename string) bool {
	lower := strings.ToLower(filename)
	for _, ext := range m.extensions {
		lowerExt := strings.ToLower(ext)
		if len(lower) >= len(lowerExt) && lower[len(lower)-len(lowerExt):] == lowerExt {
			return true
		}
	}
	return false
}

func (m *MockDecoder) FormatName() string {
	return m.formatName
}

// DecoderError represents decoder-specific errors
type DecoderError struct {
	message string
}

func NewDecoderError(message string) *DecoderError {
	return &DecoderError{message: message}
}

func (e *DecoderError) Error() string {
	return e.message
}

func TestAudioDataStructure(t *testing.T) {
	data := &AudioData{
		Samples:    []byte{0x01, 0x02, 0x03, 0x04},
		Channels:   2,
		SampleRate: 44100,
		Format:     malgo.FormatS16,
	}

	// Test basic properties
	if len(data.Samples) != 4 {
		t.Errorf("expected 4 samples, got %d", len(data.Samples))
	}

	if data.Channels != 2 {
		t.Errorf("expected 2 channels, got %d", data.Channels)
	}

	if data.SampleRate != 44100 {
		t.Errorf("expected 44100 sample rate, got %d", data.SampleRate)
	}

	if data.Format != malgo.FormatS16 {
		t.Errorf("expected FormatS16, got %v", data.Format)
	}
}

func TestMockDecoderInterface(t *testing.T) {
	decoder := &MockDecoder{
		formatName: "TEST",
		extensions: []string{".test", ".tst"},
		shouldFail: false,
	}

	// Test interface compliance
	var _ Decoder = decoder

	// Test format name
	if decoder.FormatName() != "TEST" {
		t.Errorf("expected format name 'TEST', got '%s'", decoder.FormatName())
	}

	// Test file detection
	testCases := []struct {
		filename string
		expected bool
	}{
		{"audio.test", true},
		{"sound.tst", true},
		{"music.wav", false},
		{"", false},
		{"test", false},
		{"audio.test.backup", false},
	}

	for _, tc := range testCases {
		result := decoder.CanDecode(tc.filename)
		if result != tc.expected {
			t.Errorf("CanDecode('%s') = %v, expected %v", tc.filename, result, tc.expected)
		}
	}
}

func TestMockDecoderDecoding(t *testing.T) {
	t.Run("successful decoding", func(t *testing.T) {
		decoder := &MockDecoder{
			formatName: "TEST",
			extensions: []string{".test"},
			shouldFail: false,
		}

		reader := bytes.NewReader([]byte("test audio data"))
		data, err := decoder.Decode(reader)

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if data == nil {
			t.Fatal("expected audio data, got nil")
		}

		if data.Channels != 2 {
			t.Errorf("expected 2 channels, got %d", data.Channels)
		}

		if data.SampleRate != 44100 {
			t.Errorf("expected 44100 sample rate, got %d", data.SampleRate)
		}

		if data.Format != malgo.FormatS16 {
			t.Errorf("expected FormatS16, got %v", data.Format)
		}
	})

	t.Run("decoding failure", func(t *testing.T) {
		decoder := &MockDecoder{
			formatName: "TEST",
			extensions: []string{".test"},
			shouldFail: true,
		}

		reader := bytes.NewReader([]byte("invalid data"))
		data, err := decoder.Decode(reader)

		if err == nil {
			t.Fatal("expected error, got nil")
		}

		if data != nil {
			t.Error("expected nil data on error")
		}

		if err != ErrUnsupportedFormat {
			t.Errorf("expected ErrUnsupportedFormat, got %v", err)
		}
	})
}

func TestMockDecoderCustomData(t *testing.T) {
	customData := &AudioData{
		Samples:    []byte{0xFF, 0xFE, 0xFD, 0xFC},
		Channels:   1, // Mono
		SampleRate: 22050,
		Format:     malgo.FormatF32,
	}

	decoder := &MockDecoder{
		formatName: "CUSTOM",
		extensions: []string{".custom"},
		shouldFail: false,
		returnData: customData,
	}

	reader := bytes.NewReader([]byte("test"))
	data, err := decoder.Decode(reader)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if data != customData {
		t.Error("expected custom data to be returned")
	}

	if data.Channels != 1 {
		t.Errorf("expected 1 channel, got %d", data.Channels)
	}

	if data.SampleRate != 22050 {
		t.Errorf("expected 22050 sample rate, got %d", data.SampleRate)
	}

	if data.Format != malgo.FormatF32 {
		t.Errorf("expected FormatF32, got %v", data.Format)
	}
}

func TestDecoderErrorHandling(t *testing.T) {
	err := NewDecoderError("test error message")

	if err.Error() != "test error message" {
		t.Errorf("expected 'test error message', got '%s'", err.Error())
	}

	// Test predefined errors
	if ErrUnsupportedFormat.Error() != "unsupported audio format" {
		t.Errorf("unexpected ErrUnsupportedFormat message: %s", ErrUnsupportedFormat.Error())
	}

	if ErrInvalidData.Error() != "invalid audio data" {
		t.Errorf("unexpected ErrInvalidData message: %s", ErrInvalidData.Error())
	}

	if ErrReadFailure.Error() != "failed to read audio data" {
		t.Errorf("unexpected ErrReadFailure message: %s", ErrReadFailure.Error())
	}
}
