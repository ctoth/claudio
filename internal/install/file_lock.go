package install

import (
	"log/slog"

	"github.com/gofrs/flock"
)

// FileLockInterface defines the interface for file locking operations
// This allows for mocking in tests and dependency injection
type FileLockInterface interface {
	Lock() error
	TryLock() (bool, error)
	Unlock() error
}

// FileLock wraps github.com/gofrs/flock with additional logging and error handling
// This provides a clean interface for file-based locking during installation operations
type FileLock struct {
	filePath string
	flock    *flock.Flock
}

// NewFileLock creates a new file lock for the specified path
func NewFileLock(filePath string) FileLockInterface {
	slog.Debug("creating new file lock", "file_path", filePath)

	return &FileLock{
		filePath: filePath,
		flock:    flock.New(filePath),
	}
}

// Lock acquires an exclusive lock on the file (blocking)
func (fl *FileLock) Lock() error {
	slog.Debug("attempting to acquire file lock", "file_path", fl.filePath)

	err := fl.flock.Lock()
	if err != nil {
		slog.Error("failed to acquire file lock",
			"file_path", fl.filePath,
			"error", err)
		return err
	}

	slog.Info("file lock acquired successfully", "file_path", fl.filePath)
	return nil
}

// TryLock attempts to acquire an exclusive lock on the file (non-blocking)
// Returns true if lock was acquired, false if file is already locked
func (fl *FileLock) TryLock() (bool, error) {
	slog.Debug("attempting to try-lock file", "file_path", fl.filePath)

	success, err := fl.flock.TryLock()
	if err != nil {
		slog.Error("error during try-lock attempt",
			"file_path", fl.filePath,
			"error", err)
		return false, err
	}

	if success {
		slog.Info("file lock acquired via try-lock", "file_path", fl.filePath)
	} else {
		slog.Debug("try-lock failed - file already locked", "file_path", fl.filePath)
	}

	return success, nil
}

// Unlock releases the file lock
func (fl *FileLock) Unlock() error {
	slog.Debug("attempting to release file lock", "file_path", fl.filePath)

	err := fl.flock.Unlock()
	if err != nil {
		slog.Error("failed to release file lock",
			"file_path", fl.filePath,
			"error", err)
		return err
	}

	slog.Info("file lock released successfully", "file_path", fl.filePath)
	return nil
}
