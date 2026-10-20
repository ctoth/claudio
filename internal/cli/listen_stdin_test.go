package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"testing/iotest"

	"claudio.click/internal/audio/audiotest"
	"claudio.click/internal/cli/testenv"
)

// Events piped in are played and no web server is started: whatever brought
// them to the pipe is not claudio's business.
func TestListenStdinPlaysPipedEvents(t *testing.T) {
	testenv.IsolateXDG(t)
	audiotest.ResetLastFakeBackend()
	input := `{"source":"relay","category":"success","hint":"bash-success","command":"Bash","phase":"success","operation":"tool-complete"}` + "\n" +
		"\n" +
		"this line is not an event\n" +
		`{"category":"completion","hint":"agent-complete","operation":"stop"}` + "\n"

	var stdout, stderr bytes.Buffer
	code := NewCLI().Run([]string{"claudio", "listen", "--stdin"}, strings.NewReader(input), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout %q: nothing listens on an address in stdin mode", stdout.String())
	}
	fake := audiotest.LastFakeBackend()
	if fake == nil {
		t.Fatal("no audio backend was created")
	}
	if plays := fake.Plays(); len(plays) != 2 {
		t.Errorf("plays = %+v, want two", plays)
	}
}

// When the pipe breaks the command fails, so a supervising loop or service
// knows to reconnect.
func TestListenStdinFailsWhenThePipeBreaks(t *testing.T) {
	testenv.IsolateXDG(t)
	var stdout, stderr bytes.Buffer
	code := NewCLI().Run([]string{"claudio", "listen", "--stdin"}, iotest.ErrReader(errors.New("pipe broke")), &stdout, &stderr)
	if code == 0 {
		t.Fatal("exit 0 after a read error")
	}
	if !strings.Contains(stderr.String(), "pipe broke") {
		t.Errorf("stderr %q does not carry the read error", stderr.String())
	}
}

// An address or a token means nothing without a web server; saying so beats
// silently ignoring them.
func TestListenStdinRejectsWebServerFlags(t *testing.T) {
	for _, args := range [][]string{
		{"--stdin", "--addr", "127.0.0.1:0"},
		{"--stdin", "--token", "s3cret"},
		{"--stdin", "--token-file", "token.txt"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			testenv.IsolateXDG(t)
			var stdout, stderr bytes.Buffer
			code := NewCLI().Run(append([]string{"claudio", "listen"}, args...), strings.NewReader(""), &stdout, &stderr)
			if code == 0 {
				t.Fatal("exit 0")
			}
			if !strings.Contains(stderr.String(), "stdin") {
				t.Errorf("stderr %q does not name --stdin", stderr.String())
			}
		})
	}
}
