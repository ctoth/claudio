package audio

import (
	"bytes"
	"testing"

	"github.com/gen2brain/malgo"
)

func TestWavDecoderInterface(t *testing.T) {
	decoder := NewWavDecoder()
	
	// Test interface compliance
	var _ Decoder = decoder
	
	// Test format name
	if decoder.FormatName() != "WAV" {
		t.Errorf("expected format name 'WAV', got '%s'", decoder.FormatName())
	}
}

func TestWavDecoderCanDecode(t *testing.T) {
	decoder := NewWavDecoder()
	
	testCases := []struct {
		filename string
		expected bool
	}{
		{"audio.wav", true},
		{"sound.WAV", true},
		{"music.wave", true},
		{"test.WAVE", true},
		{"audio.mp3", false},
		{"sound.flac", false},
		{"", false},
		{"wav", false},
		{"audio.wav.backup", false},
	}
	
	for _, tc := range testCases {
		result := decoder.CanDecode(tc.filename)
		if result != tc.expected {
			t.Errorf("CanDecode('%s') = %v, expected %v", tc.filename, result, tc.expected)
		}
	}
}

func TestWavDecoderDecodeInvalidData(t *testing.T) {
	decoder := NewWavDecoder()
	
	t.Run("empty data", func(t *testing.T) {
		reader := bytes.NewReader([]byte{})
		data, err := decoder.Decode(reader)
		
		if err == nil {
			t.Fatal("expected error for empty data")
		}
		
		if data != nil {
			t.Error("expected nil data on error")
		}
	})
	
	t.Run("invalid WAV header", func(t *testing.T) {
		invalidData := []byte("not a wav file")
		reader := bytes.NewReader(invalidData)
		data, err := decoder.Decode(reader)
		
		if err == nil {
			t.Fatal("expected error for invalid WAV data")
		}
		
		if data != nil {
			t.Error("expected nil data on error")
		}
	})
}

// Simple WAV file generator for testing
func generateTestWAV() []byte {
	// Minimal WAV file header (44 bytes) + some sample data
	// This creates a valid but minimal WAV file for testing
	wav := make([]byte, 0, 100)
	
	// RIFF header
	wav = append(wav, []byte("RIFF")...)           // ChunkID
	wav = append(wav, []byte{36, 0, 0, 0}...)      // ChunkSize (will be updated)
	wav = append(wav, []byte("WAVE")...)           // Format
	
	// fmt subchunk
	wav = append(wav, []byte("fmt ")...)           // Subchunk1ID
	wav = append(wav, []byte{16, 0, 0, 0}...)      // Subchunk1Size (16 for PCM)
	wav = append(wav, []byte{1, 0}...)             // AudioFormat (1 = PCM)
	wav = append(wav, []byte{2, 0}...)             // NumChannels (2 = stereo)
	wav = append(wav, []byte{68, 172, 0, 0}...)    // SampleRate (44100)
	wav = append(wav, []byte{16, 177, 2, 0}...)    // ByteRate (44100 * 2 * 2)
	wav = append(wav, []byte{4, 0}...)             // BlockAlign (2 * 2)
	wav = append(wav, []byte{16, 0}...)            // BitsPerSample (16)
	
	// data subchunk
	wav = append(wav, []byte("data")...)           // Subchunk2ID
	
	// Sample data (8 bytes = 2 samples for stereo 16-bit)
	sampleData := []byte{0x00, 0x01, 0x00, 0x02, 0x00, 0x03, 0x00, 0x04}
	dataSize := []byte{byte(len(sampleData)), 0, 0, 0} // Subchunk2Size
	wav = append(wav, dataSize...)
	wav = append(wav, sampleData...)
	
	// Update RIFF chunk size (total file size - 8)
	totalSize := len(wav) - 8
	wav[4] = byte(totalSize)
	wav[5] = byte(totalSize >> 8)
	wav[6] = byte(totalSize >> 16)
	wav[7] = byte(totalSize >> 24)
	
	return wav
}

func TestWavDecoderDecodeValidData(t *testing.T) {
	decoder := NewWavDecoder()
	
	t.Run("valid WAV file", func(t *testing.T) {
		wavData := generateTestWAV()
		reader := bytes.NewReader(wavData)
		data, err := decoder.Decode(reader)
		
		if err != nil {
			t.Fatalf("expected no error for valid WAV, got %v", err)
		}
		
		if data == nil {
			t.Fatal("expected audio data, got nil")
		}
		
		// Verify expected properties
		if data.Channels != 2 {
			t.Errorf("expected 2 channels, got %d", data.Channels)
		}
		
		if data.SampleRate != 44100 {
			t.Errorf("expected 44100 sample rate, got %d", data.SampleRate)
		}
		
		if data.Format != malgo.FormatS16 {
			t.Errorf("expected FormatS16, got %v", data.Format)
		}
		
		if len(data.Samples) == 0 {
			t.Error("expected sample data, got empty")
		}
	})
}

func TestNewWavDecoder(t *testing.T) {
	decoder := NewWavDecoder()
	
	if decoder == nil {
		t.Fatal("NewWavDecoder returned nil")
	}
	
	// Test that it implements the Decoder interface
	var _ Decoder = decoder
}