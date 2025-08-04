package audio

import (
	"log/slog"
	"os"
	"os/exec"
	"strings"
)

// IsWSL checks if the current environment is Windows Subsystem for Linux
func IsWSL() bool {
	return detectWSLFromData(readProcVersion(), os.Getenv("WSL_DISTRO_NAME"))
}

// detectWSLFromData checks for WSL indicators in the provided data (for testing)
func detectWSLFromData(procVersion, wslEnv string) bool {
	slog.Debug("checking WSL detection", "proc_version_snippet", truncateString(procVersion, 50), "wsl_env", wslEnv)

	// Check WSL_DISTRO_NAME environment variable (WSL sets this)
	if wslEnv != "" {
		slog.Debug("WSL detected via environment variable", "distro", wslEnv)
		return true
	}

	// Check /proc/version for Microsoft or WSL indicators
	procLower := strings.ToLower(procVersion)
	if strings.Contains(procLower, "microsoft") || strings.Contains(procLower, "wsl") {
		slog.Debug("WSL detected via /proc/version", "indicators", "microsoft or wsl found")
		return true
	}

	slog.Debug("no WSL indicators found")
	return false
}

// readProcVersion reads /proc/version file content
func readProcVersion() string {
	content, err := os.ReadFile("/proc/version")
	if err != nil {
		slog.Debug("failed to read /proc/version", "error", err)
		return ""
	}
	return string(content)
}

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
	return detectOptimalBackendWithChecker(IsWSL(), CommandExists)
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

// getPreferredSystemCommand finds the best available system audio command
func getPreferredSystemCommand() string {
	return getPreferredSystemCommandWithChecker(CommandExists)
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

// truncateString truncates a string to maxLen characters for logging
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
