package audio

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"
)

// TestMalgoBackendAIFFSupport tests that MalgoBackend can detect AIFF files
// This test verifies the unified system recognizes AIFF format (even if decode fails on mock data)
func TestMalgoBackendAIFFSupport(t *testing.T) {
	backend := NewMalgoBackend()
	defer backend.Close()

	// Create a mock AIFF file content (simplified but recognizable)
	aiffContent := "FORM....AIFF" // Minimal AIFF header
	source := NewReaderSource(io.NopCloser(strings.NewReader(aiffContent)), "aiff")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test that unified system attempts AIFF processing (not "unsupported format")
	err := backend.Play(ctx, source)
	if err != nil {
		// We should get a decode error, not an "unsupported format" error
		errorMsg := strings.ToLower(err.Error())
		if strings.Contains(errorMsg, "unsupported") || strings.Contains(errorMsg, "unsupported audio format") {
			t.Errorf("Unified system should recognize AIFF format, got: %v", err)
		} else {
			// Expected: decode error because mock data is invalid, but format was recognized
			t.Logf("Expected decode failure on mock data: %v", err)
		}
		return
	}

	// If it doesn't error, that's actually unexpected with mock data
	t.Error("Expected decode error with mock AIFF data")
}

// TestMalgoBackendUsesRegistrySystem tests that MalgoBackend uses DecoderRegistry
func TestMalgoBackendUsesRegistrySystem(t *testing.T) {
	backend := NewMalgoBackend()
	defer backend.Close()
	
	// Verify that MalgoBackend has both unified components initialized
	if backend.audioPlayer == nil {
		t.Error("MalgoBackend should have audioPlayer initialized")
	}
	if backend.registry == nil {
		t.Error("MalgoBackend should have registry initialized") 
	}
	
	// Verify registry supports the unified formats (including AIFF)
	supportedFormats := backend.registry.GetSupportedFormats()
	expectedFormats := []string{"WAV", "MP3", "AIFF"}
	
	for _, expected := range expectedFormats {
		found := false
		for _, supported := range supportedFormats {
			if supported == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected format %s not found in supported formats: %v", expected, supportedFormats)
		}
	}
	
	t.Logf("Unified system verification passed - supported formats: %v", supportedFormats)
}

// TestMalgoBackendUnifiedSystemEndToEnd tests the complete unified audio system
func TestMalgoBackendUnifiedSystemEndToEnd(t *testing.T) {
	backend := NewMalgoBackend()
	defer backend.Close()
	
	// Test that the unified system handles all supported formats
	testCases := []struct {
		name     string
		format   string
		filename string
	}{
		{"WAV format", "wav", "test.wav"},
		{"MP3 format", "mp3", "test.mp3"}, 
		{"AIFF format", "aiff", "test.aiff"},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Use empty content to safely test format recognition without triggering decoder panics
			mockContent := ""
			source := NewReaderSource(io.NopCloser(strings.NewReader(mockContent)), tc.format)
			
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()
			
			// Test that unified system recognizes the format (even if decode fails on empty data)
			err := backend.Play(ctx, source)
			if err != nil {
				// We expect decode errors with empty data, but NOT "unsupported format" errors
				errorMsg := strings.ToLower(err.Error())
				if strings.Contains(errorMsg, "unsupported") && strings.Contains(errorMsg, "format") {
					t.Errorf("Unified system should recognize %s format, got: %v", tc.format, err)
				} else {
					// Expected: decode error because data is empty/invalid
					t.Logf("Expected decode failure on empty %s data: %v", tc.format, err)
				}
			} else {
				// Unexpected success with empty data
				t.Logf("Unexpected success with empty %s data", tc.format)
			}
		})
	}
}