package audio

import (
	"errors"
	"testing"
)

// TestBackendFactoryInterface tests that BackendFactory interface is properly defined
func TestBackendFactoryInterface(t *testing.T) {
	// This test ensures the interface compiles and has expected methods
	var _ BackendFactory = (*DefaultBackendFactory)(nil)
}

func TestNewBackendFactory(t *testing.T) {
	factory := NewBackendFactory()
	if factory == nil {
		t.Error("NewBackendFactory should return a non-nil factory")
	}
}

func TestBackendFactory_CreateBackend(t *testing.T) {
	tests := []struct {
		name            string
		backendType     string
		isWSL           bool
		availableCommands []string
		expectedType    string
		expectError     bool
	}{
		{
			name:        "auto - WSL with paplay",
			backendType: "auto",
			isWSL:       true,
			availableCommands: []string{"paplay"},
			expectedType: "system_command",
			expectError:  false,
		},
		{
			name:        "auto - WSL with ffplay (no paplay)",
			backendType: "auto",
			isWSL:       true,
			availableCommands: []string{"ffplay"},
			expectedType: "system_command",
			expectError:  false,
		},
		{
			name:        "auto - WSL with no audio commands",
			backendType: "auto",
			isWSL:       true,
			availableCommands: []string{},
			expectedType: "malgo",
			expectError:  false,
		},
		{
			name:        "auto - native Linux",
			backendType: "auto",
			isWSL:       false,
			availableCommands: []string{"paplay"},
			expectedType: "malgo",
			expectError:  false,
		},
		{
			name:        "explicit system_command - paplay available",
			backendType: "system_command",
			isWSL:       false,
			availableCommands: []string{"paplay"},
			expectedType: "system_command",
			expectError:  false,
		},
		{
			name:        "explicit system_command - no commands available",
			backendType: "system_command",
			isWSL:       false,
			availableCommands: []string{},
			expectedType: "",
			expectError:  true,
		},
		{
			name:        "explicit malgo",
			backendType: "malgo",
			isWSL:       true,
			availableCommands: []string{"paplay"},
			expectedType: "malgo",
			expectError:  false,
		},
		{
			name:        "invalid backend type",
			backendType: "invalid",
			isWSL:       false,
			availableCommands: []string{},
			expectedType: "",
			expectError:  true,
		},
		{
			name:        "empty backend type defaults to auto",
			backendType: "",
			isWSL:       false,
			availableCommands: []string{},
			expectedType: "malgo",
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create factory with dependency injection for testing
			factory := NewBackendFactoryWithDependencies(
				func() bool { return tt.isWSL },
				func(cmd string) bool {
					for _, available := range tt.availableCommands {
						if cmd == available {
							return true
						}
					}
					return false
				},
			)

			backend, err := factory.CreateBackend(tt.backendType)

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
					// Check backend type by trying to cast to expected types
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
			}
		})
	}
}

func TestBackendFactory_GetSupportedBackends(t *testing.T) {
	factory := NewBackendFactory()
	supported := factory.GetSupportedBackends()

	expectedBackends := []string{"auto", "system_command", "malgo"}
	if len(supported) != len(expectedBackends) {
		t.Errorf("expected %d supported backends, got %d", len(expectedBackends), len(supported))
	}

	for _, expected := range expectedBackends {
		found := false
		for _, actual := range supported {
			if actual == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected backend %q not found in supported list: %v", expected, supported)
		}
	}
}

func TestBackendFactory_SystemCommandSelection(t *testing.T) {
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
			factory := NewBackendFactoryWithDependencies(
				func() bool { return false }, // Not WSL for these tests
				func(cmd string) bool {
					for _, available := range tt.availableCommands {
						if cmd == available {
							return true
						}
					}
					return false
				},
			)

			backend, err := factory.CreateBackend("system_command")

			if tt.expectError && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if !tt.expectError {
				systemBackend, ok := backend.(*SystemCommandBackend)
				if !ok {
					t.Errorf("expected SystemCommandBackend, got %T", backend)
				} else {
					// We'll need to expose the command somehow for testing
					// For now, just verify it's a SystemCommandBackend
					_ = systemBackend
				}
			}
		})
	}
}

func TestBackendFactory_ValidateBackendType(t *testing.T) {
	factory := NewBackendFactory()

	validTypes := []string{"auto", "system_command", "malgo", ""} // empty defaults to auto
	for _, backendType := range validTypes {
		if !factory.IsValidBackendType(backendType) {
			t.Errorf("backend type %q should be valid", backendType)
		}
	}

	invalidTypes := []string{"invalid", "unknown", "pulseaudio", "alsa"}
	for _, backendType := range invalidTypes {
		if factory.IsValidBackendType(backendType) {
			t.Errorf("backend type %q should be invalid", backendType)
		}
	}
}

func TestBackendFactory_ErrorHandling(t *testing.T) {
	factory := NewBackendFactory()

	// Test invalid backend type
	_, err := factory.CreateBackend("nonexistent")
	if err == nil {
		t.Error("expected error for invalid backend type")
	}
	if !errors.Is(err, ErrInvalidBackendType) {
		t.Errorf("expected ErrInvalidBackendType, got %v", err)
	}

	// Test system_command with no available commands
	factory = NewBackendFactoryWithDependencies(
		func() bool { return false },
		func(cmd string) bool { return false }, // No commands available
	)

	_, err = factory.CreateBackend("system_command")
	if err == nil {
		t.Error("expected error when no system commands available")
	}
	if !errors.Is(err, ErrBackendNotAvailable) {
		t.Errorf("expected ErrBackendNotAvailable, got %v", err)
	}
}

func TestBackendFactory_RealSystemIntegration(t *testing.T) {
	// Test against real system
	factory := NewBackendFactory()

	// Test auto detection
	backend, err := factory.CreateBackend("auto")
	if err != nil {
		t.Errorf("auto backend creation failed: %v", err)
	}
	if backend == nil {
		t.Error("auto backend should not be nil")
	}

	t.Logf("Real system auto backend type: %T", backend)

	// Test malgo (should always work)
	malgoBackend, err := factory.CreateBackend("malgo")
	if err != nil {
		t.Errorf("malgo backend creation failed: %v", err)
	}
	if malgoBackend == nil {
		t.Error("malgo backend should not be nil")
	}

	// Clean up backends
	if backend != nil {
		backend.Close()
	}
	if malgoBackend != nil {
		malgoBackend.Close()
	}
}