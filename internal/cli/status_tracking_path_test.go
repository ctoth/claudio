package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"claudio.click/internal/cli/testenv"
	"claudio.click/internal/config"
	"claudio.click/internal/tracking"
)

// status used to print "enabled ((default XDG path))"; it now prints the
// resolved default database path.
func TestStatusCommand_TrackingShowsResolvedDefaultPath(t *testing.T) {
	testenv.IsolateXDG(t)
	configPath := filepath.Join(t.TempDir(), "config.json")
	writeSeedConfig(t, configPath, &config.Config{
		DefaultSoundpack: "x",
		Enabled:          true,
		LogLevel:         "warn",
		AudioBackend:     "auto",
		SoundTracking:    &config.SoundTrackingConfig{Enabled: true},
	})

	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	if code := NewCLI().Run([]string{"claudio", "status", "--config", configPath},
		strings.NewReader(""), stdout, stderr); code != 0 {
		t.Fatalf("exit code = %d; stderr=%s", code, stderr.String())
	}

	out := stdout.String()
	if strings.Contains(out, "((") {
		t.Errorf("tracking line has doubled parentheses: %q", out)
	}
	want := "tracking:       enabled (" + tracking.DefaultDatabasePath() + ")"
	if !strings.Contains(out, want) {
		t.Errorf("expected %q in output, got: %q", want, out)
	}
}
