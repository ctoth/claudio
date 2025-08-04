package cli

import (
	"log/slog"

	"golang.org/x/term"
)

// TerminalDetector defines the interface for terminal detection
// This allows for mocking in tests and dependency injection
type TerminalDetector interface {
	IsTerminal(fd int) bool
}

// DefaultTerminalDetector is the default implementation using golang.org/x/term
type DefaultTerminalDetector struct{}

// IsTerminal implements TerminalDetector interface
func (d *DefaultTerminalDetector) IsTerminal(fd int) bool {
	slog.Debug("checking if file descriptor is interactive terminal", "fd", fd)

	isTerminal := term.IsTerminal(fd)

	slog.Debug("terminal detection result",
		"fd", fd,
		"is_terminal", isTerminal)

	return isTerminal
}

// isInteractiveTerminal checks if the given file descriptor is an interactive terminal
// This wraps the terminal detector with consistent interface
func (c *CLI) isInteractiveTerminal(fd int) bool {
	if c.terminalDetector == nil {
		c.terminalDetector = &DefaultTerminalDetector{}
	}

	return c.terminalDetector.IsTerminal(fd)
}
