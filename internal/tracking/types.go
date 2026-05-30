package tracking

import (
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
type LookupBuffer struct {
	lookups []Lookup
}

// NewLookupBuffer returns a fresh LookupBuffer with no recorded lookups.
func NewLookupBuffer() *LookupBuffer {
	return &LookupBuffer{}
}

// Observer returns a soundpack.PathObserver closure that appends one Lookup
// to the buffer per callback. The closure preserves the resolver's 1-based
// sequence and on-disk existence flag.
func (b *LookupBuffer) Observer() soundpack.PathObserver {
	return func(path string, sequence int, exists bool) {
		b.lookups = append(b.lookups, Lookup{
			Path:     path,
			Found:    exists,
			Sequence: sequence,
		})
	}
}

// Lookups returns the slice of recorded Lookup entries in the order they
// were observed. Safe to call after resolution completes; the slice
// reference is stable until the next Observer() callback fires.
func (b *LookupBuffer) Lookups() []Lookup {
	return b.lookups
}
