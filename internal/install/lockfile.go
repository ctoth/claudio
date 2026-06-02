package install

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
)

// LockSettingsDir acquires an exclusive advisory lock co-located with
// settingsPath. The lock spans the caller's whole read-modify-write
// window — callers MUST defer Unlock immediately after a successful
// acquire and BEFORE any settings-mutation work.
//
// Uses TryLock with bounded retry: 5 attempts, 200ms between attempts.
// Returns a non-nil error if the lock cannot be acquired in ~1s, with
// an actionable message naming the lock path so users can investigate
// or remove a stale lock file.
//
// Implementation uses gofrs/flock: LockFileEx on Windows, flock(2) on
// POSIX. Both are genuine OS advisory locks — cross-process, not just
// cross-goroutine. Stale locks released automatically by the OS when
// the holding process dies.
func LockSettingsDir(settingsPath string) (*flock.Flock, error) {
	lockPath := filepath.Join(filepath.Dir(settingsPath), ".claudio.lock")
	lock := flock.New(lockPath)

	const attempts = 5
	const backoff = 200 * time.Millisecond
	for i := 0; i < attempts; i++ {
		ok, err := lock.TryLock()
		if err != nil {
			return nil, fmt.Errorf("failed to try-lock %s: %w", lockPath, err)
		}
		if ok {
			slog.Debug("acquired settings lock", "path", lockPath)
			return lock, nil
		}
		if i < attempts-1 {
			time.Sleep(backoff)
		}
	}
	return nil, fmt.Errorf("another claudio install/uninstall appears to be running; "+
		"if this is stale, remove %s manually", lockPath)
}
