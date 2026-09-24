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
	"sync"
)

// isWSL is the process-wide cached detector. Tests in this package swap
// it through newWSLCache.
var isWSL = newWSLCache(detectWSL)

// newWSLCache wraps detect so it runs at most once.
func newWSLCache(detect func() bool) func() bool {
	return sync.OnceValue(detect)
}

// IsWSL reports whether the current environment is Windows Subsystem for
// Linux. It inspects /proc/version for a "microsoft" or "wsl" signature
// (case-insensitive) and the WSL_DISTRO_NAME environment variable. The
// answer is computed once per process.
func IsWSL() bool {
	return isWSL()
}

func detectWSL() bool {
	wsl := detectWSLFromData(readProcVersion(), os.Getenv("WSL_DISTRO_NAME"))
	slog.Debug("WSL detection", "is_wsl", wsl)
	return wsl
}

// detectWSLFromData checks for WSL indicators in the provided data.
func detectWSLFromData(procVersion, wslEnv string) bool {
	// WSL sets WSL_DISTRO_NAME in every distro shell.
	if wslEnv != "" {
		return true
	}
	procLower := strings.ToLower(procVersion)
	return strings.Contains(procLower, "microsoft") || strings.Contains(procLower, "wsl")
}

// readProcVersion reads /proc/version file content. Returns empty string
// when the file is unreadable (e.g. on non-Linux GOOS).
func readProcVersion() string {
	content, err := os.ReadFile("/proc/version")
	if err != nil {
		return ""
	}
	return string(content)
}
