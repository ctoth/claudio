package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"claudio.click/internal/config"
)

func TestStatusCommand_EnabledTrue_NoMutedToken(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "config.json")

	vol := 0.4
	writeSeedConfig(t, configPath, &config.Config{
		Volume:           &vol,
		DefaultSoundpack: "x",
		Enabled:          true,
		LogLevel:         "warn",
		AudioBackend:     "auto",
	})

	cli := NewCLI()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	code := cli.Run([]string{"claudio", "status", "--config", configPath},
		strings.NewReader(""), stdout, stderr)
	if code != 0 {
		t.Fatalf("exit code = %d; stderr=%s", code, stderr.String())
	}

	out := stdout.String()
	if !strings.Contains(out, "enabled:") {
		t.Errorf("expected 'enabled:' in output, got: %q", out)
	}
	if !strings.Contains(out, "true") {
		t.Errorf("expected 'true' in output (enabled), got: %q", out)
	}
	if strings.Contains(out, "MUTED") {
		t.Errorf("MUTED token should NOT appear when enabled=true, got: %q", out)
	}
	if !strings.Contains(out, "0.40") {
		t.Errorf("expected volume '0.40' in output, got: %q", out)
	}
}

// TestStatusCommand_EnabledFalse_HasMutedToken is the load-bearing
// screen-reader assertion. The literal token "MUTED" MUST appear in
// the output when enabled=false. Do not soften this test.
func TestStatusCommand_EnabledFalse_HasMutedToken(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "config.json")

	writeSeedConfig(t, configPath, &config.Config{
		DefaultSoundpack: "x",
		Enabled:          false,
		LogLevel:         "warn",
		AudioBackend:     "auto",
	})

	cli := NewCLI()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	code := cli.Run([]string{"claudio", "status", "--config", configPath},
		strings.NewReader(""), stdout, stderr)
	if code != 0 {
		t.Fatalf("exit code = %d; stderr=%s", code, stderr.String())
	}

	out := stdout.String()
	if !strings.Contains(out, "MUTED") {
		t.Errorf("MUTED token MUST appear when enabled=false (screen-reader cue); got: %q", out)
	}
	if !strings.Contains(out, "false") {
		t.Errorf("expected 'false' in output, got: %q", out)
	}
}

func TestStatusCommand_VolumeUnset_PrintsDefault(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "config.json")

	writeSeedConfig(t, configPath, &config.Config{
		// Volume nil — unset.
		DefaultSoundpack: "x",
		Enabled:          true,
		LogLevel:         "warn",
		AudioBackend:     "auto",
	})

	cli := NewCLI()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	code := cli.Run([]string{"claudio", "status", "--config", configPath},
		strings.NewReader(""), stdout, stderr)
	if code != 0 {
		t.Fatalf("exit code = %d; stderr=%s", code, stderr.String())
	}

	out := stdout.String()
	if !strings.Contains(out, "default") {
		t.Errorf("expected 'default' annotation for unset volume, got: %q", out)
	}
}

func TestStatusCommand_VolumeFromEnv_AnnotatesSource(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "config.json")

	vol := 0.3
	writeSeedConfig(t, configPath, &config.Config{
		Volume:           &vol,
		DefaultSoundpack: "x",
		Enabled:          true,
		LogLevel:         "warn",
		AudioBackend:     "auto",
	})

	t.Setenv("CLAUDIO_VOLUME", "0.95")

	cli := NewCLI()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	code := cli.Run([]string{"claudio", "status", "--config", configPath},
		strings.NewReader(""), stdout, stderr)
	if code != 0 {
		t.Fatalf("exit code = %d; stderr=%s", code, stderr.String())
	}

	out := stdout.String()
	if !strings.Contains(out, "CLAUDIO_VOLUME") {
		t.Errorf("expected CLAUDIO_VOLUME annotation in output, got: %q", out)
	}
	if !strings.Contains(out, "0.95") {
		t.Errorf("expected env-overridden volume 0.95 in output, got: %q", out)
	}
}

func TestStatusCommand_PrintsAllExpectedFields(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "config.json")

	vol := 0.5
	writeSeedConfig(t, configPath, &config.Config{
		Volume:           &vol,
		DefaultSoundpack: "test-pack",
		Enabled:          true,
		LogLevel:         "info",
		AudioBackend:     "malgo",
	})

	cli := NewCLI()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	code := cli.Run([]string{"claudio", "status", "--config", configPath},
		strings.NewReader(""), stdout, stderr)
	if code != 0 {
		t.Fatalf("exit code = %d; stderr=%s", code, stderr.String())
	}

	out := stdout.String()
	for _, field := range []string{
		"config file:",
		"enabled:",
		"volume:",
		"soundpack:",
		"log level:",
		"audio backend:",
		"version:",
	} {
		if !strings.Contains(out, field) {
			t.Errorf("expected field %q in status output, got: %q", field, out)
		}
	}
	if !strings.Contains(out, "test-pack") {
		t.Errorf("expected soundpack name in output, got: %q", out)
	}
	if !strings.Contains(out, "malgo") {
		t.Errorf("expected audio backend in output, got: %q", out)
	}
}
