package listen

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"claudio.click/internal/sounds"
)

var testNow = time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)

// recorder collects the events a Server hands to its play function.
type recorder struct {
	mu     sync.Mutex
	events []sounds.Event
}

func (r *recorder) play(event sounds.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
}

func (r *recorder) played() []sounds.Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]sounds.Event(nil), r.events...)
}

// post sends one request to a fresh Server and returns the response and
// what was played once every sound in flight has finished.
func post(t *testing.T, opts Options, request *http.Request) (*http.Response, string, []sounds.Event) {
	t.Helper()
	if opts.Now == nil {
		opts.Now = func() time.Time { return testNow }
	}
	rec := &recorder{}
	server := New(opts, rec.play)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	server.Wait()
	result := response.Result()
	body, err := io.ReadAll(result.Body)
	if err != nil {
		t.Fatal(err)
	}
	return result, string(body), rec.played()
}

func eventRequest(body string) *http.Request {
	return httptest.NewRequest(http.MethodPost, "/events", strings.NewReader(body))
}

func TestAcceptedEventIsPlayed(t *testing.T) {
	t.Parallel()
	response, _, played := post(t, Options{}, eventRequest(
		`{"source":"claude","category":"success","hint":"git-commit-success","command":"git","phase":"success"}`))
	if response.StatusCode != http.StatusAccepted {
		t.Errorf("status = %d, want 202", response.StatusCode)
	}
	if len(played) != 1 {
		t.Fatalf("played %d events, want 1", len(played))
	}
	if got := played[0]; got.Source != "claude" || got.Category != "success" || got.Hint != "git-commit-success" || got.Command != "git" {
		t.Errorf("played %+v", got)
	}
}

func TestTokenIsRequiredWhenSet(t *testing.T) {
	t.Parallel()
	const body = `{"category":"success"}`
	tests := []struct {
		name   string
		header string
		status int
		plays  int
	}{
		{"no header", "", http.StatusUnauthorized, 0},
		{"wrong token", "Bearer nope", http.StatusUnauthorized, 0},
		{"token without scheme", "s3cret", http.StatusUnauthorized, 0},
		{"right token", "Bearer s3cret", http.StatusAccepted, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			request := eventRequest(body)
			if tc.header != "" {
				request.Header.Set("Authorization", tc.header)
			}
			response, _, played := post(t, Options{Token: "s3cret"}, request)
			if response.StatusCode != tc.status || len(played) != tc.plays {
				t.Errorf("status %d with %d plays, want %d with %d", response.StatusCode, len(played), tc.status, tc.plays)
			}
		})
	}
}

func TestBadEventsAreRejected(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		body   string
		status int
	}{
		{"not JSON", `category=success`, http.StatusBadRequest},
		{"no category", `{"hint":"x"}`, http.StatusBadRequest},
		{"unknown category", `{"category":"celebration"}`, http.StatusBadRequest},
		{"too large", `{"category":"success","hint":"` + strings.Repeat("x", maxEventBytes) + `"}`, http.StatusRequestEntityTooLarge},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			response, _, played := post(t, Options{}, eventRequest(tc.body))
			if response.StatusCode != tc.status || len(played) != 0 {
				t.Errorf("status %d with %d plays, want %d with none", response.StatusCode, len(played), tc.status)
			}
		})
	}
}

// A rejected body is not echoed: an emitter may have sent the wrong thing,
// such as a whole hook payload with a prompt in it.
func TestRejectionDoesNotEchoTheBody(t *testing.T) {
	t.Parallel()
	_, body, _ := post(t, Options{}, eventRequest(`{"category":"celebration","prompt":"hunter2"}`))
	if strings.Contains(body, "hunter2") {
		t.Errorf("response echoes the request: %q", body)
	}
}

func TestOnlyPostToEventsIsServed(t *testing.T) {
	t.Parallel()
	for _, request := range []*http.Request{
		httptest.NewRequest(http.MethodGet, "/events", nil),
		httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"category":"success"}`)),
	} {
		response, _, played := post(t, Options{}, request)
		if response.StatusCode < 400 || len(played) != 0 {
			t.Errorf("%s %s: status %d with %d plays", request.Method, request.URL.Path, response.StatusCode, len(played))
		}
	}
}

// A sound that arrives long after its event is noise. The event's own time
// decides; an event without one is always played.
func TestStaleEventsAreDropped(t *testing.T) {
	t.Parallel()
	at := func(age time.Duration) string {
		return `{"category":"success","time":"` + testNow.Add(-age).Format(time.RFC3339) + `"}`
	}
	tests := []struct {
		name   string
		maxAge time.Duration
		body   string
		plays  int
	}{
		{"fresh", 30 * time.Second, at(5 * time.Second), 1},
		{"stale", 30 * time.Second, at(31 * time.Second), 0},
		{"no time", 30 * time.Second, `{"category":"success"}`, 1},
		{"sender clock ahead", 30 * time.Second, at(-10 * time.Minute), 1},
		{"check off", 0, at(time.Hour), 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			response, body, played := post(t, Options{MaxAge: tc.maxAge}, eventRequest(tc.body))
			if response.StatusCode != http.StatusAccepted {
				t.Errorf("status = %d, want 202", response.StatusCode)
			}
			if len(played) != tc.plays {
				t.Errorf("played %d, want %d", len(played), tc.plays)
			}
			if tc.plays == 0 && !strings.Contains(body, "stale") {
				t.Errorf("body %q does not say the event was stale", body)
			}
		})
	}
}

// The emitter is a hook: it gets its answer before the sound has played.
func TestServeDoesNotWaitForTheSound(t *testing.T) {
	t.Parallel()
	release := make(chan struct{})
	started := make(chan struct{})
	server := New(Options{}, func(sounds.Event) {
		close(started)
		<-release
	})
	response := httptest.NewRecorder()
	server.ServeHTTP(response, eventRequest(`{"category":"success"}`)) // returns while the sound is held
	if response.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", response.Code)
	}
	<-started
	close(release)
	server.Wait()
}

// Sounds play at once and may overlap, but a flood must not pile up
// without bound: past the limit an event is refused.
func TestEventsPastThePlayLimitAreRefused(t *testing.T) {
	t.Parallel()
	release := make(chan struct{})
	rec := &recorder{}
	server := New(Options{}, func(event sounds.Event) {
		<-release
		rec.play(event)
	})
	var refused int
	for range maxConcurrentPlays + 3 {
		response := httptest.NewRecorder()
		server.ServeHTTP(response, eventRequest(`{"category":"success"}`))
		if response.Code == http.StatusServiceUnavailable {
			refused++
		}
	}
	close(release)
	server.Wait()
	if refused != 3 || len(rec.played()) != maxConcurrentPlays {
		t.Errorf("refused %d and played %d, want 3 and %d", refused, len(rec.played()), maxConcurrentPlays)
	}
}
