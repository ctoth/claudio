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

// PathCheckedHook is called when a sound path is checked for existence.
//
// sequence is the 1-based position of `path` within the chain CheckPaths
// was given. chainType identifies WHICH chain this lookup ran under
// (e.g. "enhanced", "posttool", "simple"). Sequence is only comparable
// within a single chainType — see the v2 schema migration notes / review
// finding #20 for the long story.
type PathCheckedHook func(path string, exists bool, sequence int, chainType string, context *hooks.EventContext)

// SoundChecker manages sound path checking with optional hooks
type SoundChecker struct {
	hooks    []PathCheckedHook
	resolver SoundpackResolver
}

// SoundCheckerOption is a functional option for configuring SoundChecker
type SoundCheckerOption func(*SoundChecker)

// NewSoundChecker creates a new SoundChecker with optional hooks
func NewSoundChecker(opts ...SoundCheckerOption) *SoundChecker {
	sc := &SoundChecker{
		hooks: make([]PathCheckedHook, 0),
	}

	for _, opt := range opts {
		opt(sc)
	}

	return sc
}

// NewSoundCheckerWithResolver creates a new SoundChecker with soundpack resolver
func NewSoundCheckerWithResolver(resolver SoundpackResolver, opts ...SoundCheckerOption) *SoundChecker {
	sc := &SoundChecker{
		hooks:    make([]PathCheckedHook, 0),
		resolver: resolver,
	}

	for _, opt := range opts {
		opt(sc)
	}

	return sc
}

// WithHook adds a hook to be called when paths are checked
func WithHook(hook PathCheckedHook) SoundCheckerOption {
	return func(sc *SoundChecker) {
		sc.hooks = append(sc.hooks, hook)
	}
}

// CheckPaths checks existence of multiple paths and calls all hooks with
// 1-based sequence numbering. chainType identifies the chain these paths
// came from so hooks can record the chain-scoped meaning of sequence
// instead of conflating positions across chain shapes.
func (sc *SoundChecker) CheckPaths(context *hooks.EventContext, chainType string, paths []string) []bool {
	results := make([]bool, len(paths))

	for i, path := range paths {
		exists := sc.fileExists(path)
		results[i] = exists

		// Call all hooks with 1-based sequence numbering
		sequence := i + 1
		for _, hook := range sc.hooks {
			hook(path, exists, sequence, chainType, context)
		}
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
