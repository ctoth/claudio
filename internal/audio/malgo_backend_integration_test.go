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
	// After unification, MalgoBackend should have registry and audioPlayer fields
	// This will fail until we refactor
	_ = backend // This test documents the intended structure
	t.Skip("Will be implemented during unification")
}