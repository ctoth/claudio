package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"claudio.click/internal/fs"
)

// Helper to create float64 pointer
func ptrFloat(v float64) *float64 {
	return &v
}

// TDD RED: Test that ConfigManager uses afero filesystem abstraction
// These tests will fail until we refactor the code to accept afero.Fs

func TestConfigManagerWithMemoryFilesystem(t *testing.T) {
	// TDD RED: This test expects ConfigManager to accept afero.Fs
	factory := fs.NewDefaultFactory()
	memFS := factory.Memory()
	
	// This constructor doesn't exist yet - will fail until we implement it
	cm := NewConfigManagerWithFilesystem(memFS)
	
	if cm == nil {
		t.Fatal("Expected ConfigManager with filesystem support")
	}
}

func TestLoadFromFileWithMemoryFilesystem(t *testing.T) {
	// TDD RED: Test loading config from memory filesystem
	factory := fs.NewDefaultFactory()
	memFS := factory.Memory()
	
	// Create test config in memory filesystem
	configPath := "/test/config.json"
	testConfig := `{
		"volume": 0.8,
		"default_soundpack": "test",
		"enabled": true,
		"log_level": "debug"
	}`
	
	// Create directory and file in memory
	err := memFS.MkdirAll(filepath.Dir(configPath), 0755)
	if err != nil {
		t.Fatalf("Failed to create directory in memory fs: %v", err)
	}
	
	err = afero.WriteFile(memFS, configPath, []byte(testConfig), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config to memory fs: %v", err)
	}
	
	// This will fail until we implement filesystem abstraction
	cm := NewConfigManagerWithFilesystem(memFS)
	config, err := cm.LoadFromFile(configPath)
	
	if err != nil {
		t.Errorf("Expected successful config loading from memory fs, got error: %v", err)
	}
	
	if config == nil {
		t.Fatal("Expected config to be loaded")
	}
	
	if config.Volume == nil || *config.Volume != 0.8 {
		t.Errorf("Expected volume 0.8, got %v", config.Volume)
	}
	
	if config.DefaultSoundpack != "test" {
		t.Errorf("Expected default_soundpack 'test', got %s", config.DefaultSoundpack)
	}
}

func TestWriteConfigWithMemoryFilesystem(t *testing.T) {
	// TDD RED: Test writing config to memory filesystem
	factory := fs.NewDefaultFactory()
	memFS := factory.Memory()
	
	cm := NewConfigManagerWithFilesystem(memFS)
	config := &Config{
		Volume:           ptrFloat(0.3),
		DefaultSoundpack: "memory-test",
		Enabled:          false,
		LogLevel:         "info",
		AudioBackend:     "malgo",
	}
	
	configPath := "/test/output.json"
	
	// Create directory in memory
	err := memFS.MkdirAll(filepath.Dir(configPath), 0755)
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	
	// This will fail until we implement WriteConfig with filesystem abstraction
	err = cm.WriteConfig(configPath, config)
	if err != nil {
		t.Errorf("Expected successful config writing to memory fs, got error: %v", err)
	}
	
	// Verify file exists in memory filesystem
	exists, err := afero.Exists(memFS, configPath)
	if err != nil {
		t.Errorf("Error checking file existence: %v", err)
	}
	if !exists {
		t.Error("Expected config file to exist in memory filesystem")
	}
	
	// Verify contents
	data, err := afero.ReadFile(memFS, configPath)
	if err != nil {
		t.Errorf("Error reading config from memory fs: %v", err)
	}
	
	configContent := string(data)
	if !containsAfero(configContent, "memory-test") {
		t.Error("Expected config content to contain 'memory-test'")
	}
}

func TestConfigManagerIsolationFromRealFilesystem(t *testing.T) {
	// TDD RED: Verify memory filesystem doesn't touch real filesystem
	factory := fs.NewDefaultFactory()
	memFS := factory.Memory()
	
	cm := NewConfigManagerWithFilesystem(memFS)
	
	// Write to memory filesystem path that could exist on real filesystem
	dangerousPath := "/tmp/claudio-test-isolation.json"
	config := cm.GetDefaultConfig()
	
	err := cm.WriteConfig(dangerousPath, config)
	if err != nil {
		t.Errorf("Failed to write to memory filesystem: %v", err)
	}
	
	// Verify file does NOT exist on real filesystem (only in memory)
	if _, err := os.Stat(dangerousPath); err == nil {
		t.Error("Config was written to REAL filesystem instead of memory - isolation broken!")
	}
	
	// But should exist in memory filesystem
	exists, err := afero.Exists(memFS, dangerousPath)
	if err != nil {
		t.Errorf("Error checking memory fs: %v", err)
	}
	if !exists {
		t.Error("Config should exist in memory filesystem")
	}
}

// Helper function for string containment check
func containsAfero(s, substr string) bool {
	return len(s) >= len(substr) && indexOfAfero(s, substr) >= 0
}

func indexOfAfero(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}