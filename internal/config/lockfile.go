package config

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
)

// LockConfigDir acquires an exclusive advisory lock co-located with
// configPath. The lock spans the caller's whole read-modify-write
// window — callers MUST defer Unlock immediately after a successful
// acquire and BEFORE any config-mutation work.
//
// Uses TryLock with bounded retry: 5 attempts, 200ms between attempts.
// Returns a non-nil error if the lock cannot be acquired in ~1s, with
// an actionable message naming the lock path so users can investigate
// or remove a stale lock file.
//
// Mirrors internal/install/lockfile.go's LockSettingsDir. The lock
// filename is ".claudio.lock" — the same basename as the settings
// lock, but located in a different directory (typically
// $XDG_CONFIG_HOME/claudio/ vs. ~/.claude/), so there is no
// cross-process collision between a concurrent `claudio install` and
// `claudio volume 0.5`.
func LockConfigDir(configPath string) (*flock.Flock, error) {
	lockPath := filepath.Join(filepath.Dir(configPath), ".claudio.lock")
	lock := flock.New(lockPath)

	const attempts = 5
	const backoff = 200 * time.Millisecond
	for i := 0; i < attempts; i++ {
		ok, err := lock.TryLock()
		if err != nil {
			return nil, fmt.Errorf("failed to try-lock %s: %w", lockPath, err)
		}
		if ok {
			slog.Debug("acquired config lock", "path", lockPath)
			return lock, nil
		}
		if i < attempts-1 {
			time.Sleep(backoff)
		}
	}
	return nil, fmt.Errorf("another claudio config write appears to be running; "+
		"if this is stale, remove %s manually", lockPath)
}
