package tracking

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"claudio.click/internal/hooks"
)

// seedMixedCategoryRows writes one pre-migration row (int category,
// untagged-JSON context, exactly as older claudio versions stored it) and
// one row through the current recorder. Both are success events for Bash
// that missed default.wav.
func seedMixedCategoryRows(t *testing.T) *sql.DB {
	t.Helper()
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	legacy := `{"Category":1,"ToolName":"Bash","OriginalTool":"","IsSuccess":true,"HasError":false,"SoundHint":"","FileType":"","Operation":"tool-complete"}`
	res, err := db.Exec(`INSERT INTO hook_events
		(timestamp, session_id, tool_name, selected_path, chain_type, context)
		VALUES (?, 'old', 'Bash', 'default.wav', 'posttool', ?)`, time.Now().Unix()-60, legacy)
	if err != nil {
		t.Fatalf("insert legacy row: %v", err)
	}
	id, _ := res.LastInsertId()
	if _, err := db.Exec(`INSERT INTO path_lookups (event_id, path, sequence, found) VALUES (?, 'success/bash.wav', 1, 0)`, id); err != nil {
		t.Fatalf("insert legacy lookup: %v", err)
	}

	rec := NewDBHook(db, "new")
	ctx := &hooks.EventContext{Category: hooks.Success, ToolName: "Bash", IsSuccess: true, Operation: "tool-complete"}
	if err := rec.RecordEvent(context.Background(), ctx, "posttool",
		[]Lookup{{Path: "success/bash.wav", Found: false, Sequence: 1}}, "default.wav"); err != nil {
		t.Fatalf("RecordEvent: %v", err)
	}

	var stored string
	if err := db.QueryRow(`SELECT JSON_EXTRACT(context, '$.Category') FROM hook_events WHERE session_id = 'new'`).Scan(&stored); err != nil {
		t.Fatalf("read stored category: %v", err)
	}
	if stored != "success" {
		t.Fatalf("new rows store category %q, want the stable name \"success\"", stored)
	}
	return db
}

func TestAnalyze_OldAndNewCategoryRowsAgree(t *testing.T) {
	db := seedMixedCategoryRows(t)
	filter := QueryFilter{Category: "success"}

	usage, err := GetSoundUsage(db, filter)
	if err != nil {
		t.Fatalf("GetSoundUsage: %v", err)
	}
	if len(usage) != 1 || usage[0].PlayCount != 2 || usage[0].Category != "success" {
		t.Errorf("GetSoundUsage = %+v, want one success row with 2 plays", usage)
	}

	missing, err := GetMissingSounds(db, filter)
	if err != nil {
		t.Fatalf("GetMissingSounds: %v", err)
	}
	if len(missing) != 1 || missing[0].RequestCount != 2 || missing[0].Category != "success" {
		t.Errorf("GetMissingSounds = %+v, want one success row with 2 requests", missing)
	}

	dist, err := GetCategoryDistribution(db, QueryFilter{})
	if err != nil {
		t.Fatalf("GetCategoryDistribution: %v", err)
	}
	if len(dist) != 1 || dist[0].Category != "success" || dist[0].Count != 2 {
		t.Errorf("GetCategoryDistribution = %+v, want success x2 in one group", dist)
	}

	tools, err := GetToolUsageStats(db, QueryFilter{})
	if err != nil {
		t.Fatalf("GetToolUsageStats: %v", err)
	}
	if len(tools) != 1 || len(tools[0].Categories) != 1 || tools[0].Categories[0] != "success" {
		t.Errorf("GetToolUsageStats = %+v, want Bash with categories [success]", tools)
	}

	loading, err := GetSoundUsage(db, QueryFilter{Category: "loading"})
	if err != nil {
		t.Fatalf("GetSoundUsage(loading): %v", err)
	}
	if len(loading) != 0 {
		t.Errorf("loading filter matched %+v", loading)
	}
}

func TestAnalyze_SilentCategoryReadsBack(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, ctxJSON := range []string{`{"Category":6,"ToolName":"X"}`, `{"Category":"silent","ToolName":"X"}`} {
		if _, err := db.Exec(`INSERT INTO hook_events (timestamp, session_id, tool_name, selected_path, chain_type, context)
			VALUES (?, 's', 'X', 'a.wav', 'simple', ?)`, time.Now().Unix(), ctxJSON); err != nil {
			t.Fatal(err)
		}
	}
	dist, err := GetCategoryDistribution(db, QueryFilter{Category: "silent"})
	if err != nil {
		t.Fatalf("GetCategoryDistribution: %v", err)
	}
	if len(dist) != 1 || dist[0].Category != "silent" || dist[0].Count != 2 {
		t.Errorf("distribution = %+v, want silent x2", dist)
	}
}

func TestBuildWhereClause_UnknownCategoryErrors(t *testing.T) {
	if _, _, err := (&QueryFilter{Category: "succes"}).BuildWhereClause(); err == nil {
		t.Fatal("unknown category: want error, got nil")
	}
	db := seedMixedCategoryRows(t)
	if _, err := GetSoundUsage(db, QueryFilter{Category: "succes"}); err == nil || !strings.Contains(err.Error(), "succes") {
		t.Fatalf("GetSoundUsage with unknown category: err = %v, want unknown-category error", err)
	}
}
