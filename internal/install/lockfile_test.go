package install

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLockSettingsDir_Acquires(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")

	lock, err := LockSettingsDir(settingsPath)
	if err != nil {
		t.Fatalf("LockSettingsDir failed: %v", err)
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

func TestLockSettingsDir_FailsWhenHeld(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")

	first, err := LockSettingsDir(settingsPath)
	if err != nil {
		t.Fatalf("first LockSettingsDir failed: %v", err)
	}
	defer first.Unlock()

	start := time.Now()
	_, err = LockSettingsDir(settingsPath)
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
