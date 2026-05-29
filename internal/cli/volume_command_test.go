package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"claudio.click/internal/config"
)

// writeSeedConfig writes a starting config.json to path so the verb has
// something to read-modify-write.
func writeSeedConfig(t *testing.T, path string, cfg *config.Config) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatalf("marshal seed: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write seed config: %v", err)
	}
}

// readPersistedConfig reads and parses the written config.json so a test
// can assert what was persisted.
func readPersistedConfig(t *testing.T, path string) *config.Config {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read persisted config: %v", err)
	}
	var cfg config.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshal persisted config: %v", err)
	}
	return &cfg
}

func TestVolumeCommand_NoArgsPrintsCurrent(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "config.json")

	vol := 0.42
	writeSeedConfig(t, configPath, &config.Config{
		Volume:           &vol,
		DefaultSoundpack: "x",
		Enabled:          true,
		LogLevel:         "warn",
		AudioBackend:     "auto",
	})

	cli := NewCLI()
	stdin := strings.NewReader("")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	code := cli.Run([]string{"claudio", "volume", "--config", configPath}, stdin, stdout, stderr)
	if code != 0 {
		t.Fatalf("exit code = %d; stderr=%s", code, stderr.String())
	}

	out := stdout.String()
	if !strings.Contains(out, "0.42") {
		t.Errorf("expected output to contain '0.42', got: %q", out)
	}
}

func TestVolumeCommand_NoArgsPrintsDefaultWhenUnset(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "config.json")

	writeSeedConfig(t, configPath, &config.Config{
		// Volume nil — unset
		DefaultSoundpack: "x",
		Enabled:          true,
		LogLevel:         "warn",
		AudioBackend:     "auto",
	})

	cli := NewCLI()
	stdin := strings.NewReader("")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	code := cli.Run([]string{"claudio", "volume", "--config", configPath}, stdin, stdout, stderr)
	if code != 0 {
		t.Fatalf("exit code = %d; stderr=%s", code, stderr.String())
	}

	out := stdout.String()
	if !strings.Contains(out, "default") {
		t.Errorf("expected output to mention 'default' when volume is unset, got: %q", out)
	}
}

func TestVolumeCommand_ValidLevelPersists(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "config.json")

	startVol := 0.2
	writeSeedConfig(t, configPath, &config.Config{
		Volume:           &startVol,
		DefaultSoundpack: "x",
		Enabled:          true,
		LogLevel:         "warn",
		AudioBackend:     "auto",
	})

	cli := NewCLI()
	stdin := strings.NewReader("")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	code := cli.Run([]string{"claudio", "volume", "0.7", "--config", configPath}, stdin, stdout, stderr)
	if code != 0 {
		t.Fatalf("exit code = %d; stderr=%s", code, stderr.String())
	}

	persisted := readPersistedConfig(t, configPath)
	if persisted.Volume == nil {
		t.Fatal("Volume should be persisted, got nil")
	}
	if *persisted.Volume != 0.7 {
		t.Errorf("persisted Volume = %v, want 0.7", *persisted.Volume)
	}
	if !persisted.Enabled {
		t.Error("Enabled should be preserved as true")
	}
}

func TestVolumeCommand_RejectsOutOfRange(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "config.json")

	startVol := 0.2
	writeSeedConfig(t, configPath, &config.Config{
		Volume:           &startVol,
		DefaultSoundpack: "x",
		Enabled:          true,
		LogLevel:         "warn",
		AudioBackend:     "auto",
	})

	cli := NewCLI()
	stdin := strings.NewReader("")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	code := cli.Run([]string{"claudio", "volume", "1.5", "--config", configPath}, stdin, stdout, stderr)
	if code == 0 {
		t.Fatalf("expected non-zero exit code for out-of-range volume; stdout=%q", stdout.String())
	}

	// Persisted value must not have been overwritten.
	persisted := readPersistedConfig(t, configPath)
	if persisted.Volume == nil || *persisted.Volume != 0.2 {
		t.Errorf("Volume should be unchanged after rejection, got %v", persisted.Volume)
	}
}

func TestVolumeCommand_RejectsNonFloat(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "config.json")

	startVol := 0.2
	writeSeedConfig(t, configPath, &config.Config{
		Volume:           &startVol,
		DefaultSoundpack: "x",
		Enabled:          true,
		LogLevel:         "warn",
		AudioBackend:     "auto",
	})

	cli := NewCLI()
	stdin := strings.NewReader("")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	code := cli.Run([]string{"claudio", "volume", "abc", "--config", configPath}, stdin, stdout, stderr)
	if code == 0 {
		t.Fatalf("expected non-zero exit code for non-float; stdout=%q stderr=%q",
			stdout.String(), stderr.String())
	}
}

func TestVolumeCommand_RejectsNegative(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "config.json")

	startVol := 0.2
	writeSeedConfig(t, configPath, &config.Config{
		Volume:           &startVol,
		DefaultSoundpack: "x",
		Enabled:          true,
		LogLevel:         "warn",
		AudioBackend:     "auto",
	})

	cli := NewCLI()
	stdin := strings.NewReader("")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	code := cli.Run([]string{"claudio", "volume", "-0.1", "--config", configPath}, stdin, stdout, stderr)
	if code == 0 {
		t.Fatalf("expected non-zero exit code for negative volume")
	}
}
