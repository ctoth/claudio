package hooks

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCategories_ListsEveryCategoryIncludingSilent(t *testing.T) {
	t.Parallel()
	want := []EventCategory{Loading, Success, Error, Interactive, Completion, System, Silent}
	got := Categories()
	if len(got) != len(want) {
		t.Fatalf("Categories() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Categories()[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestEventCategory_JSONRoundTripsAsString(t *testing.T) {
	t.Parallel()
	for _, c := range Categories() {
		data, err := json.Marshal(c)
		if err != nil {
			t.Fatalf("marshal %v: %v", c, err)
		}
		if string(data) != `"`+c.String()+`"` {
			t.Errorf("marshal %v = %s, want quoted name", c, data)
		}
		var back EventCategory
		if err := json.Unmarshal(data, &back); err != nil {
			t.Fatalf("unmarshal %s: %v", data, err)
		}
		if back != c {
			t.Errorf("round trip %v -> %s -> %v", c, data, back)
		}
	}
}

// Rows written before categories were strings hold the iota int.
func TestEventCategory_UnmarshalsLegacyInt(t *testing.T) {
	t.Parallel()
	var ctx EventContext
	if err := json.Unmarshal([]byte(`{"Category":6,"ToolName":"Bash"}`), &ctx); err != nil {
		t.Fatalf("unmarshal legacy: %v", err)
	}
	if ctx.Category != Silent || ctx.ToolName != "Bash" {
		t.Errorf("legacy decode = %+v", ctx)
	}
}

func TestEventCategory_RejectsUnknown(t *testing.T) {
	t.Parallel()
	var c EventCategory
	for _, in := range []string{`"nope"`, `99`, `-1`, `true`} {
		if err := json.Unmarshal([]byte(in), &c); err == nil {
			t.Errorf("unmarshal %s: want error", in)
		}
	}
	if _, err := json.Marshal(EventCategory(99)); err == nil {
		t.Error("marshal unknown category: want error")
	}
	if _, err := ParseEventCategory("nope"); err == nil || !strings.Contains(err.Error(), "loading") {
		t.Errorf("ParseEventCategory(nope) err = %v, want error listing valid names", err)
	}
}

func TestParseEventCategory_AcceptsEveryName(t *testing.T) {
	t.Parallel()
	for _, c := range Categories() {
		got, err := ParseEventCategory(c.String())
		if err != nil || got != c {
			t.Errorf("ParseEventCategory(%q) = %v, %v", c.String(), got, err)
		}
	}
}

// The recorder stores EventContext as JSON; the key names are part of the
// on-disk format and the category value is its stable name.
func TestEventContext_JSONKeysAreStable(t *testing.T) {
	t.Parallel()
	data, err := json.Marshal(&EventContext{Category: Success, ToolName: "Bash"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"Category":"success"`, `"ToolName":"Bash"`, `"OriginalTool"`, `"IsSuccess"`, `"HasError"`, `"SoundHint"`, `"FileType"`, `"Operation"`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("EventContext JSON %s missing %s", data, want)
		}
	}
}
