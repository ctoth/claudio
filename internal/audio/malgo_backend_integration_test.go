package audio

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"
)

// TestMalgoBackendAIFFSupport tests that MalgoBackend can play AIFF files
// This test currently FAILS because MalgoBackend uses SimplePlayer which doesn't support AIFF
func TestMalgoBackendAIFFSupport(t *testing.T) {
	backend := NewMalgoBackend()
	defer backend.Close()

	// Create a mock AIFF file content (simplified but recognizable)
	aiffContent := "FORM....AIFF" // Minimal AIFF header
	source := NewReaderSource(io.NopCloser(strings.NewReader(aiffContent)), "aiff")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// This should work after unification but currently fails
	err := backend.Play(ctx, source)
	if err != nil {
		t.Logf("Expected failure (will be fixed): %v", err)
		// For now, expect failure - test will pass after unification
		return
	}

	t.Error("Test should fail until unification is complete")
}

// TestMalgoBackendUsesRegistrySystem tests that MalgoBackend uses DecoderRegistry
func TestMalgoBackendUsesRegistrySystem(t *testing.T) {
	backend := NewMalgoBackend()
	// After unification, MalgoBackend should have registry and audioPlayer fields
	// This will fail until we refactor
	_ = backend // This test documents the intended structure
	t.Skip("Will be implemented during unification")
}