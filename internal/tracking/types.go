package tracking

import (
	"os"

	"github.com/ctoth/claudio/internal/hooks"
)

// PathCheckedHook is called when a sound path is checked for existence
// sequence is 1-based (not 0-based) to match fallback level numbering
type PathCheckedHook func(path string, exists bool, sequence int, context *hooks.EventContext)

// SoundChecker manages sound path checking with optional hooks
type SoundChecker struct {
	hooks []PathCheckedHook
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
		exists := fileExists(path)
		results[i] = exists
		
		// Call all hooks with 1-based sequence numbering
		sequence := i + 1
		for _, hook := range sc.hooks {
			hook(path, exists, sequence, context)
		}
	}
	
	return results
}

// fileExists is a placeholder that returns false for now
// This will be replaced with actual file existence checking later
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}