package sounds

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"claudio.click/internal/hooks"
)

// An event that crosses the wire must pick the same sound as the context it
// was made from. Every golden payload is parsed, turned into an Event, sent
// through JSON and mapped again.
func TestEventMapsLikeTheContextItCameFrom(t *testing.T) {
	t.Parallel()
	for _, g := range goldenEvents {
		t.Run(g.name, func(t *testing.T) {
			hookEvent, err := hooks.ParseHookEvent(g.payload(t))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			local := hookEvent.GetContext()

			data, err := json.Marshal(EventFromContext(local))
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var received Event
			if err := json.Unmarshal(data, &received); err != nil {
				t.Fatalf("unmarshal %s: %v", data, err)
			}
			remote, err := received.Context()
			if err != nil {
				t.Fatalf("context from %s: %v", data, err)
			}

			want := NewSoundMapper().MapSound(context.Background(), local)
			got := NewSoundMapper().MapSound(context.Background(), remote)
			if (want == nil) != (got == nil) {
				t.Fatalf("silence differs: local %v, remote %v", want, got)
			}
			if want != nil && (got.ChainType != want.ChainType || !slices.Equal(got.AllPaths, want.AllPaths)) {
				t.Errorf("chain differs\n got: %s %q\nwant: %s %q", got.ChainType, got.AllPaths, want.ChainType, want.AllPaths)
			}
			// The tracking database stores and queries these.
			if remote.ToolName != local.ToolName || remote.HasError != local.HasError || remote.IsSuccess != local.IsSuccess {
				t.Errorf("tracked fields differ\n got: %+v\nwant: %+v", remote, local)
			}
		})
	}
}

// The wire keys are a contract with every emitter, including a shell one-liner.
func TestEventJSONKeys(t *testing.T) {
	t.Parallel()
	event := Event{
		Source:       "claude",
		ID:           "evt-1",
		Time:         time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC),
		Session:      "s1",
		Category:     "success",
		Hint:         "git-commit-success",
		Command:      "git",
		Subcommand:   "commit",
		Phase:        "success",
		OriginalTool: "Bash",
		Operation:    "tool-complete",
		Attributes:   map[string]any{"amount": 12.5},
	}
	data, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"source":"claude","id":"evt-1","time":"2026-10-10T12:00:00Z","session":"s1",` +
		`"category":"success","hint":"git-commit-success","command":"git","subcommand":"commit",` +
		`"phase":"success","original_tool":"Bash","operation":"tool-complete","attributes":{"amount":12.5}}`
	if string(data) != want {
		t.Errorf("wire form\n got: %s\nwant: %s", data, want)
	}
}

// The smallest useful event names only a category; nothing else is sent.
func TestEventOmitsEmptyFields(t *testing.T) {
	t.Parallel()
	data, err := json.Marshal(Event{Category: "completion"})
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"category":"completion"}` {
		t.Errorf("wire form = %s", data)
	}
}

func TestEventContextRejectsAnUnknownCategory(t *testing.T) {
	t.Parallel()
	for _, category := range []string{"", "celebration", "../success"} {
		if _, err := (Event{Category: category}).Context(); err == nil {
			t.Errorf("category %q: no error", category)
		} else if !strings.Contains(err.Error(), "category") {
			t.Errorf("category %q: error %q does not name the category", category, err)
		}
	}
}

// A source that is not a coding agent sends no command: the event takes the
// simple chain, hint first.
func TestEventWithoutACommandTakesTheSimpleChain(t *testing.T) {
	t.Parallel()
	eventCtx, err := Event{Source: "github", Category: "success", Hint: "pull-request-merged", Operation: "pull-request"}.Context()
	if err != nil {
		t.Fatal(err)
	}
	got := NewSoundMapper().MapSound(context.Background(), eventCtx)
	want := []string{"success/pull-request-merged.wav", "success/pull-request.wav", "success/success.wav", "default.wav"}
	if got.ChainType != ChainTypeSimple || !slices.Equal(got.AllPaths, want) {
		t.Errorf("got %s %q, want simple %q", got.ChainType, got.AllPaths, want)
	}
}
