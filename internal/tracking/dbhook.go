package tracking

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"claudio.click/internal/hooks"
)

// DBHook records sound resolution events to a SQLite database.
//
// DBHook is safe for concurrent use. Each RecordEvent call runs in its
// own SQLite transaction; concurrent calls serialize at the SQLite write
// lock. No goroutine-affinity is required and no cross-call state is
// retained — the API is one-shot per event.
//
// Errors from RecordEvent are returned to the caller, never latched.
// The caller (sound mapper) logs at WARN and continues; tracking is
// best-effort and a failure does not crash a hook.
type DBHook struct {
	db        *sql.DB
	sessionID string
}

// NewDBHook creates a new database recorder for the specified session.
func NewDBHook(db *sql.DB, sessionID string) *DBHook {
	return &DBHook{
		db:        db,
		sessionID: sessionID,
	}
}

// RecordEvent writes one hook_events row and all of its path_lookups in
// a single transaction. The caller passes the already-resolved
// selectedPath; the recorder trusts that choice and does not re-derive
// it from the lookups slice.
//
// Returns the underlying error on any failure; the transaction is
// rolled back automatically (no partial state lands).
func (d *DBHook) RecordEvent(
	ctx context.Context,
	eventCtx *hooks.EventContext,
	chainType string,
	lookups []Lookup,
	selectedPath string,
) error {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	contextJSON, err := json.Marshal(eventCtx)
	if err != nil {
		return fmt.Errorf("marshal context: %w", err)
	}

	var toolName string
	if eventCtx != nil {
		toolName = eventCtx.ToolName
	}

	res, err := tx.ExecContext(ctx, `
		INSERT INTO hook_events (timestamp, session_id, tool_name, selected_path, chain_type, context)
		VALUES (?, ?, ?, ?, ?, ?)`,
		time.Now().Unix(),
		d.sessionID,
		toolName,
		selectedPath,
		chainType,
		string(contextJSON))
	if err != nil {
		return fmt.Errorf("insert hook_event: %w", err)
	}

	eventID, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("last insert id: %w", err)
	}

	if len(lookups) > 0 {
		stmt, err := tx.PrepareContext(ctx, `
			INSERT INTO path_lookups (event_id, path, sequence, found)
			VALUES (?, ?, ?, ?)`)
		if err != nil {
			return fmt.Errorf("prepare lookup insert: %w", err)
		}
		defer stmt.Close()

		for _, lk := range lookups {
			found := 0
			if lk.Found {
				found = 1
			}
			if _, err := stmt.ExecContext(ctx, eventID, lk.Path, lk.Sequence, found); err != nil {
				return fmt.Errorf("insert path lookup (seq=%d): %w", lk.Sequence, err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}
