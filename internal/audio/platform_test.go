package audio

import (
	"testing"
)

// TestPlatformDetectionInterface tests that platform detection functions are properly defined
func TestPlatformDetectionInterface(t *testing.T) {
	// This test ensures the functions compile and have expected signatures
	_ = IsWSL()
	_ = CommandExists("test")
	_ = DetectOptimalBackend()
}

func TestIsWSL(t *testing.T) {
	tests := []struct {
		name           string
		procVersion    string
		wslEnv         string
		expectedResult bool
	}{
		{
			name:           "WSL1 detected via /proc/version",
			procVersion:    "Linux version 4.4.0-19041-Microsoft (Microsoft@Microsoft.com) (gcc version 5.4.0 (Ubuntu 5.4.0-6ubuntu1~16.04.12) ) #1237-Microsoft Sat Sep 11 14:32:00 PST 2021",
			wslEnv:         "",
			expectedResult: true,
		},
		{
			name:           "WSL2 detected via /proc/version",
			procVersion:    "Linux version 5.15.74.2-microsoft-standard-WSL2 (gcc (GCC) 11.2.0) #1 SMP Wed Oct 5 20:57:03 UTC 2022",
			wslEnv:         "",
			expectedResult: true,
		},
		{
			name:           "WSL detected via WSL_DISTRO_NAME env var",
			procVersion:    "",
			wslEnv:         "Ubuntu",
			expectedResult: true,
		},
		{
			name:           "Native Linux - no WSL indicators",
			procVersion:    "Linux version 5.15.0-56-generic (buildd@lcy02-amd64-044) (gcc (Ubuntu 11.3.0-1ubuntu1~22.04) #62-Ubuntu SMP Tue Nov 22 19:54:14 UTC 2022",
			wslEnv:         "",
			expectedResult: false,
		},
		{
			name:           "Empty proc version and no env var",
			procVersion:    "",
			wslEnv:         "",
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: This test will initially fail since IsWSL is not implemented
			// We'll implement a version that can be tested with mock data
			result := detectWSLFromData(tt.procVersion, tt.wslEnv)
			if result != tt.expectedResult {
				t.Errorf("expected %v, got %v", tt.expectedResult, result)
			}
		})
	}
}

func TestCommandExists(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		expected bool
	}{
		{
			name:     "existing command - echo",
			command:  "echo",
			expected: true,
		},
		{
			name:     "existing command - ls",
			command:  "ls",
			expected: true,
		},
		{
			name:     "non-existent command",
			command:  "nonexistent-command-12345",
			expected: false,
		},
		{
			name:     "empty command",
			command:  "",
			expected: false,
		},
		{
			name:     "command with path separators (should not exist)",
			command:  "/invalid/path/command",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CommandExists(tt.command)
			if result != tt.expected {
				t.Errorf("CommandExists(%q) = %v, expected %v", tt.command, result, tt.expected)
			}
		})
	}
}

func TestDetectOptimalBackend(t *testing.T) {
	tests := []struct {
		name              string
		isWSL             bool
		availableCommands []string
		expectedBackend   string
	}{
		{
			name:              "WSL with paplay available",
			isWSL:             true,
			availableCommands: []string{"paplay"},
			expectedBackend:   "system_command",
		},
		{
			name:              "WSL with ffplay available (no paplay)",
			isWSL:             true,
			availableCommands: []string{"ffplay"},
			expectedBackend:   "system_command",
		},
		{
			name:              "WSL with no audio commands available",
			isWSL:             true,
			availableCommands: []string{},
			expectedBackend:   "malgo", // Fallback to malgo even in WSL
		},
		{
			name:              "Native Linux with paplay",
			isWSL:             false,
			availableCommands: []string{"paplay"},
			expectedBackend:   "malgo", // Prefer malgo on native Linux
		},
		{
			name:              "Native Linux without audio commands",
			isWSL:             false,
			availableCommands: []string{},
			expectedBackend:   "malgo", // Default to malgo
		},
		{
			name:              "macOS-like environment",
			isWSL:             false,
			availableCommands: []string{"afplay"},
			expectedBackend:   "malgo", // Still prefer malgo on native systems
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock command existence checker
			commandChecker := func(cmd string) bool {
				for _, available := range tt.availableCommands {
					if cmd == available {
						return true
					}
				}
				return false
			}

			result := detectOptimalBackendWithChecker(tt.isWSL, commandChecker)
			if result != tt.expectedBackend {
				t.Errorf("expected backend %q, got %q", tt.expectedBackend, result)
			}
		})
	}
}

func TestGetPreferredSystemCommand(t *testing.T) {
	tests := []struct {
		name              string
		availableCommands []string
		expectedCommand   string
		expectEmpty       bool
	}{
		{
			name:              "paplay is preferred",
			availableCommands: []string{"paplay", "ffplay", "aplay"},
			expectedCommand:   "paplay",
			expectEmpty:       false,
		},
		{
			name:              "ffplay when paplay not available",
			availableCommands: []string{"ffplay", "aplay"},
			expectedCommand:   "ffplay",
			expectEmpty:       false,
		},
		{
			name:              "aplay when others not available",
			availableCommands: []string{"aplay"},
			expectedCommand:   "aplay",
			expectEmpty:       false,
		},
		{
			name:              "afplay on macOS",
			availableCommands: []string{"afplay"},
			expectedCommand:   "afplay",
			expectEmpty:       false,
		},
		{
			name:              "no audio commands available",
			availableCommands: []string{},
			expectedCommand:   "",
			expectEmpty:       true,
		},
		{
			name:              "multiple commands - paplay wins",
			availableCommands: []string{"aplay", "paplay", "ffplay"},
			expectedCommand:   "paplay",
			expectEmpty:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			commandChecker := func(cmd string) bool {
				for _, available := range tt.availableCommands {
					if cmd == available {
						return true
					}
				}
				return false
			}

			result := getPreferredSystemCommandWithChecker(commandChecker)
			if tt.expectEmpty && result != "" {
				t.Errorf("expected empty result, got %q", result)
			}
			if !tt.expectEmpty && result != tt.expectedCommand {
				t.Errorf("expected command %q, got %q", tt.expectedCommand, result)
			}
		})
	}
}

// TestRealSystemIntegration tests against the real system (these may vary by environment)
func TestRealSystemIntegration(t *testing.T) {
	t.Run("real WSL detection", func(t *testing.T) {
		// This will test the actual IsWSL() function against the real system
		result := IsWSL()
		t.Logf("Real system WSL detection: %v", result)

		// We can't assert a specific value since it depends on the test environment,
		// but we can ensure the function doesn't panic
	})

	t.Run("real command detection", func(t *testing.T) {
		// Test some commands that should exist on most systems
		echoExists := CommandExists("echo")
		if !echoExists {
			t.Error("echo command should exist on most systems")
		}

		lsExists := CommandExists("ls")
		if !lsExists {
			t.Error("ls command should exist on most Unix-like systems")
		}

		fakeExists := CommandExists("definitely-does-not-exist-12345")
		if fakeExists {
			t.Error("fake command should not exist")
		}
	})

	t.Run("real backend detection", func(t *testing.T) {
		backend := DetectOptimalBackend()
		t.Logf("Real system optimal backend: %s", backend)

		// Should return one of our known backend types
		validBackends := map[string]bool{
			"malgo":          true,
			"system_command": true,
		}

		if !validBackends[backend] {
			t.Errorf("DetectOptimalBackend returned invalid backend: %s", backend)
		}
	})
}

// Test helper functions that we'll need to implement for dependency injection
func TestHelperFunctions(t *testing.T) {
	t.Run("detectWSLFromData should be implemented", func(t *testing.T) {
		// Test the helper function we use for testing WSL detection
		result := detectWSLFromData("Linux version 5.15.74.2-microsoft-standard-WSL2", "")
		if !result {
			t.Error("should detect WSL2 from proc version")
		}

		result = detectWSLFromData("", "Ubuntu")
		if !result {
			t.Error("should detect WSL from environment variable")
		}

		result = detectWSLFromData("regular linux", "")
		if result {
			t.Error("should not detect WSL from regular linux")
		}
	})
}
