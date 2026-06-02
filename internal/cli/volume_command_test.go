package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"claudio.click/internal/cli/testenv"
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
	testenv.IsolateXDG(t)
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
	testenv.IsolateXDG(t)
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
	testenv.IsolateXDG(t)
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
	testenv.IsolateXDG(t)
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
	testenv.IsolateXDG(t)
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
	testenv.IsolateXDG(t)
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

	// Use "--" so cobra stops flag parsing and the verb sees "-0.1" as a
	// positional arg. Without "--" cobra rejects "-0.1" as a malformed
	// shorthand flag BEFORE runVolumeE runs, so the verb's own range check
	// (the code we want to protect) would never execute and a refactor
	// that drops the v < 0.0 arm would pass this test.
	code := cli.Run([]string{"claudio", "volume", "--config", configPath, "--", "-0.1"}, stdin, stdout, stderr)
	if code == 0 {
		t.Fatalf("expected non-zero exit code for negative volume")
	}

	// The verb's own range-check message must be the cause — not cobra's
	// flag parser. The CLI writes the error to stderr.
	combined := stderr.String() + stdout.String()
	if !strings.Contains(combined, "must be between 0.0 and 1.0") {
		t.Errorf("expected verb's range-check error, got: stderr=%q stdout=%q",
			stderr.String(), stdout.String())
	}

	// Persisted value must not have been overwritten.
	persisted := readPersistedConfig(t, configPath)
	if persisted.Volume == nil || *persisted.Volume != 0.2 {
		t.Errorf("Volume should be unchanged after rejection, got %v", persisted.Volume)
	}
}

func TestVolumeCommand_RejectsNaN(t *testing.T) {
	testenv.IsolateXDG(t)
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

	// strconv.ParseFloat accepts "NaN" as a valid float, so the parse step
	// succeeds and the math.IsNaN branch is the only thing rejecting this.
	// A refactor that drops that branch would silently allow NaN through.
	code := cli.Run([]string{"claudio", "volume", "NaN", "--config", configPath}, stdin, stdout, stderr)
	if code == 0 {
		t.Fatalf("expected non-zero exit code for NaN; stdout=%q stderr=%q",
			stdout.String(), stderr.String())
	}
	combined := stderr.String() + stdout.String()
	if !strings.Contains(combined, "finite") {
		t.Errorf("expected error mentioning 'finite' for NaN, got: stderr=%q stdout=%q",
			stderr.String(), stdout.String())
	}

	// Persisted value must not have been overwritten.
	persisted := readPersistedConfig(t, configPath)
	if persisted.Volume == nil || *persisted.Volume != 0.2 {
		t.Errorf("Volume should be unchanged after NaN rejection, got %v", persisted.Volume)
	}
}

func TestVolumeCommand_RejectsInf(t *testing.T) {
	testenv.IsolateXDG(t)
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

	// Same shape as NaN: ParseFloat accepts "Inf"; the math.IsInf branch
	// is what rejects it. Guard that branch from accidental removal.
	code := cli.Run([]string{"claudio", "volume", "Inf", "--config", configPath}, stdin, stdout, stderr)
	if code == 0 {
		t.Fatalf("expected non-zero exit code for Inf; stdout=%q stderr=%q",
			stdout.String(), stderr.String())
	}
	combined := stderr.String() + stdout.String()
	if !strings.Contains(combined, "finite") {
		t.Errorf("expected error mentioning 'finite' for Inf, got: stderr=%q stdout=%q",
			stderr.String(), stdout.String())
	}

	// Persisted value must not have been overwritten.
	persisted := readPersistedConfig(t, configPath)
	if persisted.Volume == nil || *persisted.Volume != 0.2 {
		t.Errorf("Volume should be unchanged after Inf rejection, got %v", persisted.Volume)
	}
}

// TestVolumeCommand_NoArgsRespectsEnvVar is the F1 regression test. The
// zero-arg read path must apply CLAUDIO_VOLUME the same way `claudio status`
// does — otherwise the two read commands disagree about the effective volume.
// The WRITE path is covered separately by TestVolumeCommand_WriteIgnoresEnvVar.
func TestVolumeCommand_NoArgsRespectsEnvVar(t *testing.T) {
	testenv.IsolateXDG(t)
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "config.json")

	persisted := 0.3
	writeSeedConfig(t, configPath, &config.Config{
		Volume:           &persisted,
		DefaultSoundpack: "default",
		Enabled:          true,
		LogLevel:         "warn",
		AudioBackend:     "auto",
	})

	t.Setenv("CLAUDIO_VOLUME", "0.7")

	cli := NewCLI()
	stdin := strings.NewReader("")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	code := cli.Run([]string{"claudio", "volume", "--config", configPath}, stdin, stdout, stderr)
	if code != 0 {
		t.Fatalf("exit code = %d; stderr=%s", code, stderr.String())
	}

	out := stdout.String()
	if !strings.Contains(out, "0.70") {
		t.Errorf("expected env value 0.70 in output, got: %q", out)
	}
	if !strings.Contains(out, "CLAUDIO_VOLUME") {
		t.Errorf("expected source annotation 'CLAUDIO_VOLUME' in output, got: %q", out)
	}
	// Must NOT contain the persisted value as the headline number.
	if strings.Contains(out, "0.30") {
		t.Errorf("expected env to override persisted 0.30, got: %q", out)
	}
}

// TestVolumeCommand_WriteIgnoresEnvVar pins the WRITE-path semantics: even
// with CLAUDIO_VOLUME set, `claudio volume 0.4` persists 0.4 verbatim.
// Persistence must be deterministic regardless of env state.
func TestVolumeCommand_WriteIgnoresEnvVar(t *testing.T) {
	testenv.IsolateXDG(t)
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "config.json")

	startVol := 0.2
	writeSeedConfig(t, configPath, &config.Config{
		Volume:           &startVol,
		DefaultSoundpack: "default",
		Enabled:          true,
		LogLevel:         "warn",
		AudioBackend:     "auto",
	})

	t.Setenv("CLAUDIO_VOLUME", "0.9")

	cli := NewCLI()
	stdin := strings.NewReader("")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	code := cli.Run([]string{"claudio", "volume", "0.4", "--config", configPath}, stdin, stdout, stderr)
	if code != 0 {
		t.Fatalf("exit code = %d; stderr=%s", code, stderr.String())
	}

	persisted := readPersistedConfig(t, configPath)
	if persisted.Volume == nil {
		t.Fatal("Volume should be persisted, got nil")
	}
	if *persisted.Volume != 0.4 {
		t.Errorf("persisted Volume = %v, want 0.4 (env should NOT influence write)",
			*persisted.Volume)
	}
}
