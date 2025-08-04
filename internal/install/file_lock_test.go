package install

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestFileLockBasicOperations verifies basic file locking functionality
// This is a RED test - will fail until we implement the functionality
func TestFileLockBasicOperations(t *testing.T) {
	tempDir := t.TempDir()
	lockFile := filepath.Join(tempDir, "test.lock")

	// This test expects a FileLock interface that wraps github.com/gofrs/flock
	// with proper error handling and logging

	fileLock := NewFileLock(lockFile)

	// Test acquiring lock
	err := fileLock.Lock()
	if err != nil {
		t.Fatalf("Failed to acquire lock: %v", err)
	}

	// Verify lock file exists
	if _, err := os.Stat(lockFile); os.IsNotExist(err) {
		t.Error("Lock file should exist after acquiring lock")
	}

	// Test unlocking
	err = fileLock.Unlock()
	if err != nil {
		t.Fatalf("Failed to release lock: %v", err)
	}

	t.Logf("Basic lock/unlock operations completed successfully")
}

// TestFileLockTryLock tests non-blocking lock attempts
func TestFileLockTryLock(t *testing.T) {
	tempDir := t.TempDir()
	lockFile := filepath.Join(tempDir, "try-test.lock")

	fileLock1 := NewFileLock(lockFile)
	fileLock2 := NewFileLock(lockFile)

	// First lock should succeed
	success, err := fileLock1.TryLock()
	if err != nil {
		t.Fatalf("Unexpected error on first TryLock: %v", err)
	}
	if !success {
		t.Error("First TryLock should succeed")
	}

	// Second lock should fail (non-blocking)
	success, err = fileLock2.TryLock()
	if err != nil {
		t.Fatalf("Unexpected error on second TryLock: %v", err)
	}
	if success {
		t.Error("Second TryLock should fail when file is already locked")
	}

	// Release first lock
	err = fileLock1.Unlock()
	if err != nil {
		t.Fatalf("Failed to release first lock: %v", err)
	}

	// Now second lock should succeed
	success, err = fileLock2.TryLock()
	if err != nil {
		t.Fatalf("Unexpected error on third TryLock: %v", err)
	}
	if !success {
		t.Error("TryLock should succeed after first lock is released")
	}

	// Clean up
	err = fileLock2.Unlock()
	if err != nil {
		t.Fatalf("Failed to release second lock: %v", err)
	}
}

// TestFileLockConcurrentAccess tests concurrent access protection
func TestFileLockConcurrentAccess(t *testing.T) {
	tempDir := t.TempDir()
	lockFile := filepath.Join(tempDir, "concurrent-test.lock")

	results := make(chan bool, 2)

	// Start two goroutines trying to acquire the same lock
	go func() {
		fileLock := NewFileLock(lockFile)
		success, err := fileLock.TryLock()
		if err != nil {
			t.Errorf("Goroutine 1 error: %v", err)
			results <- false
			return
		}

		if success {
			// Hold lock briefly
			time.Sleep(100 * time.Millisecond)
			fileLock.Unlock()
		}

		results <- success
	}()

	go func() {
		// Give first goroutine a slight head start
		time.Sleep(10 * time.Millisecond)

		fileLock := NewFileLock(lockFile)
		success, err := fileLock.TryLock()
		if err != nil {
			t.Errorf("Goroutine 2 error: %v", err)
			results <- false
			return
		}

		if success {
			fileLock.Unlock()
		}

		results <- success
	}()

	// Collect results
	result1 := <-results
	result2 := <-results

	// Exactly one should succeed (mutual exclusion)
	if result1 == result2 {
		t.Error("Concurrent access should be mutually exclusive - exactly one should succeed")
	}

	t.Logf("Concurrent access test: result1=%v, result2=%v", result1, result2)
}

// TestFileLockErrorHandling tests error conditions
func TestFileLockErrorHandling(t *testing.T) {
	// Test with invalid path (non-existent directory)
	invalidPath := "/nonexistent/directory/invalid.lock"
	fileLock := NewFileLock(invalidPath)

	// Should handle path errors gracefully
	err := fileLock.Lock()
	if err == nil {
		t.Error("Expected error when trying to lock file in non-existent directory")
		fileLock.Unlock() // Clean up if it somehow succeeded
	} else {
		t.Logf("Got expected error: %v", err)
	}

	// Test TryLock with invalid path
	success, err := fileLock.TryLock()
	if err == nil && success {
		t.Error("Expected error when trying to try-lock file in non-existent directory")
		fileLock.Unlock() // Clean up if it somehow succeeded
	} else {
		t.Logf("TryLock correctly handled invalid path: success=%v, err=%v", success, err)
	}

	t.Logf("Error handling test completed")
}

// TestFileLockCleanup tests proper cleanup of lock files
func TestFileLockCleanup(t *testing.T) {
	tempDir := t.TempDir()
	lockFile := filepath.Join(tempDir, "cleanup-test.lock")

	fileLock := NewFileLock(lockFile)

	// Acquire lock
	err := fileLock.Lock()
	if err != nil {
		t.Fatalf("Failed to acquire lock: %v", err)
	}

	// Verify lock file exists
	if _, err := os.Stat(lockFile); os.IsNotExist(err) {
		t.Error("Lock file should exist after acquiring lock")
	}

	// Release lock
	err = fileLock.Unlock()
	if err != nil {
		t.Fatalf("Failed to release lock: %v", err)
	}

	// Note: flock doesn't automatically remove lock files, so we don't test for file removal
	// The important thing is that the lock is released so other processes can acquire it
	t.Logf("Lock cleanup test completed")
}
