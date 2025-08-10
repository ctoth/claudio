package fs

import (
	"os"

	"github.com/spf13/afero"
)

// Factory provides filesystem instances for production and testing
type Factory interface {
	// Production returns a filesystem that operates on the real OS filesystem
	Production() afero.Fs
	// Memory returns an in-memory filesystem for testing
	Memory() afero.Fs
}

// DefaultFactory provides the standard filesystem factory implementation
type DefaultFactory struct{}

// NewDefaultFactory creates a new filesystem factory
func NewDefaultFactory() Factory {
	return &DefaultFactory{}
}

// Production returns a filesystem that operates on the real OS filesystem
func (f *DefaultFactory) Production() afero.Fs {
	return afero.NewOsFs()
}

// Memory returns an in-memory filesystem for testing
func (f *DefaultFactory) Memory() afero.Fs {
	return afero.NewMemMapFs()
}

// ExecutablePath returns the current executable path with filesystem abstraction support
// This is a utility function that can be easily mocked in tests
func ExecutablePath() (string, error) {
	return os.Executable()
}

// MockExecutablePath can be used in tests to override the executable path
var MockExecutablePath func() (string, error)

// TestExecutablePath returns the executable path, using mock if set (for testing)
func TestExecutablePath() (string, error) {
	if MockExecutablePath != nil {
		return MockExecutablePath()
	}
	return ExecutablePath()
}