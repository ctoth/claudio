package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

func main() {
	// Initialize structured logging
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)

	slog.Info("hook logger started", "args", os.Args, "stdin_available", true)

	// Read JSON from stdin (Claude Code sends hook data via stdin)
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		slog.Error("failed to read stdin", "error", err)
		os.Exit(1)
	}

	if len(input) == 0 {
		slog.Error("no input received from stdin")
		os.Exit(1)
	}

	slog.Info("received hook data", "size_bytes", len(input))

	// Parse JSON to validate and pretty-print
	var hookData map[string]interface{}
	err = json.Unmarshal(input, &hookData)
	if err != nil {
		slog.Error("failed to parse JSON", "error", err, "raw_input", string(input))
		// Still save the raw data even if invalid JSON
		saveRawData(input, "invalid")
		os.Exit(1)
	}

	// Extract hook event name for better organization
	eventName := "unknown"
	if name, ok := hookData["hook_event_name"].(string); ok {
		eventName = name
	}

	slog.Info("parsed hook event",
		"event_name", eventName,
		"fields", getJsonKeys(hookData))

	// Save the JSON data to timestamped file
	err = saveHookData(input, eventName)
	if err != nil {
		slog.Error("failed to save hook data", "error", err)
		os.Exit(1)
	}

	// Pretty print to stderr for immediate viewing
	prettyJSON, err := json.MarshalIndent(hookData, "", "  ")
	if err != nil {
		slog.Error("failed to pretty print JSON", "error", err)
	} else {
		fmt.Fprintf(os.Stderr, "\n=== HOOK EVENT: %s ===\n%s\n\n", eventName, string(prettyJSON))
	}

	slog.Info("hook logging completed successfully", "event_name", eventName)
}

// saveHookData saves hook JSON to timestamped file in logs directory
func saveHookData(data []byte, eventName string) error {
	// Create logs directory if it doesn't exist
	logsDir := "/tmp/claudio-hook-logs"
	err := os.MkdirAll(logsDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create logs directory: %w", err)
	}

	// Generate timestamped filename
	timestamp := time.Now().Format("2006-01-02_15-04-05.000")
	filename := fmt.Sprintf("%s_%s.json", timestamp, eventName)
	filepath := filepath.Join(logsDir, filename)

	// Write JSON data to file
	err = os.WriteFile(filepath, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write hook data to file: %w", err)
	}

	slog.Info("hook data saved", "file", filepath, "size_bytes", len(data))
	return nil
}

// saveRawData saves invalid JSON for debugging
func saveRawData(data []byte, suffix string) {
	logsDir := "/tmp/claudio-hook-logs"
	os.MkdirAll(logsDir, 0755)

	timestamp := time.Now().Format("2006-01-02_15-04-05.000")
	filename := fmt.Sprintf("%s_%s.raw", timestamp, suffix)
	filepath := filepath.Join(logsDir, filename)

	os.WriteFile(filepath, data, 0644)
	slog.Info("raw data saved", "file", filepath)
}

// getJsonKeys extracts top-level keys from JSON object for logging
func getJsonKeys(data map[string]interface{}) []string {
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	return keys
}
