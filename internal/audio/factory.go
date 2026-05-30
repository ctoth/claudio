//go:build cgo

package audio

import (
	"errors"
	"fmt"
	"log/slog"
)

// Factory errors
var (
	ErrInvalidBackendType    = errors.New("invalid backend type")
	ErrBackendCreationFailed = errors.New("backend creation failed")
)

// SupportedBackendTypes lists every backend type accepted by NewBackend.
// Empty string is a synonym for "auto".
var SupportedBackendTypes = []string{"auto", "system_command", "malgo"}

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

// NewBackend constructs an audio backend by name. "auto" (or empty) delegates
// to platform-aware selection via platform.go.
//
// The previous BackendFactory + DefaultBackendFactory + DI pattern existed to
// select between two concrete backends; three types and one constructor for
// what's now a switch statement.
func NewBackend(backendType string) (AudioBackend, error) {
	return newBackendWithChecker(backendType, IsWSL, CommandExists)
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
			return NewMalgoBackend(), nil
		default:
			slog.Error("auto-detection returned invalid backend type", "type", optimal)
			return nil, fmt.Errorf("%w: auto-detection failed", ErrBackendCreationFailed)
		}
	case "system_command":
		return createSystemCommandBackendWithChecker(commandExists)
	case "malgo":
		slog.Debug("creating malgo backend")
		return NewMalgoBackend(), nil
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
