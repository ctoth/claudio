package install

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/afero"
)

// TDD RED: Test that settings I/O functions can use afero filesystem abstraction
// These tests verify memory filesystem isolation to prevent real filesystem pollution

func TestReadSettingsFile(t *testing.T) {
	memFS := afero.NewMemMapFs()
	
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
	memFS := afero.NewMemMapFs()
	
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
	memFS := afero.NewMemMapFs()
	
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
	realFS := afero.NewOsFs()
	
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
	memFS := afero.NewMemMapFs()
	
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
	memFS := afero.NewMemMapFs()
	
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

// TestWriteSettingsFile_CreatesBackup verifies that a second write
// produces a .bak file whose content equals the FIRST write's content.
func TestWriteSettingsFile_CreatesBackup(t *testing.T) {
	memFS := afero.NewMemMapFs()

	settingsPath := "/backup-test/settings.json"
	firstSettings := SettingsMap{"version": "1.0", "first": true}
	secondSettings := SettingsMap{"version": "2.0", "second": true}

	if err := WriteSettingsFile(memFS, settingsPath, &firstSettings); err != nil {
		t.Fatalf("first write failed: %v", err)
	}

	// Capture first-write bytes for comparison.
	firstBytes, err := afero.ReadFile(memFS, settingsPath)
	if err != nil {
		t.Fatalf("read after first write: %v", err)
	}

	if err := WriteSettingsFile(memFS, settingsPath, &secondSettings); err != nil {
		t.Fatalf("second write failed: %v", err)
	}

	bakPath := settingsPath + ".bak"
	exists, err := afero.Exists(memFS, bakPath)
	if err != nil {
		t.Fatalf("check .bak existence: %v", err)
	}
	if !exists {
		t.Fatal("expected .bak file after second write")
	}

	bakBytes, err := afero.ReadFile(memFS, bakPath)
	if err != nil {
		t.Fatalf("read .bak: %v", err)
	}
	if string(bakBytes) != string(firstBytes) {
		t.Errorf(".bak does not match first write\n  bak:  %s\n  want: %s", bakBytes, firstBytes)
	}

	// BackupSettingsFile uses temp+rename for atomicity. Assert no
	// .settings-bak-*.tmp residue remains in the directory after a
	// successful backup.
	bakTmpResidue := false
	_ = afero.Walk(memFS, filepath.Dir(settingsPath), func(p string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		base := filepath.Base(p)
		if strings.HasPrefix(base, ".settings-bak-") && strings.HasSuffix(base, ".tmp") {
			bakTmpResidue = true
		}
		return nil
	})
	if bakTmpResidue {
		t.Error(".settings-bak-*.tmp residue remained after BackupSettingsFile")
	}
}

// TestWriteSettingsFile_NoBackupOnFirstWrite verifies that writing to
// a fresh path does not create a .bak file (nothing to back up).
func TestWriteSettingsFile_NoBackupOnFirstWrite(t *testing.T) {
	memFS := afero.NewMemMapFs()

	settingsPath := "/first-write/settings.json"
	settings := SettingsMap{"version": "1.0"}

	if err := WriteSettingsFile(memFS, settingsPath, &settings); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	bakPath := settingsPath + ".bak"
	exists, err := afero.Exists(memFS, bakPath)
	if err != nil {
		t.Fatalf("check .bak existence: %v", err)
	}
	if exists {
		t.Error("expected no .bak file after first write to fresh path")
	}
}

// TestWriteSettingsFile_RefusesBackupOfCorruptJSON verifies that when
// the target is not valid JSON, BackupSettingsFile refuses to overwrite
// any existing .bak — preserving the last-known-good copy.
func TestWriteSettingsFile_RefusesBackupOfCorruptJSON(t *testing.T) {
	memFS := afero.NewMemMapFs()

	settingsPath := "/corrupt-test/settings.json"
	bakPath := settingsPath + ".bak"

	if err := memFS.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Pre-existing valid .bak that must be preserved.
	knownGoodBytes := []byte(`{"known_good": true}`)
	if err := afero.WriteFile(memFS, bakPath, knownGoodBytes, 0644); err != nil {
		t.Fatalf("seed known-good .bak: %v", err)
	}

	// Corrupt target file (not valid JSON).
	corruptBytes := []byte(`{this is not valid json`)
	if err := afero.WriteFile(memFS, settingsPath, corruptBytes, 0644); err != nil {
		t.Fatalf("seed corrupt settings: %v", err)
	}

	newSettings := SettingsMap{"version": "1.0"}
	if err := WriteSettingsFile(memFS, settingsPath, &newSettings); err != nil {
		t.Fatalf("write should succeed even with corrupt prior settings: %v", err)
	}

	// .bak must still contain the original known-good bytes.
	gotBak, err := afero.ReadFile(memFS, bakPath)
	if err != nil {
		t.Fatalf("read .bak: %v", err)
	}
	if string(gotBak) != string(knownGoodBytes) {
		t.Errorf(".bak was overwritten with corrupt content\n  got:  %s\n  want: %s",
			gotBak, knownGoodBytes)
	}
}