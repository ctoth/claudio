package tracking

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"claudio.click/internal/soundpack"
)

// MockSoundpackResolver and TestSoundChecker_WithResolver were deleted along
// with SoundChecker. The per-candidate existence-and-resolution contract
// they asserted now lives in soundpack (TestUnifiedSoundpackResolver,
// TestResolveWithObserver_*) — the resolver owns its own os.Stat loop and
// exposes observation via PathObserver, which LookupBuffer subscribes to.

// TestLookupBuffer_CollectsObserverEvents asserts that LookupBuffer.Observer()
// returns a soundpack.PathObserver that appends one Lookup per callback in
// order, preserving the 1-based sequence and exists flag.
func TestLookupBuffer_CollectsObserverEvents(t *testing.T) {
	buf := NewLookupBuffer()
	obs := buf.Observer()

	// Feed three synthetic observer events as if the resolver had fired them.
	obs("loading/bash-thinking.wav", 1, false)
	obs("loading/bash-start.wav", 2, true)
	obs("loading/loading.wav", 3, false)

	got := buf.Lookups()
	if len(got) != 3 {
		t.Fatalf("Lookups() returned %d entries, want 3", len(got))
	}

	want := []Lookup{
		{Path: "loading/bash-thinking.wav", Sequence: 1, Found: false},
		{Path: "loading/bash-start.wav", Sequence: 2, Found: true},
		{Path: "loading/loading.wav", Sequence: 3, Found: false},
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("Lookups()[%d] = %+v, want %+v", i, got[i], w)
		}
	}
}

// TestLookupBuffer_EmptyBeforeObservation asserts a fresh buffer has zero
// lookups recorded. Sanity check on initial state.
func TestLookupBuffer_EmptyBeforeObservation(t *testing.T) {
	buf := NewLookupBuffer()
	if got := buf.Lookups(); len(got) != 0 {
		t.Errorf("fresh LookupBuffer has %d lookups, want 0", len(got))
	}
}

// TestLookupBuffer_ImplementsPathObserver pins that Observer() returns a
// value assignable to soundpack.PathObserver — the structural integration
// point with soundpack.WithObserver.
func TestLookupBuffer_ImplementsPathObserver(t *testing.T) {
	buf := NewLookupBuffer()
	var _ soundpack.PathObserver = buf.Observer()
}

// TestLookupBuffer_IntegratesWithSoundpackResolver wires a LookupBuffer
// through soundpack.WithObserver against the real UnifiedSoundpackResolver
// to assert the end-to-end observer→buffer flow records every candidate
// the resolver walked.
func TestLookupBuffer_IntegratesWithSoundpackResolver(t *testing.T) {
	tempDir := t.TempDir()
	soundpackDir := filepath.Join(tempDir, "success")
	require.NoError(t, os.MkdirAll(soundpackDir, 0755))
	// Only the 2nd candidate will resolve.
	present := filepath.Join(soundpackDir, "present.wav")
	require.NoError(t, os.WriteFile(present, []byte("data"), 0644))

	mapper := soundpack.NewDirectoryMapper("test", []string{tempDir})
	resolver := soundpack.NewSoundpackResolver(mapper)

	buf := NewLookupBuffer()
	winner, err := resolver.ResolveSoundWithFallback(
		[]string{"success/missing.wav", "success/present.wav"},
		soundpack.WithObserver(buf.Observer()),
	)
	require.NoError(t, err)
	if winner != present {
		t.Errorf("winner=%q want %q", winner, present)
	}

	got := buf.Lookups()
	if len(got) != 2 {
		t.Fatalf("Lookups returned %d entries, want 2", len(got))
	}
	if got[0].Path != "success/missing.wav" || got[0].Found {
		t.Errorf("Lookups[0]=%+v, want {missing,false}", got[0])
	}
	if got[1].Path != "success/present.wav" || !got[1].Found {
		t.Errorf("Lookups[1]=%+v, want {present,true}", got[1])
	}
}

// TestLookupBuffer_ConcurrentObserverCallsRaceClean asserts the
// buffer is goroutine-safe under -race: two goroutines hammering the
// same Observer() closure with N appends each must produce 2N entries
// without a data race. This locks the contract documented on
// soundpack.PathObserver that observers SHOULD be safe to invoke
// concurrently, even though today's resolver fires them sequentially.
func TestLookupBuffer_ConcurrentObserverCallsRaceClean(t *testing.T) {
	buf := NewLookupBuffer()
	obs := buf.Observer()
	const n = 1000

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < n; i++ {
			obs(fmt.Sprintf("path-a-%d", i), i+1, i%2 == 0)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < n; i++ {
			obs(fmt.Sprintf("path-b-%d", i), i+1, true)
		}
	}()
	wg.Wait()

	got := buf.Lookups()
	if len(got) != 2*n {
		t.Errorf("expected %d lookups, got %d", 2*n, len(got))
	}
}

// TestLookupBuffer_LookupsReturnsCopy asserts that Lookups() returns a
// fresh slice independent of the buffer's backing array — mutating the
// returned slice MUST NOT affect future Lookups() reads, and a later
// observer callback that appends to the buffer MUST NOT show through
// into a previously-returned slice.
func TestLookupBuffer_LookupsReturnsCopy(t *testing.T) {
	buf := NewLookupBuffer()
	obs := buf.Observer()
	obs("path1", 1, true)

	first := buf.Lookups()
	if len(first) != 1 {
		t.Fatalf("first snapshot len=%d want 1", len(first))
	}

	// Mutating the returned slice must not affect the buffer.
	first[0].Path = "tampered"

	obs("path2", 2, false)
	second := buf.Lookups()
	if len(second) != 2 {
		t.Fatalf("second snapshot len=%d want 2", len(second))
	}
	if second[0].Path != "path1" {
		t.Errorf("buffer corrupted by caller mutation: second[0].Path=%q want %q",
			second[0].Path, "path1")
	}

	// The first snapshot must not have grown when the new observer
	// callback appended into the buffer.
	if len(first) != 1 {
		t.Errorf("previously-returned snapshot grew to %d entries", len(first))
	}
}
