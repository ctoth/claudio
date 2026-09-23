package native

import (
	"context"
	"errors"
	"testing"
	"time"
)

// A device can stop pulling data without reporting an error (suspended
// PulseAudio sink, unplugged Bluetooth output). The hook passes a background
// context, so Play itself must give up or detached workers pile up forever.
func TestBackendAbandonsPlaybackThatNeverFinishes(t *testing.T) {
	b, o := testBackend()
	defer b.Close()
	b.stallGrace = 50 * time.Millisecond
	done := make(chan error, 1)
	go func() { done <- b.Play(context.Background(), testSource()) }()
	p := waitPlayer(t, o) // never finishes

	err := waitPlay(t, done)
	if !errors.Is(err, ErrPlaybackStalled) {
		t.Fatalf("stalled playback returned %v, want ErrPlaybackStalled", err)
	}
	if !p.paused || b.IsPlaying() {
		t.Fatal("stalled player was not released")
	}
}

func TestBackendAbandonsDeviceThatNeverStarts(t *testing.T) {
	b := NewBackend()
	defer b.Close()
	b.startTimeout = 50 * time.Millisecond
	b.openOutput = func(ctx context.Context) (outputContext, error) { <-ctx.Done(); return nil, ctx.Err() }
	done := make(chan error, 1)
	go func() { done <- b.Play(context.Background(), testSource()) }()

	if err := waitPlay(t, done); !errors.Is(err, ErrPlaybackStalled) {
		t.Fatalf("stalled device start returned %v, want ErrPlaybackStalled", err)
	}
}

// The deadline must cover the whole sound plus the silence drain, so long
// sounds are never cut short.
func TestPlaybackDeadlineCoversSoundAndDrain(t *testing.T) {
	b := NewBackend()
	b.stallGrace = time.Second
	oneSecond := sound{rate: 44100, frames: 44100}
	got := b.playbackDeadline(oneSecond)
	want := time.Second + 2*outputBufferSize + time.Second
	if got != want {
		t.Fatalf("deadline = %v, want %v", got, want)
	}
}
