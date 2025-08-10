package install

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
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


// WriteSettingsFile writes settings to a file atomically using filesystem abstraction
// Creates directory structure if it doesn't exist
func WriteSettingsFile(filesystem afero.Fs, filePath string, settings *SettingsMap) error {
	// Ensure directory exists
	dir := filepath.Dir(filePath)
	err := filesystem.MkdirAll(dir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Detect existing file permissions to preserve them
	fileMode := os.FileMode(0644) // Default for new files
	if existingInfo, err := filesystem.Stat(filePath); err == nil {
		fileMode = existingInfo.Mode() & os.ModePerm // Preserve existing permissions
	}

	// Marshal settings to JSON with indentation
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal settings to JSON: %w", err)
	}

	// Write atomically using unique temp file + rename
	tempFile, err := afero.TempFile(filesystem, dir, ".settings-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tempFileName := tempFile.Name()

	// Write data to temp file with correct permissions
	_, err = tempFile.Write(data)
	if err != nil {
		tempFile.Close()
		filesystem.Remove(tempFileName)
		return fmt.Errorf("failed to write to temp file: %w", err)
	}

	// Close temp file
	err = tempFile.Close()
	if err != nil {
		filesystem.Remove(tempFileName)
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Set permissions after closing
	err = filesystem.Chmod(tempFileName, fileMode)
	if err != nil {
		filesystem.Remove(tempFileName)
		return fmt.Errorf("failed to set temp file permissions: %w", err)
	}

	// Atomic rename
	err = filesystem.Rename(tempFileName, filePath)
	if err != nil {
		// Clean up temp file on failure
		filesystem.Remove(tempFileName)
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

