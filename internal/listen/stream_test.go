package listen

import (
	"errors"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"claudio.click/internal/sounds"
)

// readAll feeds input to a fresh Server as a stream and returns the hints
// of what was played, in order of arrival.
func readAll(t *testing.T, opts Options, input string) []string {
	t.Helper()
	if opts.Now == nil {
		opts.Now = func() time.Time { return testNow }
	}
	rec := &recorder{}
	server := New(opts, rec.play)
	if err := server.ReadEvents(strings.NewReader(input)); err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}
	server.Wait()
	hints := map[string]bool{}
	for _, event := range rec.played() {
		hints[event.Hint] = true
	}
	var got []string
	for _, hint := range []string{"a", "b", "c", "d"} {
		if hints[hint] {
			got = append(got, hint)
		}
	}
	if len(rec.played()) != len(got) {
		t.Fatalf("played %+v", rec.played())
	}
	return got
}

func line(hint string) string {
	return `{"category":"success","hint":"` + hint + `"}`
}

// A stream is one event per line. Whatever carries it (a pipe from curl,
// ssh, a relay's client) may add blank keep-alive lines and CRLF endings.
func TestReadEventsPlaysOneEventPerLine(t *testing.T) {
	t.Parallel()
	input := line("a") + "\n\n" + line("b") + "\r\n   \n" + line("c") // no final newline
	if got := readAll(t, Options{}, input); strings.Join(got, "") != "abc" {
		t.Errorf("played %q, want a, b and c", got)
	}
}

// One bad line must not end the stream: a relay can carry anything.
func TestReadEventsSkipsLinesThatAreNotEvents(t *testing.T) {
	t.Parallel()
	input := strings.Join([]string{
		line("a"),
		`not json at all`,
		`{"category":"celebration","hint":"d"}`,
		`{"category":"success","hint":"d","pad":"` + strings.Repeat("x", maxEventBytes) + `"}`,
		line("b"),
	}, "\n") + "\n"
	if got := readAll(t, Options{}, input); strings.Join(got, "") != "ab" {
		t.Errorf("played %q, want a and b", got)
	}
}

func TestReadEventsDropsStaleEvents(t *testing.T) {
	t.Parallel()
	old := `{"category":"success","hint":"d","time":"` + testNow.Add(-time.Hour).Format(time.RFC3339) + `"}`
	if got := readAll(t, Options{MaxAge: 30 * time.Second}, old+"\n"+line("a")+"\n"); strings.Join(got, "") != "a" {
		t.Errorf("played %q, want only a", got)
	}
}

// A broken pipe is reported, so the command can exit non-zero and whatever
// supervises it can reconnect.
func TestReadEventsReportsAReadError(t *testing.T) {
	t.Parallel()
	broken := errors.New("pipe broke")
	server := New(Options{}, func(sounds.Event) {})
	err := server.ReadEvents(iotest.ErrReader(broken))
	server.Wait()
	if !errors.Is(err, broken) {
		t.Errorf("err = %v, want the read error", err)
	}
}
