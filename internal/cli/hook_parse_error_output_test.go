package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"claudio.click/internal/cli/testenv"
	"claudio.click/internal/config"
)

// A malformed hook payload is a runtime error, not a usage error: stderr
// should carry the parse error once, without Cobra's full usage text.
func TestHookModeMalformedPayloadPrintsErrorWithoutUsage(t *testing.T) {
	testenv.IsolateXDG(t)
	t.Setenv("CLAUDIO_DETACH_DISABLE", "1")
	configPath := filepath.Join(t.TempDir(), "config.json")
	writeSeedConfig(t, configPath, &config.Config{
		DefaultSoundpack: "x",
		Enabled:          true,
		LogLevel:         "warn",
		AudioBackend:     "oto",
	})

	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	code := NewCLI().Run([]string{"claudio", "--config", configPath, "--silent"},
		strings.NewReader("not json"), stdout, stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stderr=%s", code, stderr.String())
	}

	// Cobra prints usage to the command's stdout, which agents read as hook
	// output, so check both streams.
	errOut := stderr.String()
	for name, s := range map[string]string{"stdout": stdout.String(), "stderr": errOut} {
		if strings.Contains(s, "Usage:") {
			t.Errorf("malformed payload printed usage text on %s:\n%s", name, s)
		}
	}
	if n := strings.Count(errOut, "Error: "); n != 1 {
		t.Errorf("want exactly one \"Error: \" line, got %d:\n%s", n, errOut)
	}
	if !strings.Contains(errOut, "parse hook JSON") {
		t.Errorf("parse error missing from stderr:\n%s", errOut)
	}
}
