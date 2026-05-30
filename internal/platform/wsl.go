// Package platform provides OS/runtime detection helpers that have
// no audio or config concerns. Created to break the
// internal/config -> internal/audio import that review finding #68
// flagged: WSL detection is a generic OS check, not an audio-backend
// responsibility.
package platform

import (
	"log/slog"
	"os"
	"strings"
)

// IsWSL checks if the current environment is Windows Subsystem for Linux.
// It inspects /proc/version for a "microsoft" or "wsl" signature (case-
// insensitive) and the WSL_DISTRO_NAME environment variable.
func IsWSL() bool {
	return detectWSLFromData(readProcVersion(), os.Getenv("WSL_DISTRO_NAME"))
}

// detectWSLFromData checks for WSL indicators in the provided data (for testing).
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

// readProcVersion reads /proc/version file content. Returns empty string
// when the file is unreadable (e.g. on non-Linux GOOS).
func readProcVersion() string {
	content, err := os.ReadFile("/proc/version")
	if err != nil {
		slog.Debug("failed to read /proc/version", "error", err)
		return ""
	}
	return string(content)
}

// truncateString truncates a string to maxLen characters for logging.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
