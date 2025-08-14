package tracking

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/tj/go-naturaldate"
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
	Soundpack string // Filter by soundpack name/path
	SessionID string // Filter by specific session

	// Output control
	Limit   int    // Maximum results (default: 20)
	Offset  int    // For pagination
	OrderBy string // Sort field
	OrderDesc bool // Sort direction
}

// ApplyTimeFilter converts QueryFilter time options to Unix timestamps
func (q *QueryFilter) ApplyTimeFilter(now time.Time) (startUnix, endUnix int64) {
	slog.Debug("applying time filter", "days", q.Days, "date_preset", q.DatePreset)

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
func (q *QueryFilter) BuildWhereClause() (string, []interface{}) {
	var clauses []string
	var args []interface{}

	slog.Debug("building where clause", "tool", q.Tool, "category", q.Category, "session_id", q.SessionID)

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

	// Category filter (stored in context JSON)
	if q.Category != "" {
		// Map category string to integer for JSON lookup
		categoryInt := categoryStringToInt(q.Category)
		clauses = append(clauses, "JSON_EXTRACT(context, '$.Category') = ?")
		args = append(args, categoryInt)
	}

	// Session filter
	if q.SessionID != "" {
		clauses = append(clauses, "session_id = ?")
		args = append(args, q.SessionID)
	}

	// Soundpack filter (would be added when soundpack tracking is implemented)
	if q.Soundpack != "" {
		clauses = append(clauses, "JSON_EXTRACT(context, '$.SoundpackName') = ?")
		args = append(args, q.Soundpack)
	}

	// Join with AND
	whereClause := ""
	if len(clauses) > 0 {
		whereClause = strings.Join(clauses, " AND ")
	}
	
	slog.Debug("built where clause", "clause", whereClause, "arg_count", len(args))
	
	return whereClause, args
}

// ParseDatePreset converts date preset strings to time ranges
func ParseDatePreset(preset string, now time.Time) (start, end time.Time, err error) {
	slog.Debug("parsing date preset", "preset", preset)

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
		slog.Error("invalid date preset", "preset", preset)
		return
	}

	slog.Debug("parsed date preset", "preset", preset, "start", start, "end", end)
	return
}

// ParseNaturalDate parses natural language dates using go-naturaldate
func ParseNaturalDate(naturalDate string) (time.Time, error) {
	slog.Debug("parsing natural language date", "input", naturalDate)

	// Use go-naturaldate to parse natural language dates
	result, err := naturaldate.Parse(naturalDate, time.Now())
	if err != nil {
		slog.Warn("failed to parse natural language date", "input", naturalDate, "error", err)
		return time.Time{}, fmt.Errorf("failed to parse natural date '%s': %w", naturalDate, err)
	}

	slog.Debug("parsed natural language date", "input", naturalDate, "result", result)
	return result, nil
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

// categoryStringToInt converts category string to integer representation for database queries
func categoryStringToInt(category string) int {
	switch category {
	case "loading":
		return 0
	case "success":
		return 1
	case "error":
		return 2
	case "interactive":
		return 3
	case "completion":
		return 4
	case "system":
		return 5
	default:
		slog.Warn("unknown category string, using 0 (loading)", "category", category)
		return 0 // Default to loading
	}
}

// categoryIntToString converts category integer to string representation  
func categoryIntToString(categoryInt int) string {
	switch categoryInt {
	case 0:
		return "loading"
	case 1:
		return "success"
	case 2:
		return "error"
	case 3:
		return "interactive"
	case 4:
		return "completion"
	case 5:
		return "system"
	default:
		return "unknown"
	}
}