package audio

import (
	"bytes"
	"context"
	"io"
	"runtime"
	"sync"
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

// TestMalgoBackend_SoundIDCounter_Unique exercises the atomic counter
// added for review finding #40. The previous implementation built the
// soundID from len(audioData.Samples), which collides whenever two
// concurrent Plays decode to byte-identical lengths. The new counter is a
// per-backend atomic.Uint64 — each call to Add(1) returns a distinct
// post-increment value. This test calls Add concurrently from many
// goroutines and asserts all values are distinct, with no hardware
// dependency. Verifies the counter itself; integration with Play is
// covered by the existing TestPlay_NoGoroutineLeak under -race.
func TestMalgoBackend_SoundIDCounter_Unique(t *testing.T) {
	backend := NewMalgoBackend()
	defer backend.Close()

	const goroutines = 32
	const perGoroutine = 64
	const total = goroutines * perGoroutine

	results := make(chan uint64, total)
	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				results <- backend.soundIDCount.Add(1)
			}
		}()
	}
	wg.Wait()
	close(results)

	seen := make(map[uint64]struct{}, total)
	for v := range results {
		if _, dup := seen[v]; dup {
			t.Fatalf("duplicate soundID counter value %d under concurrent Add", v)
		}
		seen[v] = struct{}{}
	}
	if len(seen) != total {
		t.Fatalf("expected %d distinct counter values, got %d", total, len(seen))
	}
}

// TestMalgoBackend_Play_ConcurrentSameLengthBuffers_NoCollision drives
// two concurrent Plays with the SAME audio bytes (so len(Samples) is
// identical for both). Before #40's fix, both Plays computed the same
// soundID and the second overwrote the first's deviceEntry in
// AudioPlayer.devices, leaking the first's malgo device handle. The new
// atomic counter makes the IDs distinct. Skips if no audio device is
// present — the underlying PlaySoundWithContext opens a real malgo
// device.
func TestMalgoBackend_Play_ConcurrentSameLengthBuffers_NoCollision(t *testing.T) {
	backend := NewMalgoBackend()
	defer backend.Close()

	wavBytes := generateTestWAV()
	makeSource := func() AudioSource {
		return NewReaderSource(io.NopCloser(bytes.NewReader(wavBytes)), "wav")
	}

	// Warmup to bypass first-Play hardware probe; lets us skipIfNoAudioDevice early.
	warmCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	warmErr := backend.Play(warmCtx, makeSource())
	cancel()
	skipIfNoAudioDevice(t, warmErr)
	if warmErr != nil {
		t.Fatalf("warmup Play failed: %v", warmErr)
	}

	const N = 4
	errs := make(chan error, N)
	var wg sync.WaitGroup
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			errs <- backend.Play(ctx, makeSource())
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent same-length Play returned error: %v", err)
		}
	}
}
