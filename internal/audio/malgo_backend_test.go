package audio

import (
	"bytes"
	"context"
	"io"
	"runtime"
	"testing"
	"time"
)

// TestPlay_NoGoroutineLeak asserts the cleanup goroutine that used to wait
// on ctx.Done() inside MalgoBackend.Play has been removed. Per review
// finding #3, the production caller passed context.Background(), whose
// Done() channel is nil, so the spawned receive blocked forever — one
// leaked goroutine per Play. After the fix, UnloadSound runs inline after
// the synchronous PlaySoundWithContext returns.
//
// The test snapshots runtime.NumGoroutine() before and after N Play calls
// against a deterministic context.Background() and asserts the delta is
// bounded by a small constant. Skips if no audio device is present (the
// underlying PlaySoundWithContext opens a real malgo device).
func TestPlay_NoGoroutineLeak(t *testing.T) {
	backend := NewMalgoBackend()
	defer backend.Close()

	wavBytes := generateTestWAV()
	makeSource := func() AudioSource {
		return NewReaderSource(io.NopCloser(bytes.NewReader(wavBytes)), "wav")
	}

	// Warm up: one Play to drive any one-time goroutine spawns (cgo workers,
	// malgo context init, slog handler init) before we baseline.
	warmCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	warmErr := backend.Play(warmCtx, makeSource())
	cancel()
	skipIfNoAudioDevice(t, warmErr)
	if warmErr != nil {
		t.Fatalf("warmup Play failed: %v", warmErr)
	}

	// Let any transient goroutines from warmup retire.
	time.Sleep(50 * time.Millisecond)
	runtime.GC()
	runtime.GC()
	baseline := runtime.NumGoroutine()

	const N = 10
	for i := 0; i < N; i++ {
		ctx := context.Background() // deliberately Done()==nil, the original leak trigger
		err := backend.Play(ctx, makeSource())
		if err != nil {
			t.Fatalf("Play #%d failed: %v", i, err)
		}
	}

	// Give the scheduler a beat for any cleanup goroutines to retire.
	time.Sleep(100 * time.Millisecond)
	runtime.GC()
	runtime.GC()
	after := runtime.NumGoroutine()

	// Before the fix this delta would be N (10) — one leaked ctx.Done waiter
	// per Play. After the fix the delta should be 0; we allow a small
	// constant for incidental scheduler noise / cgo workers.
	if after > baseline+2 {
		t.Errorf("goroutine leak: baseline=%d after=%d delta=%d (expected ≤2)",
			baseline, after, after-baseline)
	}
}
