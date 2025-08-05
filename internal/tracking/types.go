package tracking

import (
	"os"

	"github.com/ctoth/claudio/internal/hooks"
)

// SoundpackResolver interface for resolving logical paths to physical paths
type SoundpackResolver interface {
	ResolveSound(relativePath string) (string, error)
	ResolveSoundWithFallback(paths []string) (string, error)
	GetName() string
	GetType() string
}

// PathCheckedHook is called when a sound path is checked for existence
// sequence is 1-based (not 0-based) to match fallback level numbering
type PathCheckedHook func(path string, exists bool, sequence int, context *hooks.EventContext)

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

// CheckPaths checks existence of multiple paths and calls all hooks with 1-based sequence numbering
func (sc *SoundChecker) CheckPaths(context *hooks.EventContext, paths []string) []bool {
	results := make([]bool, len(paths))
	
	for i, path := range paths {
		exists := sc.fileExists(path)
		results[i] = exists
		
		// Call all hooks with 1-based sequence numbering
		sequence := i + 1
		for _, hook := range sc.hooks {
			hook(path, exists, sequence, context)
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