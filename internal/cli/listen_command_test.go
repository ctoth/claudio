package cli

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"claudio.click/internal/audio/audiotest"
	"claudio.click/internal/cli/testenv"
)

// listener is one `claudio listen` running in-process.
type listener struct {
	url  string
	stop func() (exitCode int, stderr string)
}

// startListener runs `claudio listen` on a free loopback port and returns
// once it has printed its address.
func startListener(t *testing.T, args ...string) listener {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	cli := NewCLI()
	cli.rootCmd.SetContext(ctx)

	stdoutR, stdoutW := io.Pipe()
	var stderr bytes.Buffer
	exited := make(chan int, 1)
	go func() {
		code := cli.Run(append([]string{"claudio", "listen", "--addr", "127.0.0.1:0"}, args...), strings.NewReader(""), stdoutW, &stderr)
		_ = stdoutW.Close()
		exited <- code
	}()

	lines := make(chan string, 1)
	go func() {
		reader := bufio.NewReader(stdoutR)
		line, _ := reader.ReadString('\n')
		lines <- line
		_, _ = io.Copy(io.Discard, reader)
	}()

	stop := func() (int, string) {
		cancel()
		select {
		case code := <-exited:
			return code, stderr.String()
		case <-time.After(10 * time.Second):
			t.Fatal("claudio listen did not stop")
			return 0, ""
		}
	}

	select {
	case line := <-lines:
		url, ok := strings.CutPrefix(strings.TrimSpace(line), "listening on ")
		if !ok {
			code, errText := stop()
			t.Fatalf("first line %q is not the address (exit %d, stderr %q)", line, code, errText)
		}
		return listener{url: url, stop: stop}
	case <-time.After(10 * time.Second):
		cancel()
		t.Fatal("claudio listen printed no address")
		return listener{}
	}
}

func postEvent(t *testing.T, url, token, body string) int {
	t.Helper()
	request, err := http.NewRequest(http.MethodPost, url+"/events", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	return response.StatusCode
}

// An event posted from elsewhere plays a sound here, through the same
// soundpack and backend a local hook uses.
func TestListenPlaysAPostedEvent(t *testing.T) {
	testenv.IsolateXDG(t)
	audiotest.ResetLastFakeBackend()
	l := startListener(t)

	status := postEvent(t, l.url, "",
		`{"source":"claude","category":"success","hint":"bash-success","command":"Bash","phase":"success","operation":"tool-complete"}`)
	if status != http.StatusAccepted {
		t.Errorf("status = %d, want 202", status)
	}

	// Stopping waits for sounds in flight, so the play is recorded by now.
	if code, stderr := l.stop(); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	fake := audiotest.LastFakeBackend()
	if fake == nil {
		t.Fatal("no audio backend was created")
	}
	plays := fake.Plays()
	if len(plays) != 1 || plays[0].SourcePath == "" {
		t.Errorf("plays = %+v, want one with a sound file", plays)
	}
}

// A token on the command line is visible to other users of the machine; a
// token file is not.
func TestListenTokenComesFromTheFlagOrAFile(t *testing.T) {
	tokenFile := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(tokenFile, []byte("s3cret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"flag", []string{"--token", "s3cret"}},
		{"file", []string{"--token-file", tokenFile}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testenv.IsolateXDG(t)
			l := startListener(t, tc.args...)
			without := postEvent(t, l.url, "", `{"category":"success"}`)
			with := postEvent(t, l.url, "s3cret", `{"category":"success"}`)
			if code, stderr := l.stop(); code != 0 {
				t.Fatalf("exit %d, stderr %q", code, stderr)
			}
			if without != http.StatusUnauthorized || with != http.StatusAccepted {
				t.Errorf("without token %d, with token %d; want 401 and 202", without, with)
			}
		})
	}
}

// Without a token anyone who can reach the port can play sounds, so only a
// loopback address may go without one.
func TestListenRefusesAnOpenAddressWithoutAToken(t *testing.T) {
	for _, addr := range []string{"0.0.0.0:0", ":0", "192.0.2.1:0"} {
		t.Run(addr, func(t *testing.T) {
			testenv.IsolateXDG(t)
			var stdout, stderr bytes.Buffer
			code := NewCLI().Run([]string{"claudio", "listen", "--addr", addr}, strings.NewReader(""), &stdout, &stderr)
			if code == 0 {
				t.Fatalf("exit 0; stdout %q", stdout.String())
			}
			if !strings.Contains(stderr.String(), "token") {
				t.Errorf("stderr %q does not say a token is needed", stderr.String())
			}
		})
	}
}

// A token file that is missing or empty must not quietly mean "no token".
func TestListenRefusesAnUnusableTokenFile(t *testing.T) {
	dir := t.TempDir()
	empty := filepath.Join(dir, "empty")
	if err := os.WriteFile(empty, []byte(" \n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, args := range map[string][]string{
		"missing":       {"--token-file", filepath.Join(dir, "absent")},
		"empty":         {"--token-file", empty},
		"flag and file": {"--token", "a", "--token-file", empty},
	} {
		t.Run(name, func(t *testing.T) {
			testenv.IsolateXDG(t)
			var stdout, stderr bytes.Buffer
			code := NewCLI().Run(append([]string{"claudio", "listen", "--addr", "127.0.0.1:0"}, args...), strings.NewReader(""), &stdout, &stderr)
			if code == 0 {
				t.Fatalf("exit 0; stdout %q", stdout.String())
			}
			if !strings.Contains(stderr.String(), "token") {
				t.Errorf("stderr %q does not name the token", stderr.String())
			}
		})
	}
}

func TestIsLoopbackAddr(t *testing.T) {
	t.Parallel()
	for addr, want := range map[string]bool{
		"127.0.0.1:19190": true,
		"localhost:19190": true,
		"[::1]:19190":     true,
		"0.0.0.0:19190":   false,
		":19190":          false,
		"192.168.1.5:80":  false,
		"example.com:80":  false,
		"nonsense":        false,
	} {
		if got := isLoopbackAddr(addr); got != want {
			t.Errorf("isLoopbackAddr(%q) = %v, want %v", addr, got, want)
		}
	}
}
