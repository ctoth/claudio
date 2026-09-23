package safeio

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLockFile_AcquiresAndReleases(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.lock")

	lock, err := LockFile(path)
	if err != nil {
		t.Fatalf("LockFile: %v", err)
	}
	if !lock.Locked() {
		t.Fatal("expected Locked()==true after acquire")
	}
	if err := lock.Unlock(); err != nil {
		t.Fatalf("Unlock: %v", err)
	}

	again, err := LockFile(path)
	if err != nil {
		t.Fatalf("re-acquire after unlock: %v", err)
	}
	_ = again.Unlock()
}

func TestLockFile_HeldLockTimesOutWithErrLockHeld(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.lock")
	first, err := LockFile(path)
	if err != nil {
		t.Fatalf("first LockFile: %v", err)
	}
	defer func() { _ = first.Unlock() }()

	start := time.Now()
	_, err = LockFile(path)
	elapsed := time.Since(start)

	if !errors.Is(err, ErrLockHeld) {
		t.Fatalf("expected ErrLockHeld, got %v", err)
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("error should name the lock path, got %q", err.Error())
	}
	if elapsed < LockTimeout-LockRetryDelay {
		t.Errorf("gave up too early: %v (timeout %v)", elapsed, LockTimeout)
	}
	if elapsed > LockTimeout+time.Second {
		t.Errorf("took too long: %v (timeout %v)", elapsed, LockTimeout)
	}
}

func TestLockFile_MissingDirectoryIsNotReportedAsHeld(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "x.lock")

	lock, err := LockFile(path)
	if err == nil {
		_ = lock.Unlock()
		t.Fatal("expected error for a lock in a missing directory")
	}
	if errors.Is(err, ErrLockHeld) {
		t.Errorf("filesystem error misreported as ErrLockHeld: %v", err)
	}
}

func TestLockFile_TimingConstants(t *testing.T) {
	if LockTimeout != time.Second {
		t.Errorf("LockTimeout = %v, want 1s", LockTimeout)
	}
	if LockRetryDelay != 200*time.Millisecond {
		t.Errorf("LockRetryDelay = %v, want 200ms", LockRetryDelay)
	}
}
