package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"claudio.click/internal/cli/testenv"
)

// A failed command reports its error exactly once on stderr, prefixed
// "Error: ", and never dumps usage text: the error is not a usage problem,
// and usage on stdout reads as hook output to an agent.
func TestCommandErrorPrintedOnceWithoutUsage(t *testing.T) {
	cases := []struct {
		name   string
		args   []string
		needle string
	}{
		{"root invalid volume flag", []string{"--volume", "abc"}, "invalid volume value"},
		{"root out of range volume flag", []string{"--volume", "7"}, "between 0.0 and 1.0"},
		{"volume out of range", []string{"volume", "2"}, "between 0.0 and 1.0"},
		{"install bogus agent", []string{"install", "--agent", "bogus"}, "bogus"},
		{"uninstall bogus scope", []string{"uninstall", "--scope", "bogus"}, "bogus"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			testenv.IsolateXDG(t)
			t.Setenv("CLAUDIO_DETACH_DISABLE", "1")
			configPath := filepath.Join(t.TempDir(), "config.json")
			args := append([]string{"claudio", "--config", configPath}, tc.args...)

			stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
			code := NewCLI().Run(args, strings.NewReader(""), stdout, stderr)
			if code != 1 {
				t.Fatalf("exit code = %d, want 1; stderr=%s", code, stderr.String())
			}

			errOut := stderr.String()
			for name, s := range map[string]string{"stdout": stdout.String(), "stderr": errOut} {
				if strings.Contains(s, "Usage:") {
					t.Errorf("printed usage text on %s:\n%s", name, s)
				}
			}
			if n := strings.Count(errOut, tc.needle); n != 1 {
				t.Errorf("error text %q appears %d times on stderr, want 1:\n%s", tc.needle, n, errOut)
			}
			if n := strings.Count(errOut, "Error: "); n != 1 {
				t.Errorf("want exactly one \"Error: \" line, got %d:\n%s", n, errOut)
			}
		})
	}
}
