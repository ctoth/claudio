package tracking

import (
	"log/slog"

	"claudio.click/internal/hooks"
)

// SlogHook provides structured logging of path checks for debugging
type SlogHook struct {
	logger *slog.Logger
}

// NewSlogHook creates a new SlogHook with the given logger
// If logger is nil, uses the default logger
func NewSlogHook(logger *slog.Logger) *SlogHook {
	if logger == nil {
		logger = slog.Default()
	}
	return &SlogHook{
		logger: logger,
	}
}

// GetHook returns the PathCheckedHook function for integration with SoundChecker
func (s *SlogHook) GetHook() PathCheckedHook {
	return func(path string, exists bool, sequence int, context *hooks.EventContext) {
		s.logger.Debug("path check",
			"path", path,
			"exists", exists,
			"sequence", sequence,
			"category", context.Category.String(),
			"tool_name", context.ToolName,
			"operation", context.Operation,
		)
	}
}

// NopHook provides a no-operation hook for disabled modes
type NopHook struct{}

// NewNopHook creates a new NopHook that does nothing
func NewNopHook() *NopHook {
	return &NopHook{}
}

// GetHook returns the PathCheckedHook function that does nothing
func (n *NopHook) GetHook() PathCheckedHook {
	return func(path string, exists bool, sequence int, context *hooks.EventContext) {
		// No-op: do nothing
	}
}