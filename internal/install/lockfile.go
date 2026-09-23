package install

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/gofrs/flock"

	"claudio.click/internal/safeio"
)

// LockSettingsDir acquires an exclusive advisory lock co-located with
// settingsPath. The lock spans the caller's whole read-modify-write
// window — callers MUST defer Unlock immediately after a successful
// acquire and BEFORE any settings-mutation work.
//
// Waits up to safeio.LockTimeout (see safeio.LockFile). Returns a
// non-nil error if the lock cannot be acquired, with an actionable
// message naming the lock path so users can investigate or remove a
// stale lock file.
func LockSettingsDir(settingsPath string) (*flock.Flock, error) {
	lockPath := filepath.Join(filepath.Dir(settingsPath), ".claudio.lock")
	lock, err := safeio.LockFile(lockPath)
	if errors.Is(err, safeio.ErrLockHeld) {
		return nil, fmt.Errorf("another claudio install/uninstall appears to be running; "+
			"if this is stale, remove %s manually", lockPath)
	}
	return lock, err
}
