package install

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SettingsMap represents a Claude Code settings JSON object
type SettingsMap map[string]interface{}

// ReadSettingsFile reads and parses a Claude Code settings.json file
// Returns default empty settings if file doesn't exist or is empty
// Returns error for permission issues or malformed JSON
func ReadSettingsFile(filePath string) (*SettingsMap, error) {
	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		// File doesn't exist - return default empty settings
		defaultSettings := make(SettingsMap)
		return &defaultSettings, nil
	}

	// Read file content
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read settings file %s: %w", filePath, err)
	}

	// Handle empty or whitespace-only files
	content := strings.TrimSpace(string(data))
	if content == "" || content == "null" {
		// Empty file or null JSON - return default empty settings
		defaultSettings := make(SettingsMap)
		return &defaultSettings, nil
	}

	// Parse JSON
	var settings SettingsMap
	err = json.Unmarshal(data, &settings)
	if err != nil {
		// Check for specific error types to provide better messages
		if strings.Contains(err.Error(), "cannot unmarshal") {
			// JSON is valid but not an object (array, string, etc.)
			return nil, fmt.Errorf("settings file must contain a JSON object, not %s: %w", getJSONType(data), err)
		}
		return nil, fmt.Errorf("invalid JSON in settings file %s: %w", filePath, err)
	}

	// Ensure we got a valid object (not null)
	if settings == nil {
		defaultSettings := make(SettingsMap)
		return &defaultSettings, nil
	}

	return &settings, nil
}

// WriteSettingsFile writes settings to a file atomically
// Creates directory structure if it doesn't exist
func WriteSettingsFile(filePath string, settings *SettingsMap) error {
	// Ensure directory exists
	dir := filepath.Dir(filePath)
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Marshal settings to JSON with indentation
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal settings to JSON: %w", err)
	}

	// Write atomically using temp file + rename
	tempFile := filePath + ".tmp"
	err = os.WriteFile(tempFile, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write temp settings file: %w", err)
	}

	// Atomic rename
	err = os.Rename(tempFile, filePath)
	if err != nil {
		// Clean up temp file on failure
		os.Remove(tempFile)
		return fmt.Errorf("failed to rename temp settings file: %w", err)
	}

	return nil
}

// getJSONType returns a human-readable description of JSON data type
func getJSONType(data []byte) string {
	content := strings.TrimSpace(string(data))
	if content == "" {
		return "empty"
	}
	if strings.HasPrefix(content, "[") {
		return "array"
	}
	if strings.HasPrefix(content, "\"") {
		return "string"
	}
	if content == "null" {
		return "null"
	}
	if content == "true" || content == "false" {
		return "boolean"
	}
	if strings.HasPrefix(content, "{") {
		return "object"
	}
	// Likely a number or unrecognized
	return "non-object value"
}

// SettingsLock interface for settings file locking with automatic cleanup
type SettingsLock interface {
	Release() error
}

// settingsLockWrapper wraps FileLockInterface to implement SettingsLock interface
type settingsLockWrapper struct {
	lock FileLockInterface
}

func (s *settingsLockWrapper) Release() error {
	if s.lock == nil {
		return nil // Already released or never acquired
	}

	err := s.lock.Unlock()
	s.lock = nil // Mark as released
	return err
}

// AcquireFileLock acquires an exclusive file lock with retry logic and timeout
func AcquireFileLock(lockFile string) (SettingsLock, error) {
	return AcquireFileLockWithTimeout(lockFile, 5*time.Second)
}

// AcquireFileLockWithTimeout acquires an exclusive file lock with timeout
func AcquireFileLockWithTimeout(lockFile string, timeout time.Duration) (SettingsLock, error) {
	fileLock := NewFileLock(lockFile)

	// Use context for timeout handling
	deadline := time.Now().Add(timeout)
	retryDelay := 10 * time.Millisecond

	for time.Now().Before(deadline) {
		locked, err := fileLock.TryLock()
		if err != nil {
			return nil, fmt.Errorf("failed to try lock %s: %w", lockFile, err)
		}

		if locked {
			return &settingsLockWrapper{lock: fileLock}, nil
		}

		// Wait before retrying, but don't exceed deadline
		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}

		sleepTime := retryDelay
		if sleepTime > remaining {
			sleepTime = remaining
		}

		time.Sleep(sleepTime)

		// Exponential backoff, but cap at 100ms
		retryDelay *= 2
		if retryDelay > 100*time.Millisecond {
			retryDelay = 100 * time.Millisecond
		}
	}

	return nil, fmt.Errorf("timeout acquiring file lock %s after %v", lockFile, timeout)
}

// ReadSettingsFileWithLock reads settings file with file locking for concurrent safety
func ReadSettingsFileWithLock(filePath string) (*SettingsMap, error) {
	lockFile := filePath + ".lock"

	// Ensure directory exists for lock file
	dir := filepath.Dir(lockFile)
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return nil, fmt.Errorf("failed to create directory for lock file %s: %w", dir, err)
	}

	// Acquire file lock
	lock, err := AcquireFileLock(lockFile)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire read lock: %w", err)
	}
	defer lock.Release()

	// Read settings with the lock held
	return ReadSettingsFile(filePath)
}

// WriteSettingsFileWithLock writes settings file with file locking for concurrent safety
func WriteSettingsFileWithLock(filePath string, settings *SettingsMap) error {
	lockFile := filePath + ".lock"

	// Ensure directory exists for lock file
	dir := filepath.Dir(lockFile)
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create directory for lock file %s: %w", dir, err)
	}

	// Acquire file lock
	lock, err := AcquireFileLock(lockFile)
	if err != nil {
		return fmt.Errorf("failed to acquire write lock: %w", err)
	}
	defer lock.Release()

	// Write settings with the lock held
	return WriteSettingsFile(filePath, settings)
}
