package cli

import (
	"context"
	"log/slog"
	"sync"
)

// maxStartupRecords bounds the pre-logging buffer; startup emits a handful.
const maxStartupRecords = 64

// startupHandler is the default logger between process start and
// setupLogging. Config loading logs warnings (deprecated values, bad env
// vars) before the file handler exists; stderr stays ERROR-only, so without
// this buffer those warnings would be lost. setupLogging replays them into
// the log file.
type startupHandler struct {
	stderr slog.Handler
	attrs  []slog.Attr
	buf    *startupBuffer
}

type startupBuffer struct {
	mu      sync.Mutex
	records []slog.Record
	dropped int
}

func newStartupHandler(stderr slog.Handler) *startupHandler {
	return &startupHandler{stderr: stderr, buf: &startupBuffer{}}
}

func (h *startupHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= slog.LevelWarn || h.stderr.Enabled(ctx, level)
}

func (h *startupHandler) Handle(ctx context.Context, r slog.Record) error {
	if r.Level >= slog.LevelWarn {
		c := r.Clone()
		c.AddAttrs(h.attrs...)
		h.buf.mu.Lock()
		if len(h.buf.records) < maxStartupRecords {
			h.buf.records = append(h.buf.records, c)
		} else {
			h.buf.dropped++
		}
		h.buf.mu.Unlock()
	}
	if h.stderr.Enabled(ctx, r.Level) {
		return h.stderr.Handle(ctx, r)
	}
	return nil
}

func (h *startupHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &startupHandler{
		stderr: h.stderr.WithAttrs(attrs),
		attrs:  append(append([]slog.Attr(nil), h.attrs...), attrs...),
		buf:    h.buf,
	}
}

// WithGroup is not buffered: nothing in startup logs through a group, and
// flattening grouped attrs into the replay would misrepresent them.
func (h *startupHandler) WithGroup(name string) slog.Handler {
	return h.stderr.WithGroup(name)
}

// replay sends buffered records to dst, honoring dst's level, and empties
// the buffer so a second setupLogging cannot duplicate them.
func (h *startupHandler) replay(ctx context.Context, dst slog.Handler) {
	h.buf.mu.Lock()
	records, dropped := h.buf.records, h.buf.dropped
	h.buf.records, h.buf.dropped = nil, 0
	h.buf.mu.Unlock()
	for _, r := range records {
		if dst.Enabled(ctx, r.Level) {
			_ = dst.Handle(ctx, r)
		}
	}
	if dropped > 0 {
		slog.New(dst).Warn("startup log records dropped", "count", dropped)
	}
}
