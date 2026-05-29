package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"claudio.click/internal/config"
)

func TestMuteCommand_SetsEnabledFalse(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "config.json")

	writeSeedConfig(t, configPath, &config.Config{
		DefaultSoundpack: "x",
		Enabled:          true,
		LogLevel:         "warn",
		AudioBackend:     "auto",
	})

	cli := NewCLI()
	stdin := strings.NewReader("")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	code := cli.Run([]string{"claudio", "mute", "--config", configPath}, stdin, stdout, stderr)
	if code != 0 {
		t.Fatalf("exit code = %d; stderr=%s", code, stderr.String())
	}

	persisted := readPersistedConfig(t, configPath)
	if persisted.Enabled {
		t.Error("Enabled should be false after mute")
	}
	if !strings.Contains(stdout.String(), "muted") {
		t.Errorf("expected output to mention 'muted', got: %q", stdout.String())
	}
}

func TestUnmuteCommand_SetsEnabledTrue(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "config.json")

	writeSeedConfig(t, configPath, &config.Config{
		DefaultSoundpack: "x",
		Enabled:          false,
		LogLevel:         "warn",
		AudioBackend:     "auto",
	})

	cli := NewCLI()
	stdin := strings.NewReader("")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	code := cli.Run([]string{"claudio", "unmute", "--config", configPath}, stdin, stdout, stderr)
	if code != 0 {
		t.Fatalf("exit code = %d; stderr=%s", code, stderr.String())
	}

	persisted := readPersistedConfig(t, configPath)
	if !persisted.Enabled {
		t.Error("Enabled should be true after unmute")
	}
	if !strings.Contains(stdout.String(), "unmuted") {
		t.Errorf("expected output to mention 'unmuted', got: %q", stdout.String())
	}
}

// TestMuteThenUnmute exercises the symmetric pair and verifies the
// final state is true.
func TestMuteThenUnmute(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "config.json")

	writeSeedConfig(t, configPath, &config.Config{
		DefaultSoundpack: "x",
		Enabled:          true,
		LogLevel:         "warn",
		AudioBackend:     "auto",
	})

	// Mute
	cli := NewCLI()
	if code := cli.Run([]string{"claudio", "mute", "--config", configPath},
		strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{}); code != 0 {
		t.Fatalf("mute exit code = %d", code)
	}
	if persisted := readPersistedConfig(t, configPath); persisted.Enabled {
		t.Fatal("after mute, Enabled should be false")
	}

	// Unmute on a fresh CLI to ensure command flag state isn't sticky.
	cli2 := NewCLI()
	if code := cli2.Run([]string{"claudio", "unmute", "--config", configPath},
		strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{}); code != 0 {
		t.Fatalf("unmute exit code = %d", code)
	}
	if persisted := readPersistedConfig(t, configPath); !persisted.Enabled {
		t.Fatal("after unmute, Enabled should be true")
	}
}

// TestMuteIdempotent verifies running mute twice does not error.
func TestMuteIdempotent(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "config.json")

	writeSeedConfig(t, configPath, &config.Config{
		DefaultSoundpack: "x",
		Enabled:          false,
		LogLevel:         "warn",
		AudioBackend:     "auto",
	})

	for i := 0; i < 2; i++ {
		cli := NewCLI()
		stderr := &bytes.Buffer{}
		code := cli.Run([]string{"claudio", "mute", "--config", configPath},
			strings.NewReader(""), &bytes.Buffer{}, stderr)
		if code != 0 {
			t.Fatalf("mute run %d exit code = %d, stderr=%s", i, code, stderr.String())
		}
	}

	if persisted := readPersistedConfig(t, configPath); persisted.Enabled {
		t.Error("after two mutes, Enabled should still be false")
	}
}
