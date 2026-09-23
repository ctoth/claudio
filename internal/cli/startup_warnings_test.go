package cli

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"claudio.click/internal/cli/testenv"
	"claudio.click/internal/config"
)

// tempLogDir returns a directory that is removed only after slog stops
// referencing lumberjack's open file; Windows refuses to delete it otherwise.
func tempLogDir(t *testing.T) string {
	t.Helper()
	prev := slog.Default()
	dir, err := os.MkdirTemp("", "claudio-startup-warnings-") //nolint:usetesting // t.TempDir fails the test if lumberjack still holds the log open on Windows
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		slog.SetDefault(prev)
		_ = os.RemoveAll(dir)
	})
	return dir
}

// Warnings logged while the config loads precede setupLogging. They must
// still reach the log file once it exists, without leaking onto stderr.
func TestSetupLoggingReplaysStartupWarningsToFile(t *testing.T) {
	testenv.IsolateXDG(t)
	logFile := filepath.Join(tempLogDir(t), "claudio.log")

	var stderr bytes.Buffer
	setupDefaultCommandLogging(&stderr)
	slog.Debug("early debug")
	slog.Warn("early warning", "source", "config.json")
	slog.Error("early error")

	setupLogging(&config.Config{
		LogLevel:    "warn",
		FileLogging: &config.FileLoggingConfig{Enabled: true, Filename: logFile, MaxSizeMB: 1},
	}, &stderr)

	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatal(err)
	}
	file := string(data)
	if !strings.Contains(file, "early warning") || !strings.Contains(file, "source=config.json") {
		t.Errorf("startup warning missing from log file: %q", file)
	}
	if !strings.Contains(file, "early error") {
		t.Errorf("startup error missing from log file: %q", file)
	}
	if strings.Contains(file, "early debug") {
		t.Errorf("record below the configured level reached the file: %q", file)
	}
	if strings.Contains(stderr.String(), "early warning") {
		t.Errorf("warning leaked to stderr: %q", stderr.String())
	}
	if n := strings.Count(stderr.String(), "early error"); n != 1 {
		t.Errorf("startup error written to stderr %d times, want 1: %q", n, stderr.String())
	}
}

// End to end: the documented malgo deprecation warning reaches the log file.
func TestLegacyMalgoConfigWarningReachesLogFile(t *testing.T) {
	testenv.IsolateXDG(t)
	t.Setenv("CLAUDIO_FILE_LOGGING", "") // IsolateXDG disables it; this test needs the file
	dir := tempLogDir(t)
	logFile := filepath.Join(dir, "claudio.log")
	cfgFile := filepath.Join(dir, "config.json")
	cfgJSON := `{"enabled":true,"default_soundpack":"default","log_level":"warn","audio_backend":"malgo",` +
		`"file_logging":{"enabled":true,"filename":` + quoteJSON(logFile) + `,"max_size_mb":1}}`
	if err := os.WriteFile(cfgFile, []byte(cfgJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	hook := `{"session_id":"t","transcript_path":"/t","cwd":"/t","hook_event_name":"Stop"}`
	var stdout, stderr bytes.Buffer
	if code := NewCLI().Run([]string{"claudio", "--config", cfgFile, "--silent"}, strings.NewReader(hook), &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}

	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "deprecated audio backend") || !strings.Contains(string(data), "malgo") {
		t.Fatalf("malgo deprecation warning missing from log file: %q", data)
	}
}

func quoteJSON(s string) string {
	return `"` + strings.ReplaceAll(s, `\`, `\\`) + `"`
}
