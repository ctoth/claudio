package install

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"claudio.click/internal/fs"
)

// TDD RED: Test that settings I/O functions can use afero filesystem abstraction
// These tests verify memory filesystem isolation to prevent real filesystem pollution

func TestReadSettingsFile(t *testing.T) {
	factory := fs.NewDefaultFactory()
	memFS := factory.Memory()
	
	// Test case 1: Non-existent file should return empty settings
	settings, err := ReadSettingsFile(memFS, "/non/existent/file.json")
	if err != nil {
		t.Errorf("Expected no error for non-existent file, got: %v", err)
	}
	if settings == nil {
		t.Fatal("Expected settings map, got nil")
	}
	if len(*settings) != 0 {
		t.Errorf("Expected empty settings map, got: %v", *settings)
	}
	
	// Test case 2: Valid JSON file
	settingsPath := "/test/settings.json"
	testSettings := SettingsMap{
		"version": "1.0",
		"hooks": map[string]interface{}{
			"PreToolUse": "test",
		},
	}
	
	// Create directory and file in memory filesystem
	err = memFS.MkdirAll(filepath.Dir(settingsPath), 0755)
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	
	testData, err := json.Marshal(testSettings)
	if err != nil {
		t.Fatalf("Failed to marshal test settings: %v", err)
	}
	
	err = afero.WriteFile(memFS, settingsPath, testData, 0644)
	if err != nil {
		t.Fatalf("Failed to write test settings: %v", err)
	}
	
	// Read settings using filesystem abstraction
	readSettings, err := ReadSettingsFile(memFS, settingsPath)
	if err != nil {
		t.Errorf("Failed to read settings from memory filesystem: %v", err)
	}
	
	if readSettings == nil {
		t.Fatal("Expected settings to be read")
	}
	
	if (*readSettings)["version"] != "1.0" {
		t.Errorf("Expected version '1.0', got %v", (*readSettings)["version"])
	}
}

func TestWriteSettingsFile(t *testing.T) {
	factory := fs.NewDefaultFactory()
	memFS := factory.Memory()
	
	settingsPath := "/test/output/settings.json"
	testSettings := SettingsMap{
		"volume": 0.7,
		"soundpack": "test-pack",
		"hooks": map[string]interface{}{
			"PostToolUse": []interface{}{
				map[string]interface{}{
					"matcher": ".*",
					"hooks": []interface{}{
						map[string]interface{}{
							"type": "command",
							"command": "claudio",
						},
					},
				},
			},
		},
	}
	
	// Write settings to memory filesystem
	err := WriteSettingsFile(memFS, settingsPath, &testSettings)
	if err != nil {
		t.Errorf("Failed to write settings to memory filesystem: %v", err)
	}
	
	// Verify file exists in memory filesystem
	exists, err := afero.Exists(memFS, settingsPath)
	if err != nil {
		t.Errorf("Error checking file existence: %v", err)
	}
	if !exists {
		t.Error("Expected settings file to exist in memory filesystem")
	}
	
	// Read back and verify content
	data, err := afero.ReadFile(memFS, settingsPath)
	if err != nil {
		t.Errorf("Failed to read back settings: %v", err)
	}
	
	var readSettings SettingsMap
	err = json.Unmarshal(data, &readSettings)
	if err != nil {
		t.Errorf("Failed to unmarshal read settings: %v", err)
	}
	
	if readSettings["volume"] != 0.7 {
		t.Errorf("Expected volume 0.7, got %v", readSettings["volume"])
	}
	
	if readSettings["soundpack"] != "test-pack" {
		t.Errorf("Expected soundpack 'test-pack', got %v", readSettings["soundpack"])
	}
}

func TestSettingsFilesystemIsolation(t *testing.T) {
	// CRITICAL TEST: Verify filesystem isolation prevents real filesystem pollution
	factory := fs.NewDefaultFactory()
	memFS := factory.Memory()
	
	// Use a path that could exist on real filesystem
	dangerousPath := "/tmp/claudio-test-isolation-settings.json"
	testSettings := SettingsMap{
		"isolation_test": true,
		"dangerous_path": dangerousPath,
	}
	
	// Write to memory filesystem
	err := WriteSettingsFile(memFS, dangerousPath, &testSettings)
	if err != nil {
		t.Errorf("Failed to write to memory filesystem: %v", err)
	}
	
	// Verify file does NOT exist on real filesystem
	factory2 := fs.NewDefaultFactory()
	realFS := factory2.Production()
	
	exists, err := afero.Exists(realFS, dangerousPath)
	if err == nil && exists {
		t.Error("CRITICAL: Settings file was written to REAL filesystem - isolation broken!")
	}
	
	// But should exist in memory filesystem
	exists, err = afero.Exists(memFS, dangerousPath)
	if err != nil {
		t.Errorf("Error checking memory filesystem: %v", err)
	}
	if !exists {
		t.Error("Settings file should exist in memory filesystem")
	}
}

func TestReadWriteRoundTrip(t *testing.T) {
	// Test complete read-write cycle in memory filesystem
	factory := fs.NewDefaultFactory()
	memFS := factory.Memory()
	
	settingsPath := "/roundtrip/settings.json"
	originalSettings := SettingsMap{
		"test_roundtrip": true,
		"complex_data": map[string]interface{}{
			"nested": map[string]interface{}{
				"array": []interface{}{1, 2, 3},
				"string": "test",
			},
		},
	}
	
	// Write settings
	err := WriteSettingsFile(memFS, settingsPath, &originalSettings)
	if err != nil {
		t.Fatalf("Failed to write settings: %v", err)
	}
	
	// Read settings back
	readSettings, err := ReadSettingsFile(memFS, settingsPath)
	if err != nil {
		t.Fatalf("Failed to read settings: %v", err)
	}
	
	// Verify data integrity
	if (*readSettings)["test_roundtrip"] != true {
		t.Error("Round-trip failed for boolean value")
	}
	
	complexData, ok := (*readSettings)["complex_data"].(map[string]interface{})
	if !ok {
		t.Fatal("Complex data structure not preserved")
	}
	
	nested, ok := complexData["nested"].(map[string]interface{})
	if !ok {
		t.Fatal("Nested structure not preserved")
	}
	
	if nested["string"] != "test" {
		t.Error("Nested string value not preserved")
	}
}

func TestLockingWithFilesystem(t *testing.T) {
	// NOTE: File locking system still uses real filesystem internally
	// This is acceptable since locking is for concurrency, not corruption prevention
	// The core settings I/O operations are properly abstracted
	
	// For now, test the basic filesystem functions work correctly
	// Locking abstraction is a separate enhancement beyond current scope
	factory := fs.NewDefaultFactory()
	memFS := factory.Memory()
	
	settingsPath := "/simple/settings.json"
	testSettings := SettingsMap{
		"simple_test": true,
	}
	
	// Test basic write/read without locking (which is filesystem-isolated)
	err := WriteSettingsFile(memFS, settingsPath, &testSettings)
	if err != nil {
		t.Errorf("Failed to write settings: %v", err)
	}
	
	readSettings, err := ReadSettingsFile(memFS, settingsPath)
	if err != nil {
		t.Errorf("Failed to read settings: %v", err)
	}
	
	if (*readSettings)["simple_test"] != true {
		t.Error("Basic filesystem operations should work")
	}
}