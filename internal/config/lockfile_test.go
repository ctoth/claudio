package config

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLockConfigDir_Acquires(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")

	lock, err := LockConfigDir(configPath)
	if err != nil {
		t.Fatalf("LockConfigDir failed: %v", err)
	}
	defer func() {
		if err := lock.Unlock(); err != nil {
			t.Errorf("Unlock failed: %v", err)
		}
	}()

	if !lock.Locked() {
		t.Error("expected Locked()==true after successful acquire")
	}
}

func TestLockConfigDir_FailsWhenHeld(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")

	first, err := LockConfigDir(configPath)
	if err != nil {
		t.Fatalf("first LockConfigDir failed: %v", err)
	}
	defer first.Unlock()

	start := time.Now()
	_, err = LockConfigDir(configPath)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error when lock is held")
	}
	// 5 attempts * 200ms backoff = ~800ms (with 4 sleeps between).
	if elapsed < 800*time.Millisecond {
		t.Errorf("retry loop too short: %v (expected >= 800ms)", elapsed)
	}
	if !strings.Contains(err.Error(), ".claudio.lock") {
		t.Errorf("error should name the lock path, got: %q", err.Error())
	}
	if !strings.Contains(err.Error(), "another claudio") {
		t.Errorf("error should mention 'another claudio', got: %q", err.Error())
	}
}

// TestLockConfigDir_DifferentDirsDoNotCollide verifies the config lock
// in one directory does not block a settings lock in a different
// directory. The two locks share the same basename but different
// dirs — independent advisory locks. (Smoke test for the
// recommended placement in scout report §E.)
func TestLockConfigDir_DifferentDirsDoNotCollide(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	lockA, err := LockConfigDir(filepath.Join(dirA, "config.json"))
	if err != nil {
		t.Fatalf("lockA failed: %v", err)
	}
	defer lockA.Unlock()

	lockB, err := LockConfigDir(filepath.Join(dirB, "config.json"))
	if err != nil {
		t.Fatalf("lockB failed despite different directory: %v", err)
	}
	defer lockB.Unlock()
}
