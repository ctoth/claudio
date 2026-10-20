package cli

import (
	"bytes"
	"strings"
	"testing"

	"claudio.click/internal/cli/testenv"
)

func statusOutput(t *testing.T) string {
	t.Helper()
	var stdout, stderr bytes.Buffer
	if code := NewCLI().Run([]string{"claudio", "status"}, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	return stdout.String()
}

// Status says in words whether this machine plays its own sounds or sends
// them elsewhere, and never prints the token.
func TestStatusReportsForwarding(t *testing.T) {
	t.Run("off", func(t *testing.T) {
		testenv.IsolateXDG(t)
		if out := statusOutput(t); !strings.Contains(out, "forwarding:     off") {
			t.Errorf("status does not say forwarding is off:\n%s", out)
		}
	})
	t.Run("on with a token", func(t *testing.T) {
		testenv.IsolateXDG(t)
		t.Setenv("CLAUDIO_FORWARD_URL", "http://user:hunter2-PASSWORD@laptop.lan:19190")
		t.Setenv("CLAUDIO_FORWARD_TOKEN", "hunter2-TOKEN")
		out := statusOutput(t)
		if !strings.Contains(out, "forwarding:     on, to http://user:xxxxx@laptop.lan:19190 (token set)") {
			t.Errorf("status does not report the listener:\n%s", out)
		}
		if strings.Contains(out, "hunter2") {
			t.Errorf("status prints a secret:\n%s", out)
		}
	})
	t.Run("on without a token", func(t *testing.T) {
		testenv.IsolateXDG(t)
		t.Setenv("CLAUDIO_FORWARD_URL", "http://127.0.0.1:19190")
		if out := statusOutput(t); !strings.Contains(out, "forwarding:     on, to http://127.0.0.1:19190 (no token)") {
			t.Errorf("status does not report the listener:\n%s", out)
		}
	})
}
