package audio

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"claudio.click/internal/platform"
)

// ErrInvalidBackendType reports a backend name the factory does not know.
var ErrInvalidBackendType = errors.New("invalid backend type")

// BackendConstructor builds an AudioBackend instance.
type BackendConstructor func() (AudioBackend, error)

var (
	backendCtorMu sync.RWMutex
	backendCtors  = map[string]BackendConstructor{}
)

// RegisterBackend registers a constructor for the given backend type and
// returns the one it replaced (nil if none); registering nil removes the
// entry. It is called from an init() in a backend's subpackage so the
// top-level audio package does not need to import the implementation: the
// native subpackage registers "oto" in every build. Tests swap in a fake
// the same way (internal/audio/audiotest).
func RegisterBackend(name string, ctor BackendConstructor) (previous BackendConstructor) {
	backendCtorMu.Lock()
	defer backendCtorMu.Unlock()
	previous = backendCtors[name]
	if ctor == nil {
		delete(backendCtors, name)
	} else {
		backendCtors[name] = ctor
	}
	return previous
}

func lookupBackendConstructor(name string) (BackendConstructor, bool) {
	backendCtorMu.RLock()
	defer backendCtorMu.RUnlock()
	ctor, ok := backendCtors[name]
	return ctor, ok
}

// NewBackend constructs an audio backend by name. "auto" (or empty) picks
// one for the platform (platform.go).
func NewBackend(backendType string) (AudioBackend, error) {
	return newBackendWithChecker(backendType, platform.IsWSL, CommandExists)
}

// ResolveBackend reports the selected backend and whether its implementation
// is available. It checks registrations and executables without opening an
// audio device; availability does not guarantee successful playback.
func ResolveBackend(backendType string) (string, error) {
	return resolveBackendWithChecker(backendType, platform.IsWSL(), CommandExists)
}

func resolveBackendWithChecker(backendType string, isWSL bool, commandExists func(string) bool) (string, error) {
	name, _, err := planBackend(backendType, isWSL, commandExists)
	return name, err
}

// newBackendWithChecker is the test seam for NewBackend.
func newBackendWithChecker(backendType string, isWSL func() bool, commandExists func(string) bool) (AudioBackend, error) {
	name, construct, err := planBackend(backendType, isWSL(), commandExists)
	if err != nil {
		slog.Debug("audio backend unavailable", "requested", backendType, "resolved", name, "error", err)
		return nil, err
	}
	slog.Debug("creating audio backend", "requested", backendType, "resolved", name)
	return construct()
}

// planBackend is the one place that knows the backend names: it resolves
// "auto", checks availability, and returns the constructor to call.
func planBackend(backendType string, isWSL bool, commandExists func(string) bool) (string, BackendConstructor, error) {
	if backendType == "" || backendType == "auto" {
		backendType = detectOptimalBackendWithChecker(isWSL, commandExists)
	}
	switch backendType {
	case "system_command":
		// Every available command, in priority order, so playback can fall
		// back when the first cannot handle a file.
		commands := getAvailableSystemCommandsWithChecker(commandExists)
		if len(commands) == 0 {
			return backendType, nil, fmt.Errorf("%w: no system audio commands found", ErrBackendNotAvailable)
		}
		return backendType, func() (AudioBackend, error) { return NewSystemCommandBackend(commands...), nil }, nil
	case "oto":
		ctor, ok := lookupBackendConstructor(backendType)
		if !ok {
			return backendType, nil, fmt.Errorf("%w: %s backend not registered", ErrBackendNotAvailable, backendType)
		}
		return backendType, ctor, nil
	}
	return backendType, nil, fmt.Errorf("%w: %s", ErrInvalidBackendType, backendType)
}
