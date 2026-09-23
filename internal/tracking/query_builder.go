package tracking

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"claudio.click/internal/hooks"
)

// QueryFilter represents common query structure for all analyze commands
type QueryFilter struct {
	// Time filters (mutually exclusive)
	StartTime  *time.Time // Start of time range (inclusive)
	EndTime    *time.Time // End of time range (exclusive)
	Days       int        // Convenience: last N days (overrides Start/End)
	DatePreset string     // Convenience: "today", "yesterday", "week", "month", "all"

	// Content filters
	Tool      string // Filter by specific tool
	Category  string // Filter by category (success/error/loading)
	SessionID string // Filter by specific session

	// Output control
	Limit     int    // Maximum results (default: 20)
	Offset    int    // For pagination
	OrderBy   string // Sort field
	OrderDesc bool   // Sort direction
}

// ApplyTimeFilter converts QueryFilter time options to Unix timestamps
func (q *QueryFilter) ApplyTimeFilter(now time.Time) (startUnix, endUnix int64) {
	endUnix = now.Unix()

	// Priority order: DatePreset > StartTime/EndTime > Days > no filter
	if q.DatePreset != "" {
		// Use date preset
		start, end, err := ParseDatePreset(q.DatePreset, now)
		if err != nil {
			slog.Warn("invalid date preset, using no time filter", "preset", q.DatePreset, "error", err)
			return 0, endUnix // No lower bound for invalid presets
		}
		return start.Unix(), end.Unix()
	}

	// Use explicit start/end times if provided
	if q.StartTime != nil && q.EndTime != nil {
		return q.StartTime.Unix(), q.EndTime.Unix()
	}
	if q.StartTime != nil {
		return q.StartTime.Unix(), endUnix
	}
	if q.EndTime != nil {
		return 0, q.EndTime.Unix() // No lower bound, use provided end
	}

	// Use days filter
	if q.Days > 0 {
		startTime := now.AddDate(0, 0, -q.Days)
		return startTime.Unix(), endUnix
	}

	// No time filter - return no lower bound
	return 0, endUnix
}

// BuildWhereClause constructs SQL WHERE clause and arguments from QueryFilter
// Using simple string building for reliability and predictability
// An unknown Category is an error rather than a filter that matches nothing.
func (q *QueryFilter) BuildWhereClause() (string, []any, error) {
	var clauses []string
	var args []any

	// Apply time filters
	if q.StartTime != nil || q.EndTime != nil || q.Days > 0 || q.DatePreset != "" {
		// Use ApplyTimeFilter to get the actual timestamps
		startUnix, endUnix := q.ApplyTimeFilter(time.Now())

		if startUnix > 0 {
			clauses = append(clauses, "timestamp >= ?")
			args = append(args, startUnix)
		}

		clauses = append(clauses, "timestamp <= ?")
		args = append(args, endUnix)
	}

	// Tool filter
	if q.Tool != "" {
		clauses = append(clauses, "tool_name = ?")
		args = append(args, q.Tool)
	}

	// Category filter (stored in context JSON, legacy rows as an int)
	if q.Category != "" {
		category, err := hooks.ParseEventCategory(q.Category)
		if err != nil {
			return "", nil, err
		}
		clauses = append(clauses, categorySQL("context")+" = ?")
		args = append(args, category.String())
	}

	// Session filter
	if q.SessionID != "" {
		clauses = append(clauses, "session_id = ?")
		args = append(args, q.SessionID)
	}

	// Join with AND
	whereClause := ""
	if len(clauses) > 0 {
		whereClause = strings.Join(clauses, " AND ")
	}

	slog.Debug("built where clause", "clause", whereClause, "arg_count", len(args))

	return whereClause, args, nil
}

// categorySQL is an SQL expression yielding the category name stored in a
// hook_events.context column. Current rows store the name; rows recorded
// before categories were names store the iota int, which is mapped here so
// old and new rows filter and group together.
func categorySQL(contextCol string) string {
	extract := fmt.Sprintf("JSON_EXTRACT(%s, '$.Category')", contextCol)
	var b strings.Builder
	b.WriteString("CASE " + extract)
	for _, c := range hooks.Categories() {
		fmt.Fprintf(&b, " WHEN %d THEN '%s'", int(c), c)
	}
	b.WriteString(" ELSE " + extract + " END")
	return b.String()
}

// DatePresets lists the canonical preset names ParseDatePreset accepts, for
// help text and validation messages. ParseDatePreset also accepts the short
// aliases week, month, and all.
var DatePresets = []string{"today", "yesterday", "this-week", "last-week", "this-month", "last-month", "all-time"}

// ParseDatePreset converts date preset strings to time ranges
func ParseDatePreset(preset string, now time.Time) (start, end time.Time, err error) {
	switch preset {
	case "today":
		start = beginningOfDay(now)
		end = now
	case "yesterday":
		yesterday := now.AddDate(0, 0, -1)
		start = beginningOfDay(yesterday)
		end = beginningOfDay(now)
	case "week", "this-week":
		start = beginningOfWeek(now)
		end = now
	case "last-week":
		start = beginningOfWeek(now).AddDate(0, 0, -7)
		end = beginningOfWeek(now)
	case "month", "this-month":
		start = beginningOfMonth(now)
		end = now
	case "last-month":
		start = beginningOfMonth(now).AddDate(0, -1, 0)
		end = beginningOfMonth(now)
	case "all", "all-time":
		start = time.Time{} // Zero value = no lower bound
		end = now
	default:
		err = fmt.Errorf("unknown preset: %s", preset)
		return
	}

	slog.Debug("parsed date preset", "preset", preset, "start", start, "end", end)
	return
}

// Helper functions for date manipulation

// beginningOfDay returns time at start of day (00:00:00)
func beginningOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// beginningOfWeek returns time at start of week (Monday 00:00:00)
func beginningOfWeek(t time.Time) time.Time {
	// Find Monday of current week
	weekday := t.Weekday()
	if weekday == time.Sunday {
		weekday = 7 // Treat Sunday as 7 to make Monday = 1
	}
	monday := t.AddDate(0, 0, -int(weekday-1))
	return beginningOfDay(monday)
}

// beginningOfMonth returns time at start of month (1st day 00:00:00)
func beginningOfMonth(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
}
