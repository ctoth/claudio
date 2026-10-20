package listen

import (
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"claudio.click/internal/sounds"
)

// maxOTLPBytes caps one export, compressed and decompressed. An exporter
// batches: Claude Code's first export of a session is about 80 KB.
const maxOTLPBytes = 4 << 20

// claudeCodeScope is the instrumentation scope of Claude Code's events.
const claudeCodeScope = "com.anthropic.claude_code.events"

// otlpExport is the part of an OTLP/JSON ExportLogsServiceRequest that
// names a sound. Unknown fields are ignored.
type otlpExport struct {
	ResourceLogs []struct {
		Resource struct {
			Attributes []otlpKeyValue `json:"attributes"`
		} `json:"resource"`
		ScopeLogs []struct {
			Scope struct {
				Name string `json:"name"`
			} `json:"scope"`
			LogRecords []otlpRecord `json:"logRecords"`
		} `json:"scopeLogs"`
	} `json:"resourceLogs"`
}

type otlpRecord struct {
	TimeUnixNano         otlpInt        `json:"timeUnixNano"`
	ObservedTimeUnixNano otlpInt        `json:"observedTimeUnixNano"`
	SeverityNumber       otlpInt        `json:"severityNumber"`
	EventName            string         `json:"eventName"`
	Body                 otlpValue      `json:"body"`
	Attributes           []otlpKeyValue `json:"attributes"`
}

type otlpKeyValue struct {
	Key   string    `json:"key"`
	Value otlpValue `json:"value"`
}

// otlpValue is an AnyValue; only the scalar kinds can name a sound.
type otlpValue struct {
	StringValue *string  `json:"stringValue"`
	BoolValue   *bool    `json:"boolValue"`
	IntValue    *otlpInt `json:"intValue"`
	DoubleValue *float64 `json:"doubleValue"`
}

// text is the value as text, or "" for a kind that has none.
func (v otlpValue) text() string {
	switch {
	case v.StringValue != nil:
		return *v.StringValue
	case v.BoolValue != nil:
		return strconv.FormatBool(*v.BoolValue)
	case v.IntValue != nil:
		return strconv.FormatInt(int64(*v.IntValue), 10)
	case v.DoubleValue != nil:
		return strconv.FormatFloat(*v.DoubleValue, 'g', -1, 64)
	}
	return ""
}

// otlpInt is a 64-bit integer, which OTLP/JSON writes as a number or as a
// string of digits. Anything else (an enum name) reads as zero.
type otlpInt int64

func (n *otlpInt) UnmarshalJSON(data []byte) error {
	parsed, err := strconv.ParseInt(strings.Trim(string(data), `"`), 10, 64)
	if err != nil {
		parsed = 0
	}
	*n = otlpInt(parsed)
	return nil
}

// attributeMap flattens key-values to text; a later duplicate key wins.
func attributeMap(pairs []otlpKeyValue) map[string]string {
	attrs := make(map[string]string, len(pairs))
	for _, pair := range pairs {
		attrs[pair.Key] = pair.Value.text()
	}
	return attrs
}

// eventsFromOTLPLogs turns an OTLP/JSON logs export into the events it
// holds, in order. A record that names no event is left out.
func eventsFromOTLPLogs(data []byte) ([]sounds.Event, error) {
	var export otlpExport
	if err := json.Unmarshal(data, &export); err != nil {
		return nil, fmt.Errorf("not an OTLP/JSON logs export: %w", err)
	}
	var events []sounds.Event
	for _, resource := range export.ResourceLogs {
		service := attributeMap(resource.Resource.Attributes)["service.name"]
		for _, scope := range resource.ScopeLogs {
			for _, record := range scope.LogRecords {
				attrs := attributeMap(record.Attributes)
				var event sounds.Event
				var ok bool
				if scope.Scope.Name == claudeCodeScope {
					event, ok = claudeCodeEvent(record, attrs)
				} else {
					event, ok = genericEvent(record, attrs, service)
				}
				if !ok {
					continue
				}
				event.Time = recordTime(record)
				event.Session = attrs["session.id"]
				event.Attributes = magnitudes(attrs)
				events = append(events, event)
			}
		}
	}
	return events, nil
}

func recordTime(record otlpRecord) time.Time {
	nanos := int64(record.TimeUnixNano)
	if nanos == 0 {
		nanos = int64(record.ObservedTimeUnixNano)
	}
	if nanos == 0 {
		return time.Time{}
	}
	return time.Unix(0, nanos).UTC()
}

// magnitudes carries the attributes a sound could one day be shaped by.
func magnitudes(attrs map[string]string) map[string]any {
	out := map[string]any{}
	if duration, err := strconv.ParseFloat(attrs["duration_ms"], 64); err == nil {
		out["duration_ms"] = duration
	}
	if errorType := attrs["error_type"]; errorType != "" {
		out["error_type"] = errorType
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// claudeCodeEvent maps a Claude Code telemetry record onto the event its
// hook counterpart produces, so one soundpack serves both. Records with no
// hook counterpart are silent: a session start alone sends dozens of
// hook_registered and plugin_loaded records.
//
// Telemetry says less than a hook does. Without OTEL_LOG_TOOL_DETAILS a
// Bash call does not name its command, and nothing reports a turn ending
// or the agent waiting for permission.
func claudeCodeEvent(record otlpRecord, attrs map[string]string) (sounds.Event, bool) {
	name := attrs["event.name"]
	if name == "" {
		name = strings.TrimPrefix(record.Body.text(), "claude_code.")
	}
	event := sounds.Event{Source: "claude"}
	switch name {
	case "user_prompt":
		event.Category, event.Hint, event.Operation = "interactive", "message-sent", "prompt"
	case "tool_decision":
		if attrs["decision"] != "accept" {
			event.Category, event.Hint, event.Operation = "error", "permission-denied", "permission-denied"
			break
		}
		event.Category, event.Operation = "loading", "tool-start"
		setTool(&event, attrs["tool_name"], "start")
	case "tool_result":
		event.Category, event.Operation = "success", "tool-complete"
		phase := "success"
		if attrs["success"] != "true" {
			event.Category, phase = "error", "error"
		}
		setTool(&event, attrs["tool_name"], phase)
	case "compaction":
		event.Category, event.Hint, event.Operation = "system", "post-compact", "post-compact"
	case "subagent_completed":
		event.Category, event.Hint, event.Operation = "completion", "subagent-complete", "subagent-stop"
	case "api_retries_exhausted":
		event.Category, event.Hint, event.Operation = "error", "stop-failure", "stop-failure"
	default:
		return sounds.Event{}, false
	}
	return event, true
}

// setTool names the tool an event is about the way the hook parser does:
// every MCP tool is "mcp", with its full name kept as the original tool.
func setTool(event *sounds.Event, tool, phase string) {
	if tool == "" {
		event.Hint = "tool-" + map[string]string{"start": "loading", "success": "success", "error": "error"}[phase]
		return
	}
	if strings.HasPrefix(tool, "mcp__") {
		event.OriginalTool, tool = tool, "mcp"
	}
	event.Command, event.Phase = tool, phase
	event.Hint = strings.ToLower(tool) + "-" + phase
}

// eventNamePattern is what a body must look like to count as an event name
// and not a line of free text.
var eventNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/-]{0,99}$`)

// genericEvent maps a record from any other service: its name is the sound
// hint, and its outcome picks the category. The name is the record's
// eventName, else its "event.name" attribute, else a body that is a single
// identifier. A record with none of those is a log line, not an event.
func genericEvent(record otlpRecord, attrs map[string]string, service string) (sounds.Event, bool) {
	name := record.EventName
	if name == "" {
		name = attrs["event.name"]
	}
	if name == "" {
		if body := record.Body.text(); eventNamePattern.MatchString(body) {
			name = body
		}
	}
	if name == "" {
		return sounds.Event{}, false
	}

	category := "system"
	switch {
	case attrs["success"] == "false", record.SeverityNumber >= 17: // 17 is ERROR
		category = "error"
	case attrs["success"] == "true":
		category = "success"
	}
	return sounds.Event{Source: service, Category: category, Hint: name}, true
}

// handleOTLPLogs is an OTLP/HTTP logs receiver for the JSON encoding. Like
// handleEvent it never logs or echoes a body: telemetry can carry prompts
// and account details.
func (s *Server) handleOTLPLogs(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		slog.Warn("telemetry refused: missing or wrong token", "remote", r.RemoteAddr)
		http.Error(w, "missing or wrong token", http.StatusUnauthorized)
		return
	}
	if mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); mediaType != "application/json" {
		slog.Warn("telemetry refused: not the JSON encoding", "remote", r.RemoteAddr, "content_type", mediaType)
		http.Error(w, "only OTLP over HTTP with JSON is accepted: set OTEL_EXPORTER_OTLP_PROTOCOL=http/json", http.StatusUnsupportedMediaType)
		return
	}

	data, err := readOTLPBody(w, r)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) || errors.Is(err, errOTLPTooLarge) {
			slog.Warn("telemetry refused: export too large", "remote", r.RemoteAddr, "limit_bytes", maxOTLPBytes)
			http.Error(w, "export too large", http.StatusRequestEntityTooLarge)
			return
		}
		slog.Warn("telemetry refused: unreadable body", "remote", r.RemoteAddr, "error", err)
		http.Error(w, "unreadable body", http.StatusBadRequest)
		return
	}
	events, err := eventsFromOTLPLogs(data)
	if err != nil {
		slog.Warn("telemetry refused: not an OTLP logs export", "remote", r.RemoteAddr, "bytes", len(data))
		http.Error(w, "body is not an OTLP/JSON logs export", http.StatusBadRequest)
		return
	}
	for _, event := range events {
		s.submitEvent(event, r.RemoteAddr)
	}

	// The whole export was received; an empty response says so. Events that
	// were stale or over the play limit are not the exporter's to retry.
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, "{}\n")
}

var errOTLPTooLarge = errors.New("decompressed export too large")

// readOTLPBody reads the export, gunzipping it when the exporter says it
// is compressed. The cap applies before and after decompression.
func readOTLPBody(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	var reader io.Reader = http.MaxBytesReader(w, r.Body, maxOTLPBytes)
	if r.Header.Get("Content-Encoding") == "gzip" {
		unzipped, err := gzip.NewReader(reader)
		if err != nil {
			return nil, err
		}
		defer unzipped.Close()
		reader = io.LimitReader(unzipped, maxOTLPBytes+1)
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	if len(data) > maxOTLPBytes {
		return nil, errOTLPTooLarge
	}
	return data, nil
}
