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
	return audio.NewReaderSource(io.NopCloser(bytes.NewReader(buildWAV(wavTagPCM, 16, 44100, sineFrames(2, 2, 0.1)))), "wav")
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

func TestBackendCloseStopsConcurrentIdenticalSounds(t *testing.T) {
	b, o := testBackend()
	done := make(chan error, 2)
	for range 2 {
		go func() { done <- b.Play(context.Background(), testSource()) }()
	}
	p1, p2 := waitPlayer(t, o), waitPlayer(t, o)
	if !isPlaying(b) {
		t.Fatal("not playing")
	}
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := waitPlay(t, done); err != nil {
			t.Fatal(err)
		}
	}
	if isPlaying(b) || !p1.paused || !p2.paused {
		t.Fatal("Close left a player active")
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
		if volume != 0.25 || backendVolume(b) != 0.25 {
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
		if !p.paused || isPlaying(b) {
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

// drainingOutput behaves like Oto: a player stops playing only after it has
// read its source to EOF, while the device still holds the last bytes read.
type drainingOutput struct{ read chan []byte }

type drainingPlayer struct {
	testPlayer
	r    io.Reader
	read chan []byte
}

func (o *drainingOutput) NewPlayer(r io.Reader) outputPlayer {
	return &drainingPlayer{testPlayer: testPlayer{started: make(chan struct{})}, r: r, read: o.read}
}
func (o *drainingOutput) Err() error { return nil }

func (p *drainingPlayer) Play() {
	p.testPlayer.Play()
	go func() {
		data, err := io.ReadAll(p.r)
		p.read <- data
		p.finish(err)
	}()
}

func TestBackendFlushesDeviceBufferWithSilenceBeforeReturning(t *testing.T) {
	b := NewBackend()
	defer b.Close()
	o := &drainingOutput{read: make(chan []byte, 1)}
	b.openOutput = func(context.Context) (outputContext, error) { return o, nil }
	if err := b.Play(context.Background(), testSource()); err != nil {
		t.Fatal(err)
	}
	data := <-o.read

	const bytesPerSecond = outputSampleRate * 2 * 4
	buffered := int(outputBufferSize.Seconds() * bytesPerSecond)
	if outputBufferSize <= 0 {
		t.Fatal("device buffer size must be explicit so the drain can cover it")
	}
	// The process exits after Play returns, discarding whatever the device
	// still buffers. Trailing silence must push all real audio out first;
	// PulseAudio may hold up to twice its target length.
	if len(data) <= 2*buffered || len(data)%8 != 0 {
		t.Fatalf("read %d bytes, want frame-aligned audio plus at least %d bytes of drain", len(data), 2*buffered)
	}
	tail := data[len(data)-2*buffered:]
	if !bytes.Equal(tail, make([]byte, len(tail))) {
		t.Fatalf("last %v of output is not silence; the end of the sound would be cut off", 2*outputBufferSize)
	}
	if bytes.Equal(data[:len(data)-len(tail)], make([]byte, len(data)-len(tail))) {
		t.Fatal("no audio before the drain")
	}
}

// isPlaying reports whether any admitted playback has a live player.
func isPlaying(b *Backend) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	for p := range b.plays {
		if p.player != nil && p.player.IsPlaying() {
			return true
		}
	}
	return false
}

// backendVolume reads the volume new players start at.
func backendVolume(b *Backend) float32 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.volume
}
