package native

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"claudio.click/internal/audio"
	"claudio.click/internal/testutil/wavfixture"
)

func toneSource(rate, channels, milliseconds int) audio.AudioSource {
	data := wavfixture.WAV(wavfixture.TagPCM, 16, rate, wavfixture.ToneFrames(rate, channels, milliseconds, 2000.0/32768))
	return audio.NewReaderSource(io.NopCloser(bytes.NewReader(data)), "wav")
}

func waitForDevicePlayback(t *testing.T, b *Backend, done <-chan error) {
	t.Helper()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(5 * time.Millisecond)
	defer tick.Stop()
	for {
		if isPlaying(b) {
			return
		}
		select {
		case err := <-done:
			t.Fatalf("playback ended before start: %v", err)
		case <-deadline.C:
			t.Fatal("device never started")
		case <-tick.C:
		}
	}
}

// Opt in on a host with an audio device. Missing devices are failures, not skips,
// once requested. Run with CGO_ENABLED=0 to cover issue 48's installation path.
func TestOtoDevicePlaybackAndCancellation(t *testing.T) {
	if os.Getenv("CLAUDIO_TEST_AUDIO") != "1" {
		t.Skip("set CLAUDIO_TEST_AUDIO=1 to exercise the real audio device")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	for _, rate := range []int{22050, 44100, 48000} {
		b := NewBackend()
		start := time.Now()
		if err := b.Play(ctx, toneSource(rate, 1, 250)); err != nil {
			t.Fatal(err)
		}
		if time.Since(start) < 150*time.Millisecond {
			t.Fatal("playback returned before draining the sound")
		}
		if err := b.Close(); err != nil {
			t.Fatal(err)
		}
		t.Logf("completed native playback at source rate %d", rate)
	}
	b1, b2 := NewBackend(), NewBackend()
	defer b1.Close()
	defer b2.Close()
	playCtx, stop := context.WithCancel(ctx)
	defer stop()
	done1, done2 := make(chan error, 1), make(chan error, 1)
	go func() { done1 <- b1.Play(playCtx, toneSource(44100, 2, 10000)) }()
	go func() { done2 <- b2.Play(playCtx, toneSource(48000, 1, 10000)) }()
	waitForDevicePlayback(t, b1, done1)
	waitForDevicePlayback(t, b2, done2)
	if err := b1.Close(); err != nil {
		t.Fatal(err)
	}
	if err := waitPlay(t, done1); err != nil {
		t.Fatal(err)
	}
	if !isPlaying(b2) {
		t.Fatal("closing one backend stopped the other's player")
	}
	stop()
	if err := waitPlay(t, done2); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
	if isPlaying(b1) || isPlaying(b2) {
		t.Fatal("playback still active after shutdown")
	}
	t.Log("concurrent backends, isolated close, and caller cancellation completed")
}
