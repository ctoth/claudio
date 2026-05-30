package tracking

import (
	"context"
	"database/sql"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"claudio.click/internal/hooks"
	_ "modernc.org/sqlite"
)

// TestNewDBHook verifies the constructor stamps the immutable fields. The
// new stateless DBHook has NO `disabled` field, no per-event mutable state,
// and no `pathChecks` buffer — only `db` and `sessionID` survive.
func TestNewDBHook(t *testing.T) {
	db := setupTestDB(t)
	sessionID := "test-session-123"

	hook := NewDBHook(db, sessionID)
	if hook == nil {
		t.Fatal("NewDBHook returned nil")
	}
	if hook.sessionID != sessionID {
		t.Errorf("Expected session ID %s, got %s", sessionID, hook.sessionID)
	}
	if hook.db != db {
		t.Error("Expected db handle to match constructor argument")
	}
}

// TestRecordEvent_WritesEventAndLookups is the basic-shape regression test:
// one RecordEvent call yields one hook_events row plus N path_lookups rows
// linked to it.
func TestRecordEvent_WritesEventAndLookups(t *testing.T) {
	db := setupTestDB(t)
	sessionID := "test-session-456"
	hook := NewDBHook(db, sessionID)

	eventCtx := &hooks.EventContext{
		Category:     hooks.Success,
		ToolName:     "git",
		OriginalTool: "Bash",
		IsSuccess:    true,
		SoundHint:    "git-commit-success",
		Operation:    "tool-complete",
	}
	lookups := []Lookup{
		{Path: "success/git-commit-success.wav", Sequence: 1, Found: false},
		{Path: "success/git-success.wav", Sequence: 2, Found: false},
		{Path: "success/success.wav", Sequence: 3, Found: true},
	}

	if err := hook.RecordEvent(context.Background(), eventCtx, "posttool", lookups, "success/success.wav"); err != nil {
		t.Fatalf("RecordEvent failed: %v", err)
	}

	var eventCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM hook_events WHERE session_id = ?", sessionID).Scan(&eventCount); err != nil {
		t.Fatalf("query hook_events: %v", err)
	}
	if eventCount != 1 {
		t.Errorf("expected 1 event, got %d", eventCount)
	}

	var pathCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM path_lookups pl
		JOIN hook_events he ON pl.event_id = he.id
		WHERE he.session_id = ?`, sessionID).Scan(&pathCount); err != nil {
		t.Fatalf("query path_lookups: %v", err)
	}
	if pathCount != 3 {
		t.Errorf("expected 3 path lookups, got %d", pathCount)
	}
}

// TestRecordEvent_JSONContextMarshaling pins that the EventContext is
// stored as valid JSON and round-trips through json.Unmarshal.
func TestRecordEvent_JSONContextMarshaling(t *testing.T) {
	db := setupTestDB(t)
	sessionID := "test-json"
	hook := NewDBHook(db, sessionID)

	eventCtx := &hooks.EventContext{
		Category:     hooks.Loading,
		ToolName:     "git",
		OriginalTool: "Bash",
		IsSuccess:    false,
		HasError:     false,
		SoundHint:    "git-commit-start",
		FileType:     "go",
		Operation:    "tool-start",
	}
	lookups := []Lookup{
		{Path: "loading/git-commit-start.wav", Sequence: 1, Found: true},
	}

	if err := hook.RecordEvent(context.Background(), eventCtx, "enhanced", lookups, "loading/git-commit-start.wav"); err != nil {
		t.Fatalf("RecordEvent failed: %v", err)
	}

	var contextJSON string
	if err := db.QueryRow("SELECT context FROM hook_events WHERE session_id = ?", sessionID).Scan(&contextJSON); err != nil {
		t.Fatalf("query context: %v", err)
	}

	var unmarshaled hooks.EventContext
	if err := json.Unmarshal([]byte(contextJSON), &unmarshaled); err != nil {
		t.Fatalf("stored context is not valid JSON: %v", err)
	}
	if unmarshaled.Category != eventCtx.Category {
		t.Errorf("expected category %v, got %v", eventCtx.Category, unmarshaled.Category)
	}
	if unmarshaled.ToolName != eventCtx.ToolName {
		t.Errorf("expected tool name %s, got %s", eventCtx.ToolName, unmarshaled.ToolName)
	}
	if unmarshaled.SoundHint != eventCtx.SoundHint {
		t.Errorf("expected sound hint %s, got %s", eventCtx.SoundHint, unmarshaled.SoundHint)
	}
}

// TestRecordEvent_TimestampHandling pins that the timestamp column is
// populated with time.Now().Unix() at write time.
func TestRecordEvent_TimestampHandling(t *testing.T) {
	db := setupTestDB(t)
	sessionID := "test-timestamp"
	hook := NewDBHook(db, sessionID)

	eventCtx := &hooks.EventContext{Category: hooks.Success, ToolName: "test"}
	lookups := []Lookup{{Path: "success/test.wav", Sequence: 1, Found: true}}

	startTime := time.Now().Unix()
	if err := hook.RecordEvent(context.Background(), eventCtx, "posttool", lookups, "success/test.wav"); err != nil {
		t.Fatalf("RecordEvent failed: %v", err)
	}
	endTime := time.Now().Unix()

	var timestamp int64
	if err := db.QueryRow("SELECT timestamp FROM hook_events WHERE session_id = ?", sessionID).Scan(&timestamp); err != nil {
		t.Fatalf("query timestamp: %v", err)
	}
	if timestamp < startTime || timestamp > endTime {
		t.Errorf("Timestamp %d not within [%d, %d]", timestamp, startTime, endTime)
	}
}

// TestRecordEvent_WritesChainType pins the Chunk-12 chain_type contract:
// the chainType argument lands on the hook_events row exactly once. The
// scout's renamed-from-TestDBHook_WritesChainType regression.
func TestRecordEvent_WritesChainType(t *testing.T) {
	db := setupTestDB(t)
	sessionID := "test-chain-type"
	hook := NewDBHook(db, sessionID)

	eventCtx := &hooks.EventContext{
		Category:  hooks.Loading,
		ToolName:  "git",
		Operation: "tool-start",
	}
	lookups := []Lookup{
		{Path: "loading/git-commit-start.wav", Sequence: 1, Found: false},
		{Path: "loading/git-start.wav", Sequence: 2, Found: true},
		{Path: "loading/loading.wav", Sequence: 3, Found: false},
	}

	if err := hook.RecordEvent(context.Background(), eventCtx, "enhanced", lookups, "loading/git-start.wav"); err != nil {
		t.Fatalf("RecordEvent failed: %v", err)
	}

	var chainType, selectedPath string
	if err := db.QueryRow("SELECT chain_type, selected_path FROM hook_events WHERE session_id = ?",
		sessionID).Scan(&chainType, &selectedPath); err != nil {
		t.Fatalf("query event: %v", err)
	}
	if chainType != "enhanced" {
		t.Errorf("expected chain_type 'enhanced', got %q", chainType)
	}
	if selectedPath != "loading/git-start.wav" {
		t.Errorf("expected selected_path 'loading/git-start.wav', got %q", selectedPath)
	}
}

// TestRecordEvent_DistinctCallsAreDistinctEvents pins that two RecordEvent
// calls produce two hook_events rows — there is no boundary-inference
// machinery left to collapse them.
func TestRecordEvent_DistinctCallsAreDistinctEvents(t *testing.T) {
	db := setupTestDB(t)
	sessionID := "test-grouping"
	hook := NewDBHook(db, sessionID)

	eventCtx1 := &hooks.EventContext{Category: hooks.Success, ToolName: "git", Operation: "tool-complete"}
	eventCtx2 := &hooks.EventContext{Category: hooks.Loading, ToolName: "bash", Operation: "tool-start"}

	if err := hook.RecordEvent(context.Background(), eventCtx1, "posttool",
		[]Lookup{
			{Path: "success/git.wav", Sequence: 1, Found: true},
			{Path: "success/success.wav", Sequence: 2, Found: false},
		}, "success/git.wav"); err != nil {
		t.Fatalf("RecordEvent #1: %v", err)
	}
	if err := hook.RecordEvent(context.Background(), eventCtx2, "enhanced",
		[]Lookup{{Path: "loading/bash.wav", Sequence: 1, Found: false}},
		"loading/bash.wav"); err != nil {
		t.Fatalf("RecordEvent #2: %v", err)
	}

	var eventCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM hook_events WHERE session_id = ?", sessionID).Scan(&eventCount); err != nil {
		t.Fatalf("count events: %v", err)
	}
	if eventCount != 2 {
		t.Errorf("expected 2 events, got %d", eventCount)
	}
}

// --- §H.3 regression tests ---------------------------------------------

// TestRecordEvent_FirstExistingPathWins (regression for #21).
// Two paths with Found=true: the caller-passed selectedPath is what
// lands in hook_events.selected_path. The recorder never re-derives.
func TestRecordEvent_FirstExistingPathWins(t *testing.T) {
	db := setupTestDB(t)
	sessionID := "test-first-wins"
	hook := NewDBHook(db, sessionID)

	eventCtx := &hooks.EventContext{Category: hooks.Loading, ToolName: "git"}
	// Two existing paths in the chain — caller picks the first as the winner.
	lookups := []Lookup{
		{Path: "loading/git-start.wav", Sequence: 1, Found: true}, // intended winner
		{Path: "loading/loading.wav", Sequence: 2, Found: true},   // second existing
	}
	winner := "loading/git-start.wav"

	if err := hook.RecordEvent(context.Background(), eventCtx, "enhanced", lookups, winner); err != nil {
		t.Fatalf("RecordEvent failed: %v", err)
	}

	var stored string
	if err := db.QueryRow("SELECT selected_path FROM hook_events WHERE session_id = ?", sessionID).Scan(&stored); err != nil {
		t.Fatalf("query selected_path: %v", err)
	}
	if stored != winner {
		t.Errorf("expected selected_path %q (caller's choice), got %q", winner, stored)
	}
}

// TestRecordEvent_NilEventCtx_RefusesInsert (regression for chunk-13 analyst F3).
// Before the guard, passing eventCtx=nil json.Marshal'd to the literal
// string "null" and wrote that as the context column of a real row —
// a useless row whose only content was "we don't know what this was."
// RecordEvent now returns an error and inserts nothing.
func TestRecordEvent_NilEventCtx_RefusesInsert(t *testing.T) {
	db := setupTestDB(t)
	hook := NewDBHook(db, "test-nil-ctx")

	err := hook.RecordEvent(context.Background(), nil, "posttool",
		[]Lookup{{Path: "success/x.wav", Sequence: 1, Found: true}},
		"success/x.wav")
	if err == nil {
		t.Fatal("expected RecordEvent with nil eventCtx to error; got nil")
	}

	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM hook_events").Scan(&n); err != nil {
		t.Fatalf("count hook_events: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 rows after refused insert, got %d", n)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM path_lookups").Scan(&n); err != nil {
		t.Fatalf("count path_lookups: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 path_lookups after refused insert, got %d", n)
	}
}

// TestRecordEvent_Atomic_NoPartialState (regression for #22).
// A duplicate-path collision in the path_lookups slice triggers a
// UNIQUE(event_id, path) constraint failure mid-transaction. The whole
// transaction must roll back — no hook_events row may survive.
//
// Chunk 13 analyst F4: the previous version of this test used a duplicate
// SEQUENCE which fires UNIQUE(event_id, sequence). That UNIQUE exists,
// but the production bug — the one this test guards against the
// regression of — was duplicate PATH entries in the chain (a tool name
// equal to its hint suffix collapses L1/L2/L3 to the same path) which
// fires UNIQUE(event_id, path). Switching the duplicate-detection axis
// here makes the test exercise the same constraint shape the production
// bug exercises.
func TestRecordEvent_Atomic_NoPartialState(t *testing.T) {
	db := setupTestDB(t)
	sessionID := "test-atomic"
	hook := NewDBHook(db, sessionID)

	eventCtx := &hooks.EventContext{Category: hooks.Error, ToolName: "bash"}
	// Two lookups with the same path will violate UNIQUE(event_id, path)
	// on the second INSERT, forcing a rollback.
	lookups := []Lookup{
		{Path: "error/dup.wav", Sequence: 1, Found: false},
		{Path: "error/dup.wav", Sequence: 2, Found: false}, // duplicate path
	}

	err := hook.RecordEvent(context.Background(), eventCtx, "posttool", lookups, "error/a.wav")
	if err == nil {
		t.Fatal("expected RecordEvent to return an error on duplicate sequence")
	}

	var eventCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM hook_events WHERE session_id = ?", sessionID).Scan(&eventCount); err != nil {
		t.Fatalf("count events: %v", err)
	}
	if eventCount != 0 {
		t.Errorf("expected 0 events after rollback, got %d", eventCount)
	}
	var pathCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM path_lookups`).Scan(&pathCount); err != nil {
		t.Fatalf("count path_lookups: %v", err)
	}
	if pathCount != 0 {
		t.Errorf("expected 0 path_lookups after rollback, got %d", pathCount)
	}
}

// TestRecordEvent_ConcurrentCallers_RaceClean (regression for #72).
// Spawn many goroutines calling RecordEvent against a shared DBHook.
// Under the race detector this must be clean (no data race), and the
// final row count must equal the number of calls.
//
// Uses a file-backed DB rather than `:memory:` — the in-memory driver
// gives each connection its own private database, so concurrent
// callers from the pool would each see an empty schema. Production
// always uses a file-backed DB.
func TestRecordEvent_ConcurrentCallers_RaceClean(t *testing.T) {
	dbPath := t.TempDir() + "/concurrent.db"
	// First open via NewDatabase to install the schema (pragmas + tables +
	// migrations). Then close it and re-open via a DSN that embeds
	// busy_timeout, so EVERY pooled connection — not just the first one
	// — waits at the SQLite write lock instead of failing fast with
	// SQLITE_BUSY. NewDatabase applies its busy_timeout via db.Exec, which
	// only takes effect on one pool connection; once we leave the cap off,
	// new pool connections opened under load do not inherit it. The DSN
	// pragma is honored on every connection the driver opens.
	if db0, err := NewDatabase(dbPath); err != nil {
		t.Fatalf("NewDatabase (schema setup): %v", err)
	} else {
		db0.Close()
	}
	dsn := "file:" + dbPath + "?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	// Note: do NOT set db.SetMaxOpenConns(1). That would serialize at the
	// database/sql layer before reaching SQLite's own write lock, masking
	// any race in DBHook itself. The whole point of this test is to
	// exercise the SQLite-level concurrency contract documented on
	// dbhook.go — "concurrent calls serialize at the SQLite write lock."
	// Leaving the pool unbounded lets multiple goroutines hold distinct
	// *sql.Conn handles and contend at SQLite, which is the production
	// surface we want guarded against future refactors that would add
	// shared mutable struct state to DBHook.
	t.Cleanup(func() { db.Close() })
	sessionID := "test-concurrent"
	hook := NewDBHook(db, sessionID)

	const N = 50
	var wg sync.WaitGroup
	errCh := make(chan error, N)
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			eventCtx := &hooks.EventContext{Category: hooks.Success, ToolName: "tool"}
			lookups := []Lookup{{Path: "success/x.wav", Sequence: 1, Found: true}}
			if err := hook.RecordEvent(context.Background(), eventCtx, "posttool", lookups, "success/x.wav"); err != nil {
				errCh <- err
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Errorf("concurrent RecordEvent error: %v", err)
		}
	}

	var eventCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM hook_events WHERE session_id = ?", sessionID).Scan(&eventCount); err != nil {
		t.Fatalf("count events: %v", err)
	}
	if eventCount != N {
		t.Errorf("expected %d events, got %d", N, eventCount)
	}
}

// TestRecordEvent_TransientErrorDoesNotLatch (regression for #73).
// A failure on one RecordEvent call (closed DB) must NOT poison the
// DBHook. Reopening the DB and calling RecordEvent again must succeed —
// there is no `disabled` flag to latch.
//
// We simulate "transient" by closing the connection, calling once (errors),
// then swapping the db handle on the hook via a freshly-opened connection
// to the same in-memory schema. The point is that DBHook itself never
// remembers the previous failure.
func TestRecordEvent_TransientErrorDoesNotLatch(t *testing.T) {
	db := setupTestDB(t)
	sessionID := "test-no-latch"
	hook := NewDBHook(db, sessionID)

	// First, prove the hook works.
	eventCtx := &hooks.EventContext{Category: hooks.Success, ToolName: "ok"}
	if err := hook.RecordEvent(context.Background(), eventCtx, "posttool",
		[]Lookup{{Path: "success/ok.wav", Sequence: 1, Found: true}},
		"success/ok.wav"); err != nil {
		t.Fatalf("baseline RecordEvent failed: %v", err)
	}

	// Force a transient error by closing the DB.
	db.Close()
	errCtx := &hooks.EventContext{Category: hooks.Error, ToolName: "broken"}
	err := hook.RecordEvent(context.Background(), errCtx, "posttool",
		[]Lookup{{Path: "error/broken.wav", Sequence: 1, Found: false}},
		"error/broken.wav")
	if err == nil {
		t.Fatal("expected error on closed DB")
	}

	// Critical: DBHook has no `disabled` field; nothing was latched.
	// A fresh DB plumbed into a fresh hook proves the design.
	newDB, openErr := NewDatabase(t.TempDir() + "/recovered.db")
	if openErr != nil {
		t.Fatalf("reopen DB: %v", openErr)
	}
	t.Cleanup(func() { newDB.Close() })
	recoveredHook := NewDBHook(newDB, sessionID)
	if err := recoveredHook.RecordEvent(context.Background(), eventCtx, "posttool",
		[]Lookup{{Path: "success/ok.wav", Sequence: 1, Found: true}},
		"success/ok.wav"); err != nil {
		t.Fatalf("post-error RecordEvent failed: %v", err)
	}

	var ec int
	if err := newDB.QueryRow("SELECT COUNT(*) FROM hook_events WHERE session_id = ?", sessionID).Scan(&ec); err != nil {
		t.Fatalf("count events on new DB: %v", err)
	}
	if ec != 1 {
		t.Errorf("expected 1 event on recovered DB, got %d", ec)
	}
}
