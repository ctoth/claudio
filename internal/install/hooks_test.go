package install

import (
	"encoding/json"
	"testing"
)

func TestGenerateClaudiaHooks(t *testing.T) {
	// TDD RED: Test hook configuration generation for Claudio installation
	testCases := []struct {
		name           string
		expectHooks    []string
		expectCommands []string
	}{
		{
			name:           "basic hook generation",
			expectHooks:    []string{"PreToolUse", "PostToolUse", "UserPromptSubmit"},
			expectCommands: []string{"claudio", "claudio", "claudio"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test the GenerateClaudiaHooks function
			hooks, err := GenerateClaudiaHooks()
			
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
			
			// Verify hooks have correct command structure
			for _, hookName := range tc.expectHooks {
				hookValue, exists := parsedHooks[hookName]
				if !exists {
					continue // Already reported above
				}
				
				hookStr, ok := hookValue.(string)
				if !ok {
					t.Errorf("Hook '%s' should be a string command, got: %T", hookName, hookValue)
					continue
				}
				
				// Should contain "claudio" command
				found := false
				for _, expectedCmd := range tc.expectCommands {
					if hookStr == expectedCmd {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Hook '%s' command '%s' doesn't match expected commands %v", 
						hookName, hookStr, tc.expectCommands)
				}
			}
			
			t.Logf("Successfully generated hooks: %v", getHookNames(parsedHooks))
		})
	}
}

func TestGenerateClaudiaHooksStructure(t *testing.T) {
	// TDD RED: Test that generated hooks have correct JSON structure for Claude Code
	hooks, err := GenerateClaudiaHooks()
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
		preToolUseStr, ok := preToolUse.(string)
		if !ok {
			t.Errorf("PreToolUse should be a string, got: %T", preToolUse)
		} else if preToolUseStr == "" {
			t.Error("PreToolUse should not be empty")
		}
	} else {
		t.Error("PreToolUse hook should be present")
	}
	
	// Test PostToolUse hook
	if postToolUse, exists := hooksMap["PostToolUse"]; exists {
		postToolUseStr, ok := postToolUse.(string)
		if !ok {
			t.Errorf("PostToolUse should be a string, got: %T", postToolUse)
		} else if postToolUseStr == "" {
			t.Error("PostToolUse should not be empty")
		}
	} else {
		t.Error("PostToolUse hook should be present")
	}
	
	// Test UserPromptSubmit hook
	if userPromptSubmit, exists := hooksMap["UserPromptSubmit"]; exists {
		userPromptSubmitStr, ok := userPromptSubmit.(string)
		if !ok {
			t.Errorf("UserPromptSubmit should be a string, got: %T", userPromptSubmit)
		} else if userPromptSubmitStr == "" {
			t.Error("UserPromptSubmit should not be empty")
		}
	} else {
		t.Error("UserPromptSubmit hook should be present")
	}
	
	t.Logf("Hook structure validation passed with %d hooks", len(hooksMap))
}

func TestGenerateClaudiaHooksConsistency(t *testing.T) {
	// TDD RED: Test that hook generation is consistent across multiple calls
	hooks1, err1 := GenerateClaudiaHooks()
	if err1 != nil {
		t.Fatalf("First hook generation failed: %v", err1)
	}
	
	hooks2, err2 := GenerateClaudiaHooks()
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

func TestGenerateClaudiaHooksValidJSON(t *testing.T) {
	// TDD RED: Test that generated hooks produce valid JSON that Claude Code can parse
	hooks, err := GenerateClaudiaHooks()
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

func TestGenerateClaudiaHooksIntegration(t *testing.T) {
	// TDD RED: Test that generated hooks can be integrated into settings structure
	hooks, err := GenerateClaudiaHooks()
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
// - GenerateClaudiaHooks() (interface{}, error)