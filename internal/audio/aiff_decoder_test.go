package audio

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/gen2brain/malgo"
)

func TestAiffDecoderInterface(t *testing.T) {
	decoder := NewAiffDecoder()
	
	// Test interface compliance
	var _ Decoder = decoder
	
	// Test format name
	if decoder.FormatName() != "AIFF" {
		t.Errorf("expected format name 'AIFF', got '%s'", decoder.FormatName())
	}
}

func TestAiffDecoderCanDecode(t *testing.T) {
	decoder := NewAiffDecoder()
	
	testCases := []struct {
		filename string
		expected bool
	}{
		{"audio.aiff", true},
		{"sound.AIFF", true},
		{"music.aif", true},
		{"test.AIF", true},
		{"audio.mp3", false},
		{"sound.wav", false},
		{"sound.flac", false},
		{"", false},
		{"aiff", false},
		{"audio.aiff.backup", false},
	}
	
	for _, tc := range testCases {
		result := decoder.CanDecode(tc.filename)
		if result != tc.expected {
			t.Errorf("CanDecode('%s') = %v, expected %v", tc.filename, result, tc.expected)
		}
	}
}

func TestAiffDecoderDecodeInvalidData(t *testing.T) {
	decoder := NewAiffDecoder()
	
	testCases := []struct {
		name string
		data []byte
	}{
		{"empty data", []byte{}},
		{"invalid data", []byte("not an aiff file")},
		{"partial header", []byte("FORM")},
		{"wrong format", []byte("RIFF" + string(make([]byte, 100)))},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reader := bytes.NewReader(tc.data)
			audioData, err := decoder.Decode(reader)
			
			if err == nil {
				t.Errorf("expected error for %s, got nil", tc.name)
			}
			
			if audioData != nil {
				t.Errorf("expected nil audio data for %s, got %v", tc.name, audioData)
			}
			
			// Should return appropriate error types
			if err != ErrInvalidData && err != ErrReadFailure {
				t.Errorf("expected ErrInvalidData or ErrReadFailure, got %v", err)
			}
		})
	}
}

func TestAiffDecoderDecodeValidData(t *testing.T) {
	decoder := NewAiffDecoder()
	
	// Create minimal valid AIFF data (will be replaced with actual data once we have generator)
	aiffData := createMinimalAiffFile(44100, 2, 16, 1000) // 44.1kHz, stereo, 16-bit, 1000 samples
	
	reader := bytes.NewReader(aiffData)
	audioData, err := decoder.Decode(reader)
	
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	
	if audioData == nil {
		t.Fatal("expected audio data, got nil")
	}
	
	// Test expected format
	if audioData.SampleRate != 44100 {
		t.Errorf("expected sample rate 44100, got %d", audioData.SampleRate)
	}
	
	if audioData.Channels != 2 {
		t.Errorf("expected 2 channels, got %d", audioData.Channels)
	}
	
	if audioData.Format != malgo.FormatS16 {
		t.Errorf("expected format S16, got %v", audioData.Format)
	}
	
	if len(audioData.Samples) == 0 {
		t.Error("expected audio samples, got empty slice")
	}
	
	// For 16-bit stereo, each sample should be 4 bytes (2 bytes per channel)
	expectedBytes := 1000 * 2 * 2 // samples * channels * bytes_per_sample
	if len(audioData.Samples) != expectedBytes {
		t.Errorf("expected %d sample bytes, got %d", expectedBytes, len(audioData.Samples))
	}
}

func TestAiffDecoderDifferentBitDepths(t *testing.T) {
	decoder := NewAiffDecoder()
	
	testCases := []struct {
		bitDepth       int
		expectedFormat malgo.FormatType
		bytesPerSample int
	}{
		{16, malgo.FormatS16, 2},
		{24, malgo.FormatS24, 3},
		{32, malgo.FormatS32, 4},
	}
	
	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%d-bit", tc.bitDepth), func(t *testing.T) {
			aiffData := createMinimalAiffFile(44100, 1, tc.bitDepth, 100) // mono, 100 samples
			
			reader := bytes.NewReader(aiffData)
			audioData, err := decoder.Decode(reader)
			
			if err != nil {
				t.Fatalf("expected no error for %d-bit, got %v", tc.bitDepth, err)
			}
			
			if audioData.Format != tc.expectedFormat {
				t.Errorf("expected format %v for %d-bit, got %v", tc.expectedFormat, tc.bitDepth, audioData.Format)
			}
			
			expectedBytes := 100 * 1 * tc.bytesPerSample // samples * channels * bytes_per_sample
			if len(audioData.Samples) != expectedBytes {
				t.Errorf("expected %d bytes for %d-bit, got %d", expectedBytes, tc.bitDepth, len(audioData.Samples))
			}
		})
	}
}

func TestAiffDecoderMonoAndStereo(t *testing.T) {
	decoder := NewAiffDecoder()
	
	testCases := []struct {
		name     string
		channels int
	}{
		{"mono", 1},
		{"stereo", 2},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			aiffData := createMinimalAiffFile(44100, tc.channels, 16, 100)
			
			reader := bytes.NewReader(aiffData)
			audioData, err := decoder.Decode(reader)
			
			if err != nil {
				t.Fatalf("expected no error for %s, got %v", tc.name, err)
			}
			
			if int(audioData.Channels) != tc.channels {
				t.Errorf("expected %d channels, got %d", tc.channels, audioData.Channels)
			}
			
			expectedBytes := 100 * tc.channels * 2 // samples * channels * 2 bytes (16-bit)
			if len(audioData.Samples) != expectedBytes {
				t.Errorf("expected %d bytes for %s, got %d", expectedBytes, tc.name, len(audioData.Samples))
			}
		})
	}
}

// Helper function to create minimal AIFF file for testing
// This will be implemented after we create the test file generator
func createMinimalAiffFile(sampleRate, channels, bitDepth, numSamples int) []byte {
	// Placeholder - will be implemented in test generator
	// For now, return empty bytes to make tests compile but fail
	return []byte{}
}