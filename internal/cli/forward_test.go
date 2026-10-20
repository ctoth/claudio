package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"claudio.click/internal/audio/audiotest"
	"claudio.click/internal/cli/testenv"
	"claudio.click/internal/sounds"
)

// forwardSecret stands in for everything in a hook payload that must stay
// on the machine the hook ran on.
const forwardSecret = "hunter2-FORWARD-SECRET"

const forwardedHook = `{"session_id":"sess-1","transcript_path":"/t/` + forwardSecret + `","cwd":"/c/` + forwardSecret + `",` +
	`"hook_event_name":"PostToolUse","tool_name":"Bash",` +
	`"tool_input":{"command":"git commit -m ` + forwardSecret + `"},` +
	`"tool_response":{"stdout":"` + forwardSecret + `","stderr":"","interrupted":false}}`

// received is one request a capture server got.
type received struct {
	path, auth string
	body       []byte
}

// captureServer stands in for a listener and records what it is sent.
func captureServer(t *testing.T, status int) (string, func() []received) {
	t.Helper()
	var mu sync.Mutex
	var requests []received
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		requests = append(requests, received{path: r.URL.Path, auth: r.Header.Get("Authorization"), body: body})
		mu.Unlock()
		w.WriteHeader(status)
	}))
	t.Cleanup(server.Close)
	return server.URL, func() []received {
		mu.Lock()
		defer mu.Unlock()
		return append([]received(nil), requests...)
	}
}

func runHook(t *testing.T, payload string, args ...string) (int, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := NewCLI().Run(append([]string{"claudio"}, args...), strings.NewReader(payload), &stdout, &stderr)
	return code, stderr.String()
}

// With a forward URL the hook sends the names a sound is chosen from, and
// nothing else in the payload, and plays nothing here.
func TestHookForwardsTheEventInsteadOfPlayingIt(t *testing.T) {
	testenv.IsolateXDG(t)
	audiotest.ResetLastFakeBackend()
	url, requests := captureServer(t, http.StatusOK)
	// Any address that takes a POST will do; the hook does not rewrite it.
	t.Setenv("CLAUDIO_FORWARD_URL", url+"/topics/my-sounds?priority=low")
	t.Setenv("CLAUDIO_FORWARD_TOKEN", "s3cret")
	trackingDB := filepath.Join(t.TempDir(), "sounds.db")
	t.Setenv("CLAUDIO_SOUND_TRACKING", "true")
	t.Setenv("CLAUDIO_SOUND_TRACKING_DB", trackingDB)

	before := time.Now().Add(-time.Second)
	if code, stderr := runHook(t, forwardedHook, "--hook-agent", "claude"); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}

	got := requests()
	if len(got) != 1 {
		t.Fatalf("listener got %d requests, want 1", len(got))
	}
	if got[0].path != "/topics/my-sounds" || got[0].auth != "Bearer s3cret" {
		t.Errorf("request to %q with Authorization %q", got[0].path, got[0].auth)
	}
	if bytes.Contains(got[0].body, []byte(forwardSecret)) {
		t.Errorf("payload text was forwarded: %s", got[0].body)
	}
	var event sounds.Event
	if err := json.Unmarshal(got[0].body, &event); err != nil {
		t.Fatalf("body %s: %v", got[0].body, err)
	}
	if event.Time.Before(before) || event.Time.After(time.Now().Add(time.Second)) {
		t.Errorf("time = %v, want now", event.Time)
	}
	event.Time = time.Time{}
	want := sounds.Event{
		Source: "claude", Session: "sess-1",
		Category: "success", Hint: "git-commit-success", Command: "git", Subcommand: "commit",
		Phase: "success", OriginalTool: "Bash", Operation: "tool-complete",
	}
	gotJSON, _ := json.Marshal(event)
	wantJSON, _ := json.Marshal(want)
	if string(gotJSON) != string(wantJSON) {
		t.Errorf("event\n got: %s\nwant: %s", gotJSON, wantJSON)
	}

	// A forwarding machine has no audio device to open and keeps no
	// tracking database: the listening machine does both.
	if fake := audiotest.LastFakeBackend(); fake != nil {
		t.Errorf("an audio backend was created; plays: %+v", fake.Plays())
	}
	if _, err := os.Stat(trackingDB); err == nil {
		t.Error("a tracking database was created on the forwarding machine")
	}
}

func TestMutedHookForwardsNothing(t *testing.T) {
	testenv.IsolateXDG(t)
	url, requests := captureServer(t, http.StatusAccepted)
	t.Setenv("CLAUDIO_FORWARD_URL", url)
	if code, stderr := runHook(t, forwardedHook, "--silent"); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	if got := requests(); len(got) != 0 {
		t.Errorf("a muted hook sent %d requests", len(got))
	}
}

// An event the mapper would play nothing for is not worth a request.
func TestSilentEventIsNotForwarded(t *testing.T) {
	testenv.IsolateXDG(t)
	url, requests := captureServer(t, http.StatusAccepted)
	t.Setenv("CLAUDIO_FORWARD_URL", url)
	silent := `{"session_id":"s","transcript_path":"/t","cwd":"/c","hook_event_name":"MessageDisplay"}`
	if code, stderr := runHook(t, silent); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	if got := requests(); len(got) != 0 {
		t.Errorf("a silent event sent %d requests", len(got))
	}
}

// A listener that is down or says no must not fail the agent's hook, and
// the sound is not played here instead.
func TestHookSucceedsWhenTheListenerDoesNot(t *testing.T) {
	refusing, _ := captureServer(t, http.StatusUnauthorized)
	gone := httptest.NewServer(http.NotFoundHandler())
	unreachable := gone.URL
	gone.Close()

	for name, url := range map[string]string{"refuses": refusing, "unreachable": unreachable} {
		t.Run(name, func(t *testing.T) {
			testenv.IsolateXDG(t)
			audiotest.ResetLastFakeBackend()
			t.Setenv("CLAUDIO_FORWARD_URL", url)
			code, stderr := runHook(t, forwardedHook)
			if code != 0 {
				t.Errorf("exit %d, stderr %q", code, stderr)
			}
			if stderr != "" {
				t.Errorf("stderr %q: an agent shows hook stderr to the user", stderr)
			}
			if audiotest.LastFakeBackend() != nil {
				t.Error("fell back to local playback")
			}
		})
	}
}

// The two halves together: a hook on one side, a listener on the other, a
// sound out of the listener's backend.
func TestForwardedHookPlaysOnTheListener(t *testing.T) {
	testenv.IsolateXDG(t)
	audiotest.ResetLastFakeBackend()
	l := startListener(t, "--token", "s3cret")
	t.Setenv("CLAUDIO_FORWARD_URL", l.url+"/events")
	t.Setenv("CLAUDIO_FORWARD_TOKEN", "s3cret")

	if code, stderr := runHook(t, forwardedHook); code != 0 {
		t.Fatalf("hook exit %d, stderr %q", code, stderr)
	}
	if code, stderr := l.stop(); code != 0 {
		t.Fatalf("listener exit %d, stderr %q", code, stderr)
	}
	plays := audiotest.LastFakeBackend().Plays()
	if len(plays) != 1 || plays[0].SourcePath == "" {
		t.Errorf("listener plays = %+v, want one with a sound file", plays)
	}
}
