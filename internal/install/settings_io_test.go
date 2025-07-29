package install

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadSettingsEmpty(t *testing.T) {
	// TDD RED: Test reading empty settings files
	testCases := []struct {
		name        string
		fileContent string
		expectError bool
		expectEmpty bool
	}{
		{
			name:        "completely empty file",
			fileContent: "",
			expectError: false, // Should create default settings
			expectEmpty: true,
		},
		{
			name:        "whitespace only file",
			fileContent: "   \n\t  \n  ",
			expectError: false, // Should create default settings
			expectEmpty: true,
		},
		{
			name:        "empty JSON object",
			fileContent: "{}",
			expectError: false,
			expectEmpty: false, // Valid JSON, not empty
		},
		{
			name:        "null JSON",
			fileContent: "null",
			expectError: false, // Should create default settings
			expectEmpty: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tempDir := t.TempDir()
			settingsFile := filepath.Join(tempDir, "settings.json")
			
			// Create test file with content
			err := os.WriteFile(settingsFile, []byte(tc.fileContent), 0644)
			if err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			// Test the ReadSettingsFile function
			settings, err := ReadSettingsFile(settingsFile)
			
			if tc.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tc.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			
			if !tc.expectError {
				if settings == nil {
					t.Error("Expected settings object but got nil")
				}
				
				if tc.expectEmpty {
					// Should have created default/empty settings
					if len(*settings) > 0 {
						t.Errorf("Expected empty settings but got: %v", *settings)
					}
				}
			}
			
			t.Logf("File content: %q, Settings: %v", tc.fileContent, settings)
		})
	}
}

func TestReadSettingsInvalidJSON(t *testing.T) {
	// TDD RED: Test reading files with invalid JSON
	testCases := []struct {
		name        string
		fileContent string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "malformed JSON - missing quote",
			fileContent: `{"key": value}`,
			expectError: true,
			errorMsg:    "invalid character",
		},
		{
			name:        "malformed JSON - trailing comma",
			fileContent: `{"key": "value",}`,
			expectError: true,
			errorMsg:    "invalid character",
		},
		{
			name:        "malformed JSON - missing brace",
			fileContent: `{"key": "value"`,
			expectError: true,
			errorMsg:    "unexpected end",
		},
		{
			name:        "malformed JSON - invalid escape",
			fileContent: `{"key": "val\xue"}`,
			expectError: true,
			errorMsg:    "invalid character",
		},
		{
			name:        "not JSON at all",
			fileContent: "This is not JSON at all!",
			expectError: true,
			errorMsg:    "invalid character",
		},
		{
			name:        "JSON array instead of object",
			fileContent: `["not", "an", "object"]`,
			expectError: true,
			errorMsg:    "must contain a JSON object",
		},
		{
			name:        "JSON string instead of object",
			fileContent: `"just a string"`,
			expectError: true,
			errorMsg:    "must contain a JSON object",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tempDir := t.TempDir()
			settingsFile := filepath.Join(tempDir, "settings.json")
			
			// Create test file with invalid content
			err := os.WriteFile(settingsFile, []byte(tc.fileContent), 0644)
			if err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			// Test the ReadSettingsFile function
			settings, err := ReadSettingsFile(settingsFile)
			
			if tc.expectError {
				if err == nil {
					t.Errorf("Expected error but got none for content: %s", tc.fileContent)
				} else {
					if !strings.Contains(err.Error(), tc.errorMsg) {
						t.Errorf("Expected error containing '%s', got: %v", tc.errorMsg, err)
					}
				}
				if settings != nil {
					t.Errorf("Expected nil settings on error, got: %v", settings)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if settings == nil {
					t.Error("Expected settings object but got nil")
				}
			}
			
			t.Logf("File content: %q, Error: %v", tc.fileContent, err)
		})
	}
}

func TestReadSettingsPermissionDenied(t *testing.T) {
	// TDD RED: Test reading files with permission issues
	tempDir := t.TempDir()
	settingsFile := filepath.Join(tempDir, "settings.json")
	
	// Create a valid settings file
	validContent := `{"hooks": {"PreToolUse": "test"}}`
	err := os.WriteFile(settingsFile, []byte(validContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	// Remove read permissions (use different approach for Windows compatibility)
	err = os.Chmod(settingsFile, 0000)
	if err != nil {
		t.Skipf("Cannot test permission denied on this system: %v", err)
	}
	
	// Restore permissions after test for cleanup
	defer func() {
		os.Chmod(settingsFile, 0644)
	}()

	// Test the ReadSettingsFile function
	settings, err := ReadSettingsFile(settingsFile)
	
	// On some systems (like WSL or when running as root), permission changes may not work
	// Skip the test if we can still read the file
	if err == nil {
		t.Skipf("Permission test skipped - unable to create unreadable file on this system")
		return
	}
	
	// Check for permission-related errors
	if !strings.Contains(err.Error(), "permission denied") && 
	   !strings.Contains(err.Error(), "access is denied") &&
	   !strings.Contains(err.Error(), "operation not permitted") {
		t.Errorf("Expected permission error, got: %v", err)
	}
	
	if settings != nil {
		t.Errorf("Expected nil settings on permission error, got: %v", settings)
	}
	
	t.Logf("Permission denied error: %v", err)
}

func TestReadSettingsFileNotFound(t *testing.T) {
	// TDD RED: Test reading non-existent files
	testCases := []struct {
		name        string
		filePath    string
		expectError bool
		shouldCreate bool
	}{
		{
			name:        "non-existent file",
			filePath:    "/tmp/nonexistent/settings.json",
			expectError: false, // Should create default settings
			shouldCreate: true,
		},
		{
			name:        "non-existent directory and file",
			filePath:    "/tmp/deeply/nested/nonexistent/settings.json",
			expectError: false, // Should create default settings
			shouldCreate: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Ensure file doesn't exist
			os.RemoveAll(filepath.Dir(tc.filePath))
			
			// Test the ReadSettingsFile function
			settings, err := ReadSettingsFile(tc.filePath)
			
			if tc.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tc.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			
			if !tc.expectError {
				if settings == nil {
					t.Error("Expected settings object but got nil")
				}
				
				if tc.shouldCreate {
					// Should have created default empty settings
					if len(*settings) > 0 {
						t.Errorf("Expected empty default settings but got: %v", *settings)
					}
				}
			}
			
			t.Logf("File path: %s, Settings created: %v", tc.filePath, settings != nil)
		})
	}
}

func TestReadSettingsValidJSON(t *testing.T) {
	// TDD RED: Test reading valid settings files
	testCases := []struct {
		name        string
		fileContent string
		expectKeys  []string
	}{
		{
			name:        "simple settings with hooks",
			fileContent: `{"hooks": {"PreToolUse": "echo 'before'", "PostToolUse": "echo 'after'"}}`,
			expectKeys:  []string{"hooks"},
		},
		{
			name:        "complex settings",
			fileContent: `{"hooks": {"PreToolUse": "test"}, "other": {"key": "value"}, "version": "1.0"}`,
			expectKeys:  []string{"hooks", "other", "version"},
		},
		{
			name:        "nested settings",
			fileContent: `{"hooks": {"PreToolUse": {"command": "test", "args": ["--verbose"]}}}`,
			expectKeys:  []string{"hooks"},
		},
		{
			name:        "settings with arrays",
			fileContent: `{"hooks": {"PreToolUse": "test"}, "plugins": ["plugin1", "plugin2"]}`,
			expectKeys:  []string{"hooks", "plugins"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tempDir := t.TempDir()
			settingsFile := filepath.Join(tempDir, "settings.json")
			
			// Create test file with valid content
			err := os.WriteFile(settingsFile, []byte(tc.fileContent), 0644)
			if err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			// Test the ReadSettingsFile function
			settings, err := ReadSettingsFile(settingsFile)
			
			if err != nil {
				t.Errorf("Unexpected error reading valid JSON: %v", err)
			}
			
			if settings == nil {
				t.Fatal("Expected settings object but got nil")
			}
			
			// Check that expected keys are present
			for _, key := range tc.expectKeys {
				if _, exists := (*settings)[key]; !exists {
					t.Errorf("Expected key '%s' not found in settings", key)
				}
			}
			
			// Verify settings can be marshaled back to JSON
			_, err = json.Marshal(*settings)
			if err != nil {
				t.Errorf("Settings object cannot be marshaled back to JSON: %v", err)
			}
			
			t.Logf("Successfully read settings with keys: %v", getKeys(*settings))
		})
	}
}

func TestReadSettingsLargeFile(t *testing.T) {
	// TDD RED: Test reading large settings files
	tempDir := t.TempDir()
	settingsFile := filepath.Join(tempDir, "large-settings.json")
	
	// Create a large JSON object
	largeSettings := make(map[string]interface{})
	largeSettings["hooks"] = map[string]string{
		"PreToolUse":      "echo 'before'",
		"PostToolUse":     "echo 'after'",
		"UserPromptSubmit": "echo 'prompt'",
	}
	
	// Add many dummy entries to make it large
	dummyData := make(map[string]string)
	for i := 0; i < 1000; i++ {
		dummyData[fmt.Sprintf("key_%d", i)] = fmt.Sprintf("value_%d", i)
	}
	largeSettings["large_data"] = dummyData
	
	// Marshal to JSON
	jsonData, err := json.Marshal(largeSettings)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}
	
	// Write to file
	err = os.WriteFile(settingsFile, jsonData, 0644)
	if err != nil {
		t.Fatalf("Failed to create large test file: %v", err)
	}
	
	// Test the ReadSettingsFile function
	settings, err := ReadSettingsFile(settingsFile)
	
	if err != nil {
		t.Errorf("Unexpected error reading large file: %v", err)
	}
	
	if settings == nil {
		t.Fatal("Expected settings object but got nil")
	}
	
	// Verify hooks are present
	if _, exists := (*settings)["hooks"]; !exists {
		t.Error("Expected 'hooks' key not found in large settings")
	}
	
	// Verify large data is present
	if _, exists := (*settings)["large_data"]; !exists {
		t.Error("Expected 'large_data' key not found in large settings")
	}
	
	fileInfo, _ := os.Stat(settingsFile)
	t.Logf("Successfully read large settings file (%d bytes)", fileInfo.Size())
}

// Helper function to get keys from a map
func getKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

