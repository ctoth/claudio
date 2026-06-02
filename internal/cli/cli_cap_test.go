package cli

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"claudio.click/internal/cli/testenv"
	"claudio.click/internal/safeio"
)

// oversizedReader reports a size one byte over MaxHookPayloadBytes
// without allocating that many bytes up front. The CLI's hook input
// reader should reject the read before consuming the whole stream.
type oversizedReader struct {
	remaining int64
}

func (o *oversizedReader) Read(p []byte) (int, error) {
	if o.remaining <= 0 {
		return 0, io.EOF
	}
	n := int64(len(p))
	if n > o.remaining {
		n = o.remaining
	}
	for i := int64(0); i < n; i++ {
		// Cycle alphanumerics so the bytes are valid stream bytes but
		// not parseable JSON.
		p[i] = 'x'
	}
	o.remaining -= n
	return int(n), nil
}

// TestCLI_StdinPayloadCap asserts that an oversized hook payload on
// stdin causes the CLI to fail rather than consuming all the bytes and
// attempting to parse them. The stdin contains MaxHookPayloadBytes+1
// non-JSON bytes; the cap fires first.
func TestCLI_StdinPayloadCap(t *testing.T) {
	testenv.IsolateXDG(t)
	cli := NewCLI()

	r := &oversizedReader{remaining: safeio.MaxHookPayloadBytes + 1}
	var stdout, stderr bytes.Buffer

	exitCode := cli.Run([]string{"claudio", "--silent"}, r, &stdout, &stderr)
	if exitCode == 0 {
		t.Fatalf("expected non-zero exit for oversized stdin payload, got 0")
	}
	combined := stdout.String() + stderr.String()
	if !strings.Contains(combined, "hook payload") {
		t.Errorf("expected output to mention 'hook payload' from safeio cap, got: %s", combined)
	}
}
