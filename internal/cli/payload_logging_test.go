package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"claudio.click/internal/cli/testenv"
)

// payloadSecret stands in for prompt text, notification messages or tool
// output a hook payload may carry. It must never reach the log, even at
// debug level.
const payloadSecret = "hunter2-PAYLOAD-SECRET"

// runHookWithDebugLog runs one hook with file logging at debug level and
// returns the log file content and stderr.
func runHookWithDebugLog(t *testing.T, payload string) (string, string) {
	t.Helper()
	testenv.IsolateXDG(t)
	t.Setenv("CLAUDIO_FILE_LOGGING", "") // IsolateXDG disables it; this test needs the file
	dir := tempLogDir(t)
	logFile := filepath.Join(dir, "claudio.log")
	cfgFile := filepath.Join(dir, "config.json")
	cfgJSON := `{"enabled":true,"default_soundpack":"default","log_level":"debug",` +
		`"file_logging":{"enabled":true,"filename":` + quoteJSON(logFile) + `,"max_size_mb":1}}`
	if err := os.WriteFile(cfgFile, []byte(cfgJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	NewCLI().Run([]string{"claudio", "--config", cfgFile, "--silent"}, strings.NewReader(payload), &stdout, &stderr)
	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatal(err)
	}
	return string(data), stderr.String()
}

func TestHookPayloadTextNeverReachesTheLog(t *testing.T) {
	payloads := map[string]string{
		"broken JSON": `{"session_id":"s","prompt":"` + payloadSecret + `" oops`,
		"missing cwd": `{"session_id":"s","hook_event_name":"UserPromptSubmit","prompt":"` + payloadSecret + `"}`,
		"prompt":      `{"session_id":"s","cwd":"/c","hook_event_name":"UserPromptSubmit","prompt":"` + payloadSecret + `"}`,
		"notification": `{"session_id":"s","cwd":"/c","hook_event_name":"Notification","message":"` +
			payloadSecret + ` needs your permission"}`,
		"tool output": `{"session_id":"s","cwd":"/c","hook_event_name":"PostToolUse","tool_name":"Bash",` +
			`"tool_input":{"command":"echo hi"},"tool_response":{"stdout":"` + payloadSecret + `","stderr":""}}`,
	}
	for name, payload := range payloads {
		t.Run(name, func(t *testing.T) {
			log, stderr := runHookWithDebugLog(t, payload)
			if strings.Contains(log, payloadSecret) {
				t.Errorf("payload text in log file:\n%s", log)
			}
			if strings.Contains(stderr, payloadSecret) {
				t.Errorf("payload text on stderr: %q", stderr)
			}
		})
	}
}

// A payload that fails to parse is identified by its size and a short
// hash, so a report can be matched to a captured payload without logging it.
func TestRejectedHookPayloadIsLoggedAsLengthAndHash(t *testing.T) {
	payload := `{"session_id":"s","prompt":"` + payloadSecret + `" oops`
	sum := sha256.Sum256([]byte(payload))
	log, _ := runHookWithDebugLog(t, payload)
	for _, want := range []string{
		fmt.Sprintf("payload_bytes=%d", len(payload)),
		"payload_sha256=" + hex.EncodeToString(sum[:])[:12],
	} {
		if !strings.Contains(log, want) {
			t.Errorf("log lacks %q:\n%s", want, log)
		}
	}
}
