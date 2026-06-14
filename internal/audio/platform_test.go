package audio

import (
	"reflect"
	"runtime"
	"testing"
)

// TestPlatformDetectionInterface tests that the audio package's
// command/backend detection helpers compile and have expected
// signatures. WSL detection itself now lives in internal/platform.
func TestPlatformDetectionInterface(t *testing.T) {
	_ = CommandExists("test")
	_ = DetectOptimalBackend()
}

func TestCommandExists(t *testing.T) {
	firstExisting, secondExisting := existingCommandFixtures()
	tests := []struct {
		name     string
		command  string
		expected bool
	}{
		{
			name:     "existing command - primary",
			command:  firstExisting,
			expected: true,
		},
		{
			name:     "existing command - secondary",
			command:  secondExisting,
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

func TestGetAvailableSystemCommands(t *testing.T) {
	tests := []struct {
		name              string
		availableCommands []string
		want              []string
	}{
		{
			name:              "returns all commands in priority order",
			availableCommands: []string{"aplay", "paplay", "afplay", "ffplay"},
			want:              []string{"paplay", "ffplay", "aplay", "afplay"},
		},
		{
			name:              "returns subset in priority order",
			availableCommands: []string{"aplay", "ffplay"},
			want:              []string{"ffplay", "aplay"},
		},
		{
			name:              "returns empty slice when none are available",
			availableCommands: []string{},
			want:              nil,
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

			got := getAvailableSystemCommandsWithChecker(commandChecker)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("getAvailableSystemCommandsWithChecker() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestRealSystemIntegration tests against the real system (these may vary by environment)
func TestRealSystemIntegration(t *testing.T) {
	t.Run("real command detection", func(t *testing.T) {
		firstExisting, secondExisting := existingCommandFixtures()
		if !CommandExists(firstExisting) {
			t.Errorf("%s command should exist on this system", firstExisting)
		}

		if !CommandExists(secondExisting) {
			t.Errorf("%s command should exist on this system", secondExisting)
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

func existingCommandFixtures() (string, string) {
	if runtime.GOOS == "windows" {
		return "cmd.exe", "where.exe"
	}
	return "sh", "true"
}
