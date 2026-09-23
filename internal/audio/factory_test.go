package audio

import (
	"errors"
	"reflect"
	"testing"

	"claudio.click/internal/config"
)

// registerFakeOtoForTest installs a stub "oto" constructor for the
// duration of the test and restores the previous registration on cleanup.
// The audio package itself can't import internal/audio/native (that would
// be a cycle: native imports audio). The fake lets factory_test exercise
// the registration seam without dragging the real output backend into the
// audio package's test binary.
func registerFakeOtoForTest(t *testing.T) {
	t.Helper()
	backendCtorMu.Lock()
	prev, hadPrev := backendCtors["oto"]
	backendCtors["oto"] = func() (AudioBackend, error) {
		return &mockAudioBackend{}, nil
	}
	backendCtorMu.Unlock()

	t.Cleanup(func() {
		backendCtorMu.Lock()
		defer backendCtorMu.Unlock()
		if hadPrev {
			backendCtors["oto"] = prev
		} else {
			delete(backendCtors, "oto")
		}
	})
}

func TestResolveBackendWithChecker(t *testing.T) {
	for _, tc := range []struct {
		name, requested, resolved string
		wsl, command, registered  bool
		wantErr                   error
	}{
		{"native missing", "auto", "oto", false, false, false, ErrBackendNotAvailable},
		{"explicit missing", "oto", "oto", false, false, false, ErrBackendNotAvailable},
		{"native available", "auto", "oto", false, false, true, nil},
		{"empty auto", "", "oto", false, false, true, nil},
		{"WSL without cgo", "auto", "system_command", true, true, false, nil},
		{"WSL missing both", "auto", "oto", true, false, false, ErrBackendNotAvailable},
		{"explicit command", "system_command", "system_command", false, true, false, nil},
		{"missing command", "system_command", "system_command", false, false, false, ErrBackendNotAvailable},
		{"invalid", "unknown", "unknown", false, false, false, ErrInvalidBackendType},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.registered {
				registerFakeOtoForTest(t)
			}
			got, err := resolveBackendWithChecker(tc.requested, tc.wsl, func(string) bool { return tc.command })
			if got != tc.resolved || !errors.Is(err, tc.wantErr) {
				t.Fatalf("got (%q, %v), want (%q, %v)", got, err, tc.resolved, tc.wantErr)
			}
		})
	}
}

func TestNewBackend_WithChecker(t *testing.T) {
	tests := []struct {
		name              string
		backendType       string
		isWSL             bool
		availableCommands []string
		expectedKind      string // "system_command" | "oto" | ""
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
			expectedKind:      "oto",
		},
		{
			name:              "auto - native Linux",
			backendType:       "auto",
			isWSL:             false,
			availableCommands: []string{"paplay"},
			expectedKind:      "oto",
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
			name:              "explicit oto",
			backendType:       "oto",
			isWSL:             true,
			availableCommands: []string{"paplay"},
			expectedKind:      "oto",
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
			expectedKind:      "oto",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registerFakeOtoForTest(t)

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
				case "oto":
					if _, ok := backend.(*mockAudioBackend); !ok {
						t.Errorf("expected fake oto (*mockAudioBackend), got %T", backend)
					}
				}
			}
		})
	}
}

// The factory switch and config validation name the same backends: every
// name config accepts is one the factory knows, and nothing else is.
func TestFactoryKnowsExactlyTheConfigBackends(t *testing.T) {
	for _, name := range config.NewConfigManager().GetSupportedAudioBackends() {
		if _, _, err := planBackend(name, false, func(string) bool { return true }); errors.Is(err, ErrInvalidBackendType) {
			t.Errorf("config accepts %q but the factory rejects it: %v", name, err)
		}
	}
	for _, name := range []string{"invalid", "unknown", "pulseaudio", "alsa", "malgo"} {
		if _, _, err := planBackend(name, false, func(string) bool { return true }); !errors.Is(err, ErrInvalidBackendType) {
			t.Errorf("planBackend(%q) = %v, want ErrInvalidBackendType", name, err)
		}
	}
}

func TestNewBackend_SystemCommandSelection(t *testing.T) {
	tests := []struct {
		name              string
		availableCommands []string
		expectedCommands  []string
		expectError       bool
	}{
		{"paplay preferred", []string{"aplay", "paplay", "ffplay"}, []string{"paplay", "ffplay", "aplay"}, false},
		{"ffplay when no paplay", []string{"aplay", "ffplay"}, []string{"ffplay", "aplay"}, false},
		{"aplay fallback", []string{"aplay"}, []string{"aplay"}, false},
		{"afplay on macOS-like", []string{"afplay"}, []string{"afplay"}, false},
		{"no commands", []string{}, nil, true},
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
				scb, ok := backend.(*SystemCommandBackend)
				if !ok {
					t.Errorf("expected *SystemCommandBackend, got %T", backend)
				}
				if ok && !reflect.DeepEqual(scb.commands, tt.expectedCommands) {
					t.Errorf("commands = %v, want %v", scb.commands, tt.expectedCommands)
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

// TestNewBackend_OtoUnregistered verifies that without registration, the
// "oto" case fails with ErrBackendNotAvailable when the application does not
// import the native implementation, rather than panicking.
func TestNewBackend_OtoUnregistered(t *testing.T) {
	// Snapshot and clear any pre-existing oto registration (e.g. from
	// another test that ran first in the same package binary).
	backendCtorMu.Lock()
	prev, hadPrev := backendCtors["oto"]
	delete(backendCtors, "oto")
	backendCtorMu.Unlock()

	t.Cleanup(func() {
		backendCtorMu.Lock()
		defer backendCtorMu.Unlock()
		if hadPrev {
			backendCtors["oto"] = prev
		}
	})

	_, err := NewBackend("oto")
	if err == nil {
		t.Error("expected error when oto is not registered")
	}
	if !errors.Is(err, ErrBackendNotAvailable) {
		t.Errorf("expected ErrBackendNotAvailable, got %v", err)
	}
}

// The fake backend is test-only (internal/audio/audiotest); the factory does
// not know the name.
func TestNewBackend_FakeIsNotABackendName(t *testing.T) {
	if _, err := NewBackend("fake"); !errors.Is(err, ErrInvalidBackendType) {
		t.Fatalf(`NewBackend("fake") = %v, want ErrInvalidBackendType`, err)
	}
}
