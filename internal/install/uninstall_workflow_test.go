package install

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/afero"
)

func TestInstallUninstallWithExecutablePath(t *testing.T) {
	// TDD RED: Test complete install/uninstall workflow with executable paths
	testCases := []struct {
		name         string
		scope        string
		installFirst bool
		expectError  bool
	}{
		{
			name:         "install with executable path then uninstall - user scope",
			scope:        "user",
			installFirst: true,
			expectError:  false,
		},
		{
			name:         "install with executable path then uninstall - project scope",
			scope:        "project",
			installFirst: true,
			expectError:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create temporary directory for test settings
			tempDir, err := os.MkdirTemp("", "claudio-test-*")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tempDir)

			// Set up test environment
			var settingsPath string
			if tc.scope == "user" {
				settingsPath = filepath.Join(tempDir, ".claude", "settings.json")
			} else {
				settingsPath = filepath.Join(tempDir, ".claude", "settings.json")
			}

			// Create settings directory
			settingsDir := filepath.Dir(settingsPath)
			err = os.MkdirAll(settingsDir, 0755)
			if err != nil {
				t.Fatalf("Failed to create settings dir: %v", err)
			}

			if tc.installFirst {
				// Step 1: Install claudio hooks (which should use executable path)
				// Use an explicit fixture path so the test does not depend on go test's
				// binary name leaking through GetExecutablePath().
				execPath := "/usr/local/bin/claudio"
				claudioHooks, err := GenerateClaudioHooksForAgent(execPath, AgentClaude)
				if err != nil {
					t.Fatalf("Failed to generate claudio hooks: %v", err)
				}

				// Create empty settings to install into
				initialSettings := SettingsMap{"version": "1.0"}
				mergedSettings, err := MergeHooksIntoSettings(&initialSettings, claudioHooks)
				if err != nil {
					t.Fatalf("Failed to merge hooks: %v", err)
				}

				// Write settings file
				settingsJSON, err := json.MarshalIndent(mergedSettings, "", "  ")
				if err != nil {
					t.Fatalf("Failed to marshal settings: %v", err)
				}
				err = os.WriteFile(settingsPath, settingsJSON, 0644)
				if err != nil {
					t.Fatalf("Failed to write settings file: %v", err)
				}

				// Step 2: Read back the settings to verify executable paths were used
				settingsData, err := os.ReadFile(settingsPath)
				if err != nil {
					t.Fatalf("Failed to read settings file: %v", err)
				}

				// Debug: Print the actual settings content
				t.Logf("Settings file content: %s", string(settingsData))

				var readSettings SettingsMap
				err = json.Unmarshal(settingsData, &readSettings)
				if err != nil {
					t.Fatalf("Failed to unmarshal settings: %v", err)
				}

				// Verify hooks contain executable paths (not just "claudio")
				hooksInterface, exists := readSettings["hooks"]
				if !exists {
					t.Fatal("No hooks section found after install")
				}

				hooksMap, ok := hooksInterface.(map[string]interface{})
				if !ok {
					t.Fatal("Hooks section is not a map")
				}

				// Check that hooks are properly formatted with executable path
				foundExecutableHooks := false
				for hookName, hookValue := range hooksMap {
					if hookArray, ok := hookValue.([]interface{}); ok && len(hookArray) > 0 {
						if hookConfig, ok := hookArray[0].(map[string]interface{}); ok {
							if hooksField, ok := hookConfig["hooks"].([]interface{}); ok && len(hooksField) > 0 {
								if cmd, ok := hooksField[0].(map[string]interface{}); ok {
									if cmdStr, ok := cmd["command"].(string); ok {
										// Use the same detection logic as production code
										if IsClaudioCommandString(cmdStr) {
											foundExecutableHooks = true
											t.Logf("Hook %s uses executable command: %s", hookName, cmdStr)
											break
										}
									}
								}
							}
						}
					}
				}

				if !foundExecutableHooks {
					t.Error("Expected at least one hook to use executable command")
				}

				// Step 3: Run uninstall workflow to remove hooks. Inject the
				// test's tempdir path via swapAgentResolver so we don't write
				// to the user's real settings file.
				err = RunUninstallWorkflow(afero.NewOsFs(), AgentTarget{Agent: AgentClaude, ConfigPath: settingsPath})
				if tc.expectError && err == nil {
					t.Error("Expected error but got none")
				}
				if !tc.expectError && err != nil {
					t.Errorf("Unexpected error: %v", err)
				}

				// Step 4: Verify all claudio hooks were removed
				finalData, err := os.ReadFile(settingsPath)
				if err != nil {
					t.Fatalf("Failed to read final settings: %v", err)
				}

				var finalSettings SettingsMap
				err = json.Unmarshal(finalData, &finalSettings)
				if err != nil {
					t.Fatalf("Failed to unmarshal final settings: %v", err)
				}

				// Check that hooks section is empty or doesn't exist
				if finalHooks, exists := finalSettings["hooks"]; exists {
					if finalHooksMap, ok := finalHooks.(map[string]interface{}); ok {
						for hookName := range finalHooksMap {
							t.Errorf("Hook '%s' was not removed during uninstall", hookName)
						}
					}
				}

				t.Logf("Install/uninstall executable path workflow test passed for %s", tc.name)
			}
		})
	}
}

func TestRunUninstallWorkflow(t *testing.T) {
	// TDD RED: Test complete uninstall workflow integration
	testCases := []struct {
		name                 string
		scope                string
		existingSettings     map[string]interface{}
		existingSettingsFile bool
		expectError          bool
		expectedHooksCount   int
		expectNoHooksSection bool
	}{
		{
			name:  "uninstall from settings with claudio hooks",
			scope: "user",
			existingSettings: map[string]interface{}{
				"hooks": map[string]interface{}{
					"PreToolUse":       "claudio",
					"PostToolUse":      "claudio",
					"UserPromptSubmit": "claudio",
					"Other":            "keep-this",
				},
				"version": "1.0",
			},
			existingSettingsFile: true,
			expectError:          false,
			expectedHooksCount:   1, // Only "Other" should remain
			expectNoHooksSection: false,
		},
		{
			name:  "uninstall from settings without claudio - no changes",
			scope: "user",
			existingSettings: map[string]interface{}{
				"hooks": map[string]interface{}{
					"Other": "different-tool",
				},
				"version": "1.0",
			},
			existingSettingsFile: true,
			expectError:          false,
			expectedHooksCount:   1, // "Other" hook preserved
			expectNoHooksSection: false,
		},
		{
			name:  "uninstall all hooks - hooks section deleted",
			scope: "project",
			existingSettings: map[string]interface{}{
				"hooks": map[string]interface{}{
					"PreToolUse":       "claudio",
					"PostToolUse":      "claudio",
					"UserPromptSubmit": "claudio",
				},
				"version": "1.0",
			},
			existingSettingsFile: true,
			expectError:          false,
			expectedHooksCount:   0,
			expectNoHooksSection: true,
		},
		{
			name:  "uninstall from complex array hooks",
			scope: "user",
			existingSettings: map[string]interface{}{
				"hooks": map[string]interface{}{
					"Notification": []interface{}{
						map[string]interface{}{
							"hooks": []interface{}{
								map[string]interface{}{
									"command": "claudio",
									"type":    "command",
								},
								map[string]interface{}{
									"command": "other-tool",
									"type":    "command",
								},
							},
						},
					},
					"Stop": []interface{}{
						map[string]interface{}{
							"hooks": []interface{}{
								map[string]interface{}{
									"command": "claudio",
									"type":    "command",
								},
							},
						},
					},
				},
			},
			existingSettingsFile: true,
			expectError:          false,
			expectedHooksCount:   1, // Only "Notification" should remain (with other-tool)
			expectNoHooksSection: false,
		},
		{
			name:  "uninstall from mixed simple and complex hooks",
			scope: "user",
			existingSettings: map[string]interface{}{
				"hooks": map[string]interface{}{
					"PreToolUse": "claudio",
					"SubagentStop": []interface{}{
						map[string]interface{}{
							"hooks": []interface{}{
								map[string]interface{}{
									"command": "claudio",
									"type":    "command",
								},
							},
						},
					},
					"Other": "preserve-this",
				},
			},
			existingSettingsFile: true,
			expectError:          false,
			expectedHooksCount:   1, // Only "Other" should remain
			expectNoHooksSection: false,
		},
		{
			name:  "uninstall from empty settings file",
			scope: "user",
			existingSettings: map[string]interface{}{
				"version": "1.0",
			},
			existingSettingsFile: true,
			expectError:          false,
			expectedHooksCount:   0,
			expectNoHooksSection: true,
		},
		{
			name:                 "uninstall from non-existent settings file",
			scope:                "user",
			existingSettings:     nil,
			existingSettingsFile: false,
			expectError:          false,
			expectedHooksCount:   0,
			expectNoHooksSection: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create temporary directory for settings
			tempDir := t.TempDir()
			settingsDir := filepath.Join(tempDir, ".claude")
			err := os.MkdirAll(settingsDir, 0755)
			if err != nil {
				t.Fatalf("Failed to create settings directory: %v", err)
			}

			settingsFile := filepath.Join(settingsDir, "settings.json")

			// Create existing settings file if specified
			if tc.existingSettingsFile && tc.existingSettings != nil {
				settingsJSON, err := json.MarshalIndent(tc.existingSettings, "", "  ")
				if err != nil {
					t.Fatalf("Failed to marshal existing settings: %v", err)
				}

				err = os.WriteFile(settingsFile, settingsJSON, 0644)
				if err != nil {
					t.Fatalf("Failed to write existing settings file: %v", err)
				}
			}

			// Test the complete uninstall workflow. Inject the test's tempdir
			// path via swapAgentResolver so the workflow targets settingsFile
			// instead of resolving via agent.BestConfigPath.
			err = RunUninstallWorkflow(afero.NewOsFs(), AgentTarget{Agent: AgentClaude, ConfigPath: settingsFile})

			if tc.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tc.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if tc.expectError {
				return // Skip verification if error was expected
			}

			// Verify uninstall results
			// 1. Settings file should exist after uninstall
			if _, err := os.Stat(settingsFile); os.IsNotExist(err) {
				if tc.existingSettingsFile {
					t.Error("Settings file should exist after uninstall")
				}
				return // File doesn't exist, which is fine for non-existent cases
			}

			// 2. Settings file should be valid JSON
			settingsData, err := os.ReadFile(settingsFile)
			if err != nil {
				t.Errorf("Failed to read settings file: %v", err)
				return
			}

			var settingsMap SettingsMap
			err = json.Unmarshal(settingsData, &settingsMap)
			if err != nil {
				t.Errorf("Failed to parse settings JSON: %v", err)
				return
			}
			settings := &settingsMap

			// 3. Check hooks section
			if tc.expectNoHooksSection {
				if _, exists := (*settings)["hooks"]; exists {
					t.Error("Hooks section should be deleted when all hooks removed")
				}
			} else {
				if hooks, exists := (*settings)["hooks"]; exists {
					hooksMap, ok := hooks.(map[string]interface{})
					if !ok {
						t.Errorf("Hooks should be a map, got: %T", hooks)
					} else {
						if len(hooksMap) != tc.expectedHooksCount {
							t.Errorf("Expected %d hooks, got %d: %v",
								tc.expectedHooksCount, len(hooksMap), getMapKeys(hooksMap))
						}

						// 4. Verify no claudio hooks remain
						claudioHooks := ClaudioHookNames(settings)
						if len(claudioHooks) > 0 {
							t.Errorf("Claudio hooks still present after uninstall: %v", claudioHooks)
						}
					}
				} else if tc.expectedHooksCount > 0 {
					t.Errorf("Expected %d hooks but hooks section missing", tc.expectedHooksCount)
				}
			}

			// 5. Existing non-claudio settings should be preserved
			if tc.existingSettings != nil {
				for key, expectedValue := range tc.existingSettings {
					if key == "hooks" {
						continue // Already tested above
					}

					if actualValue, exists := (*settings)[key]; !exists {
						t.Errorf("Existing setting '%s' was not preserved", key)
					} else {
						// Deep comparison using JSON
						expectedJSON, _ := json.Marshal(expectedValue)
						actualJSON, _ := json.Marshal(actualValue)
						if string(expectedJSON) != string(actualJSON) {
							t.Errorf("Existing setting '%s' was modified:\nExpected: %s\nActual:   %s",
								key, string(expectedJSON), string(actualJSON))
						}
					}
				}
			}

			t.Logf("Uninstall workflow test passed for %s", tc.name)
		})
	}
}

func TestUninstallWorkflowErrorHandling(t *testing.T) {
	// TDD RED: Test error handling in uninstall workflow
	testCases := []struct {
		name        string
		scope       string
		setupFunc   func() (settingsPath string, cleanup func())
		expectError bool
		errorMsg    string
	}{
		{
			name:  "permission denied directory",
			scope: "user",
			setupFunc: func() (string, func()) {
				tempDir := t.TempDir()
				settingsDir := filepath.Join(tempDir, "restricted")
				_ = os.MkdirAll(settingsDir, 0000) // No permissions

				return filepath.Join(settingsDir, "settings.json"), func() {
					_ = os.Chmod(settingsDir, 0755) // Restore permissions for cleanup
				}
			},
			expectError: true,
			errorMsg:    "permission",
		},
		{
			name:  "corrupted existing settings file",
			scope: "user",
			setupFunc: func() (string, func()) {
				tempDir := t.TempDir()
				settingsDir := filepath.Join(tempDir, ".claude")
				_ = os.MkdirAll(settingsDir, 0755)

				settingsFile := filepath.Join(settingsDir, "settings.json")
				// Write invalid JSON
				_ = os.WriteFile(settingsFile, []byte("{invalid json"), 0644)

				return settingsFile, func() {}
			},
			expectError: true,
			errorMsg:    "invalid JSON",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Skip permission test on Windows (different permission model) or when running as root
			if tc.name == "permission denied directory" {
				if runtime.GOOS == "windows" {
					t.Skip("Skipping permission test on Windows - Unix permission semantics don't apply")
				}
				if os.Getuid() == 0 {
					t.Skip("Skipping permission test when running as root")
				}
			}

			settingsPath, cleanup := tc.setupFunc()
			defer cleanup()

			err := RunUninstallWorkflow(afero.NewOsFs(), AgentTarget{Agent: AgentClaude, ConfigPath: settingsPath})

			if tc.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tc.errorMsg != "" && !strings.Contains(err.Error(), tc.errorMsg) {
					t.Errorf("Expected error containing '%s', got: %v", tc.errorMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}

			t.Logf("Error handling test passed for %s", tc.name)
		})
	}
}

// getMapKeys lists a map's keys for failure messages.
func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// statFailFs fails every Stat, standing in for an unreadable settings dir
// on any OS (the chmod-based case above cannot run on Windows).
type statFailFs struct{ afero.Fs }

func (statFailFs) Stat(string) (os.FileInfo, error) { return nil, os.ErrPermission }

func TestRunUninstallWorkflowReportsUncheckableSettingsPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	err := RunUninstallWorkflow(statFailFs{afero.NewMemMapFs()}, AgentTarget{Agent: AgentClaude, ConfigPath: path})
	if err == nil || !strings.Contains(err.Error(), "failed to check settings path") {
		t.Fatalf("err = %v, want a settings-path check failure", err)
	}
}

// TestRunUninstallWorkflowUsesTargetConfigPath asserts that the workflow
// rewrites exactly the target's resolved ConfigPath (the path the CLI
// printed), not a path it re-resolves itself, and leaves other files alone.
func TestRunUninstallWorkflowUsesTargetConfigPath(t *testing.T) {
	tempDir := t.TempDir()
	resolvedPath := filepath.Join(tempDir, "resolved", "settings.json")
	decoyPath := filepath.Join(tempDir, "decoy", "settings.json")

	if err := os.MkdirAll(filepath.Dir(resolvedPath), 0755); err != nil {
		t.Fatalf("mkdir resolved dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(decoyPath), 0755); err != nil {
		t.Fatalf("mkdir decoy dir: %v", err)
	}

	// Seed both files with claudio hooks so we can tell which one the
	// workflow rewrote.
	initial := SettingsMap{
		"hooks": map[string]interface{}{
			"PreToolUse": "/usr/local/bin/claudio",
		},
		"version": "test",
	}
	data, err := json.Marshal(initial)
	if err != nil {
		t.Fatalf("marshal initial settings: %v", err)
	}
	if err := os.WriteFile(resolvedPath, data, 0644); err != nil {
		t.Fatalf("write resolved file: %v", err)
	}
	if err := os.WriteFile(decoyPath, data, 0644); err != nil {
		t.Fatalf("write decoy file: %v", err)
	}

	if err := RunUninstallWorkflow(afero.NewOsFs(), AgentTarget{Agent: AgentClaude, ConfigPath: resolvedPath}); err != nil {
		t.Fatalf("workflow failed: %v", err)
	}

	// The resolved path must have had its claudio hook removed.
	resolvedAfter, err := os.ReadFile(resolvedPath)
	if err != nil {
		t.Fatalf("read resolved after: %v", err)
	}
	var resolvedSettings SettingsMap
	if err := json.Unmarshal(resolvedAfter, &resolvedSettings); err != nil {
		t.Fatalf("unmarshal resolved after: %v", err)
	}
	if hooks, ok := resolvedSettings["hooks"].(map[string]interface{}); ok {
		if _, present := hooks["PreToolUse"]; present {
			t.Errorf("resolved path's PreToolUse hook should have been removed, but it is still present: %v", hooks)
		}
	}

	// The decoy path must be untouched.
	decoyAfter, err := os.ReadFile(decoyPath)
	if err != nil {
		t.Fatalf("read decoy after: %v", err)
	}
	if string(decoyAfter) != string(data) {
		t.Errorf("decoy path was modified — workflow should only write to the agent-resolved path.\nbefore: %s\nafter:  %s", data, decoyAfter)
	}
}

func TestRunUninstallWorkflowMissingSettingsFileIsNoop(t *testing.T) {
	settingsPath := filepath.Join(t.TempDir(), "missing", "settings.json")

	if err := RunUninstallWorkflow(afero.NewOsFs(), AgentTarget{Agent: AgentClaude, ConfigPath: settingsPath}); err != nil {
		t.Fatalf("missing settings file should be an idempotent uninstall, got: %v", err)
	}

	if _, err := os.Stat(filepath.Dir(settingsPath)); !os.IsNotExist(err) {
		t.Fatalf("uninstall should not create missing settings directory, stat err: %v", err)
	}
}

func TestRunUninstallWorkflowCodexPreservesMixedGroupSibling(t *testing.T) {
	settingsPath := filepath.Join(t.TempDir(), "settings.json")
	initial := SettingsMap{
		"hooks": map[string]interface{}{
			"Stop": []interface{}{
				map[string]interface{}{
					"matcher": "*",
					"hooks": []interface{}{
						map[string]interface{}{
							"type":           "command",
							"command":        "C:/Users/Q/bin/claudio.exe",
							"commandWindows": `& "C:/Users/Q/bin/claudio.exe"`,
						},
						map[string]interface{}{
							"type":    "command",
							"command": "custom-stop-hook",
						},
					},
				},
			},
		},
		"version": "test",
	}
	data, err := json.Marshal(initial)
	if err != nil {
		t.Fatalf("marshal initial settings: %v", err)
	}
	if err := os.WriteFile(settingsPath, data, 0644); err != nil {
		t.Fatalf("write settings: %v", err)
	}

	if err := RunUninstallWorkflow(afero.NewOsFs(), AgentTarget{Agent: AgentCodex, ConfigPath: settingsPath}); err != nil {
		t.Fatalf("workflow failed: %v", err)
	}

	after, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read settings after uninstall: %v", err)
	}
	var settings SettingsMap
	if err := json.Unmarshal(after, &settings); err != nil {
		t.Fatalf("unmarshal settings after uninstall: %v", err)
	}
	hooks := settings["hooks"].(map[string]interface{})
	groups := hooks["Stop"].([]interface{})
	group := groups[0].(map[string]interface{})
	entries := group["hooks"].([]interface{})
	if len(entries) != 1 {
		t.Fatalf("expected one surviving custom command, got %v", entries)
	}
	entry := entries[0].(map[string]interface{})
	if command := entry["command"]; command != "custom-stop-hook" {
		t.Fatalf("expected custom sibling to survive, got command %v", command)
	}
}
