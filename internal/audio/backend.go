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

// AudioBackend represents a system for playing audio from various sources
// Implementations handle the actual audio playback mechanism (malgo, system commands, etc.)
type AudioBackend interface {
	// Lifecycle management
	Start() error
	Stop() error
	Close() error

	// State management
	IsPlaying() bool
	SetVolume(volume float32) error
	GetVolume() float32

	// Playback - unified interface supporting both file paths and readers
	Play(ctx context.Context, source AudioSource) error
}
