package install

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"strings"

	"github.com/spf13/afero"

	"claudio.click/internal/safeio"
)

// SettingsMap represents a Claude Code settings JSON object
type SettingsMap map[string]interface{}

// ReadSettingsFile reads and parses a Claude Code settings.json file using filesystem abstraction
// Returns default empty settings if file doesn't exist or is empty
// Returns error for permission issues or malformed JSON
func ReadSettingsFile(filesystem afero.Fs, filePath string) (*SettingsMap, error) {
	// Check if file exists
	if _, err := filesystem.Stat(filePath); os.IsNotExist(err) {
		// File doesn't exist - return default empty settings
		defaultSettings := make(SettingsMap)
		return &defaultSettings, nil
	}

	// Read file content
	data, err := afero.ReadFile(filesystem, filePath)
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

// BackupSettingsFile copies filePath to filePath+".bak" if filePath
// exists and parses as a JSON object. See safeio.BackupJSONFile for the
// failure policy: errors are logged, never returned (callers proceed
// with the write regardless), and a corrupt source never overwrites the
// last-known-good .bak.
func BackupSettingsFile(filesystem afero.Fs, filePath string) {
	safeio.BackupJSONFile(filesystem, filePath, &SettingsMap{}, ".settings-bak-*.tmp")
}

// WriteSettingsFile backs up the existing settings (see BackupSettingsFile)
// and then replaces filePath with settings via safeio.WriteJSONFile, which
// owns the temp-file + fsync + rename + parent-dir-fsync atomic write.
// The caller's advisory lock (see internal/install/lockfile.go) keeps the
// backup safely inside the read-mutate-write window.
func WriteSettingsFile(filesystem afero.Fs, filePath string, settings *SettingsMap) error {
	// Non-fatal: a missing .bak is better than a blocked write.
	BackupSettingsFile(filesystem, filePath)
	return safeio.WriteJSONFile(filesystem, filePath, settings, ".settings-*.tmp")
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

// ModifySettings runs one locked read-modify-write of the settings file at
// filePath: it takes the settings-dir lock (LockSettingsDir), reads the
// file (missing means empty), passes the settings to fn, and writes fn's
// result back via WriteSettingsFile. When fn returns an error, or a result
// equal to what was read, the file is not written.
//
// fn must return a new map and leave its argument unmodified (as
// InstallAgentHooks and RemoveAgentHooks do); the argument is what the
// result is compared against.
func ModifySettings(filesystem afero.Fs, filePath string, fn func(*SettingsMap) (*SettingsMap, error)) error {
	lock, err := LockSettingsDir(filePath)
	if err != nil {
		return err
	}
	defer func() {
		if unlockErr := lock.Unlock(); unlockErr != nil {
			slog.Warn("failed to release settings lock", "error", unlockErr)
		}
	}()

	settings, err := ReadSettingsFile(filesystem, filePath)
	if err != nil {
		return fmt.Errorf("failed to read settings from %s: %w", filePath, err)
	}
	updated, err := fn(settings)
	if err != nil {
		return err
	}
	if reflect.DeepEqual(settings, updated) {
		slog.Debug("settings unchanged, not rewriting", "path", filePath)
		return nil
	}
	if err := WriteSettingsFile(filesystem, filePath, updated); err != nil {
		return fmt.Errorf("failed to write settings to %s: %w", filePath, err)
	}
	return nil
}
