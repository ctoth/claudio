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
	// Priority order: paplay (PulseAudio) > ffplay (FFmpeg) > aplay (ALSA) > afplay (macOS)
	preferredCommands := []string{
		"paplay", // PulseAudio - most common on modern Linux
		"ffplay", // FFmpeg - widely available and versatile
		"aplay",  // ALSA - lower-level Linux audio
		"afplay", // macOS built-in audio player
	}

	for _, cmd := range preferredCommands {
		if commandChecker(cmd) {
			slog.Debug("preferred system command found", "command", cmd)
			return cmd
		}
	}

	slog.Debug("no preferred system audio commands found")
	return ""
}
