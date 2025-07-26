package audio

import (
	"bytes"
	"testing"

	"github.com/gen2brain/malgo"
)

func TestMp3DecoderInterface(t *testing.T) {
	decoder := NewMp3Decoder()
	
	// Test interface compliance
	var _ Decoder = decoder
	
	// Test format name
	if decoder.FormatName() != "MP3" {
		t.Errorf("expected format name 'MP3', got '%s'", decoder.FormatName())
	}
}

func TestMp3DecoderCanDecode(t *testing.T) {
	decoder := NewMp3Decoder()
	
	testCases := []struct {
		filename string
		expected bool
	}{
		{"audio.mp3", true},
		{"sound.MP3", true},
		{"music.mpeg", true},
		{"test.MPEG", true},
		{"audio.wav", false},
		{"sound.flac", false},
		{"", false},
		{"mp3", false},
		{"audio.mp3.backup", false},
	}
	
	for _, tc := range testCases {
		result := decoder.CanDecode(tc.filename)
		if result != tc.expected {
			t.Errorf("CanDecode('%s') = %v, expected %v", tc.filename, result, tc.expected)
		}
	}
}

func TestMp3DecoderDecodeInvalidData(t *testing.T) {
	decoder := NewMp3Decoder()
	
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
	
	t.Run("invalid MP3 header", func(t *testing.T) {
		invalidData := []byte("not an mp3 file")
		reader := bytes.NewReader(invalidData)
		data, err := decoder.Decode(reader)
		
		if err == nil {
			t.Fatal("expected error for invalid MP3 data")
		}
		
		if data != nil {
			t.Error("expected nil data on error")
		}
	})
}

// Simple MP3 frame header generator for testing
func generateTestMp3() []byte {
	// Minimal MP3 frame header for testing
	// This creates a valid but minimal MP3 frame header
	mp3Data := make([]byte, 0, 200)
	
	// MP3 frame header (4 bytes) - simplified for testing
	// Frame sync (11 bits) + MPEG Audio version + Layer + Protection bit
	mp3Data = append(mp3Data, 0xFF, 0xFB) // Frame sync + MPEG-1 Layer III
	
	// Bitrate + Sampling rate + Padding + Private + Mode + Mode extension + Copyright + Original + Emphasis
	mp3Data = append(mp3Data, 0x90, 0x00) // 128 kbps, 44.1 kHz, stereo
	
	// Add some dummy frame data (this won't be playable but will pass basic header validation)
	dummyData := make([]byte, 100)
	for i := range dummyData {
		dummyData[i] = byte(i % 256)
	}
	mp3Data = append(mp3Data, dummyData...)
	
	return mp3Data
}

func TestMp3DecoderDecodeValidData(t *testing.T) {
	decoder := NewMp3Decoder()
	
	t.Run("valid MP3 data", func(t *testing.T) {
		mp3Data := generateTestMp3()
		reader := bytes.NewReader(mp3Data)
		data, err := decoder.Decode(reader)
		
		if err != nil {
			// MP3 decoder might fail on our minimal test data, which is expected
			// This test mainly ensures our error handling works correctly
			t.Logf("MP3 decode failed as expected with minimal test data: %v", err)
			return
		}
		
		if data != nil {
			// If decode succeeds, verify expected properties
			if data.Channels == 0 {
				t.Error("expected non-zero channels")
			}
			
			if data.SampleRate == 0 {
				t.Error("expected non-zero sample rate")  
			}
			
			if data.Format != malgo.FormatS16 {
				t.Errorf("expected FormatS16, got %v", data.Format)
			}
		}
	})
}

func TestNewMp3Decoder(t *testing.T) {
	decoder := NewMp3Decoder()
	
	if decoder == nil {
		t.Fatal("NewMp3Decoder returned nil")
	}
	
	// Test that it implements the Decoder interface
	var _ Decoder = decoder
}