package tracking

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"claudio.click/internal/hooks"
)

func TestSlogHook_WithCustomLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	hook := NewSlogHook(logger)
	hookFunc := hook.GetHook()

	context := &hooks.EventContext{
		Category:  hooks.Success,
		ToolName:  "bash",
		Operation: "tool-complete",
	}

	hookFunc("test/path.wav", true, 1, context)

	output := buf.String()
	if !strings.Contains(output, "path=test/path.wav") {
		t.Errorf("Expected log to contain path, got: %s", output)
	}
	if !strings.Contains(output, "exists=true") {
		t.Errorf("Expected log to contain exists=true, got: %s", output)
	}
	if !strings.Contains(output, "sequence=1") {
		t.Errorf("Expected log to contain sequence=1, got: %s", output)
	}
	if !strings.Contains(output, "category=success") {
		t.Errorf("Expected log to contain category=success, got: %s", output)
	}
}

func TestSlogHook_WithNilLogger(t *testing.T) {
	// Capture default logger output
	var buf bytes.Buffer
	defaultLogger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(defaultLogger)

	hook := NewSlogHook(nil)
	hookFunc := hook.GetHook()

	context := &hooks.EventContext{
		Category:  hooks.Error,
		ToolName:  "read",
		Operation: "file-error",
	}

	hookFunc("missing/file.wav", false, 2, context)

	output := buf.String()
	if !strings.Contains(output, "path=missing/file.wav") {
		t.Errorf("Expected log to contain path, got: %s", output)
	}
	if !strings.Contains(output, "exists=false") {
		t.Errorf("Expected log to contain exists=false, got: %s", output)
	}
}

func TestNopHook_DoesNothing(t *testing.T) {
	hook := NewNopHook()
	hookFunc := hook.GetHook()

	context := &hooks.EventContext{
		Category:  hooks.Loading,
		ToolName:  "grep",
		Operation: "search-thinking",
	}

	// This should not panic or cause any side effects
	hookFunc("any/path.wav", true, 3, context)
	hookFunc("another/path.wav", false, 4, context)

	// Test passes if no panic occurs
}

func TestSlogHook_IntegratesWithSoundChecker(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	slogHook := NewSlogHook(logger)
	checker := NewSoundChecker(WithHook(slogHook.GetHook()))

	context := &hooks.EventContext{
		Category:  hooks.Interactive,
		ToolName:  "edit",
		Operation: "user-prompt",
	}

	paths := []string{"interactive/edit.wav", "interactive/interactive.wav"}
	results := checker.CheckPaths(context, paths)

	// Both paths should be checked (return false since no real files)
	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}

	output := buf.String()
	// Should log both path checks
	pathCount := strings.Count(output, "path=interactive/")
	if pathCount != 2 {
		t.Errorf("Expected 2 path log entries, got %d in: %s", pathCount, output)
	}
}

func TestNopHook_IntegratesWithSoundChecker(t *testing.T) {
	nopHook := NewNopHook()
	checker := NewSoundChecker(WithHook(nopHook.GetHook()))

	context := &hooks.EventContext{
		Category:  hooks.Success,
		ToolName:  "write",
		Operation: "file-complete",
	}

	paths := []string{"success/write.wav", "success/success.wav", "default.wav"}
	results := checker.CheckPaths(context, paths)

	// All paths should be checked (return false since no real files)
	if len(results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(results))
	}

	// Test passes if no side effects occur
}

func TestMultipleHooks_CoexistProperly(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	slogHook := NewSlogHook(logger)
	nopHook := NewNopHook()

	checker := NewSoundChecker(
		WithHook(slogHook.GetHook()),
		WithHook(nopHook.GetHook()),
	)

	context := &hooks.EventContext{
		Category:  hooks.Loading,
		ToolName:  "bash",
		Operation: "command-thinking",
	}

	paths := []string{"loading/bash-thinking.wav"}
	results := checker.CheckPaths(context, paths)

	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}

	output := buf.String()
	// SlogHook should have logged the path check
	if !strings.Contains(output, "path=loading/bash-thinking.wav") {
		t.Errorf("Expected slog hook to log path, got: %s", output)
	}

	// NopHook should not interfere (test passes if no errors)
}

func TestSlogHook_LogsAllContextFields(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	hook := NewSlogHook(logger)
	hookFunc := hook.GetHook()

	context := &hooks.EventContext{
		Category:  hooks.Error,
		ToolName:  "glob",
		Operation: "pattern-error",
	}

	hookFunc("error/glob-error.wav", false, 5, context)

	output := buf.String()
	expectedFields := []string{
		"category=error",
		"tool_name=glob", 
		"operation=pattern-error",
		"path=error/glob-error.wav",
		"exists=false",
		"sequence=5",
	}

	for _, field := range expectedFields {
		if !strings.Contains(output, field) {
			t.Errorf("Expected log to contain %s, got: %s", field, output)
		}
	}
}