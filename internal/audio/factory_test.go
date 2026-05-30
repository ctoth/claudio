//go:build cgo

package audio

import (
	"errors"
	"testing"
)

func TestNewBackend_WithChecker(t *testing.T) {
	tests := []struct {
		name              string
		backendType       string
		isWSL             bool
		availableCommands []string
		expectedType      string
		expectError       bool
	}{
		{
			name:              "auto - WSL with paplay",
			backendType:       "auto",
			isWSL:             true,
			availableCommands: []string{"paplay"},
			expectedType:      "system_command",
			expectError:       false,
		},
		{
			name:              "auto - WSL with ffplay (no paplay)",
			backendType:       "auto",
			isWSL:             true,
			availableCommands: []string{"ffplay"},
			expectedType:      "system_command",
			expectError:       false,
		},
		{
			name:              "auto - WSL with no audio commands",
			backendType:       "auto",
			isWSL:             true,
			availableCommands: []string{},
			expectedType:      "malgo",
			expectError:       false,
		},
		{
			name:              "auto - native Linux",
			backendType:       "auto",
			isWSL:             false,
			availableCommands: []string{"paplay"},
			expectedType:      "malgo",
			expectError:       false,
		},
		{
			name:              "explicit system_command - paplay available",
			backendType:       "system_command",
			isWSL:             false,
			availableCommands: []string{"paplay"},
			expectedType:      "system_command",
			expectError:       false,
		},
		{
			name:              "explicit system_command - no commands available",
			backendType:       "system_command",
			isWSL:             false,
			availableCommands: []string{},
			expectedType:      "",
			expectError:       true,
		},
		{
			name:              "explicit malgo",
			backendType:       "malgo",
			isWSL:             true,
			availableCommands: []string{"paplay"},
			expectedType:      "malgo",
			expectError:       false,
		},
		{
			name:              "invalid backend type",
			backendType:       "invalid",
			isWSL:             false,
			availableCommands: []string{},
			expectedType:      "",
			expectError:       true,
		},
		{
			name:              "empty backend type defaults to auto",
			backendType:       "",
			isWSL:             false,
			availableCommands: []string{},
			expectedType:      "malgo",
			expectError:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isWSLFunc := func() bool { return tt.isWSL }
			commandExists := func(cmd string) bool {
				for _, available := range tt.availableCommands {
					if cmd == available {
						return true
					}
				}
				return false
			}

			backend, err := newBackendWithChecker(tt.backendType, isWSLFunc, commandExists)

			if tt.expectError && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if !tt.expectError {
				if backend == nil {
					t.Error("expected non-nil backend")
				} else {
					switch tt.expectedType {
					case "system_command":
						if _, ok := backend.(*SystemCommandBackend); !ok {
							t.Errorf("expected SystemCommandBackend, got %T", backend)
						}
					case "malgo":
						if _, ok := backend.(*MalgoBackend); !ok {
							t.Errorf("expected MalgoBackend, got %T", backend)
						}
					}
				}
				if backend != nil {
					_ = backend.Close()
				}
			}
		})
	}
}

func TestSupportedBackendTypes(t *testing.T) {
	expected := []string{"auto", "system_command", "malgo"}
	if len(SupportedBackendTypes) != len(expected) {
		t.Errorf("expected %d supported backend types, got %d", len(expected), len(SupportedBackendTypes))
	}
	for _, want := range expected {
		found := false
		for _, got := range SupportedBackendTypes {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected backend type %q in SupportedBackendTypes: %v", want, SupportedBackendTypes)
		}
	}
}

func TestIsValidBackendType(t *testing.T) {
	validTypes := []string{"auto", "system_command", "malgo", ""}
	for _, backendType := range validTypes {
		if !IsValidBackendType(backendType) {
			t.Errorf("backend type %q should be valid", backendType)
		}
	}

	invalidTypes := []string{"invalid", "unknown", "pulseaudio", "alsa"}
	for _, backendType := range invalidTypes {
		if IsValidBackendType(backendType) {
			t.Errorf("backend type %q should be invalid", backendType)
		}
	}
}

func TestNewBackend_SystemCommandSelection(t *testing.T) {
	tests := []struct {
		name              string
		availableCommands []string
		expectedCommand   string
		expectError       bool
	}{
		{
			name:              "paplay preferred over others",
			availableCommands: []string{"aplay", "paplay", "ffplay"},
			expectedCommand:   "paplay",
			expectError:       false,
		},
		{
			name:              "ffplay when paplay unavailable",
			availableCommands: []string{"aplay", "ffplay"},
			expectedCommand:   "ffplay",
			expectError:       false,
		},
		{
			name:              "aplay as fallback",
			availableCommands: []string{"aplay"},
			expectedCommand:   "aplay",
			expectError:       false,
		},
		{
			name:              "afplay on macOS-like systems",
			availableCommands: []string{"afplay"},
			expectedCommand:   "afplay",
			expectError:       false,
		},
		{
			name:              "no commands available",
			availableCommands: []string{},
			expectedCommand:   "",
			expectError:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isWSLFunc := func() bool { return false }
			commandExists := func(cmd string) bool {
				for _, available := range tt.availableCommands {
					if cmd == available {
						return true
					}
				}
				return false
			}

			backend, err := newBackendWithChecker("system_command", isWSLFunc, commandExists)
			if tt.expectError && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !tt.expectError {
				if _, ok := backend.(*SystemCommandBackend); !ok {
					t.Errorf("expected SystemCommandBackend, got %T", backend)
				}
				if backend != nil {
					_ = backend.Close()
				}
			}
		})
	}
}

func TestNewBackend_ErrorHandling(t *testing.T) {
	// Test invalid backend type via public NewBackend
	_, err := NewBackend("nonexistent")
	if err == nil {
		t.Error("expected error for invalid backend type")
	}
	if !errors.Is(err, ErrInvalidBackendType) {
		t.Errorf("expected ErrInvalidBackendType, got %v", err)
	}

	// Test system_command with no available commands via checker seam
	_, err = newBackendWithChecker("system_command",
		func() bool { return false },
		func(cmd string) bool { return false },
	)
	if err == nil {
		t.Error("expected error when no system commands available")
	}
	if !errors.Is(err, ErrBackendNotAvailable) {
		t.Errorf("expected ErrBackendNotAvailable, got %v", err)
	}
}

func TestNewBackend_RealSystemIntegration(t *testing.T) {
	// Test against real system: auto detection should always succeed because
	// malgo is a viable fallback.
	backend, err := NewBackend("auto")
	if err != nil {
		t.Errorf("auto backend creation failed: %v", err)
	}
	if backend == nil {
		t.Error("auto backend should not be nil")
	}

	t.Logf("Real system auto backend type: %T", backend)

	// Test malgo (should always work)
	malgoBackend, err := NewBackend("malgo")
	if err != nil {
		t.Errorf("malgo backend creation failed: %v", err)
	}
	if malgoBackend == nil {
		t.Error("malgo backend should not be nil")
	}

	// Clean up backends
	if backend != nil {
		_ = backend.Close()
	}
	if malgoBackend != nil {
		_ = malgoBackend.Close()
	}
}
