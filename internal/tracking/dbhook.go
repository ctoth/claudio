package tracking

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/ctoth/claudio/internal/hooks"
)

// DBHook implements database logging for sound path checks
type DBHook struct {
	db           *sql.DB
	sessionID    string
	disabled     bool
	eventID      int64 // Current event ID for grouping path checks
	context      *hooks.EventContext // Current context for grouping
	pathChecks   []pathCheckEntry // Buffer for current event's path checks
	selectedPath string
	fallbackLevel int
}

// pathCheckEntry represents a single path check
type pathCheckEntry struct {
	path     string
	exists   bool
	sequence int
}

// NewDBHook creates a new database hook for the specified session
func NewDBHook(db *sql.DB, sessionID string) *DBHook {
	return &DBHook{
		db:            db,
		sessionID:     sessionID,
		disabled:      false,
		eventID:       0, // Will be set when first path check is logged
		context:       nil, // Will be set for context grouping
		pathChecks:    make([]pathCheckEntry, 0),
		selectedPath:  "",
		fallbackLevel: 0,
	}
}

// LogPathCheck logs a path check to the database with transaction handling
func (d *DBHook) LogPathCheck(path string, exists bool, sequence int, context *hooks.EventContext) {
	// Skip if disabled due to previous errors
	if d.disabled {
		return
	}

	// Check if this is a new context (new event group)
	if d.needsNewEvent(context) {
		d.startNewEvent(context)
		// For the first path in a new event, assume it's selected
		// This will be updated if a later path exists
		d.selectedPath = path
		d.fallbackLevel = sequence
		
		if err := d.ensureEvent(d.context, d.selectedPath, d.fallbackLevel); err != nil {
			slog.Warn("sound tracking failed to create event", "error", err, "path", path)
			d.disabled = true
			return
		}
	}

	// Update selected path if this one exists (first existing path wins)
	if exists && (d.selectedPath == "" || !d.hasExistingPath()) {
		d.selectedPath = path
		d.fallbackLevel = sequence
		// Update the event record with the correct selected path
		if err := d.updateEventSelection(path, sequence); err != nil {
			slog.Warn("sound tracking failed to update event selection", "error", err, "path", path)
			d.disabled = true
			return
		}
	}

	// Insert path lookup
	if err := d.insertPathCheck(path, exists, sequence); err != nil {
		slog.Warn("sound tracking failed to log path check", "error", err, "path", path)
		d.disabled = true
		return
	}

	slog.Debug("sound tracking logged path check",
		"session_id", d.sessionID,
		"event_id", d.eventID,
		"path", path,
		"exists", exists,
		"sequence", sequence)
}

// needsNewEvent determines if a new event should be created for this context
func (d *DBHook) needsNewEvent(context *hooks.EventContext) bool {
	// Always create new event if we don't have one yet
	if d.context == nil {
		return true
	}
	
	// Create new event if key context fields differ
	return d.context.Category != context.Category ||
		d.context.ToolName != context.ToolName ||
		d.context.Operation != context.Operation
}

// startNewEvent initializes a new event context
func (d *DBHook) startNewEvent(context *hooks.EventContext) {
	d.context = context
	d.pathChecks = make([]pathCheckEntry, 0)
	d.selectedPath = ""
	d.fallbackLevel = 0
	d.eventID = 0
}

// hasExistingPath checks if the current selected path exists
func (d *DBHook) hasExistingPath() bool {
	for _, check := range d.pathChecks {
		if check.path == d.selectedPath && check.exists {
			return true
		}
	}
	return false
}

// updateEventSelection updates the event record with new selected path and fallback level
func (d *DBHook) updateEventSelection(selectedPath string, fallbackLevel int) error {
	_, err := d.db.Exec(`
		UPDATE hook_events 
		SET selected_path = ?, fallback_level = ?
		WHERE id = ?`,
		selectedPath,
		fallbackLevel,
		d.eventID)
	return err
}

// ensureEvent creates a hook event record if one doesn't exist for this context
func (d *DBHook) ensureEvent(context *hooks.EventContext, selectedPath string, fallbackLevel int) error {
	// Marshal context to JSON
	contextJSON, err := json.Marshal(context)
	if err != nil {
		return err
	}

	// Insert event and get ID
	result, err := d.db.Exec(`
		INSERT INTO hook_events (timestamp, session_id, tool_name, selected_path, fallback_level, context)
		VALUES (?, ?, ?, ?, ?, ?)`,
		time.Now().Unix(),
		d.sessionID,
		context.ToolName,
		selectedPath,
		fallbackLevel,
		string(contextJSON))
	if err != nil {
		return err
	}

	eventID, err := result.LastInsertId()
	if err != nil {
		return err
	}

	d.eventID = eventID
	return nil
}

// insertPathCheck inserts a path lookup record
func (d *DBHook) insertPathCheck(path string, exists bool, sequence int) error {
	found := 0
	if exists {
		found = 1
	}

	_, err := d.db.Exec(`
		INSERT INTO path_lookups (event_id, path, sequence, found)
		VALUES (?, ?, ?, ?)`,
		d.eventID,
		path,
		sequence,
		found)
	return err
}

// GetHook returns the PathCheckedHook function for use with SoundChecker
func (d *DBHook) GetHook() PathCheckedHook {
	return d.LogPathCheck
}