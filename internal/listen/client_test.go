package listen

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"claudio.click/internal/sounds"
)

// startServer runs a real Server behind HTTP and returns its address and
// what it has played.
func startServer(t *testing.T, opts Options) (string, *recorder, *Server) {
	t.Helper()
	rec := &recorder{}
	server := New(opts, rec.play)
	httpServer := httptest.NewServer(server)
	t.Cleanup(httpServer.Close)
	return httpServer.URL, rec, server
}

func TestPostDeliversTheEventToAListener(t *testing.T) {
	t.Parallel()
	event := sounds.Event{
		Source:       "claude",
		Session:      "s1",
		Time:         time.Now().UTC().Truncate(time.Second),
		Category:     "success",
		Hint:         "git-commit-success",
		Command:      "git",
		Subcommand:   "commit",
		Phase:        "success",
		OriginalTool: "Bash",
		Operation:    "tool-complete",
	}
	for _, suffix := range []string{"", "/"} {
		url, rec, server := startServer(t, Options{Token: "s3cret", MaxAge: time.Minute})
		if err := Post(context.Background(), http.DefaultClient, url+suffix, "s3cret", event); err != nil {
			t.Fatalf("url suffix %q: %v", suffix, err)
		}
		server.Wait()
		played := rec.played()
		if len(played) != 1 {
			t.Fatalf("url suffix %q: played %d events, want 1", suffix, len(played))
		}
		got := played[0]
		if !got.Time.Equal(event.Time) {
			t.Errorf("time = %v, want %v", got.Time, event.Time)
		}
		got.Time = event.Time
		if got.Source != event.Source || got.Session != event.Session || got.Hint != event.Hint ||
			got.Command != event.Command || got.Subcommand != event.Subcommand || got.Phase != event.Phase ||
			got.OriginalTool != event.OriginalTool || got.Operation != event.Operation || got.Category != event.Category {
			t.Errorf("received %+v, sent %+v", got, event)
		}
	}
}

// A refusal is an error that says what the listener answered, so the hook
// log shows why no sound played.
func TestPostReportsARefusal(t *testing.T) {
	t.Parallel()
	url, rec, server := startServer(t, Options{Token: "s3cret"})
	err := Post(context.Background(), http.DefaultClient, url, "hunter2", sounds.Event{Category: "success"})
	server.Wait()
	if err == nil {
		t.Fatal("no error for a wrong token")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error %q does not carry the status", err)
	}
	if strings.Contains(err.Error(), "hunter2") {
		t.Errorf("error %q carries the token", err)
	}
	if len(rec.played()) != 0 {
		t.Error("a refused event was played")
	}
}

func TestPostReportsAnUnreachableListener(t *testing.T) {
	t.Parallel()
	httpServer := httptest.NewServer(http.NotFoundHandler())
	url := httpServer.URL
	httpServer.Close() // nothing listens there now
	if err := Post(context.Background(), http.DefaultClient, url, "", sounds.Event{Category: "success"}); err == nil {
		t.Error("no error for an unreachable listener")
	}
}

func TestPostStopsWhenTheContextEnds(t *testing.T) {
	t.Parallel()
	release := make(chan struct{})
	httpServer := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { <-release }))
	t.Cleanup(httpServer.Close)
	t.Cleanup(func() { close(release) })

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	started := time.Now()
	if err := Post(ctx, http.DefaultClient, httpServer.URL, "", sounds.Event{Category: "success"}); err == nil {
		t.Error("no error from a listener that never answers")
	}
	if waited := time.Since(started); waited > 5*time.Second {
		t.Errorf("waited %v for a 50ms deadline", waited)
	}
}
