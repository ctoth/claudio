//go:build cgo

package malgo

import (
	"bytes"
	"context"
	"encoding/binary"
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
		data, err := decoder.Decode(context.Background(), reader)

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
		data, err := decoder.Decode(context.Background(), reader)

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
	wav = append(wav, []byte("RIFF")...)      // ChunkID
	wav = append(wav, []byte{36, 0, 0, 0}...) // ChunkSize (will be updated)
	wav = append(wav, []byte("WAVE")...)      // Format

	// fmt subchunk
	wav = append(wav, []byte("fmt ")...)        // Subchunk1ID
	wav = append(wav, []byte{16, 0, 0, 0}...)   // Subchunk1Size (16 for PCM)
	wav = append(wav, []byte{1, 0}...)          // AudioFormat (1 = PCM)
	wav = append(wav, []byte{2, 0}...)          // NumChannels (2 = stereo)
	wav = append(wav, []byte{68, 172, 0, 0}...) // SampleRate (44100)
	wav = append(wav, []byte{16, 177, 2, 0}...) // ByteRate (44100 * 2 * 2)
	wav = append(wav, []byte{4, 0}...)          // BlockAlign (2 * 2)
	wav = append(wav, []byte{16, 0}...)         // BitsPerSample (16)

	// data subchunk
	wav = append(wav, []byte("data")...) // Subchunk2ID

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
		data, err := decoder.Decode(context.Background(), reader)

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

func TestWavDecoderDecodePreservesPCMBytesAcrossReadChunks(t *testing.T) {
	const frames = 5001 // More than two go-wav ReadSamples chunks.

	want := make([]byte, frames*4)
	for frame := 0; frame < frames; frame++ {
		left := int16(frame*31 - 16000)
		right := int16(12000 - frame*17)
		binary.LittleEndian.PutUint16(want[frame*4:], uint16(left))
		binary.LittleEndian.PutUint16(want[frame*4+2:], uint16(right))
	}

	wavData := make([]byte, 44+len(want))
	copy(wavData[0:4], "RIFF")
	binary.LittleEndian.PutUint32(wavData[4:8], uint32(len(wavData)-8))
	copy(wavData[8:12], "WAVE")
	copy(wavData[12:16], "fmt ")
	binary.LittleEndian.PutUint32(wavData[16:20], 16)
	binary.LittleEndian.PutUint16(wavData[20:22], 1)
	binary.LittleEndian.PutUint16(wavData[22:24], 2)
	binary.LittleEndian.PutUint32(wavData[24:28], 44100)
	binary.LittleEndian.PutUint32(wavData[28:32], 44100*4)
	binary.LittleEndian.PutUint16(wavData[32:34], 4)
	binary.LittleEndian.PutUint16(wavData[34:36], 16)
	copy(wavData[36:40], "data")
	binary.LittleEndian.PutUint32(wavData[40:44], uint32(len(want)))
	copy(wavData[44:], want)

	got, err := NewWavDecoder().Decode(context.Background(), bytes.NewReader(wavData))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if !bytes.Equal(got.Samples, want) {
		t.Fatalf("Decode() returned %d PCM bytes, want exact %d-byte input", len(got.Samples), len(want))
	}
}

func TestNewWavDecoder(t *testing.T) {
	decoder := NewWavDecoder()

	if decoder == nil {
		t.Fatal("NewWavDecoder returned nil")
	}

	// Test that it implements the Decoder interface
	var _ Decoder = decoder
}
