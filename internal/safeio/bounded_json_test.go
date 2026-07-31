package safeio

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

// heldOpenReader delivers its payload, then blocks forever — the Windows
// git-bash hook pipe that never sends EOF.
type heldOpenReader struct {
	payload []byte
	served  bool
}

func (h *heldOpenReader) Read(p []byte) (int, error) {
	if !h.served {
		h.served = true
		return copy(p, h.payload), nil
	}
	select {} // never EOF, never data
}

func TestReadJSONBounded_CompletePayloadOnHeldOpenPipe(t *testing.T) {
	payload := `{"hook_event_name":"Stop","session_id":"abc"}`
	r := &heldOpenReader{payload: []byte(payload)}

	start := time.Now()
	got, err := ReadJSONBounded(r, MaxHookPayloadBytes, 5*time.Second, "hook payload")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != payload {
		t.Errorf("got %q, want %q", got, payload)
	}
	// The whole point: a complete payload must not wait for EOF or deadline.
	if elapsed > time.Second {
		t.Errorf("took %v; complete JSON should return immediately", elapsed)
	}
}

func TestReadJSONBounded_EOFWithoutJSON(t *testing.T) {
	got, err := ReadJSONBounded(strings.NewReader("not json"), 1024, 5*time.Second, "hook payload")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != "not json" {
		t.Errorf("got %q, want the raw bytes back on EOF", got)
	}
}

func TestReadJSONBounded_EmptyEOF(t *testing.T) {
	got, err := ReadJSONBounded(strings.NewReader(""), 1024, 5*time.Second, "hook payload")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %q, want empty", got)
	}
}

func TestReadJSONBounded_DeadlineOnSilentPipe(t *testing.T) {
	r := &heldOpenReader{payload: nil, served: true} // nothing ever arrives

	start := time.Now()
	got, err := ReadJSONBounded(r, 1024, 200*time.Millisecond, "hook payload")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %q, want empty on deadline", got)
	}
	if elapsed > 2*time.Second {
		t.Errorf("deadline did not bound the read: %v", elapsed)
	}
}

func TestReadJSONBounded_DeadlineOnIncompleteJSON(t *testing.T) {
	r := &heldOpenReader{payload: []byte(`{"hook_event_name":"Sto`)}

	got, err := ReadJSONBounded(r, 1024, 200*time.Millisecond, "hook payload")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != `{"hook_event_name":"Sto` {
		t.Errorf("got %q, want the partial bytes on deadline", got)
	}
}

func TestReadJSONBounded_CapExceeded(t *testing.T) {
	big := bytes.Repeat([]byte("x"), 100) // not JSON, so the cap check governs
	_, err := ReadJSONBounded(bytes.NewReader(big), 99, 5*time.Second, "hook payload")
	if err == nil {
		t.Fatal("want error when input exceeds cap")
	}
	if !strings.Contains(err.Error(), "99 byte limit") {
		t.Errorf("error should name the limit: %v", err)
	}
}

func TestReadJSONBounded_WhitespaceAroundJSON(t *testing.T) {
	payload := "  {\"a\":1}\n"
	r := &heldOpenReader{payload: []byte(payload)}
	got, err := ReadJSONBounded(r, 1024, 5*time.Second, "hook payload")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != payload {
		t.Errorf("got %q, want %q", got, payload)
	}
}
