package listen

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"claudio.click/internal/sounds"
)

func claudeCodeLogs(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/claude_code_logs.json")
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// eventJSON renders an event without its time and attributes, for comparing.
func eventJSON(t *testing.T, event sounds.Event) string {
	t.Helper()
	event.Time = time.Time{}
	event.Attributes = nil
	data, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// What Claude Code really exports becomes the events its hooks would have
// produced, so a soundpack made for hooks plays the same sounds. Records
// with no hook counterpart (hook_registered, api_request) are silent: one
// session start sends dozens of them.
func TestClaudeCodeLogsBecomeHookShapedEvents(t *testing.T) {
	t.Parallel()
	events, err := eventsFromOTLPLogs(claudeCodeLogs(t))
	if err != nil {
		t.Fatal(err)
	}
	const session = `"session":"1292c713-0a69-444c-bb55-1ad10f2a4c4c"`
	want := []string{
		`{"source":"claude",` + session + `,"category":"interactive","hint":"message-sent","operation":"prompt"}`,
		`{"source":"claude",` + session + `,"category":"loading","hint":"bash-start","command":"Bash","phase":"start","operation":"tool-start"}`,
		`{"source":"claude",` + session + `,"category":"success","hint":"bash-success","command":"Bash","phase":"success","operation":"tool-complete"}`,
		`{"source":"claude",` + session + `,"category":"error","hint":"bash-error","command":"Bash","phase":"error","operation":"tool-complete"}`,
	}
	if len(events) != len(want) {
		t.Fatalf("got %d events, want %d: %+v", len(events), len(want), events)
	}
	for i, event := range events {
		if got := eventJSON(t, event); got != want[i] {
			t.Errorf("event %d\n got: %s\nwant: %s", i, got, want[i])
		}
	}

	// The record's own time and the tool's duration ride along.
	prompt, failed := events[0], events[3]
	if at := time.Date(2026, 10, 10, 22, 8, 44, 315000000, time.UTC); !prompt.Time.Equal(at) {
		t.Errorf("prompt time = %v, want %v", prompt.Time, at)
	}
	if got := failed.Attributes["duration_ms"]; got != float64(2358) {
		t.Errorf("duration_ms = %#v, want 2358", got)
	}
	if got := failed.Attributes["error_type"]; got != "ShellError" {
		t.Errorf("error_type = %#v, want ShellError", got)
	}
}

// otlpLogs builds a one-record export from some other service.
func otlpLogs(service, scope, record string) []byte {
	return []byte(`{"resourceLogs":[{"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"` + service + `"}}]},` +
		`"scopeLogs":[{"scope":{"name":"` + scope + `"},"logRecords":[` + record + `]}]}]}`)
}

// Anything else that speaks OpenTelemetry is heard too: the event's name is
// the sound hint, and its success or severity picks the category.
func TestOtherServicesLogsMapByNameAndOutcome(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, record, want string
	}{
		{"named by eventName, succeeded",
			`{"eventName":"deploy.finished","attributes":[{"key":"success","value":{"boolValue":true}}]}`,
			`{"source":"shipit","category":"success","hint":"deploy.finished"}`},
		{"named by attribute, failed",
			`{"attributes":[{"key":"event.name","value":{"stringValue":"deploy.finished"}},{"key":"success","value":{"stringValue":"false"}}]}`,
			`{"source":"shipit","category":"error","hint":"deploy.finished"}`},
		{"named by body, error severity",
			`{"severityNumber":17,"body":{"stringValue":"payment.declined"}}`,
			`{"source":"shipit","category":"error","hint":"payment.declined"}`},
		{"no outcome",
			`{"severityNumber":9,"body":{"stringValue":"cache.warmed"}}`,
			`{"source":"shipit","category":"system","hint":"cache.warmed"}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			events, err := eventsFromOTLPLogs(otlpLogs("shipit", "com.example.shipit", tc.record))
			if err != nil {
				t.Fatal(err)
			}
			if len(events) != 1 {
				t.Fatalf("got %d events: %+v", len(events), events)
			}
			if got := eventJSON(t, events[0]); got != tc.want {
				t.Errorf("\n got: %s\nwant: %s", got, tc.want)
			}
		})
	}
}

// A record with nothing to name a sound by is skipped, not played as a
// default beep: free-text application logs are not events.
func TestLogRecordsWithoutANameAreSkipped(t *testing.T) {
	t.Parallel()
	for _, record := range []string{
		`{"severityNumber":9}`,
		`{"body":{"stringValue":"connection from 10.0.0.1 closed after 3 retries"}}`,
		`{"body":{"kvlistValue":{"values":[]}}}`,
	} {
		events, err := eventsFromOTLPLogs(otlpLogs("shipit", "com.example.shipit", record))
		if err != nil {
			t.Fatalf("%s: %v", record, err)
		}
		if len(events) != 0 {
			t.Errorf("%s: got %+v, want nothing", record, events)
		}
	}
}

func TestBrokenOTLPIsAnError(t *testing.T) {
	t.Parallel()
	for _, data := range []string{``, `not json`, `[]`, `{"resourceLogs":"nope"}`} {
		if _, err := eventsFromOTLPLogs([]byte(data)); err == nil {
			t.Errorf("%q: no error", data)
		}
	}
}

func logsRequest(body []byte) *http.Request {
	request := httptest.NewRequest(http.MethodPost, "/v1/logs", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	return request
}

// The listener is an OTLP/HTTP logs receiver at the standard path: an
// exporter gets the success answer the protocol expects and each record
// that maps to an event is played.
func TestListenerReceivesOTLPLogs(t *testing.T) {
	t.Parallel()
	response, body, played := post(t, Options{}, logsRequest(claudeCodeLogs(t)))
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body %q", response.StatusCode, body)
	}
	if got := response.Header.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q", got)
	}
	if strings.TrimSpace(body) != "{}" {
		t.Errorf("body = %q, want {}", body)
	}
	if len(played) != 4 {
		t.Errorf("played %d events, want 4: %+v", len(played), played)
	}
}

func TestOTLPLogsCanArriveGzipped(t *testing.T) {
	t.Parallel()
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	_, _ = writer.Write(claudeCodeLogs(t))
	_ = writer.Close()
	request := logsRequest(compressed.Bytes())
	request.Header.Set("Content-Encoding", "gzip")
	response, body, played := post(t, Options{}, request)
	if response.StatusCode != http.StatusOK || len(played) != 4 {
		t.Errorf("status %d with %d plays, body %q", response.StatusCode, len(played), body)
	}
}

func TestOTLPLogsRefusals(t *testing.T) {
	t.Parallel()
	protobuf := logsRequest([]byte{0x0a, 0x00})
	protobuf.Header.Set("Content-Type", "application/x-protobuf")
	withToken := logsRequest(claudeCodeLogs(t))
	withToken.Header.Set("Authorization", "Bearer s3cret")

	tests := []struct {
		name    string
		opts    Options
		request *http.Request
		status  int
		plays   int
	}{
		{"token missing", Options{Token: "s3cret"}, logsRequest(claudeCodeLogs(t)), http.StatusUnauthorized, 0},
		{"token given", Options{Token: "s3cret"}, withToken, http.StatusOK, 4},
		{"protobuf", Options{}, protobuf, http.StatusUnsupportedMediaType, 0},
		{"not OTLP", Options{}, logsRequest([]byte(`not json`)), http.StatusBadRequest, 0},
		{"too large", Options{}, logsRequest(bytes.Repeat([]byte(" "), maxOTLPBytes+1)), http.StatusRequestEntityTooLarge, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			response, body, played := post(t, tc.opts, tc.request)
			if response.StatusCode != tc.status || len(played) != tc.plays {
				t.Errorf("status %d with %d plays, want %d with %d; body %q", response.StatusCode, len(played), tc.status, tc.plays, body)
			}
		})
	}
}

// A batch that sat in an exporter's queue is as stale as any other event.
func TestStaleOTLPRecordsAreDropped(t *testing.T) {
	t.Parallel()
	old := testNow.Add(-time.Hour).UnixNano()
	record := `{"timeUnixNano":"` + itoa(old) + `","eventName":"deploy.finished"}`
	response, _, played := post(t, Options{MaxAge: 30 * time.Second}, logsRequest(otlpLogs("shipit", "com.example.shipit", record)))
	if response.StatusCode != http.StatusOK || len(played) != 0 {
		t.Errorf("status %d with %d plays, want 200 with none", response.StatusCode, len(played))
	}
}

func itoa(n int64) string {
	data, _ := json.Marshal(n)
	return string(data)
}
