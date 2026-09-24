package cli

import (
	"bytes"
	"strings"
	"testing"

	"claudio.click/internal/cli/testenv"
)

// With sound tracking off there is nothing to analyze, which is not an
// error: both analyze subcommands print the same hint and exit 0.
func TestAnalyzeWithTrackingDisabledPrintsHint(t *testing.T) {
	for _, sub := range []string{"missing", "usage"} {
		t.Run(sub, func(t *testing.T) {
			testenv.IsolateXDG(t)
			t.Setenv("CLAUDIO_SOUND_TRACKING", "false")

			stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
			code := NewCLI().Run([]string{"claudio", "analyze", sub}, strings.NewReader(""), stdout, stderr)
			if code != 0 {
				t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
			}
			if stderr.Len() != 0 {
				t.Errorf("stderr = %q, want empty", stderr.String())
			}
			want := "Sound tracking is not enabled or database not available.\n" +
				"Enable tracking with CLAUDIO_SOUND_TRACKING=true\n"
			if stdout.String() != want {
				t.Errorf("stdout = %q, want %q", stdout.String(), want)
			}
		})
	}
}
