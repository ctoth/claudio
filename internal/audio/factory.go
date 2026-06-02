package audio

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"claudio.click/internal/platform"
)

// Factory errors
var (
	ErrInvalidBackendType    = errors.New("invalid backend type")
	ErrBackendCreationFailed = errors.New("backend creation failed")
)

// SupportedBackendTypes lists every backend type accepted by NewBackend.
// Empty string is a synonym for "auto". "fake" is a test-only backend
// included unconditionally so cross-package tests (notably internal/cli)
// can configure cfg.AudioBackend = "fake" without rebuilding under a
// special tag.
var SupportedBackendTypes = []string{"auto", "system_command", "malgo", "fake"}

// IsValidBackendType reports whether the given backend type string is
// accepted by NewBackend. Empty string is treated as "auto".
func IsValidBackendType(backendType string) bool {
	if backendType == "" {
		return true
	}
	for _, t := range SupportedBackendTypes {
		if backendType == t {
			return true
		}
	}
	return false
}

// BackendConstructor builds an AudioBackend instance.
type BackendConstructor func() (AudioBackend, error)

var (
	backendCtorMu sync.RWMutex
	backendCtors  = map[string]BackendConstructor{}
)

// RegisterBackend registers a constructor for the given backend type. It
// is intended to be called from an init() in a backend's subpackage so
// the top-level audio package does not need to import the backend's
// implementation. Concretely: the malgo subpackage registers itself
// under "malgo" via an init(), guarded by //go:build cgo. Under !cgo no
// registration happens and NewBackend("malgo") returns errCGORequired.
func RegisterBackend(name string, ctor BackendConstructor) {
	backendCtorMu.Lock()
	defer backendCtorMu.Unlock()
	backendCtors[name] = ctor
}

func lookupBackendConstructor(name string) (BackendConstructor, bool) {
	backendCtorMu.RLock()
	defer backendCtorMu.RUnlock()
	ctor, ok := backendCtors[name]
	return ctor, ok
}

// NewBackend constructs an audio backend by name. "auto" (or empty) delegates
// to platform-aware selection via platform.go.
//
// The previous BackendFactory + DefaultBackendFactory + DI pattern existed to
// select between two concrete backends; three types and one constructor for
// what is now a switch statement.
func NewBackend(backendType string) (AudioBackend, error) {
	return newBackendWithChecker(backendType, platform.IsWSL, CommandExists)
}

// newBackendWithChecker is the seam used by tests to inject platform detection
// without rebuilding the whole factory-with-dependencies dance. Production code
// goes through NewBackend.
func newBackendWithChecker(backendType string, isWSLFunc func() bool, commandExists func(string) bool) (AudioBackend, error) {
	if backendType == "" {
		backendType = "auto"
	}

	slog.Debug("creating audio backend", "type", backendType)

	switch backendType {
	case "auto":
		optimal := detectOptimalBackendWithChecker(isWSLFunc(), commandExists)
		slog.Debug("auto-detection result", "selected_type", optimal)
		switch optimal {
		case "system_command":
			return createSystemCommandBackendWithChecker(commandExists)
		case "malgo":
			return createRegisteredBackend("malgo")
		default:
			slog.Error("auto-detection returned invalid backend type", "type", optimal)
			return nil, fmt.Errorf("%w: auto-detection failed", ErrBackendCreationFailed)
		}
	case "system_command":
		return createSystemCommandBackendWithChecker(commandExists)
	case "malgo":
		return createRegisteredBackend("malgo")
	case "fake":
		return createRegisteredBackend("fake")
	default:
		slog.Error("invalid backend type requested", "type", backendType)
		return nil, fmt.Errorf("%w: %s", ErrInvalidBackendType, backendType)
	}
}

// createSystemCommandBackendWithChecker picks the best available system
// audio command and constructs a SystemCommandBackend.
func createSystemCommandBackendWithChecker(commandExists func(string) bool) (AudioBackend, error) {
	preferred := getPreferredSystemCommandWithChecker(commandExists)
	if preferred == "" {
		slog.Error("no system audio commands available")
		return nil, fmt.Errorf("%w: no system audio commands found", ErrBackendNotAvailable)
	}
	slog.Debug("system command backend created", "command", preferred)
	return NewSystemCommandBackend(preferred), nil
}

// createRegisteredBackend instantiates a backend whose constructor was
// registered via RegisterBackend. Returns ErrBackendNotAvailable if no
// registration is present — this is how the !cgo build communicates
// "malgo unavailable, you need to build with cgo" without dragging the
// malgo subpackage into the audio package's import graph.
func createRegisteredBackend(name string) (AudioBackend, error) {
	ctor, ok := lookupBackendConstructor(name)
	if !ok {
		return nil, fmt.Errorf("%w: %s backend not registered (build with cgo?)", ErrBackendNotAvailable, name)
	}
	return ctor()
}
