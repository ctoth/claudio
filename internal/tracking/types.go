package tracking

import (
	"sync"

	"claudio.click/internal/soundpack"
)

// LookupBuffer is a single-use adapter that converts soundpack.PathObserver
// callbacks into the buffered Lookup slice EventRecorder expects. Construct
// one per MapSound invocation, pass Observer() to soundpack.WithObserver,
// then read Lookups() after resolution completes to feed RecordEvent.
//
// LookupBuffer is the bridge that lets tracking observe soundpack's
// resolution loop without owning the os.Stat I/O. Single-use: do not reuse
// across resolutions — spawn a fresh buffer per event.
//
// LookupBuffer is goroutine-safe: an internal sync.Mutex guards the append
// in Observer() and the read in Lookups(). This holds the
// soundpack.PathObserver contract that observers SHOULD be safe to call
// from multiple goroutines even though today's UnifiedSoundpackResolver
// fires them sequentially. The mutex is also why Lookups() returns a copy
// rather than the live slice — callers must not race with a still-firing
// observer over the buffer's backing array.
type LookupBuffer struct {
	mu      sync.Mutex
	lookups []Lookup
}

// NewLookupBuffer returns a fresh LookupBuffer with no recorded lookups.
func NewLookupBuffer() *LookupBuffer {
	return &LookupBuffer{}
}

// Observer returns a soundpack.PathObserver closure that appends one Lookup
// to the buffer per callback. The closure preserves the resolver's 1-based
// sequence and on-disk existence flag. The append is guarded by the
// buffer's mutex so the observer is safe to invoke concurrently from
// multiple goroutines.
func (b *LookupBuffer) Observer() soundpack.PathObserver {
	return func(path string, sequence int, exists bool) {
		b.mu.Lock()
		b.lookups = append(b.lookups, Lookup{
			Path:     path,
			Found:    exists,
			Sequence: sequence,
		})
		b.mu.Unlock()
	}
}

// Lookups returns a copy of the recorded Lookup entries in the order they
// were observed. Returning a copy guarantees callers cannot mutate the
// buffer's backing slice and cannot race with a still-firing observer
// reallocating it on append.
func (b *LookupBuffer) Lookups() []Lookup {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]Lookup, len(b.lookups))
	copy(out, b.lookups)
	return out
}
