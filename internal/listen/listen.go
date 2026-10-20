// Package listen receives sound events over HTTP, so the machine that sees
// an event and the machine that plays its sound can be different machines.
package listen

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"claudio.click/internal/sounds"
)

const (
	// maxEventBytes caps a request body. An event is a few short names.
	maxEventBytes = 64 << 10
	// maxConcurrentPlays caps the sounds in flight; an event past it is refused.
	maxConcurrentPlays = 8
)

// Options configures a Server.
type Options struct {
	// Token, when set, must arrive as "Authorization: Bearer <token>".
	Token string
	// MaxAge drops an event whose Time is older than this. Zero plays
	// every event.
	MaxAge time.Duration
	// Now is the clock; nil means time.Now.
	Now func() time.Time
}

// Server is an http.Handler that takes one sounds.Event per POST to
// /events and hands it to its play function.
type Server struct {
	opts  Options
	play  func(sounds.Event)
	mux   *http.ServeMux
	slots chan struct{}
	plays sync.WaitGroup
}

// New returns a Server. play is called on its own goroutine for each
// accepted event and may block for as long as the sound lasts.
func New(opts Options, play func(sounds.Event)) *Server {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	s := &Server{
		opts:  opts,
		play:  play,
		mux:   http.NewServeMux(),
		slots: make(chan struct{}, maxConcurrentPlays),
	}
	s.mux.HandleFunc("POST /events", s.handleEvent)
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// Wait returns when every sound in flight has finished.
func (s *Server) Wait() {
	s.plays.Wait()
}

func (s *Server) authorized(r *http.Request) bool {
	if s.opts.Token == "" {
		return true
	}
	token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	return ok && subtle.ConstantTimeCompare([]byte(token), []byte(s.opts.Token)) == 1
}

// handleEvent never logs or echoes the body: a misconfigured emitter may
// send a whole hook payload, prompt included.
func (s *Server) handleEvent(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		slog.Warn("event refused: missing or wrong token", "remote", r.RemoteAddr)
		http.Error(w, "missing or wrong token", http.StatusUnauthorized)
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxEventBytes))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			slog.Warn("event refused: body too large", "remote", r.RemoteAddr, "limit_bytes", maxEventBytes)
			http.Error(w, "event too large", http.StatusRequestEntityTooLarge)
			return
		}
		slog.Warn("event refused: unreadable body", "remote", r.RemoteAddr, "error", err)
		http.Error(w, "unreadable body", http.StatusBadRequest)
		return
	}

	switch s.submit(body, r.RemoteAddr) {
	case notAnEvent:
		http.Error(w, "body is not a JSON event", http.StatusBadRequest)
	case badCategory:
		http.Error(w, "event has no known category", http.StatusBadRequest)
	case busy:
		http.Error(w, "too many sounds playing", http.StatusServiceUnavailable)
	case stale:
		w.WriteHeader(http.StatusAccepted)
		_, _ = io.WriteString(w, "dropped: stale\n")
	case accepted:
		w.WriteHeader(http.StatusAccepted)
		_, _ = io.WriteString(w, "accepted\n")
	}
}

// outcome is what became of one submitted event.
type outcome int

const (
	accepted outcome = iota
	stale
	notAnEvent
	badCategory
	busy
)

// submit decodes one event and, unless it is refused or stale, starts its
// sound. from names where it came from, for the log. The data itself is
// never logged: whatever sent it may have sent the wrong thing.
func (s *Server) submit(data []byte, from string) outcome {
	var event sounds.Event
	if err := json.Unmarshal(data, &event); err != nil {
		slog.Warn("event refused: not an event", "from", from, "bytes", len(data))
		return notAnEvent
	}
	if _, err := event.Context(); err != nil {
		slog.Warn("event refused: bad category", "from", from, "source", event.Source, "bytes", len(data))
		return badCategory
	}

	if age := s.opts.Now().Sub(event.Time); s.opts.MaxAge > 0 && !event.Time.IsZero() && age > s.opts.MaxAge {
		slog.Info("event dropped: stale", "source", event.Source, "id", event.ID, "age", age, "max_age", s.opts.MaxAge)
		return stale
	}

	select {
	case s.slots <- struct{}{}:
	default:
		slog.Warn("event refused: too many sounds playing", "source", event.Source, "id", event.ID, "limit", maxConcurrentPlays)
		return busy
	}
	slog.Info("event accepted", "source", event.Source, "id", event.ID,
		"category", event.Category, "hint", event.Hint, "from", from)
	s.plays.Go(func() {
		defer func() { <-s.slots }()
		s.play(event)
	})
	return accepted
}
