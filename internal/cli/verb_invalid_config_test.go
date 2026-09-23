package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"claudio.click/internal/cli/testenv"
)

// Every config-writing verb refuses a config that ValidateConfig rejects,
// with the same error and without touching the file. volume and mute used
// to carry their own read-modify-write copy of mutateConfigForCommand.
func TestConfigVerbsRejectInvalidConfigAlike(t *testing.T) {
	commands := map[string][]string{
		"volume":        {"volume", "0.3"},
		"mute":          {"mute"},
		"unmute":        {"unmute"},
		"soundpack use": {"soundpack", "use", "windows"},
	}
	for name, verb := range commands {
		t.Run(name, func(t *testing.T) {
			testenv.IsolateXDG(t)
			configPath := filepath.Join(t.TempDir(), "config.json")
			seed := []byte(`{"default_soundpack": "windows", "log_level": "loud"}`)
			if err := os.WriteFile(configPath, seed, 0o644); err != nil {
				t.Fatal(err)
			}

			stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
			args := append([]string{"claudio", "--config", configPath}, verb...)
			if code := NewCLI().Run(args, nil, stdout, stderr); code != 1 {
				t.Fatalf("exit code = %d, want 1; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
			}

			want := "Error: failed to update config: load config " + configPath + ": "
			if !strings.Contains(stderr.String(), want) {
				t.Errorf("stderr = %q, want it to contain %q", stderr.String(), want)
			}
			if !strings.Contains(stderr.String(), "invalid log level 'loud'") {
				t.Errorf("stderr does not name the invalid value: %q", stderr.String())
			}
			after, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(after, seed) {
				t.Errorf("config rewritten:\n%s", after)
			}
		})
	}
}
