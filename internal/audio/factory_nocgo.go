//go:build !cgo

package audio

import (
	"errors"
	"fmt"
)

// Factory errors (mirrored from factory.go cgo half).
var (
	ErrInvalidBackendType    = errors.New("invalid backend type")
	ErrBackendCreationFailed = errors.New("backend creation failed")
)

// SupportedBackendTypes lists every backend type accepted by NewBackend.
// Under !cgo only system_command is functional; malgo and auto return
// errCGORequired.
var SupportedBackendTypes = []string{"auto", "system_command", "malgo"}

// IsValidBackendType reports whether the given backend type string is
// accepted by NewBackend.
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

// NewBackend constructs an audio backend by name. Under !cgo, only
// system_command (and auto when a system command is available) is
// functional; malgo returns errCGORequired.
func NewBackend(backendType string) (AudioBackend, error) {
	if backendType == "" {
		backendType = "auto"
	}
	switch backendType {
	case "system_command":
		preferred := getPreferredSystemCommandWithChecker(CommandExists)
		if preferred == "" {
			return nil, fmt.Errorf("%w: no system audio commands found", ErrBackendNotAvailable)
		}
		return NewSystemCommandBackend(preferred), nil
	case "auto":
		// Without cgo, the only viable backend is system_command.
		preferred := getPreferredSystemCommandWithChecker(CommandExists)
		if preferred == "" {
			return nil, errCGORequired
		}
		return NewSystemCommandBackend(preferred), nil
	case "malgo":
		return nil, errCGORequired
	default:
		return nil, fmt.Errorf("%w: %s", ErrInvalidBackendType, backendType)
	}
}
