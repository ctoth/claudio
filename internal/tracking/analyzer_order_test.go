package tracking

import (
	"slices"
	"testing"
	"time"
)

// The tools that requested one missing sound come back sorted, not in Go
// map order, so repeated runs print the same list.
func TestGetMissingSounds_ToolsAreSorted(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	defer db.Close()

	for i, tool := range []string{"Zed", "Alpha", "Mid", "Beta"} {
		result, execErr := db.Exec(`INSERT INTO hook_events
			(timestamp, session_id, tool_name, selected_path, chain_type, context)
			VALUES (?, 'session', ?, 'x.wav', 'simple', '{"Category":1,"ToolName":"Edit"}')`,
			time.Now().Unix(), tool)
		if execErr != nil {
			t.Fatalf("insert event: %v", execErr)
		}
		eventID, _ := result.LastInsertId()
		if _, execErr = db.Exec(`INSERT INTO path_lookups (event_id, path, sequence, found) VALUES (?, 'x.wav', ?, 0)`, eventID, i+1); execErr != nil {
			t.Fatalf("insert lookup: %v", execErr)
		}
	}

	want := []string{"Alpha", "Beta", "Mid", "Zed"}
	for range 20 {
		got, err := GetMissingSounds(db, QueryFilter{})
		if err != nil {
			t.Fatalf("GetMissingSounds: %v", err)
		}
		if len(got) != 1 || !slices.Equal(got[0].Tools, want) {
			t.Fatalf("Tools = %+v, want one row with %v", got, want)
		}
	}
}

// Equal counts are ordered by the grouping key, so a --limit cut and the
// printed order do not depend on SQLite's scan order.
func TestAnalyzerQueriesBreakTiesByKey(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	defer db.Close()

	// Inserted in reverse so insertion order cannot pass for key order.
	for i, path := range []string{"d.wav", "c.wav", "b.wav", "a.wav"} {
		result, execErr := db.Exec(`INSERT INTO hook_events
			(timestamp, session_id, tool_name, selected_path, chain_type, context)
			VALUES (?, 'session', 'Bash', ?, 'simple', '{"Category":1,"ToolName":"Bash"}')`,
			time.Now().Unix(), path)
		if execErr != nil {
			t.Fatalf("insert event: %v", execErr)
		}
		eventID, _ := result.LastInsertId()
		if _, execErr = db.Exec(`INSERT INTO path_lookups (event_id, path, sequence, found) VALUES (?, ?, ?, 0)`, eventID, path, i+1); execErr != nil {
			t.Fatalf("insert lookup: %v", execErr)
		}
	}

	missing, err := GetMissingSounds(db, QueryFilter{Limit: 2})
	if err != nil {
		t.Fatalf("GetMissingSounds: %v", err)
	}
	var missingPaths []string
	for _, m := range missing {
		missingPaths = append(missingPaths, m.Path)
	}
	if !slices.Equal(missingPaths, []string{"a.wav", "b.wav"}) {
		t.Errorf("missing paths = %v, want [a.wav b.wav]", missingPaths)
	}

	usage, err := GetSoundUsage(db, QueryFilter{Limit: 2})
	if err != nil {
		t.Fatalf("GetSoundUsage: %v", err)
	}
	var usagePaths []string
	for _, u := range usage {
		usagePaths = append(usagePaths, u.Path)
	}
	if !slices.Equal(usagePaths, []string{"a.wav", "b.wav"}) {
		t.Errorf("usage paths = %v, want [a.wav b.wav]", usagePaths)
	}
}
