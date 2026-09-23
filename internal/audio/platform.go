package audio

import (
	"log/slog"
	"os/exec"
)

// CommandExists checks if a command is available in the system's PATH using exec.LookPath
func CommandExists(command string) bool {
	if command == "" {
		return false
	}

	_, err := exec.LookPath(command)
	return err == nil
}

// detectOptimalBackendWithChecker picks the "auto" backend; the command
// checker is injected so tests can simulate any PATH.
func detectOptimalBackendWithChecker(isWSL bool, commandChecker func(string) bool) string {
	if isWSL {
		// Preserve WSL's system-command routing to the host audio server.

		preferredCmd := getPreferredSystemCommandWithChecker(commandChecker)
		if preferredCmd != "" {
			slog.Debug("system command found for WSL", "command", preferredCmd)
			return "system_command"
		}

		slog.Debug("no system audio commands found in WSL, using oto")
		return "oto"
	}

	// On native Linux/macOS, prefer oto for better performance and control
	slog.Debug("native system detected, preferring oto backend")
	return "oto"
}

// getPreferredSystemCommandWithChecker allows dependency injection for testing
func getPreferredSystemCommandWithChecker(commandChecker func(string) bool) string {
	available := getAvailableSystemCommandsWithChecker(commandChecker)
	if len(available) == 0 {
		return ""
	}

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
