package safeio

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/gofrs/flock"
)

const (
	// LockTimeout bounds how long LockFile waits for a held lock.
	LockTimeout = time.Second
	// LockRetryDelay is the pause between LockFile's try-lock attempts.
	LockRetryDelay = 200 * time.Millisecond
)

// ErrLockHeld reports that another process kept the lock for the whole
// LockTimeout window. Callers match it with errors.Is to print their own
// "another claudio ... is running" message.
var ErrLockHeld = errors.New("lock is held by another process")

// LockFile acquires an exclusive advisory lock on lockPath, retrying
// every LockRetryDelay for up to LockTimeout. The lock's directory must
// already exist.
//
// Uses gofrs/flock: LockFileEx on Windows, flock(2) on POSIX. Both are
// genuine cross-process OS locks, released automatically by the OS when
// the holding process dies. Callers MUST defer Unlock immediately after a
// successful acquire.
//
// A lock still held at the deadline returns an error wrapping
// ErrLockHeld and naming lockPath; any other failure (for example a
// missing directory) is returned wrapped, without ErrLockHeld.
func LockFile(lockPath string) (*flock.Flock, error) {
	ctx, cancel := context.WithTimeout(context.Background(), LockTimeout)
	defer cancel()

	lock := flock.New(lockPath)
	locked, err := lock.TryLockContext(ctx, LockRetryDelay)
	if locked {
		slog.Debug("acquired file lock", "path", lockPath)
		return lock, nil
	}
	if err == nil || errors.Is(err, context.DeadlineExceeded) {
		slog.Debug("file lock still held at deadline", "path", lockPath, "timeout", LockTimeout)
		return nil, fmt.Errorf("%w: %s", ErrLockHeld, lockPath)
	}
	return nil, fmt.Errorf("failed to try-lock %s: %w", lockPath, err)
}
