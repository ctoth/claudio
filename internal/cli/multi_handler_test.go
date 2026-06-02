package cli

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"
)

// failingHandler is an slog.Handler used to verify MultiLevelHandler keeps
// invoking subsequent handlers even after an earlier handler errors.
type failingHandler struct {
	level  slog.Level
	err    error
	called bool
}

func (f *failingHandler) Enabled(_ context.Context, lvl slog.Level) bool {
	return lvl >= f.level
}

func (f *failingHandler) Handle(_ context.Context, _ slog.Record) error {
	f.called = true
	return f.err
}

func (f *failingHandler) WithAttrs(_ []slog.Attr) slog.Handler { return f }
func (f *failingHandler) WithGroup(_ string) slog.Handler      { return f }

// TestMultiLevelHandler_AllHandlersRunOnError covers finding #46: a failing
// stderr handler must not silently skip the file handler. All handlers must
// fire and their errors aggregate via errors.Join.
func TestMultiLevelHandler_AllHandlersRunOnError(t *testing.T) {
	firstErr := errors.New("first handler boom")
	first := &failingHandler{level: slog.LevelDebug, err: firstErr}

	var secondBuf bytes.Buffer
	second := slog.NewTextHandler(&secondBuf, &slog.HandlerOptions{Level: slog.LevelDebug})

	multi := NewMultiLevelHandler(first, second)

	err := multi.Handle(context.Background(), slog.NewRecord(time.Time{}, slog.LevelInfo, "msg via multi", 0))
	if err == nil {
		t.Fatal("expected aggregated error from failing handler, got nil")
	}
	if !errors.Is(err, firstErr) {
		t.Fatalf("expected errors.Is to match first handler error, got: %v", err)
	}
	if !first.called {
		t.Error("first (failing) handler should have been invoked")
	}
	if got := secondBuf.String(); !strings.Contains(got, "msg via multi") {
		t.Errorf("second handler should have received record even though first errored, got: %q", got)
	}
}

// TestMultiLevelHandler_AggregatesMultipleErrors verifies errors.Join behavior
// when multiple handlers fail simultaneously.
func TestMultiLevelHandler_AggregatesMultipleErrors(t *testing.T) {
	errA := errors.New("a fail")
	errB := errors.New("b fail")
	a := &failingHandler{level: slog.LevelDebug, err: errA}
	b := &failingHandler{level: slog.LevelDebug, err: errB}

	multi := NewMultiLevelHandler(a, b)
	err := multi.Handle(context.Background(), slog.NewRecord(time.Time{}, slog.LevelError, "x", 0))
	if err == nil {
		t.Fatal("expected aggregated error, got nil")
	}
	if !errors.Is(err, errA) || !errors.Is(err, errB) {
		t.Fatalf("expected aggregated error to contain both errA and errB, got: %v", err)
	}
	if !a.called || !b.called {
		t.Fatalf("both handlers should have been called: a=%v b=%v", a.called, b.called)
	}
}

func TestMultiLevelHandler_DifferentLevels(t *testing.T) {
	// Setup: Create two buffers for different outputs
	var stderrBuf, fileBuf bytes.Buffer

	// Create stderr handler that only accepts ERROR level
	stderrHandler := slog.NewTextHandler(&stderrBuf, &slog.HandlerOptions{
		Level: slog.LevelError,
	})

	// Create file handler that accepts DEBUG level
	fileHandler := slog.NewTextHandler(&fileBuf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})

	// Create multi-level handler combining both
	multiHandler := NewMultiLevelHandler(stderrHandler, fileHandler)
	logger := slog.New(multiHandler)

	// Test: Log messages at different levels
	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warn message")
	logger.Error("error message")

	// Verify: stderr should only have ERROR
	stderrOutput := stderrBuf.String()
	if !strings.Contains(stderrOutput, "error message") {
		t.Errorf("stderr should contain error message, got: %s", stderrOutput)
	}
	if strings.Contains(stderrOutput, "debug message") {
		t.Errorf("stderr should not contain debug message, got: %s", stderrOutput)
	}
	if strings.Contains(stderrOutput, "info message") {
		t.Errorf("stderr should not contain info message, got: %s", stderrOutput)
	}
	if strings.Contains(stderrOutput, "warn message") {
		t.Errorf("stderr should not contain warn message, got: %s", stderrOutput)
	}

	// Verify: file should have all messages
	fileOutput := fileBuf.String()
	if !strings.Contains(fileOutput, "debug message") {
		t.Errorf("file should contain debug message, got: %s", fileOutput)
	}
	if !strings.Contains(fileOutput, "info message") {
		t.Errorf("file should contain info message, got: %s", fileOutput)
	}
	if !strings.Contains(fileOutput, "warn message") {
		t.Errorf("file should contain warn message, got: %s", fileOutput)
	}
	if !strings.Contains(fileOutput, "error message") {
		t.Errorf("file should contain error message, got: %s", fileOutput)
	}
}

func TestMultiLevelHandler_Enabled(t *testing.T) {
	// Setup: Create handlers with different levels
	var buf1, buf2 bytes.Buffer
	handler1 := slog.NewTextHandler(&buf1, &slog.HandlerOptions{Level: slog.LevelError})
	handler2 := slog.NewTextHandler(&buf2, &slog.HandlerOptions{Level: slog.LevelDebug})
	multiHandler := NewMultiLevelHandler(handler1, handler2)

	ctx := context.Background()

	// Test: Enabled should return true if ANY handler accepts the level
	if !multiHandler.Enabled(ctx, slog.LevelDebug) {
		t.Error("multi-handler should be enabled for DEBUG (handler2 accepts it)")
	}
	if !multiHandler.Enabled(ctx, slog.LevelInfo) {
		t.Error("multi-handler should be enabled for INFO (handler2 accepts it)")
	}
	if !multiHandler.Enabled(ctx, slog.LevelWarn) {
		t.Error("multi-handler should be enabled for WARN (handler2 accepts it)")
	}
	if !multiHandler.Enabled(ctx, slog.LevelError) {
		t.Error("multi-handler should be enabled for ERROR (both handlers accept it)")
	}
}

func TestMultiLevelHandler_WithAttrs(t *testing.T) {
	// Setup: Create multi-handler
	var buf1, buf2 bytes.Buffer
	handler1 := slog.NewTextHandler(&buf1, &slog.HandlerOptions{Level: slog.LevelError})
	handler2 := slog.NewTextHandler(&buf2, &slog.HandlerOptions{Level: slog.LevelDebug})
	multiHandler := NewMultiLevelHandler(handler1, handler2)

	// Test: WithAttrs should propagate to all handlers
	attrs := []slog.Attr{slog.String("key", "value")}
	newHandler := multiHandler.WithAttrs(attrs)

	logger := slog.New(newHandler)
	logger.Error("test message")

	// Verify: Both outputs should contain the attribute
	if !strings.Contains(buf1.String(), "key=value") {
		t.Errorf("handler1 output should contain attribute, got: %s", buf1.String())
	}
	if !strings.Contains(buf2.String(), "key=value") {
		t.Errorf("handler2 output should contain attribute, got: %s", buf2.String())
	}
}

func TestMultiLevelHandler_WithGroup(t *testing.T) {
	// Setup: Create multi-handler
	var buf1, buf2 bytes.Buffer
	handler1 := slog.NewTextHandler(&buf1, &slog.HandlerOptions{Level: slog.LevelError})
	handler2 := slog.NewTextHandler(&buf2, &slog.HandlerOptions{Level: slog.LevelDebug})
	multiHandler := NewMultiLevelHandler(handler1, handler2)

	// Test: WithGroup should propagate to all handlers
	newHandler := multiHandler.WithGroup("testgroup")

	logger := slog.New(newHandler)
	logger.Error("test message", "key", "value")

	// Verify: Both outputs should contain the group
	if !strings.Contains(buf1.String(), "testgroup") {
		t.Errorf("handler1 output should contain group, got: %s", buf1.String())
	}
	if !strings.Contains(buf2.String(), "testgroup") {
		t.Errorf("handler2 output should contain group, got: %s", buf2.String())
	}
}

func TestMultiLevelHandler_EmptyHandlers(t *testing.T) {
	// Setup: Create multi-handler with no handlers
	multiHandler := NewMultiLevelHandler()

	ctx := context.Background()

	// Test: Should handle gracefully with no handlers
	if multiHandler.Enabled(ctx, slog.LevelError) {
		t.Error("multi-handler with no handlers should not be enabled")
	}

	// Test: Handle should not panic with no handlers
	logger := slog.New(multiHandler)
	logger.Error("test") // This should not panic even with no handlers
}

func TestMultiLevelHandler_SingleHandler(t *testing.T) {
	// Setup: Create multi-handler with single handler
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})
	multiHandler := NewMultiLevelHandler(handler)

	logger := slog.New(multiHandler)

	// Test: Should work correctly with single handler
	logger.Debug("debug message")
	logger.Warn("warn message")

	// Verify: Should only contain warn message
	output := buf.String()
	if strings.Contains(output, "debug message") {
		t.Errorf("output should not contain debug message, got: %s", output)
	}
	if !strings.Contains(output, "warn message") {
		t.Errorf("output should contain warn message, got: %s", output)
	}
}
