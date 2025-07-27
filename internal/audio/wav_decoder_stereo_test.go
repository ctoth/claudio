package audio

import (
	"bytes"
	"testing"
)

// generateStereoTestWAV creates a WAV file with known stereo data
func generateStereoTestWAV() []byte {
	wav := make([]byte, 0, 200)
	
	// RIFF header
	wav = append(wav, []byte("RIFF")...)           // ChunkID
	wav = append(wav, []byte{0, 0, 0, 0}...)       // ChunkSize (will be updated)
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
	
	// Create recognizable stereo pattern
	// Left channel: 0x1000, 0x2000, 0x3000, 0x4000
	// Right channel: 0x0100, 0x0200, 0x0300, 0x0400
	sampleData := []byte{
		0x00, 0x10, // L: 0x1000
		0x00, 0x01, // R: 0x0100
		0x00, 0x20, // L: 0x2000
		0x00, 0x02, // R: 0x0200
		0x00, 0x30, // L: 0x3000
		0x00, 0x03, // R: 0x0300
		0x00, 0x40, // L: 0x4000
		0x00, 0x04, // R: 0x0400
	}
	
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

func TestWavDecoderStereoChannelHandling(t *testing.T) {
	decoder := NewWavDecoder()
	
	t.Run("verify stereo channel extraction", func(t *testing.T) {
		wavData := generateStereoTestWAV()
		reader := bytes.NewReader(wavData)
		data, err := decoder.Decode(reader)
		
		if err != nil {
			t.Fatalf("failed to decode test WAV: %v", err)
		}
		
		if data.Channels != 2 {
			t.Errorf("expected 2 channels, got %d", data.Channels)
		}
		
		// Check if we have the right amount of data
		// 4 stereo samples * 2 channels * 2 bytes = 16 bytes
		expectedBytes := 16
		if len(data.Samples) != expectedBytes {
			t.Errorf("expected %d bytes, got %d", expectedBytes, len(data.Samples))
		}
		
		// Verify the data is properly interleaved
		// Should be: L1, R1, L2, R2, L3, R3, L4, R4
		expected := []byte{
			0x00, 0x10, // L1: 0x1000
			0x00, 0x01, // R1: 0x0100
			0x00, 0x20, // L2: 0x2000
			0x00, 0x02, // R2: 0x0200
			0x00, 0x30, // L3: 0x3000
			0x00, 0x03, // R3: 0x0300
			0x00, 0x40, // L4: 0x4000
			0x00, 0x04, // R4: 0x0400
		}
		
		// Check if we got properly interleaved stereo data
		for i := 0; i < len(expected) && i < len(data.Samples); i++ {
			if data.Samples[i] != expected[i] {
				t.Errorf("byte %d: expected 0x%02X, got 0x%02X", i, expected[i], data.Samples[i])
			}
		}
		
		// Print first few samples for debugging
		t.Logf("First 16 bytes of decoded data:")
		for i := 0; i < 16 && i < len(data.Samples); i++ {
			t.Logf("  [%d]: 0x%02X", i, data.Samples[i])
		}
	})
}

func TestWavDecoderMonoHandling(t *testing.T) {
	decoder := NewWavDecoder()
	
	t.Run("verify mono channel handling", func(t *testing.T) {
		// Create mono WAV
		wav := make([]byte, 0, 100)
		
		// RIFF header
		wav = append(wav, []byte("RIFF")...)
		wav = append(wav, []byte{0, 0, 0, 0}...)
		wav = append(wav, []byte("WAVE")...)
		
		// fmt subchunk
		wav = append(wav, []byte("fmt ")...)
		wav = append(wav, []byte{16, 0, 0, 0}...)
		wav = append(wav, []byte{1, 0}...)          // PCM
		wav = append(wav, []byte{1, 0}...)          // 1 channel (mono)
		wav = append(wav, []byte{68, 172, 0, 0}...) // 44100 Hz
		wav = append(wav, []byte{136, 88, 1, 0}...) // ByteRate
		wav = append(wav, []byte{2, 0}...)          // BlockAlign
		wav = append(wav, []byte{16, 0}...)         // 16 bits
		
		// data subchunk
		wav = append(wav, []byte("data")...)
		sampleData := []byte{0x00, 0x10, 0x00, 0x20, 0x00, 0x30}
		wav = append(wav, []byte{byte(len(sampleData)), 0, 0, 0}...)
		wav = append(wav, sampleData...)
		
		// Update RIFF size
		totalSize := len(wav) - 8
		wav[4] = byte(totalSize)
		wav[5] = byte(totalSize >> 8)
		
		reader := bytes.NewReader(wav)
		data, err := decoder.Decode(reader)
		
		if err != nil {
			t.Fatalf("failed to decode mono WAV: %v", err)
		}
		
		if data.Channels != 1 {
			t.Errorf("expected 1 channel, got %d", data.Channels)
		}
		
		// Verify we get all mono samples
		if len(data.Samples) != len(sampleData) {
			t.Errorf("expected %d bytes, got %d", len(sampleData), len(data.Samples))
		}
	})
}