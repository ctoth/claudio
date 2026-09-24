package audio

import (
	"context"
	"errors"
)

// Common errors for AudioBackend implementations
var (
	ErrBackendNotAvailable = errors.New("audio backend not available")
	ErrBackendClosed       = errors.New("audio backend is closed")
)

// AudioBackend plays audio. It is what production uses: the hook plays one
// sound at the configured volume and closes the backend.
type AudioBackend interface {
	Play(ctx context.Context, source AudioSource) error
	SetVolume(volume float32) error
	Close() error
}
