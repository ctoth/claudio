package install

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/ctoth/claudio/internal/fs"
)

// TDD RED: Test that GenerateClaudioHooks can accept filesystem abstraction
// These tests will fail until we refactor to use afero.Fs instead of os.Executable

func TestGenerateClaudioHooksWithAfero(t *testing.T) {
	// TDD RED: This test expects GenerateClaudioHooks to accept filesystem parameter
	factory := fs.NewDefaultFactory()
	memFS := factory.Memory()
	
	// This function signature doesn't exist yet - will fail until we implement it
	hooks, err := GenerateClaudioHooks(memFS, "/mock/claudio")
	
	if err != nil {
		t.Errorf("Expected successful hook generation with filesystem, got error: %v", err)
	}
	
	if hooks == nil {
		t.Fatal("Expected hooks to be generated")
	}
	
	// Verify hooks contain mock executable path instead of test executable path
	hooksMap, ok := hooks.(map[string]interface{})
	if !ok {
		if hm, ok := hooks.(HooksMap); ok {
			hooksMap = map[string]interface{}(hm)
		} else {
			t.Fatalf("Expected hooks to be map[string]interface{} or HooksMap, got %T", hooks)
		}
	}
	
	// Check that hooks use the mock executable path
	for hookName, hookValue := range hooksMap {
		hookArray := hookValue.([]interface{})
		hookConfig := hookArray[0].(map[string]interface{})
		commandsField := hookConfig["hooks"].([]interface{})
		command := commandsField[0].(map[string]interface{})
		
		commandStr, ok := command["command"].(string)
		if !ok {
			t.Errorf("Hook %s command should be string", hookName)
			continue
		}
		
		// Should use mock path, not test executable path
		if commandStr != `"/mock/claudio"` {
			t.Errorf("Hook %s should use mock executable path '/mock/claudio', got %s", hookName, commandStr)
		}
		
		// CRITICAL: Should NOT contain test executable path that corrupts config
		if containsHooksAfero(commandStr, "cli.test") || containsHooksAfero(commandStr, "go-build") {
			t.Errorf("Hook %s contains test executable path, this CORRUPTS config: %s", hookName, commandStr)
		}
	}
}

func TestGenerateClaudioHooksFilesystemIsolation(t *testing.T) {
	// TDD RED: Test that hooks generation doesn't touch real filesystem
	factory := fs.NewDefaultFactory()
	memFS := factory.Memory()
	
	// Generate hooks using memory filesystem - should not interact with real filesystem
	hooks, err := GenerateClaudioHooks(memFS, "/isolated/claudio")
	if err != nil {
		t.Errorf("Hook generation with isolated filesystem failed: %v", err)
	}
	
	if hooks == nil {
		t.Fatal("Expected hooks to be generated in isolation")
	}
	
	// Verify hooks use isolated path
	var hooksMap map[string]interface{}
	switch h := hooks.(type) {
	case HooksMap:
		hooksMap = map[string]interface{}(h)
	case map[string]interface{}:
		hooksMap = h
	default:
		t.Fatalf("Expected hooks to be HooksMap or map[string]interface{}, got %T", hooks)
	}
	
	for hookName, hookValue := range hooksMap {
		hookArray := hookValue.([]interface{})
		hookConfig := hookArray[0].(map[string]interface{})
		commandsField := hookConfig["hooks"].([]interface{})
		command := commandsField[0].(map[string]interface{})
		
		commandStr, ok := command["command"].(string)
		if !ok {
			continue
		}
		
		// Should use isolated path
		if commandStr != `"/isolated/claudio"` {
			t.Errorf("Hook %s should use isolated path, got %s", hookName, commandStr)
		}
	}
}

func TestMergeHooksIntoSettingsWithFilesystem(t *testing.T) {
	// TDD RED: Test merging hooks with filesystem support 
	factory := fs.NewDefaultFactory()
	memFS := factory.Memory()
	
	// Create test settings in memory filesystem
	settingsPath := "/test/settings.json"
	existingSettings := SettingsMap{
		"version": "1.0",
		"existing": "value",
	}
	
	// Create directory and write settings to memory filesystem
	err := memFS.MkdirAll(filepath.Dir(settingsPath), 0755)
	if err != nil {
		t.Fatalf("Failed to create directory in memory fs: %v", err)
	}
	
	settingsData, err := json.Marshal(existingSettings)
	if err != nil {
		t.Fatalf("Failed to marshal settings: %v", err)
	}
	
	err = afero.WriteFile(memFS, settingsPath, settingsData, 0644)
	if err != nil {
		t.Fatalf("Failed to write settings to memory fs: %v", err)
	}
	
	// Generate Claudio hooks with filesystem support
	claudioHooks, err := GenerateClaudioHooks(memFS, "/memory/claudio")
	if err != nil {
		t.Fatalf("Failed to generate hooks: %v", err)
	}
	
	// Merge hooks into settings
	mergedSettings, err := MergeHooksIntoSettings(&existingSettings, claudioHooks)
	if err != nil {
		t.Errorf("Failed to merge hooks into settings: %v", err)
	}
	
	if mergedSettings == nil {
		t.Fatal("Expected merged settings")
	}
	
	// Verify hooks section exists
	if _, exists := (*mergedSettings)["hooks"]; !exists {
		t.Error("Expected hooks section in merged settings")
	}
	
	// Verify existing settings preserved
	if (*mergedSettings)["version"] != "1.0" {
		t.Error("Existing settings should be preserved")
	}
	
	if (*mergedSettings)["existing"] != "value" {
		t.Error("Existing values should be preserved")
	}
}

func TestConfigCorruptionPrevention(t *testing.T) {
	// TDD RED: Critical test - verify we prevent config corruption
	// This is the PRIMARY REASON for this refactoring
	
	factory := fs.NewDefaultFactory()
	memFS := factory.Memory()
	
	// Generate hooks using memory filesystem with mock executable
	hooks, err := GenerateClaudioHooks(memFS, "/safe/claudio")
	if err != nil {
		t.Errorf("Safe hook generation failed: %v", err)
	}
	
	if hooks == nil {
		t.Fatal("Expected safe hooks to be generated")
	}
	
	// CRITICAL: Verify no test executable paths that corrupt config
	hooksJSON, err := json.Marshal(hooks)
	if err != nil {
		t.Fatalf("Failed to marshal hooks: %v", err)
	}
	
	hooksString := string(hooksJSON)
	
	// These patterns indicate config corruption
	corruptionPatterns := []string{
		"cli.test",
		"go-build",
		"/tmp/go-build",
		"install.test",
		"hooks_test",
	}
	
	for _, pattern := range corruptionPatterns {
		if containsHooksAfero(hooksString, pattern) {
			t.Errorf("CRITICAL: Hook configuration contains corruption pattern '%s': %s", pattern, hooksString)
			t.Errorf("This WILL CORRUPT the user's Claude Code configuration!")
		}
	}
	
	// Should contain our safe mock path
	if !containsHooksAfero(hooksString, "/safe/claudio") {
		t.Errorf("Expected hooks to contain safe mock path '/safe/claudio', got: %s", hooksString)
	}
}

// Helper function for string containment check
func containsHooksAfero(s, substr string) bool {
	return len(s) >= len(substr) && indexOfHooksAfero(s, substr) >= 0
}

func indexOfHooksAfero(s, substr string) int {
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