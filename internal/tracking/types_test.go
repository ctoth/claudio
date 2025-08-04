package tracking

import (
	"testing"

	"github.com/ctoth/claudio/internal/hooks"
)

func TestNewSoundChecker(t *testing.T) {
	sc := NewSoundChecker()
	if sc == nil {
		t.Fatal("NewSoundChecker returned nil")
	}
	if len(sc.hooks) != 0 {
		t.Errorf("expected 0 hooks, got %d", len(sc.hooks))
	}
}

func TestWithHook(t *testing.T) {
	called := false
	hook := func(path string, exists bool, sequence int, context *hooks.EventContext) {
		called = true
	}

	sc := NewSoundChecker(WithHook(hook))
	if len(sc.hooks) != 1 {
		t.Errorf("expected 1 hook, got %d", len(sc.hooks))
	}

	// Test hook is actually called
	context := &hooks.EventContext{}
	sc.CheckPaths(context, []string{"test.wav"})

	if !called {
		t.Error("hook was not called")
	}
}

func TestCheckPaths(t *testing.T) {
	var capturedPaths []string
	var capturedSequences []int
	var capturedExists []bool
	var capturedContext *hooks.EventContext

	hook := func(path string, exists bool, sequence int, context *hooks.EventContext) {
		capturedPaths = append(capturedPaths, path)
		capturedSequences = append(capturedSequences, sequence)
		capturedExists = append(capturedExists, exists)
		capturedContext = context
	}

	sc := NewSoundChecker(WithHook(hook))
	context := &hooks.EventContext{
		ToolName: "git",
		Category: hooks.Success,
	}
	paths := []string{"path1.wav", "path2.wav", "path3.wav"}

	results := sc.CheckPaths(context, paths)

	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}

	if len(capturedPaths) != 3 {
		t.Errorf("expected 3 captured paths, got %d", len(capturedPaths))
	}

	// Test 1-based sequence numbering (not 0-based)
	expectedSequences := []int{1, 2, 3}
	for i, seq := range capturedSequences {
		if seq != expectedSequences[i] {
			t.Errorf("expected sequence %d, got %d", expectedSequences[i], seq)
		}
	}

	// Test that context was passed through
	if capturedContext == nil {
		t.Error("expected context to be captured")
	} else if capturedContext.ToolName != "git" {
		t.Errorf("expected tool name 'git', got '%s'", capturedContext.ToolName)
	}

	// Test that all paths were captured in order
	expectedPaths := []string{"path1.wav", "path2.wav", "path3.wav"}
	for i, path := range capturedPaths {
		if path != expectedPaths[i] {
			t.Errorf("expected path '%s', got '%s'", expectedPaths[i], path)
		}
	}
}

func TestMultipleHooks(t *testing.T) {
	var hook1Calls []string
	var hook2Calls []string

	hook1 := func(path string, exists bool, sequence int, context *hooks.EventContext) {
		hook1Calls = append(hook1Calls, path)
	}

	hook2 := func(path string, exists bool, sequence int, context *hooks.EventContext) {
		hook2Calls = append(hook2Calls, path)
	}

	sc := NewSoundChecker(WithHook(hook1), WithHook(hook2))
	if len(sc.hooks) != 2 {
		t.Errorf("expected 2 hooks, got %d", len(sc.hooks))
	}

	context := &hooks.EventContext{}
	paths := []string{"test1.wav", "test2.wav"}

	sc.CheckPaths(context, paths)

	// Both hooks should be called for each path
	if len(hook1Calls) != 2 {
		t.Errorf("expected hook1 to be called 2 times, got %d", len(hook1Calls))
	}
	if len(hook2Calls) != 2 {
		t.Errorf("expected hook2 to be called 2 times, got %d", len(hook2Calls))
	}

	// Check that both hooks got the same paths
	for i, path := range hook1Calls {
		if path != paths[i] {
			t.Errorf("hook1: expected path '%s', got '%s'", paths[i], path)
		}
	}
	for i, path := range hook2Calls {
		if path != paths[i] {
			t.Errorf("hook2: expected path '%s', got '%s'", paths[i], path)
		}
	}
}

func TestPathCheckedHookSignature(t *testing.T) {
	// Test that PathCheckedHook has the correct signature
	var hook PathCheckedHook = func(path string, exists bool, sequence int, context *hooks.EventContext) {
		// This test passes if the signature compiles correctly
	}

	if hook == nil {
		t.Error("PathCheckedHook should not be nil")
	}
}