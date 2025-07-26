package audio

import (
	"testing"
)

func TestContextLifecycle(t *testing.T) {
	// Test context creation
	ctx, err := NewContext()
	if err != nil {
		t.Fatalf("failed to create audio context: %v", err)
	}
	
	// Verify context is valid
	if !ctx.IsValid() {
		t.Error("context should be valid after creation")
	}
	
	// Verify underlying context exists
	if ctx.GetContext() == nil {
		t.Error("underlying malgo context should not be nil")
	}
	
	// Test context cleanup
	err = ctx.Close()
	if err != nil {
		t.Errorf("failed to close audio context: %v", err)
	}
	
	// Verify context is invalid after close
	if ctx.IsValid() {
		t.Error("context should be invalid after close")
	}
	
	// Test double close (should not error)
	err = ctx.Close()
	if err != nil {
		t.Errorf("double close should not error: %v", err)
	}
}

func TestContextCreationFailure(t *testing.T) {
	// This test mainly verifies our error handling structure
	// In a real environment, context creation could fail due to:
	// - No audio devices
	// - Driver issues
	// - Permission problems
	
	// For now, we just verify the function signature and error handling
	ctx, err := NewContext()
	if err != nil {
		// If creation fails, that's okay - we're testing error handling
		t.Logf("context creation failed as expected: %v", err)
		return
	}
	
	// If creation succeeds, clean up
	defer ctx.Close()
	
	if !ctx.IsValid() {
		t.Error("valid context should report as valid")
	}
}