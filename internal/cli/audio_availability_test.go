package cli

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"claudio.click/internal/cli/testenv"
)

// Exercise the real parent process: in-process tests disable detachment and
// cannot catch diagnostics lost when the worker's stderr goes to the null device.
func TestNoCGOBinaryAudioAvailability(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "claudio.exe")
	build := exec.Command("go", "build", "-o", binary, "../../cmd/claudio")
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	root := testenv.IsolateXDG(t)
	// Subprocesses must not discover a host's system-wide Claudio installation.
	t.Setenv("XDG_CONFIG_DIRS", filepath.Join(root, "config-dirs"))
	t.Setenv("XDG_DATA_DIRS", filepath.Join(root, "data-dirs"))
	t.Setenv("PATH", "") // Keep auto deterministic even on WSL hosts with players.
	t.Setenv("CLAUDIO_ENABLED", "true")
	t.Setenv("CLAUDIO_SOUND_TRACKING", "false")
	t.Setenv("CLAUDIO_DETACH_DISABLE", "")
	t.Setenv("CLAUDIO_DAEMON_CHILD", "")
	t.Setenv("CLAUDIO_TEST_RECOGNIZE_GO_TEST", "")
	for _, tc := range []struct {
		name, backend string
		args          []string
		wantError     bool
		want          string
	}{
		{"status", "oto", []string{"status"}, false, "oto -> oto (available; playback not tested)"},
		{"auto status", "auto", []string{"status"}, false, "auto -> oto (available; playback not tested)"},
		{"available status", "fake", []string{"status"}, false, "fake -> fake (available; playback not tested)"},
		{"unavailable hook", "system_command", nil, true, "no system audio commands found"},
		{"silent hook", "oto", []string{"--silent"}, false, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("CLAUDIO_AUDIO_BACKEND", tc.backend)
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, binary, tc.args...)
			cmd.Stdin = strings.NewReader(`{"session_id":"test","cwd":".","hook_event_name":"Stop"}`)
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			err := cmd.Run()
			if (err != nil) != tc.wantError {
				t.Errorf("run error = %v, wantError = %v; stderr=%s", err, tc.wantError, &stderr)
			}
			out := stdout.String()
			if tc.wantError {
				out = stderr.String()
				if strings.Count(out, "\n") != 1 {
					t.Errorf("want one actionable diagnostic, got %q", out)
				}
			}
			if tc.want != "" && !strings.Contains(out, tc.want) {
				t.Errorf("output = %q, want %q", out, tc.want)
			}
			if tc.want == "" && stderr.Len() != 0 {
				t.Errorf("silent hook stderr = %q", &stderr)
			}
		})
	}
}
