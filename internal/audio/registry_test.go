package audio

import (
	"bytes"
	"strings"
	"testing"

	"github.com/gen2brain/malgo"
)

func TestDecoderRegistry(t *testing.T) {
	registry := NewDecoderRegistry()
	
	if registry == nil {
		t.Fatal("NewDecoderRegistry returned nil")
	}
	
	// Test that registry starts empty
	decoders := registry.GetDecoders()
	if len(decoders) != 0 {
		t.Errorf("expected empty registry, got %d decoders", len(decoders))
	}
}

func TestDecoderRegistryRegister(t *testing.T) {
	registry := NewDecoderRegistry()
	
	// Create test decoder
	decoder := &MockDecoder{
		formatName: "TEST",
		extensions: []string{".test"},
	}
	
	// Register decoder
	registry.Register(decoder)
	
	// Verify registration
	decoders := registry.GetDecoders()
	if len(decoders) != 1 {
		t.Errorf("expected 1 decoder after registration, got %d", len(decoders))
	}
	
	if decoders[0] != decoder {
		t.Error("registered decoder not found in registry")
	}
}

func TestDecoderRegistryRegisterMultiple(t *testing.T) {
	registry := NewDecoderRegistry()
	
	decoder1 := &MockDecoder{formatName: "TEST1", extensions: []string{".test1"}}
	decoder2 := &MockDecoder{formatName: "TEST2", extensions: []string{".test2"}}
	
	registry.Register(decoder1)
	registry.Register(decoder2)
	
	decoders := registry.GetDecoders()
	if len(decoders) != 2 {
		t.Errorf("expected 2 decoders, got %d", len(decoders))
	}
	
	// Verify both decoders are present
	found1, found2 := false, false
	for _, d := range decoders {
		if d == decoder1 {
			found1 = true
		}
		if d == decoder2 {
			found2 = true
		}
	}
	
	if !found1 || !found2 {
		t.Error("not all registered decoders found in registry")
	}
}

func TestDecoderRegistryDetectFormat(t *testing.T) {
	registry := NewDecoderRegistry()
	
	wavDecoder := &MockDecoder{
		formatName: "WAV",
		extensions: []string{".wav", ".wave"},
	}
	mp3Decoder := &MockDecoder{
		formatName: "MP3", 
		extensions: []string{".mp3", ".mpeg"},
	}
	
	registry.Register(wavDecoder)
	registry.Register(mp3Decoder)
	
	testCases := []struct {
		filename string
		expected Decoder
	}{
		{"audio.wav", wavDecoder},
		{"sound.WAV", wavDecoder},
		{"music.wave", wavDecoder},
		{"song.mp3", mp3Decoder},
		{"track.MP3", mp3Decoder},
		{"file.mpeg", mp3Decoder},
		{"unknown.flac", nil},
		{"", nil},
		{"no-extension", nil},
	}
	
	for _, tc := range testCases {
		result := registry.DetectFormat(tc.filename)
		if result != tc.expected {
			t.Errorf("DetectFormat('%s') = %v, expected %v", tc.filename, result, tc.expected)
		}
	}
}

func TestDecoderRegistryDetectFormatWithMagicBytes(t *testing.T) {
	registry := NewDecoderRegistry()
	
	wavDecoder := &MockDecoder{
		formatName: "WAV",
		extensions: []string{".wav", ".wave"},
	}
	mp3Decoder := &MockDecoder{
		formatName: "MP3",
		extensions: []string{".mp3", ".mpeg"},
	}
	
	registry.Register(wavDecoder)
	registry.Register(mp3Decoder)
	
	// Test magic byte detection - file extension lies but content tells truth
	testCases := []struct {
		name         string
		filename     string
		content      []byte
		expected     Decoder
		description  string
	}{
		{
			name:        "WAV content with MP3 extension",
			filename:    "fake.mp3",
			content:     []byte("RIFF\x24\x00\x00\x00WAVEfmt "), // Complete WAV header
			expected:    wavDecoder,
			description: "Should detect WAV content despite .mp3 extension",
		},
		{
			name:        "MP3 content with WAV extension", 
			filename:    "fake.wav",
			content:     []byte("\xFF\xFB\x90\x00"), // MP3 magic bytes
			expected:    mp3Decoder,
			description: "Should detect MP3 content despite .wav extension",
		},
		{
			name:        "Invalid content with valid extension",
			filename:    "test.wav",
			content:     []byte("not audio data"),
			expected:    wavDecoder, // Should fallback to extension
			description: "Should fallback to extension when magic detection fails",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reader := bytes.NewReader(tc.content)
			result := registry.DetectFormatWithContent(tc.filename, reader)
			if result != tc.expected {
				t.Errorf("%s: DetectFormatWithContent('%s') = %v, expected %v", 
					tc.description, tc.filename, result, tc.expected)
			}
		})
	}
}

func TestDecoderRegistryDetectFormatPriority(t *testing.T) {
	registry := NewDecoderRegistry()
	
	// Register two decoders that can handle the same extension
	decoder1 := &MockDecoder{
		formatName: "FIRST",
		extensions: []string{".test"},
	}
	decoder2 := &MockDecoder{
		formatName: "SECOND", 
		extensions: []string{".test"},
	}
	
	registry.Register(decoder1)
	registry.Register(decoder2)
	
	// First registered decoder should have priority
	result := registry.DetectFormat("file.test")
	if result != decoder1 {
		t.Errorf("expected first registered decoder to have priority, got %v", result)
	}
}

func TestDecoderRegistryDecodeFile(t *testing.T) {
	registry := NewDecoderRegistry()
	
	// Register mock decoder
	testData := &AudioData{
		Samples:    []byte{0x01, 0x02, 0x03, 0x04},
		Channels:   2,
		SampleRate: 44100,
		Format:     malgo.FormatS16,
	}
	
	decoder := &MockDecoder{
		formatName: "TEST",
		extensions: []string{".test"},
		returnData: testData,
	}
	
	registry.Register(decoder)
	
	t.Run("successful decode", func(t *testing.T) {
		reader := bytes.NewReader([]byte("test audio data"))
		result, err := registry.DecodeFile("audio.test", reader)
		
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		
		if result != testData {
			t.Error("expected custom test data to be returned")
		}
	})
	
	t.Run("unsupported format", func(t *testing.T) {
		reader := bytes.NewReader([]byte("test audio data"))
		result, err := registry.DecodeFile("audio.unknown", reader)
		
		if err == nil {
			t.Fatal("expected error for unsupported format")
		}
		
		if result != nil {
			t.Error("expected nil result on error")
		}
		
		if !strings.Contains(err.Error(), "unsupported") {
			t.Errorf("expected 'unsupported' in error message, got: %v", err)
		}
	})
	
	t.Run("decode failure", func(t *testing.T) {
		failingDecoder := &MockDecoder{
			formatName: "FAIL",
			extensions: []string{".fail"},
			shouldFail: true,
		}
		
		registry.Register(failingDecoder)
		
		reader := bytes.NewReader([]byte("test data"))
		result, err := registry.DecodeFile("audio.fail", reader)
		
		if err == nil {
			t.Fatal("expected error from failing decoder")
		}
		
		if result != nil {
			t.Error("expected nil result on decode failure")
		}
	})
}

func TestDecoderRegistryGetSupportedFormats(t *testing.T) {
	registry := NewDecoderRegistry()
	
	// Initially should be empty
	formats := registry.GetSupportedFormats()
	if len(formats) != 0 {
		t.Errorf("expected empty formats list, got %d", len(formats))
	}
	
	// Register decoders
	wavDecoder := &MockDecoder{
		formatName: "WAV",
		extensions: []string{".wav", ".wave"},
	}
	mp3Decoder := &MockDecoder{
		formatName: "MP3",
		extensions: []string{".mp3", ".mpeg"},
	}
	
	registry.Register(wavDecoder)
	registry.Register(mp3Decoder)
	
	formats = registry.GetSupportedFormats()
	if len(formats) != 2 {
		t.Errorf("expected 2 formats, got %d", len(formats))
	}
	
	// Check that both format names are present
	foundWAV, foundMP3 := false, false
	for _, format := range formats {
		if format == "WAV" {
			foundWAV = true
		}
		if format == "MP3" {
			foundMP3 = true
		}
	}
	
	if !foundWAV || !foundMP3 {
		t.Error("not all format names found in supported formats list")
	}
}

func TestNewDefaultRegistry(t *testing.T) {
	registry := NewDefaultRegistry()
	
	if registry == nil {
		t.Fatal("NewDefaultRegistry returned nil")
	}
	
	// Should have WAV and MP3 decoders registered
	formats := registry.GetSupportedFormats()
	if len(formats) != 2 {
		t.Errorf("expected 2 default formats, got %d", len(formats))
	}
	
	// Check for WAV support
	wavDecoder := registry.DetectFormat("test.wav")
	if wavDecoder == nil {
		t.Error("default registry should support WAV files")
	}
	
	if wavDecoder.FormatName() != "WAV" {
		t.Errorf("expected WAV decoder, got %s", wavDecoder.FormatName())
	}
	
	// Check for MP3 support
	mp3Decoder := registry.DetectFormat("test.mp3")
	if mp3Decoder == nil {
		t.Error("default registry should support MP3 files")
	}
	
	if mp3Decoder.FormatName() != "MP3" {
		t.Errorf("expected MP3 decoder, got %s", mp3Decoder.FormatName())
	}
}

// Helper function to check if a format is in the list
func containsFormat(formats []string, target string) bool {
	for _, format := range formats {
		if format == target {
			return true
		}
	}
	return false
}

// AIFF Registry Integration Tests

func TestDefaultRegistrySupportsAIFF(t *testing.T) {
	registry := NewDefaultRegistry()
	
	formats := registry.GetSupportedFormats()
	if !containsFormat(formats, "AIFF") {
		t.Error("expected default registry to support AIFF format")
	}
	
	// Verify AIFF decoder is properly registered
	decoders := registry.GetDecoders()
	var aiffDecoder Decoder
	for _, decoder := range decoders {
		if decoder.FormatName() == "AIFF" {
			aiffDecoder = decoder
			break
		}
	}
	
	if aiffDecoder == nil {
		t.Fatal("AIFF decoder not found in default registry")
	}
	
	// Test that it's a real AIFF decoder, not a mock
	if aiffDecoder.FormatName() != "AIFF" {
		t.Errorf("expected AIFF decoder format name to be 'AIFF', got '%s'", aiffDecoder.FormatName())
	}
}

func TestAiffFormatDetectionByExtension(t *testing.T) {
	registry := NewDefaultRegistry()
	
	testCases := []struct {
		filename     string
		shouldDetect bool
		description  string
	}{
		{"audio.aiff", true, "standard .aiff extension"},
		{"sound.AIFF", true, "uppercase .AIFF extension"},
		{"music.aif", true, "short .aif extension"},
		{"track.AIF", true, "uppercase .AIF extension"},
		{"file.aiff.backup", false, "AIFF extension not at end"},
		{"aiff", false, "no extension"},
		{"audio.wav", false, "different format (WAV)"},
		{"audio.mp3", false, "different format (MP3)"},
	}
	
	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			decoder := registry.DetectFormat(tc.filename)
			
			if tc.shouldDetect {
				if decoder == nil {
					t.Errorf("expected to detect AIFF format for '%s', got nil", tc.filename)
				} else if decoder.FormatName() != "AIFF" {
					t.Errorf("expected AIFF decoder for '%s', got %s", tc.filename, decoder.FormatName())
				}
			} else {
				if decoder != nil && decoder.FormatName() == "AIFF" {
					t.Errorf("should not detect AIFF format for '%s', but got AIFF decoder", tc.filename)
				}
			}
		})
	}
}

func TestAiffMagicByteDetection(t *testing.T) {
	registry := NewDefaultRegistry()
	
	// Create minimal AIFF file with proper magic bytes
	aiffData := []byte("FORM\x00\x00\x00\x1EAIFFCOMM\x00\x00\x00\x12\x00\x02\x00\x00\x00\x64\x00\x10\x40\x0E\xAC\x44\x00\x00\x00\x00\x00\x00")
	
	testCases := []struct {
		name        string
		filename    string
		content     []byte
		expected    string
		description string
	}{
		{
			name:        "AIFF content with wrong extension",
			filename:    "fake.mp3",
			content:     aiffData,
			expected:    "AIFF",
			description: "Should detect AIFF content despite .mp3 extension",
		},
		{
			name:        "AIFF content with correct extension", 
			filename:    "audio.aiff",
			content:     aiffData,
			expected:    "AIFF",
			description: "Should detect AIFF content with .aiff extension",
		},
		{
			name:        "Invalid AIFF content",
			filename:    "fake.aiff",
			content:     []byte("NOT_AIFF_DATA"),
			expected:    "AIFF", // Should fallback to extension-based detection
			description: "Should fallback to extension when magic bytes fail",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reader := bytes.NewReader(tc.content)
			decoder := registry.DetectFormatWithContent(tc.filename, reader)
			
			if decoder == nil {
				t.Errorf("expected to detect %s format for '%s', got nil", tc.expected, tc.filename)
			} else if decoder.FormatName() != tc.expected {
				t.Errorf("expected %s decoder for '%s', got %s", tc.expected, tc.filename, decoder.FormatName())
			}
		})
	}
}

func TestAiffDecodeFileIntegration(t *testing.T) {
	registry := NewDefaultRegistry()
	
	// Create a valid AIFF file for testing
	aiffData := createMinimalAIFFForRegistry(44100, 2, 16, 100) // 44.1kHz, stereo, 16-bit, 100 samples
	
	t.Run("successful AIFF file decode", func(t *testing.T) {
		reader := bytes.NewReader(aiffData)
		audioData, err := registry.DecodeFile("test.aiff", reader)
		
		if err != nil {
			t.Fatalf("expected no error decoding AIFF file, got %v", err)
		}
		
		if audioData == nil {
			t.Fatal("expected audio data, got nil")
		}
		
		// Verify decoded data properties
		if audioData.SampleRate != 44100 {
			t.Errorf("expected sample rate 44100, got %d", audioData.SampleRate)
		}
		
		if audioData.Channels != 2 {
			t.Errorf("expected 2 channels, got %d", audioData.Channels)
		}
		
		if audioData.Format != malgo.FormatS16 {
			t.Errorf("expected format S16, got %v", audioData.Format)
		}
		
		expectedBytes := 100 * 2 * 2 // samples * channels * bytes_per_sample (16-bit)
		if len(audioData.Samples) != expectedBytes {
			t.Errorf("expected %d sample bytes, got %d", expectedBytes, len(audioData.Samples))
		}
	})
	
	t.Run("AIFF decode with invalid data", func(t *testing.T) {
		reader := bytes.NewReader([]byte("invalid aiff data"))
		audioData, err := registry.DecodeFile("invalid.aiff", reader)
		
		if err == nil {
			t.Error("expected error for invalid AIFF data, got nil")
		}
		
		if audioData != nil {
			t.Error("expected nil audio data for invalid AIFF, got data")
		}
	})
}

// Helper function to create minimal AIFF file specifically for registry tests
func createMinimalAIFFForRegistry(sampleRate, channels, bitDepth, numSamples int) []byte {
	// Reuse the same AIFF generation logic from the decoder tests
	// This ensures consistency between decoder and registry tests
	
	bytesPerSample := bitDepth / 8
	dataSize := numSamples * channels * bytesPerSample
	
	// COMM chunk data
	commData := make([]byte, 18)
	// Channels (2 bytes)
	commData[0] = byte(channels >> 8)
	commData[1] = byte(channels)
	// Sample frames (4 bytes)
	frames := uint32(numSamples)
	commData[2] = byte(frames >> 24)
	commData[3] = byte(frames >> 16)
	commData[4] = byte(frames >> 8)
	commData[5] = byte(frames)
	// Sample size (2 bytes)
	commData[6] = byte(bitDepth >> 8)
	commData[7] = byte(bitDepth)
	// Sample rate (10 bytes IEEE 754 extended precision)
	sampleRateBytes := simpleIEEE754Extended(float64(sampleRate))
	copy(commData[8:18], sampleRateBytes)
	
	// SSND chunk data
	ssndData := make([]byte, 8+dataSize) // 8 bytes header + data
	
	// Calculate total file size
	totalSize := 4 + // "AIFF"
		8 + len(commData) + // "COMM" + size + data
		8 + len(ssndData) // "SSND" + size + data
	
	// Build the complete AIFF file
	var buf []byte
	
	// FORM header
	buf = append(buf, []byte("FORM")...)
	buf = appendBigEndianUint32Registry(buf, uint32(totalSize))
	buf = append(buf, []byte("AIFF")...)
	
	// COMM chunk
	buf = append(buf, []byte("COMM")...)
	buf = appendBigEndianUint32Registry(buf, uint32(len(commData)))
	buf = append(buf, commData...)
	
	// SSND chunk
	buf = append(buf, []byte("SSND")...)
	buf = appendBigEndianUint32Registry(buf, uint32(len(ssndData)))
	buf = append(buf, ssndData...)
	
	return buf
}

// Helper functions for AIFF generation in registry tests
func appendBigEndianUint32Registry(buf []byte, val uint32) []byte {
	return append(buf,
		byte(val>>24),
		byte(val>>16),
		byte(val>>8),
		byte(val))
}

func simpleIEEE754Extended(f float64) []byte {
	// Simplified implementation for common sample rates
	switch int(f) {
	case 44100:
		return []byte{0x40, 0x0E, 0xAC, 0x44, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	case 48000:
		return []byte{0x40, 0x0E, 0xBB, 0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	default:
		return []byte{0x40, 0x0E, 0xAC, 0x44, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	}
}