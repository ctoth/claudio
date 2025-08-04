package install

import (
	"encoding/json"
	"os"
	"testing"
)

func TestGenerateClaudioHooks(t *testing.T) {
	// TDD RED: Test hook configuration generation for Claudio installation using registry
	testCases := []struct {
		name           string
		expectHooks    []string
		expectCommands []string
	}{
		{
			name:           "registry-based hook generation",
			expectHooks:    GetHookNames(),                                                                                   // Should use registry instead of hardcoded
			expectCommands: []string{"claudio", "claudio", "claudio", "claudio", "claudio", "claudio", "claudio", "claudio"}, // 8 hooks now
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test the GenerateClaudioHooks function
			hooks, err := GenerateClaudioHooks()

			if err != nil {
				t.Errorf("Unexpected error generating hooks: %v", err)
			}

			if hooks == nil {
				t.Fatal("Expected hooks object but got nil")
			}

			// Convert to JSON and back to verify structure
			jsonData, err := json.Marshal(hooks)
			if err != nil {
				t.Errorf("Generated hooks cannot be marshaled to JSON: %v", err)
			}

			var parsedHooks map[string]interface{}
			err = json.Unmarshal(jsonData, &parsedHooks)
			if err != nil {
				t.Errorf("Generated hooks JSON cannot be parsed back: %v", err)
			}

			// Check that all expected hooks are present
			for _, expectedHook := range tc.expectHooks {
				if _, exists := parsedHooks[expectedHook]; !exists {
					t.Errorf("Expected hook '%s' not found in generated hooks", expectedHook)
				}
			}

			// Verify hooks exist and have correct structure
			// (detailed structure testing is in TestGenerateClaudioHooksCorrectFormat)
			expectedCount := len(GetEnabledHooks()) // Should be 8 hooks from registry
			if len(parsedHooks) != expectedCount {
				t.Errorf("Expected %d hooks from registry, got %d", expectedCount, len(parsedHooks))
			}

			for _, hookName := range tc.expectHooks {
				hookValue, exists := parsedHooks[hookName]
				if !exists {
					continue // Already reported above
				}

				// Just verify it's not nil
				if hookValue == nil {
					t.Errorf("Hook '%s' should not be nil", hookName)
				}
			}

			t.Logf("Successfully generated hooks: %v", getHookNames(parsedHooks))
		})
	}
}

func TestGenerateClaudioHooksStructure(t *testing.T) {
	// TDD RED: Test that generated hooks have correct JSON structure for Claude Code
	hooks, err := GenerateClaudioHooks()
	if err != nil {
		t.Fatalf("Failed to generate hooks: %v", err)
	}

	if hooks == nil {
		t.Fatal("Expected hooks object but got nil")
	}

	// Convert to settings map format
	jsonData, err := json.Marshal(hooks)
	if err != nil {
		t.Fatalf("Failed to marshal hooks: %v", err)
	}

	var hooksMap map[string]interface{}
	err = json.Unmarshal(jsonData, &hooksMap)
	if err != nil {
		t.Fatalf("Failed to unmarshal hooks: %v", err)
	}

	// Test PreToolUse hook
	if preToolUse, exists := hooksMap["PreToolUse"]; exists {
		if preToolUse == nil {
			t.Error("PreToolUse should not be nil")
		}
	} else {
		t.Error("PreToolUse hook should be present")
	}

	// Test PostToolUse hook
	if postToolUse, exists := hooksMap["PostToolUse"]; exists {
		if postToolUse == nil {
			t.Error("PostToolUse should not be nil")
		}
	} else {
		t.Error("PostToolUse hook should be present")
	}

	// Test UserPromptSubmit hook
	if userPromptSubmit, exists := hooksMap["UserPromptSubmit"]; exists {
		if userPromptSubmit == nil {
			t.Error("UserPromptSubmit should not be nil")
		}
	} else {
		t.Error("UserPromptSubmit hook should be present")
	}

	t.Logf("Hook structure validation passed with %d hooks", len(hooksMap))
}

func TestGenerateClaudioHooksConsistency(t *testing.T) {
	// TDD RED: Test that hook generation is consistent across multiple calls
	hooks1, err1 := GenerateClaudioHooks()
	if err1 != nil {
		t.Fatalf("First hook generation failed: %v", err1)
	}

	hooks2, err2 := GenerateClaudioHooks()
	if err2 != nil {
		t.Fatalf("Second hook generation failed: %v", err2)
	}

	// Convert both to JSON for comparison
	json1, err := json.Marshal(hooks1)
	if err != nil {
		t.Fatalf("Failed to marshal first hooks: %v", err)
	}

	json2, err := json.Marshal(hooks2)
	if err != nil {
		t.Fatalf("Failed to marshal second hooks: %v", err)
	}

	// Should be identical
	if string(json1) != string(json2) {
		t.Errorf("Hook generation is not consistent:\nFirst:  %s\nSecond: %s",
			string(json1), string(json2))
	}

	t.Logf("Hook generation consistency verified")
}

func TestGenerateClaudioHooksValidJSON(t *testing.T) {
	// TDD RED: Test that generated hooks produce valid JSON that Claude Code can parse
	hooks, err := GenerateClaudioHooks()
	if err != nil {
		t.Fatalf("Failed to generate hooks: %v", err)
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(hooks)
	if err != nil {
		t.Fatalf("Failed to marshal hooks to JSON: %v", err)
	}

	// JSON should not be empty
	if len(jsonData) == 0 {
		t.Error("Generated JSON should not be empty")
	}

	// JSON should start and end with braces (object)
	jsonStr := string(jsonData)
	if len(jsonStr) < 2 || jsonStr[0] != '{' || jsonStr[len(jsonStr)-1] != '}' {
		t.Errorf("Generated JSON should be an object, got: %s", jsonStr)
	}

	// Should be valid JSON that can be unmarshaled
	var testUnmarshal map[string]interface{}
	err = json.Unmarshal(jsonData, &testUnmarshal)
	if err != nil {
		t.Errorf("Generated JSON is not valid: %v", err)
	}

	// Should have at least one hook
	if len(testUnmarshal) == 0 {
		t.Error("Generated hooks should contain at least one hook")
	}

	t.Logf("Generated valid JSON with %d hooks: %s", len(testUnmarshal), jsonStr)
}

func TestGenerateClaudioHooksIntegration(t *testing.T) {
	// TDD RED: Test that generated hooks can be integrated into settings structure
	hooks, err := GenerateClaudioHooks()
	if err != nil {
		t.Fatalf("Failed to generate hooks: %v", err)
	}

	// Create a mock settings structure like Claude Code would have
	settings := SettingsMap{
		"version": "1.0",
		"other":   "setting",
	}

	// Add hooks to settings
	settings["hooks"] = hooks

	// Should be able to marshal the entire settings object
	settingsJSON, err := json.Marshal(settings)
	if err != nil {
		t.Errorf("Failed to marshal settings with hooks: %v", err)
	}

	// Should be able to unmarshal back
	var parsedSettings SettingsMap
	err = json.Unmarshal(settingsJSON, &parsedSettings)
	if err != nil {
		t.Errorf("Failed to unmarshal settings with hooks: %v", err)
	}

	// Hooks should still be present and accessible
	if _, exists := parsedSettings["hooks"]; !exists {
		t.Error("Hooks should be present in parsed settings")
	}

	// Other settings should still be there
	if parsedSettings["version"] != "1.0" {
		t.Error("Other settings should be preserved")
	}

	t.Logf("Integration test passed - hooks integrated into settings")
}

// Helper function to get hook names from a parsed hooks map
func getHookNames(hooks map[string]interface{}) []string {
	names := make([]string, 0, len(hooks))
	for name := range hooks {
		names = append(names, name)
	}
	return names
}

// Functions that will need to be implemented (currently undefined):
// - GenerateClaudioHooks() (interface{}, error)

func TestGenerateClaudioHooksCorrectFormat(t *testing.T) {
	// TDD RED: Test that generated hooks follow Claude Code's required format
	hooks, err := GenerateClaudioHooks()
	if err != nil {
		t.Fatalf("Failed to generate hooks: %v", err)
	}

	// Handle both HooksMap and map[string]interface{} types
	var hooksMap map[string]interface{}
	switch h := hooks.(type) {
	case HooksMap:
		hooksMap = map[string]interface{}(h)
	case map[string]interface{}:
		hooksMap = h
	default:
		t.Fatalf("Expected hooks to be HooksMap or map[string]interface{}, got %T", hooks)
	}

	// Check each hook has the correct array/object structure
	for _, hookName := range []string{"PreToolUse", "PostToolUse", "UserPromptSubmit"} {
		t.Run(hookName, func(t *testing.T) {
			hookValue, exists := hooksMap[hookName]
			if !exists {
				t.Errorf("Hook %s should exist", hookName)
				return
			}

			// Hook should be an array
			hookArray, ok := hookValue.([]interface{})
			if !ok {
				t.Errorf("Hook %s should be an array, got %T", hookName, hookValue)
				return
			}

			// Array should have exactly one element
			if len(hookArray) != 1 {
				t.Errorf("Hook %s array should have 1 element, got %d", hookName, len(hookArray))
				return
			}

			// Element should be a map with hooks array
			hookConfig, ok := hookArray[0].(map[string]interface{})
			if !ok {
				t.Errorf("Hook %s array element should be a map, got %T", hookName, hookArray[0])
				return
			}

			// Should have a hooks array
			hooksField, exists := hookConfig["hooks"]
			if !exists {
				t.Errorf("Hook %s config should have 'hooks' field", hookName)
				return
			}

			// hooks field should be an array
			commandArray, ok := hooksField.([]interface{})
			if !ok {
				t.Errorf("Hook %s 'hooks' field should be an array, got %T", hookName, hooksField)
				return
			}

			// Should have exactly one command
			if len(commandArray) != 1 {
				t.Errorf("Hook %s command array should have 1 command, got %d", hookName, len(commandArray))
				return
			}

			// Command should be a map with type and command fields
			cmdMap, ok := commandArray[0].(map[string]interface{})
			if !ok {
				t.Errorf("Hook %s command should be a map, got %T", hookName, commandArray[0])
				return
			}

			// Check type field
			typeField, exists := cmdMap["type"]
			if !exists {
				t.Errorf("Hook %s command should have 'type' field", hookName)
				return
			}
			if typeField != "command" {
				t.Errorf("Hook %s type should be 'command', got %v", hookName, typeField)
			}

			// Check command field
			commandField, exists := cmdMap["command"]
			if !exists {
				t.Errorf("Hook %s command should have 'command' field", hookName)
				return
			}
			// Check that command is an executable path ending with "claudio"
			commandStr, ok := commandField.(string)
			if !ok {
				t.Errorf("Hook %s command should be string, got %T", hookName, commandField)
				return
			}

			// Get expected executable path for comparison
			expectedPath, err := os.Executable()
			if err != nil {
				// If os.Executable() fails, we expect fallback to "claudio"
				expectedPath = "claudio"
			}

			if commandStr != expectedPath {
				t.Errorf("Hook %s command should be '%s', got '%s'", hookName, expectedPath, commandStr)
			}
		})
	}
}

func TestGenerateClaudioHooksHasMatcher(t *testing.T) {
	// TDD RED: Test that all generated hooks have required matcher field
	hooks, err := GenerateClaudioHooks()
	if err != nil {
		t.Fatalf("Failed to generate hooks: %v", err)
	}

	// Handle both HooksMap and map[string]interface{} types
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

		// Check matcher field exists
		matcher, exists := hookConfig["matcher"]
		if !exists {
			t.Errorf("Hook %s missing required 'matcher' field", hookName)
		}

		// Check matcher value is correct
		if matcher != ".*" {
			t.Errorf("Hook %s matcher should be '.*', got '%v'", hookName, matcher)
		}
	}
}

func TestGenerateClaudioHooksUsesExecutablePath(t *testing.T) {
	// TDD RED: Test that generated hooks use current executable path, not hardcoded "claudio"
	hooks, err := GenerateClaudioHooks()
	if err != nil {
		t.Fatalf("Failed to generate hooks: %v", err)
	}

	// Handle both HooksMap and map[string]interface{} types
	var hooksMap map[string]interface{}
	switch h := hooks.(type) {
	case HooksMap:
		hooksMap = map[string]interface{}(h)
	case map[string]interface{}:
		hooksMap = h
	default:
		t.Fatalf("Expected hooks to be HooksMap or map[string]interface{}, got %T", hooks)
	}

	// Get expected executable path for comparison
	expectedPath, err := os.Executable()
	if err != nil {
		// If os.Executable() fails, we expect fallback to "claudio"
		expectedPath = "claudio"
	}

	for hookName, hookValue := range hooksMap {
		hookArray := hookValue.([]interface{})
		hookConfig := hookArray[0].(map[string]interface{})

		// Get hooks array from config
		hooksField, exists := hookConfig["hooks"]
		if !exists {
			t.Errorf("Hook %s missing 'hooks' field", hookName)
			continue
		}

		hooksArray := hooksField.([]interface{})
		if len(hooksArray) == 0 {
			t.Errorf("Hook %s has empty hooks array", hookName)
			continue
		}

		hookCommand := hooksArray[0].(map[string]interface{})

		// Check command field uses executable path
		command, exists := hookCommand["command"]
		if !exists {
			t.Errorf("Hook %s missing 'command' field", hookName)
			continue
		}

		commandStr, ok := command.(string)
		if !ok {
			t.Errorf("Hook %s command should be string, got %T", hookName, command)
			continue
		}

		// Verify command uses executable path, not hardcoded "claudio"
		if commandStr != expectedPath {
			t.Errorf("Hook %s command should be '%s', got '%s'", hookName, expectedPath, commandStr)
		}

		// Specifically check that it's NOT the hardcoded "claudio" (unless that's the fallback)
		if commandStr == "claudio" && expectedPath != "claudio" {
			t.Errorf("Hook %s using hardcoded 'claudio' instead of executable path '%s'", hookName, expectedPath)
		}
	}
}
