package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"claudio.click/internal/safeio"
)

func main() {
	os.Exit(run(os.Stdin, os.Stderr))
}

func run(stdin io.Reader, stderr io.Writer) int {
	logger := slog.New(slog.NewTextHandler(stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	logger.Info("hook logger started")
	logger.Warn("hook logger stores and prints complete valid hook payloads; logs may contain sensitive data")

	input, err := safeio.ReadJSONBounded(
		stdin,
		safeio.MaxHookPayloadBytes,
		safeio.DefaultHookReadDeadline,
		"hook payload",
	)
	if err != nil {
		logger.Error("failed to read stdin", "error", err)
		return 1
	}
	if len(bytes.TrimSpace(input)) == 0 {
		logger.Error("no input received from stdin")
		return 1
	}

	var hookData map[string]any
	if err := json.Unmarshal(input, &hookData); err != nil {
		logger.Error("failed to parse hook JSON", "error", err)
		return 1
	}

	eventName := "unknown"
	if name, ok := hookData["hook_event_name"].(string); ok {
		eventName = sanitizeEventName(name)
	}

	logger.Info("parsed hook event",
		"event_name", eventName,
		"size_bytes", len(input),
		"fields", getJSONKeys(hookData))

	savedPath, err := saveHookData(input, eventName)
	if err != nil {
		logger.Error("failed to save hook data", "error", err)
		return 1
	}
	logger.Info("hook data saved", "file", savedPath, "size_bytes", len(input))

	prettyJSON, err := json.MarshalIndent(hookData, "", "  ")
	if err != nil {
		logger.Error("failed to pretty print hook JSON", "error", err)
	} else {
		fmt.Fprintf(stderr, "\n=== HOOK EVENT: %s ===\n%s\n\n", eventName, prettyJSON)
	}

	logger.Info("hook logging completed successfully", "event_name", eventName)
	return 0
}

func saveHookData(data []byte, eventName string) (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("find user cache directory: %w", err)
	}

	logsDir := filepath.Join(cacheDir, "claudio", "hook-logs")
	if err := os.MkdirAll(logsDir, 0o700); err != nil {
		return "", fmt.Errorf("create hook log directory: %w", err)
	}
	if err := os.Chmod(logsDir, 0o700); err != nil {
		return "", fmt.Errorf("secure hook log directory: %w", err)
	}

	timestamp := time.Now().Format("2006-01-02_15-04-05.000")
	pattern := fmt.Sprintf("%s_%s_*.json", timestamp, sanitizeEventName(eventName))
	file, err := os.CreateTemp(logsDir, pattern)
	if err != nil {
		return "", fmt.Errorf("create hook log file: %w", err)
	}
	path := file.Name()

	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return "", fmt.Errorf("secure hook log file: %w", err)
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return "", fmt.Errorf("write hook log file: %w", err)
	}
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("close hook log file: %w", err)
	}
	return path, nil
}

func sanitizeEventName(name string) string {
	name = strings.TrimSpace(name)
	var sanitized strings.Builder
	previousSeparator := false
	for _, r := range name {
		allowed := r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_')
		if allowed {
			sanitized.WriteRune(r)
			previousSeparator = false
		} else if !previousSeparator {
			sanitized.WriteByte('_')
			previousSeparator = true
		}
		if sanitized.Len() >= 64 {
			break
		}
	}
	result := strings.Trim(sanitized.String(), "_-.")
	if result == "" {
		return "unknown"
	}
	return result
}

func getJSONKeys(data map[string]any) []string {
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
