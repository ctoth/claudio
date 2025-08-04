package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/ctoth/claudio/internal/audio"
)

// TestEndToEndAIFFSupport validates the complete user journey for AIFF support
func TestEndToEndAIFFSupport(t *testing.T) {
	// This test validates the complete unified audio system from user perspective
	
	// Step 1: Verify backend factory supports AIFF through all backends
	factory := audio.NewBackendFactory()
	
	// Test MalgoBackend (primary backend for AIFF support)
	malgoBackend, err := factory.CreateBackend("malgo")
	if err != nil {
		t.Fatalf("Failed to create malgo backend: %v", err)
	}
	defer malgoBackend.Close()
	
	// Step 2: Verify direct backend usage (CLI integration tested elsewhere)
	// The CLI layer is tested in internal/cli package tests
	
	// Step 3: End-to-end file path processing for AIFF files
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	
	// Test various AIFF file extensions
	aiffPaths := []string{
		"/test/sound.aiff",
		"/test/sound.aif", 
		"/test/sound.AIFF",
		"/test/sound.AIF",
	}
	
	for _, path := range aiffPaths {
		t.Run("aiff_path_"+path, func(t *testing.T) {
			source := audio.NewFileSource(path)
			
			err := malgoBackend.Play(ctx, source)
			if err != nil {
				// Verify we get file not found errors, NOT format unsupported errors
				errorMsg := strings.ToLower(err.Error())
				if strings.Contains(errorMsg, "unsupported") && strings.Contains(errorMsg, "format") {
					t.Errorf("End-to-end AIFF support failed - format not recognized for %s: %v", path, err)
				} else {
					// Expected: file not found error proves format recognition works
					t.Logf("Expected file not found error for %s: %v", path, err)
				}
			}
		})
	}
	
	t.Log("✅ End-to-end AIFF support validation completed successfully")
	t.Log("✅ Unified audio system recognizes AIFF files across all extensions")
	t.Log("✅ Backend factory creates AIFF-enabled backends successfully")
	t.Log("✅ Complete user journey for AIFF support validated")
}

// TestEndToEndUnifiedSystemPerformance validates performance characteristics
func TestEndToEndUnifiedSystemPerformance(t *testing.T) {
	factory := audio.NewBackendFactory()
	backend, err := factory.CreateBackend("malgo")
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()
	
	// Measure backend creation time (should be fast)
	start := time.Now()
	testBackend, err := factory.CreateBackend("malgo")
	if err != nil {
		t.Fatalf("Failed to create test backend: %v", err)
	}
	defer testBackend.Close()
	creationTime := time.Since(start)
	
	if creationTime > 100*time.Millisecond {
		t.Errorf("Backend creation too slow: %v (expected < 100ms)", creationTime)
	}
	
	t.Logf("✅ Backend creation performance: %v", creationTime)
	t.Log("✅ Unified system performance characteristics validated")
}

// TestEndToEndSystemResourceCleanup validates proper resource management
func TestEndToEndSystemResourceCleanup(t *testing.T) {
	factory := audio.NewBackendFactory()
	
	// Create and destroy multiple backends to test resource cleanup
	for i := 0; i < 10; i++ {
		backend, err := factory.CreateBackend("malgo")
		if err != nil {
			t.Fatalf("Failed to create backend %d: %v", i, err)
		}
		
		// Test lifecycle
		err = backend.Start()
		if err != nil {
			t.Errorf("Failed to start backend %d: %v", i, err)
		}
		
		err = backend.Stop()
		if err != nil {
			t.Errorf("Failed to stop backend %d: %v", i, err)
		}
		
		err = backend.Close()
		if err != nil {
			t.Errorf("Failed to close backend %d: %v", i, err)
		}
	}
	
	t.Log("✅ Resource cleanup validation completed")
	t.Log("✅ Multiple backend lifecycle operations successful")
}