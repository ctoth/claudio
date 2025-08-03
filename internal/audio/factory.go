package audio

import (
	"errors"
	"fmt"
	"log/slog"
)

// BackendFactory creates AudioBackend instances based on configuration
type BackendFactory interface {
	CreateBackend(backendType string) (AudioBackend, error)
	GetSupportedBackends() []string
	IsValidBackendType(backendType string) bool
}

// DefaultBackendFactory implements BackendFactory with platform detection
type DefaultBackendFactory struct {
	isWSLFunc     func() bool
	commandExists func(string) bool
}

// Factory errors
var (
	ErrInvalidBackendType = errors.New("invalid backend type")
	ErrBackendCreationFailed = errors.New("backend creation failed")
)

// NewBackendFactory creates a new DefaultBackendFactory with real platform detection
func NewBackendFactory() *DefaultBackendFactory {
	return &DefaultBackendFactory{
		isWSLFunc:     IsWSL,
		commandExists: CommandExists,
	}
}

// NewBackendFactoryWithDependencies creates a factory with injected dependencies for testing
func NewBackendFactoryWithDependencies(isWSLFunc func() bool, commandExists func(string) bool) *DefaultBackendFactory {
	return &DefaultBackendFactory{
		isWSLFunc:     isWSLFunc,
		commandExists: commandExists,
	}
}

// CreateBackend creates an AudioBackend instance based on the specified type
func (f *DefaultBackendFactory) CreateBackend(backendType string) (AudioBackend, error) {
	// Default empty string to "auto"
	if backendType == "" {
		backendType = "auto"
	}

	slog.Debug("creating audio backend", "type", backendType)

	switch backendType {
	case "auto":
		return f.createAutoBackend()
	case "system_command":
		return f.createSystemCommandBackend()
	case "malgo":
		return f.createMalgoBackend()
	default:
		slog.Error("invalid backend type requested", "type", backendType)
		return nil, fmt.Errorf("%w: %s", ErrInvalidBackendType, backendType)
	}
}

// GetSupportedBackends returns a list of all supported backend types
func (f *DefaultBackendFactory) GetSupportedBackends() []string {
	return []string{"auto", "system_command", "malgo"}
}

// IsValidBackendType checks if a backend type is supported
func (f *DefaultBackendFactory) IsValidBackendType(backendType string) bool {
	// Empty string is valid (defaults to auto)
	if backendType == "" {
		return true
	}
	
	supported := f.GetSupportedBackends()
	for _, supportedType := range supported {
		if backendType == supportedType {
			return true
		}
	}
	return false
}

// createAutoBackend automatically selects the best backend for the current platform
func (f *DefaultBackendFactory) createAutoBackend() (AudioBackend, error) {
	slog.Debug("auto-detecting optimal backend")
	
	optimalType := f.detectOptimalBackendType()
	slog.Debug("auto-detection result", "selected_type", optimalType)
	
	switch optimalType {
	case "system_command":
		return f.createSystemCommandBackend()
	case "malgo":
		return f.createMalgoBackend()
	default:
		slog.Error("auto-detection returned invalid backend type", "type", optimalType)
		return nil, fmt.Errorf("%w: auto-detection failed", ErrBackendCreationFailed)
	}
}

// createSystemCommandBackend creates a SystemCommandBackend with the best available command
func (f *DefaultBackendFactory) createSystemCommandBackend() (AudioBackend, error) {
	slog.Debug("creating system command backend")
	
	preferredCommand := f.getPreferredSystemCommand()
	if preferredCommand == "" {
		slog.Error("no system audio commands available")
		return nil, fmt.Errorf("%w: no system audio commands found", ErrBackendNotAvailable)
	}
	
	slog.Debug("system command backend created", "command", preferredCommand)
	backend := NewSystemCommandBackend(preferredCommand)
	return backend, nil
}

// createMalgoBackend creates a MalgoBackend
func (f *DefaultBackendFactory) createMalgoBackend() (AudioBackend, error) {
	slog.Debug("creating malgo backend")
	backend := NewMalgoBackend()
	slog.Debug("malgo backend created successfully")
	return backend, nil
}

// detectOptimalBackendType uses platform detection to determine the best backend
func (f *DefaultBackendFactory) detectOptimalBackendType() string {
	return detectOptimalBackendWithChecker(f.isWSLFunc(), f.commandExists)
}

// getPreferredSystemCommand finds the best available system audio command
func (f *DefaultBackendFactory) getPreferredSystemCommand() string {
	return getPreferredSystemCommandWithChecker(f.commandExists)
}