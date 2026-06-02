package cli

import (
	"context"
	"errors"
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
// Every enabled handler is attempted even if an earlier handler returns an
// error; the per-handler errors are aggregated via errors.Join so that a
// failing stderr handler does not silently drop the file handler (or vice
// versa).
func (h *MultiLevelHandler) Handle(ctx context.Context, record slog.Record) error {
	var errs []error
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, record.Level) {
			if err := handler.Handle(ctx, record); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
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
