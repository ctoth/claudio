package audio

import (
	"context"
	"errors"
	"sync"
)

// FakeBackend is a test fake that records Play invocations and never
// touches real audio hardware. It is registered under the name "fake"
// via init() so any caller of NewBackend("fake") (typically tests
// configuring cfg.AudioBackend = "fake") gets one. Construction is
// cheap and observation is via the public helper methods.
//
// The fake is intentionally included in the production binary (not
// under a build tag) so cross-package tests in internal/cli can reach
// it through the audio package's import graph. Its ~120 LOC cost is
// the price of avoiding a separate test-support subpackage and a
// manual RegisterBackend call from every cli test.
type FakeBackend struct {
	mu        sync.Mutex
	plays     []FakePlay
	volume    float32
	isPlaying bool
	closed    bool
}

// FakePlay records a single Play call.
type FakePlay struct {
	SourcePath string // best-effort file path if the source implements FilePather; empty otherwise
	Volume     float32
}

// NewFakeBackend constructs a fresh FakeBackend with default volume 1.0.
func NewFakeBackend() *FakeBackend {
	return &FakeBackend{volume: 1.0}
}

// Play records the invocation. If the source implements FilePather, the
// resolved file path is captured. Otherwise SourcePath remains empty.
func (f *FakeBackend) Play(ctx context.Context, source AudioSource) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return ErrBackendClosed
	}
	var path string
	if fp, ok := source.(FilePather); ok {
		if p, err := fp.FilePath(); err == nil {
			path = p
		}
	}
	f.plays = append(f.plays, FakePlay{SourcePath: path, Volume: f.volume})
	f.isPlaying = true
	return nil
}

// Stop flips the isPlaying flag to false.
func (f *FakeBackend) Stop() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.isPlaying = false
	return nil
}

// Close marks the fake closed so subsequent Play returns ErrBackendClosed.
func (f *FakeBackend) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	f.isPlaying = false
	return nil
}

// IsPlaying returns the most-recent Play/Stop state.
func (f *FakeBackend) IsPlaying() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.isPlaying
}

// SetVolume stores the supplied volume; range-checks to [0.0, 1.0].
func (f *FakeBackend) SetVolume(volume float32) error {
	if volume < 0.0 || volume > 1.0 {
		return errors.New("volume out of range [0.0, 1.0]")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.volume = volume
	return nil
}

// GetVolume returns the current stored volume.
func (f *FakeBackend) GetVolume() float32 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.volume
}

// Plays returns a copy of recorded Play invocations under the lock.
func (f *FakeBackend) Plays() []FakePlay {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]FakePlay, len(f.plays))
	copy(out, f.plays)
	return out
}

// Closed reports whether Close has been called.
func (f *FakeBackend) Closed() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.closed
}

var (
	lastFakeBackendMu sync.Mutex
	lastFakeBackend   *FakeBackend
)

// LastFakeBackend returns the most recently constructed FakeBackend
// (via NewBackend("fake")), or nil if no fake backend has been
// created yet. Test-only accessor for asserting on recorded Play
// invocations across the audio→cli package boundary.
func LastFakeBackend() *FakeBackend {
	lastFakeBackendMu.Lock()
	defer lastFakeBackendMu.Unlock()
	return lastFakeBackend
}

// ResetLastFakeBackend clears the stashed fake-backend pointer. Tests
// that share process state (e.g. table-driven runs against multiple
// cli.Run invocations) can call this before each iteration to keep
// LastFakeBackend from returning a stale handle.
func ResetLastFakeBackend() {
	lastFakeBackendMu.Lock()
	defer lastFakeBackendMu.Unlock()
	lastFakeBackend = nil
}

func init() {
	RegisterBackend("fake", func() (AudioBackend, error) {
		fb := NewFakeBackend()
		lastFakeBackendMu.Lock()
		lastFakeBackend = fb
		lastFakeBackendMu.Unlock()
		return fb, nil
	})
}
