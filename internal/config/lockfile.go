package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gofrs/flock"

	"claudio.click/internal/safeio"
)

// LockConfigDir acquires an exclusive advisory lock co-located with
// configPath. The lock spans the caller's whole read-modify-write
// window — callers MUST defer Unlock immediately after a successful
// acquire and BEFORE any config-mutation work.
//
// Waits up to safeio.LockTimeout (see safeio.LockFile). Returns a
// non-nil error if the lock cannot be acquired, with an actionable
// message naming the lock path so users can investigate or remove a
// stale lock file.
//
// Mirrors internal/install/lockfile.go's LockSettingsDir. The lock
// filename is ".claudio.lock" — the same basename as the settings
// lock, but located in a different directory (typically
// $XDG_CONFIG_HOME/claudio/ vs. ~/.claude/), so there is no
// cross-process collision between a concurrent `claudio install` and
// `claudio volume 0.5`.
func LockConfigDir(configPath string) (*flock.Flock, error) {
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory for lock: %w", err)
	}
	lockPath := filepath.Join(filepath.Dir(configPath), ".claudio.lock")
	lock, err := safeio.LockFile(lockPath)
	if errors.Is(err, safeio.ErrLockHeld) {
		return nil, fmt.Errorf("another claudio config write appears to be running; "+
			"if this is stale, remove %s manually", lockPath)
	}
	return lock, err
}
