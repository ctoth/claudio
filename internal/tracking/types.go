package tracking

import (
	"os"

	"claudio.click/internal/hooks"
	"claudio.click/internal/soundpack"
)

// SoundpackResolver interface for resolving logical paths to physical paths.
//
// DEPRECATED: structural duplicate of soundpack.SoundpackResolver. Scheduled
// for deletion in the next commit; the signature is being kept in lockstep
// with soundpack.SoundpackResolver here so the duplicate compiles against
// the shared real implementation during the migration.
type SoundpackResolver interface {
	ResolveSound(relativePath string) (string, error)
	ResolveSoundWithFallback(paths []string, opts ...soundpack.ResolveOption) (string, error)
	GetName() string
	GetType() string
}

// SoundChecker resolves logical chain paths to physical paths and reports
// per-path existence.
//
// DEPRECATED: scheduled for deletion in the next commit. Once
// sounds.SoundMapper switches to calling soundpack.ResolveSoundWithFallback
// directly with a WithObserver-wired LookupBuffer, this type and its mirror
// SoundpackResolver interface go away — the same os.Stat loop happens once
// inside soundpack, observed (not duplicated) by tracking.
type SoundChecker struct {
	resolver SoundpackResolver
}

// NewSoundCheckerWithResolver creates a new SoundChecker with soundpack
// resolver. The streaming hook ecosystem (PathCheckedHook, SlogHook, NopHook,
// WithHook) was deleted in favor of soundpack.WithObserver + LookupBuffer;
// the resolver-less constructor (NewSoundChecker) and the hook-attaching
// option went with it.
func NewSoundCheckerWithResolver(resolver SoundpackResolver) *SoundChecker {
	return &SoundChecker{
		resolver: resolver,
	}
}

// CheckPaths checks existence of multiple paths.
//
// chainType and context are retained on the signature for the brief
// transition window while sounds.SoundMapper still drives this path; after
// the next commit removes the caller, this method goes away with the rest
// of SoundChecker.
func (sc *SoundChecker) CheckPaths(context *hooks.EventContext, chainType string, paths []string) []bool {
	_ = context
	_ = chainType
	results := make([]bool, len(paths))
	for i, path := range paths {
		results[i] = sc.fileExists(path)
	}
	return results
}

// fileExists checks if a path exists, using resolver if available
func (sc *SoundChecker) fileExists(path string) bool {
	if sc.resolver != nil {
		// Use resolver to convert logical path to physical path
		resolved, err := sc.resolver.ResolveSound(path)
		if err != nil {
			return false // Path doesn't resolve to existing file
		}
		// Check if resolved physical path exists
		_, err = os.Stat(resolved)
		return err == nil
	}

	// Fallback to direct path checking (backward compatibility)
	_, err := os.Stat(path)
	return err == nil
}

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
