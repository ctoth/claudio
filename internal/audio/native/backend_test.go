package native

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math"
	"sync"
	"testing"
	"time"

	"claudio.click/internal/audio"
)

type testPlayer struct {
	mu              sync.Mutex
	playing, paused bool
	volume          float64
	err             error
	started         chan struct{}
}

func (p *testPlayer) Play() { p.mu.Lock(); defer p.mu.Unlock(); p.playing = true; close(p.started) }
func (p *testPlayer) PauseAndStopReading() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.playing = false
	p.paused = true
}
func (p *testPlayer) IsPlaying() bool     { p.mu.Lock(); defer p.mu.Unlock(); return p.playing }
func (p *testPlayer) Err() error          { p.mu.Lock(); defer p.mu.Unlock(); return p.err }
func (p *testPlayer) SetVolume(v float64) { p.mu.Lock(); defer p.mu.Unlock(); p.volume = v }
func (p *testPlayer) finish(err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.err = err
	p.playing = false
}

type testOutput struct {
	players chan *testPlayer
	err     error
}

func (o *testOutput) NewPlayer(io.Reader) outputPlayer {
	p := &testPlayer{started: make(chan struct{})}
	o.players <- p
	return p
}
func (o *testOutput) Err() error { return o.err }

func testBackend() (*Backend, *testOutput) {
	b := NewBackend()
	o := &testOutput{players: make(chan *testPlayer, 10)}
	b.openOutput = func(context.Context) (outputContext, error) { return o, nil }
	return b, o
}

func testSource() audio.AudioSource {
	return audio.NewReaderSource(io.NopCloser(bytes.NewReader(generateTestWAV())), "wav")
}
func waitPlayer(t *testing.T, o *testOutput) *testPlayer {
	t.Helper()
	select {
	case p := <-o.players:
		select {
		case <-p.started:
			return p
		case <-time.After(3 * time.Second):
			t.Fatal("player did not start")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("player not created")
	}
	return nil
}
func waitPlay(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(3 * time.Second):
		t.Fatal("playback did not return")
	}
	return nil
}

func TestBackendStopConcurrentIdenticalSounds(t *testing.T) {
	b, o := testBackend()
	defer b.Close()
	done := make(chan error, 2)
	for range 2 {
		go func() { done <- b.Play(context.Background(), testSource()) }()
	}
	p1, p2 := waitPlayer(t, o), waitPlayer(t, o)
	if !b.IsPlaying() {
		t.Fatal("not playing")
	}
	if err := b.Stop(); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := waitPlay(t, done); err != nil {
			t.Fatal(err)
		}
	}
	if b.IsPlaying() || !p1.paused || !p2.paused {
		t.Fatal("Stop left a player active")
	}
	// Stop leaves the backend reusable; Close does not.
	go func() { done <- b.Play(context.Background(), testSource()) }()
	p3 := waitPlayer(t, o)
	p3.finish(nil)
	if err := waitPlay(t, done); err != nil {
		t.Fatal(err)
	}
}

func TestBackendCloseCancelsStartupAndRejectsNewPlayback(t *testing.T) {
	b := NewBackend()
	entered := make(chan struct{})
	b.openOutput = func(ctx context.Context) (outputContext, error) { close(entered); <-ctx.Done(); return nil, ctx.Err() }
	done := make(chan error, 1)
	go func() { done <- b.Play(context.Background(), testSource()) }()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("startup never entered")
	}
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	if err := waitPlay(t, done); err != nil {
		t.Fatal(err)
	}
	if err := b.Play(context.Background(), testSource()); !errors.Is(err, audio.ErrBackendClosed) {
		t.Fatalf("late Play: %v", err)
	}
	if err := b.SetVolume(0.5); !errors.Is(err, audio.ErrBackendClosed) {
		t.Fatalf("late SetVolume: %v", err)
	}
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestBackendCancellationAndCompletion(t *testing.T) {
	for _, cancelPlayback := range []bool{false, true} {
		b, o := testBackend()
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { done <- b.Play(ctx, testSource()) }()
		p := waitPlayer(t, o)
		if err := b.SetVolume(0.25); err != nil {
			t.Fatal(err)
		}
		p.mu.Lock()
		volume := p.volume
		p.mu.Unlock()
		if volume != 0.25 || b.GetVolume() != 0.25 {
			t.Fatalf("volume=%v", volume)
		}
		for _, v := range []float32{-1, 2, float32(math.NaN()), float32(math.Inf(1))} {
			if b.SetVolume(v) == nil {
				t.Errorf("accepted volume %v", v)
			}
		}
		if cancelPlayback {
			cancel()
		} else {
			p.finish(nil)
		}
		err := waitPlay(t, done)
		if cancelPlayback && !errors.Is(err, context.Canceled) {
			t.Fatalf("cancel: %v", err)
		}
		if !cancelPlayback && err != nil {
			t.Fatalf("completion: %v", err)
		}
		if !p.paused || b.IsPlaying() {
			t.Fatal("player was not released")
		}
		cancel()
		_ = b.Close()
	}
}

func TestBackendSurfacesOutputAndPlayerErrors(t *testing.T) {
	want := errors.New("device failed")
	b, o := testBackend()
	defer b.Close()
	done := make(chan error, 1)
	go func() { done <- b.Play(context.Background(), testSource()) }()
	p := waitPlayer(t, o)
	p.finish(want)
	if err := waitPlay(t, done); !errors.Is(err, want) {
		t.Fatalf("player error: %v", err)
	}
	o.err = want
	if err := b.Play(context.Background(), testSource()); !errors.Is(err, want) {
		t.Fatalf("output error: %v", err)
	}
}

func TestBackendPrecancelledDoesNotInitializeOutput(t *testing.T) {
	b := NewBackend()
	defer b.Close()
	b.openOutput = func(context.Context) (outputContext, error) {
		t.Fatal("initialized for cancelled playback")
		return nil, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := b.Play(ctx, testSource()); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
