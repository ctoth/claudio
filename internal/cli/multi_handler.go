package cli

import (
	"context"
	"log/slog"
)

// MultiLevelHandler wraps multiple handlers with independent level filtering.
// This allows sending ERROR logs to stderr while sending all logs to a file.
type MultiLevelHandler struct {
	handlers []slog.Handler
}

// NewMultiLevelHandler creates a new multi-level handler that distributes logs to multiple handlers.
// Each handler maintains its own level filtering.
func NewMultiLevelHandler(handlers ...slog.Handler) *MultiLevelHandler {
	return &MultiLevelHandler{
		handlers: handlers,
	}
}

// Enabled reports whether the handler handles records at the given level.
// Returns true if ANY of the wrapped handlers would handle the level.
func (h *MultiLevelHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

// Handle sends the record to all wrapped handlers that would handle it.
func (h *MultiLevelHandler) Handle(ctx context.Context, record slog.Record) error {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, record.Level) {
			if err := handler.Handle(ctx, record); err != nil {
				return err
			}
		}
	}
	return nil
}

// WithAttrs returns a new handler with the given attributes added.
func (h *MultiLevelHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		handlers[i] = handler.WithAttrs(attrs)
	}
	return NewMultiLevelHandler(handlers...)
}

// WithGroup returns a new handler with the given group added.
func (h *MultiLevelHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		handlers[i] = handler.WithGroup(name)
	}
	return NewMultiLevelHandler(handlers...)
}
