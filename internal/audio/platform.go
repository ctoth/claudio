package audio

import (
	"log/slog"
	"os/exec"

	"claudio.click/internal/platform"
)

// CommandExists checks if a command is available in the system's PATH using exec.LookPath
func CommandExists(command string) bool {
	if command == "" {
		return false
	}

	_, err := exec.LookPath(command)
	exists := err == nil
	slog.Debug("command existence check", "command", command, "exists", exists)
	return exists
}

// DetectOptimalBackend determines the best audio backend for the current system
func DetectOptimalBackend() string {
	return detectOptimalBackendWithChecker(platform.IsWSL(), CommandExists)
}

// detectOptimalBackendWithChecker allows dependency injection for testing
func detectOptimalBackendWithChecker(isWSL bool, commandChecker func(string) bool) string {
	slog.Debug("detecting optimal audio backend", "is_wsl", isWSL)

	if isWSL {
		// In WSL, prefer system commands to avoid malgo crackling issues
		slog.Debug("WSL detected, preferring system commands over malgo")

		preferredCmd := getPreferredSystemCommandWithChecker(commandChecker)
		if preferredCmd != "" {
			slog.Debug("system command found for WSL", "command", preferredCmd)
			return "system_command"
		}

		slog.Warn("no system audio commands found in WSL, falling back to malgo (may have crackling)")
		return "malgo"
	}

	// On native Linux/macOS, prefer malgo for better performance and control
	slog.Debug("native system detected, preferring malgo backend")
	return "malgo"
}

// getPreferredSystemCommandWithChecker allows dependency injection for testing
func getPreferredSystemCommandWithChecker(commandChecker func(string) bool) string {
	available := getAvailableSystemCommandsWithChecker(commandChecker)
	if len(available) == 0 {
		slog.Debug("no preferred system audio commands found")
		return ""
	}

	slog.Debug("preferred system command found", "command", available[0])
	return available[0]
}

// getAvailableSystemCommandsWithChecker returns all available system audio
// commands in priority order.
func getAvailableSystemCommandsWithChecker(commandChecker func(string) bool) []string {
	allCommands := []string{
		"paplay", // PulseAudio - most common on modern Linux
		"ffplay", // FFmpeg - widely available and versatile
		"aplay",  // ALSA - lower-level Linux audio
		"afplay", // macOS built-in audio player
	}

	var available []string
	for _, cmd := range allCommands {
		if commandChecker(cmd) {
			available = append(available, cmd)
		}
	}

	slog.Debug("available system audio commands", "commands", available, "count", len(available))
	return available
}
