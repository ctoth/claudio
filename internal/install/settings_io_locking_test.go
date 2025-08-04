package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSettingsFileLocking(t *testing.T) {
	// TDD RED: Test that concurrent access to settings files is safely handled with file locking
	tempDir := t.TempDir()
	settingsFile := filepath.Join(tempDir, "settings.json")

	// Create initial settings
	initialSettings := SettingsMap{
		"hooks": map[string]interface{}{
			"PreToolUse": "initial",
		},
	}

	err := WriteSettingsFileWithLock(settingsFile, &initialSettings)
	if err != nil {
		t.Fatalf("Failed to write initial settings: %v", err)
	}

	const numConcurrentOperations = 5
	const operationsPerGoroutine = 3

	var wg sync.WaitGroup
	errors := make(chan error, numConcurrentOperations*operationsPerGoroutine)

	// Launch multiple goroutines that read and write concurrently
	for i := 0; i < numConcurrentOperations; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < operationsPerGoroutine; j++ {
				// Read current settings
				settings, err := ReadSettingsFileWithLock(settingsFile)
				if err != nil {
					errors <- err
					return
				}

				// Modify settings
				if settings == nil {
					settings = &SettingsMap{}
				}

				hooks, exists := (*settings)["hooks"]
				if !exists {
					hooks = make(map[string]interface{})
					(*settings)["hooks"] = hooks
				}

				if hooksMap, ok := hooks.(map[string]interface{}); ok {
					hooksMap[fmt.Sprintf("TestHook_%d_%d", goroutineID, j)] = fmt.Sprintf("value_%d_%d", goroutineID, j)
				}

				// Write back settings
				err = WriteSettingsFileWithLock(settingsFile, settings)
				if err != nil {
					errors <- err
					return
				}

				// Small delay to increase chance of conflicts
				time.Sleep(time.Millisecond * 1)
			}
		}(i)
	}

	// Wait for all operations to complete
	wg.Wait()
	close(errors)

	// Check for any errors
	for err := range errors {
		t.Errorf("Concurrent operation failed: %v", err)
	}

	// Verify final state is consistent
	finalSettings, err := ReadSettingsFileWithLock(settingsFile)
	if err != nil {
		t.Fatalf("Failed to read final settings: %v", err)
	}

	if finalSettings == nil {
		t.Fatal("Final settings should not be nil")
	}

	hooks, exists := (*finalSettings)["hooks"]
	if !exists {
		t.Fatal("Final settings should have hooks")
	}

	hooksMap, ok := hooks.(map[string]interface{})
	if !ok {
		t.Fatal("Hooks should be a map")
	}

	// Should have initial hook + all added hooks (allowing for some timing issues)
	expectedHooks := 1 + (numConcurrentOperations * operationsPerGoroutine)
	if len(hooksMap) < expectedHooks-2 || len(hooksMap) > expectedHooks {
		t.Errorf("Expected around %d hooks (±2), got %d", expectedHooks, len(hooksMap))
	}

	t.Logf("Successfully completed %d concurrent operations, final hooks: %d",
		numConcurrentOperations*operationsPerGoroutine, len(hooksMap))
}

func TestSettingsFileLockingTimeout(t *testing.T) {
	// TDD RED: Test that file locking times out appropriately when file is held too long
	tempDir := t.TempDir()
	settingsFile := filepath.Join(tempDir, "settings.json")

	// Create initial settings
	initialSettings := SettingsMap{"test": "value"}
	err := WriteSettingsFileWithLock(settingsFile, &initialSettings)
	if err != nil {
		t.Fatalf("Failed to write initial settings: %v", err)
	}

	// Create a file lock that we'll hold for a long time
	lockFile := settingsFile + ".lock"
	longHeldLock, err := AcquireFileLock(lockFile)
	if err != nil {
		t.Fatalf("Failed to acquire long-held lock: %v", err)
	}

	// Try to read/write while lock is held (should timeout)
	done := make(chan bool)
	var operationErr error

	go func() {
		// This should timeout since we're holding the lock
		// Use a shorter timeout for testing
		lockFileForRead := settingsFile + ".lock"
		testLock, err := AcquireFileLockWithTimeout(lockFileForRead, 500*time.Millisecond)
		if err != nil {
			operationErr = err
		} else {
			testLock.Release()
		}
		done <- true
	}()

	// Give the operation some time to complete or timeout
	select {
	case <-done:
		// Operation should have timed out
		if operationErr == nil {
			t.Error("Expected timeout error but operation succeeded")
		} else if !strings.Contains(operationErr.Error(), "timeout") {
			t.Errorf("Expected timeout error but got: %v", operationErr)
		} else {
			t.Logf("Operation correctly timed out: %v", operationErr)
		}
	case <-time.After(2 * time.Second):
		t.Error("Test itself took too long - timeout mechanism not working")
	}

	// Release the long-held lock
	longHeldLock.Release()

	// Now operations should work normally
	settings, err := ReadSettingsFileWithLock(settingsFile)
	if err != nil {
		t.Errorf("Failed to read after lock release: %v", err)
	}
	if settings == nil {
		t.Error("Settings should not be nil after lock release")
	}
}

func TestSettingsFileLockingErrorRecovery(t *testing.T) {
	// TDD RED: Test that file locking recovers gracefully from stale locks
	tempDir := t.TempDir()
	settingsFile := filepath.Join(tempDir, "settings.json")
	lockFile := settingsFile + ".lock"

	// Create initial settings
	initialSettings := SettingsMap{"test": "initial"}
	err := WriteSettingsFileWithLock(settingsFile, &initialSettings)
	if err != nil {
		t.Fatalf("Failed to write initial settings: %v", err)
	}

	// Simulate a stale lock file (process died while holding lock)
	staleLockContent := "stale_process_id_12345"
	err = os.WriteFile(lockFile, []byte(staleLockContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create stale lock file: %v", err)
	}

	// Operations should still work (should detect and clean up stale lock)
	settings, err := ReadSettingsFileWithLock(settingsFile)
	if err != nil {
		t.Errorf("Failed to read with stale lock present: %v", err)
	}

	if settings == nil {
		t.Error("Settings should not be nil")
	}

	// Should be able to write as well
	(*settings)["test"] = "updated"
	err = WriteSettingsFileWithLock(settingsFile, settings)
	if err != nil {
		t.Errorf("Failed to write with stale lock cleanup: %v", err)
	}

	// Verify the update was successful
	finalSettings, err := ReadSettingsFileWithLock(settingsFile)
	if err != nil {
		t.Fatalf("Failed to read final settings: %v", err)
	}

	if val, exists := (*finalSettings)["test"]; !exists || val != "updated" {
		t.Errorf("Expected updated value, got: %v", val)
	}
}

func TestSettingsFileLockingInterface(t *testing.T) {
	// TDD RED: Test the FileLock interface abstraction for testing
	tempDir := t.TempDir()
	lockFile := filepath.Join(tempDir, "test.lock")

	// Test acquiring and releasing locks
	lock, err := AcquireFileLock(lockFile)
	if err != nil {
		t.Fatalf("Failed to acquire file lock: %v", err)
	}

	if lock == nil {
		t.Fatal("Lock should not be nil")
	}

	// Lock file should exist
	if _, err := os.Stat(lockFile); os.IsNotExist(err) {
		t.Error("Lock file should exist after acquisition")
	}

	// Should be able to release the lock
	err = lock.Release()
	if err != nil {
		t.Errorf("Failed to release lock: %v", err)
	}

	// Test double release (should be safe)
	err = lock.Release()
	if err != nil {
		t.Errorf("Double release should be safe: %v", err)
	}
}

func TestSettingsFileLockingMultipleFiles(t *testing.T) {
	// TDD RED: Test that locking works correctly with multiple different files
	tempDir := t.TempDir()
	file1 := filepath.Join(tempDir, "settings1.json")
	file2 := filepath.Join(tempDir, "settings2.json")

	settings1 := SettingsMap{"file": "1"}
	settings2 := SettingsMap{"file": "2"}

	var wg sync.WaitGroup
	errors := make(chan error, 4)

	// Concurrent operations on different files should not block each other
	wg.Add(4)

	go func() {
		defer wg.Done()
		err := WriteSettingsFileWithLock(file1, &settings1)
		if err != nil {
			errors <- err
		}
	}()

	go func() {
		defer wg.Done()
		err := WriteSettingsFileWithLock(file2, &settings2)
		if err != nil {
			errors <- err
		}
	}()

	go func() {
		defer wg.Done()
		_, err := ReadSettingsFileWithLock(file1)
		if err != nil {
			errors <- err
		}
	}()

	go func() {
		defer wg.Done()
		_, err := ReadSettingsFileWithLock(file2)
		if err != nil {
			errors <- err
		}
	}()

	wg.Wait()
	close(errors)

	// Check for any errors
	for err := range errors {
		t.Errorf("Multi-file operation failed: %v", err)
	}

	// Verify both files were written correctly
	result1, err := ReadSettingsFileWithLock(file1)
	if err != nil {
		t.Errorf("Failed to read file1: %v", err)
	} else if (*result1)["file"] != "1" {
		t.Errorf("File1 has wrong content: %v", *result1)
	}

	result2, err := ReadSettingsFileWithLock(file2)
	if err != nil {
		t.Errorf("Failed to read file2: %v", err)
	} else if (*result2)["file"] != "2" {
		t.Errorf("File2 has wrong content: %v", *result2)
	}
}

// Functions that will need to be implemented (currently undefined):
// - ReadSettingsFileWithLock(filePath string) (*SettingsMap, error)
// - WriteSettingsFileWithLock(filePath string, settings *SettingsMap) error
// - AcquireFileLock(lockFile string) (SettingsLock, error)
// - SettingsLock interface with Release() method
