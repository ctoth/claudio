package audio

import (
	"errors"
	"testing"
)

// registerFakeMalgoForTest installs a stub "malgo" constructor for the
// duration of the test and restores the previous registration on cleanup.
// The audio package itself can't import internal/audio/malgo (that would
// be a cycle: malgo imports audio). The fake lets factory_test exercise
// the registration seam without dragging the real cgo backend into the
// audio package's test binary.
func registerFakeMalgoForTest(t *testing.T) {
	t.Helper()
	backendCtorMu.Lock()
	prev, hadPrev := backendCtors["malgo"]
	backendCtors["malgo"] = func() (AudioBackend, error) {
		return &mockAudioBackend{}, nil
	}
	backendCtorMu.Unlock()

	t.Cleanup(func() {
		backendCtorMu.Lock()
		defer backendCtorMu.Unlock()
		if hadPrev {
			backendCtors["malgo"] = prev
		} else {
			delete(backendCtors, "malgo")
		}
	})
}

func TestNewBackend_WithChecker(t *testing.T) {
	tests := []struct {
		name              string
		backendType       string
		isWSL             bool
		availableCommands []string
		expectedKind      string // "system_command" | "malgo" | ""
		expectError       bool
	}{
		{
			name:              "auto - WSL with paplay",
			backendType:       "auto",
			isWSL:             true,
			availableCommands: []string{"paplay"},
			expectedKind:      "system_command",
		},
		{
			name:              "auto - WSL with no audio commands",
			backendType:       "auto",
			isWSL:             true,
			availableCommands: []string{},
			expectedKind:      "malgo",
		},
		{
			name:              "auto - native Linux",
			backendType:       "auto",
			isWSL:             false,
			availableCommands: []string{"paplay"},
			expectedKind:      "malgo",
		},
		{
			name:              "explicit system_command - paplay available",
			backendType:       "system_command",
			isWSL:             false,
			availableCommands: []string{"paplay"},
			expectedKind:      "system_command",
		},
		{
			name:              "explicit system_command - no commands available",
			backendType:       "system_command",
			isWSL:             false,
			availableCommands: []string{},
			expectError:       true,
		},
		{
			name:              "explicit malgo",
			backendType:       "malgo",
			isWSL:             true,
			availableCommands: []string{"paplay"},
			expectedKind:      "malgo",
		},
		{
			name:              "invalid backend type",
			backendType:       "invalid",
			isWSL:             false,
			availableCommands: []string{},
			expectError:       true,
		},
		{
			name:              "empty backend type defaults to auto",
			backendType:       "",
			isWSL:             false,
			availableCommands: []string{},
			expectedKind:      "malgo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registerFakeMalgoForTest(t)

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
					t.Fatal("expected non-nil backend")
				}
				switch tt.expectedKind {
				case "system_command":
					if _, ok := backend.(*SystemCommandBackend); !ok {
						t.Errorf("expected *SystemCommandBackend, got %T", backend)
					}
				case "malgo":
					if _, ok := backend.(*mockAudioBackend); !ok {
						t.Errorf("expected fake malgo (*mockAudioBackend), got %T", backend)
					}
				}
			}
		})
	}
}

func TestSupportedBackendTypes(t *testing.T) {
	expected := []string{"auto", "system_command", "malgo", "fake"}
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
	validTypes := []string{"auto", "system_command", "malgo", "fake", ""}
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
		expectError       bool
	}{
		{"paplay preferred", []string{"aplay", "paplay", "ffplay"}, false},
		{"ffplay when no paplay", []string{"aplay", "ffplay"}, false},
		{"aplay fallback", []string{"aplay"}, false},
		{"afplay on macOS-like", []string{"afplay"}, false},
		{"no commands", []string{}, true},
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
					t.Errorf("expected *SystemCommandBackend, got %T", backend)
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

// TestNewBackend_MalgoUnregistered verifies that without registration, the
// "malgo" case fails with ErrBackendNotAvailable. This is the contract the
// !cgo build relies on — the malgo subpackage's init() does not run, so
// the registration is missing and NewBackend("malgo") reports the
// condition explicitly rather than panicking.
func TestNewBackend_MalgoUnregistered(t *testing.T) {
	// Snapshot and clear any pre-existing malgo registration (e.g. from
	// another test that ran first in the same package binary).
	backendCtorMu.Lock()
	prev, hadPrev := backendCtors["malgo"]
	delete(backendCtors, "malgo")
	backendCtorMu.Unlock()

	t.Cleanup(func() {
		backendCtorMu.Lock()
		defer backendCtorMu.Unlock()
		if hadPrev {
			backendCtors["malgo"] = prev
		}
	})

	_, err := NewBackend("malgo")
	if err == nil {
		t.Error("expected error when malgo is not registered")
	}
	if !errors.Is(err, ErrBackendNotAvailable) {
		t.Errorf("expected ErrBackendNotAvailable, got %v", err)
	}
}
