package audio

import (
	"log/slog"

	"github.com/gen2brain/malgo"
)

// Context wraps malgo.AllocatedContext with proper lifecycle management and logging
type Context struct {
	ctx *malgo.AllocatedContext
}

// NewContext initializes a new audio context with slog integration
func NewContext() (*Context, error) {
	slog.Debug("initializing audio context")

	// Initialize malgo context with logging callback
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, func(message string) {
		slog.Debug("malgo internal", "message", message)
	})
	if err != nil {
		slog.Error("failed to initialize audio context", "error", err)
		return nil, err
	}

	slog.Info("audio context initialized successfully")
	return &Context{ctx: ctx}, nil
}

// Close properly cleans up the audio context
func (c *Context) Close() error {
	if c.ctx == nil {
		slog.Debug("audio context already closed")
		return nil
	}

	slog.Debug("closing audio context")

	// malgo requires both Uninit() and Free()
	err := c.ctx.Uninit()
	if err != nil {
		slog.Error("failed to uninitialize audio context", "error", err)
		return err
	}

	c.ctx.Free()
	c.ctx = nil

	slog.Info("audio context closed successfully")
	return nil
}

// GetContext returns the underlying malgo context for device operations
func (c *Context) GetContext() *malgo.AllocatedContext {
	return c.ctx
}

// IsValid checks if the context is still valid
func (c *Context) IsValid() bool {
	return c.ctx != nil
}
