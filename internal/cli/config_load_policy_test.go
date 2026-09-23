package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"claudio.click/internal/cli/testenv"
	"claudio.click/internal/config"
)

const policyHookJSON = `{"session_id":"t","transcript_path":"/t","cwd":"/t","hook_event_name":"PostToolUse","tool_name":"Bash","tool_response":{"stdout":"ok","stderr":"","interrupted":false}}`

func writeMalformedConfig(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"volume": 0.5,`), 0o644); err != nil {
		t.Fatal(err)
	}
}

// userConfigPath is where XDG discovery looks first inside IsolateXDG.
func userConfigPath(t *testing.T) string {
	t.Helper()
	return config.NewXDGDirs().GetConfigPaths("config.json")[0]
}

func runPolicy(t *testing.T, stdin string, args ...string) (int, string, string) {
	t.Helper()
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	code := NewCLI().Run(append([]string{"claudio"}, args...), strings.NewReader(stdin), stdout, stderr)
	return code, stdout.String(), stderr.String()
}

// assertOneConfigWarning checks the malformed config was surfaced exactly
// once on stderr, naming the file, and never as an ERROR log record.
func assertOneConfigWarning(t *testing.T, stderr, path string) {
	t.Helper()
	if n := strings.Count(stderr, "Warning:"); n != 1 {
		t.Errorf("want exactly one Warning line on stderr, got %d:\n%s", n, stderr)
	}
	if !strings.Contains(stderr, path) || !strings.Contains(stderr, "using defaults") {
		t.Errorf("warning should name %s and say defaults are used:\n%s", path, stderr)
	}
	if strings.Contains(stderr, "level=ERROR") {
		t.Errorf("malformed config logged an ERROR record to stderr:\n%s", stderr)
	}
}

func TestConfigPolicy_HookModeMalformedWarnsAndContinues(t *testing.T) {
	for _, explicit := range []bool{true, false} {
		name := "xdg"
		if explicit {
			name = "flag"
		}
		t.Run(name, func(t *testing.T) {
			testenv.IsolateXDG(t)
			t.Setenv("CLAUDIO_DETACH_DISABLE", "1")
			t.Setenv("CLAUDIO_SOUND_TRACKING", "false")
			path := userConfigPath(t)
			args := []string{"--silent"}
			if explicit {
				path = filepath.Join(t.TempDir(), "bad.json")
				args = append(args, "--config", path)
			}
			writeMalformedConfig(t, path)

			code, _, stderr := runPolicy(t, policyHookJSON, args...)
			if code != 0 {
				t.Fatalf("hook mode must not fail on a malformed config: exit %d\n%s", code, stderr)
			}
			assertOneConfigWarning(t, stderr, path)
		})
	}
}

func TestConfigPolicy_StatusMalformedWarnsAndReportsDefaults(t *testing.T) {
	testenv.IsolateXDG(t)
	path := userConfigPath(t)
	writeMalformedConfig(t, path)

	code, stdout, stderr := runPolicy(t, "", "status")
	if code != 0 {
		t.Fatalf("status exit %d\n%s", code, stderr)
	}
	assertOneConfigWarning(t, stderr, path)
	if !strings.Contains(stdout, path) || !strings.Contains(stdout, "invalid") {
		t.Errorf("status should report the config file as invalid:\n%s", stdout)
	}
	if !strings.Contains(stdout, "enabled:        true") {
		t.Errorf("status should report the defaults hooks actually use:\n%s", stdout)
	}
}

func TestConfigPolicy_AnalyzeMalformedWarnsAndContinues(t *testing.T) {
	testenv.IsolateXDG(t)
	t.Setenv("CLAUDIO_SOUND_TRACKING_DB", filepath.Join(t.TempDir(), "t.db"))
	path := userConfigPath(t)
	writeMalformedConfig(t, path)

	code, _, stderr := runPolicy(t, "", "analyze", "usage")
	if code != 0 {
		t.Fatalf("analyze exit %d\n%s", code, stderr)
	}
	assertOneConfigWarning(t, stderr, path)
}

func TestConfigPolicy_MissingConfigIsQuietDefaults(t *testing.T) {
	testenv.IsolateXDG(t)
	t.Setenv("CLAUDIO_DETACH_DISABLE", "1")
	t.Setenv("CLAUDIO_SOUND_TRACKING", "false")
	missing := filepath.Join(t.TempDir(), "nope.json")

	for _, args := range [][]string{
		{"--silent"},
		{"--silent", "--config", missing},
		{"status"},
		{"status", "--config", missing},
		{"analyze", "usage", "--config", missing},
	} {
		code, _, stderr := runPolicy(t, policyHookJSON, args...)
		if code != 0 || stderr != "" {
			t.Errorf("%v: exit %d, stderr %q; want 0 and quiet defaults", args, code, stderr)
		}
	}
}

func TestConfigPolicy_NullDeviceConfigIsQuiet(t *testing.T) {
	testenv.IsolateXDG(t)
	t.Setenv("CLAUDIO_DETACH_DISABLE", "1")
	t.Setenv("CLAUDIO_SOUND_TRACKING", "false")
	nullDevice := "/dev/null"
	if runtime.GOOS == "windows" {
		nullDevice = "NUL"
	}
	code, _, stderr := runPolicy(t, policyHookJSON, "--config", nullDevice, "--silent")
	if code != 0 || stderr != "" {
		t.Fatalf("--config %s: exit %d, stderr %q; want 0 and nothing on stderr", nullDevice, code, stderr)
	}
}
