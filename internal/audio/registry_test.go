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