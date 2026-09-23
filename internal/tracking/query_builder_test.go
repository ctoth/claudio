package tracking

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TDD RED: Test QueryFilter struct and methods that don't exist yet

func TestQueryFilter_ApplyTimeFilter(t *testing.T) {
	tests := []struct {
		name      string
		filter    QueryFilter
		now       time.Time
		wantStart int64
		wantEnd   int64
	}{
		{
			name: "Days filter - 7 days",
			filter: QueryFilter{
				Days: 7,
			},
			now:       time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC),
			wantStart: time.Date(2024, 1, 8, 12, 0, 0, 0, time.UTC).Unix(),
			wantEnd:   time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC).Unix(),
		},
		{
			name: "StartTime and EndTime specified",
			filter: QueryFilter{
				StartTime: timePtr(time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC)),
				EndTime:   timePtr(time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)),
			},
			now:       time.Date(2024, 1, 20, 12, 0, 0, 0, time.UTC),
			wantStart: time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC).Unix(),
			wantEnd:   time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC).Unix(),
		},
		{
			name: "DatePreset - today",
			filter: QueryFilter{
				DatePreset: "today",
			},
			now:       time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC),
			wantStart: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC).Unix(),
			wantEnd:   time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC).Unix(),
		},
		{
			name: "DatePreset - yesterday",
			filter: QueryFilter{
				DatePreset: "yesterday",
			},
			now:       time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC),
			wantStart: time.Date(2024, 1, 14, 0, 0, 0, 0, time.UTC).Unix(),
			wantEnd:   time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC).Unix(),
		},
		{
			name: "DatePreset - last week",
			filter: QueryFilter{
				DatePreset: "last-week",
			},
			now:       time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC),      // Monday
			wantStart: time.Date(2024, 1, 8, 0, 0, 0, 0, time.UTC).Unix(),  // Previous Monday
			wantEnd:   time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC).Unix(), // Current Monday (start of today)
		},
		{
			name:      "No time filter - should return zero start and current time end",
			filter:    QueryFilter{},
			now:       time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC),
			wantStart: 0, // No lower bound
			wantEnd:   time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC).Unix(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// TDD RED: This method doesn't exist yet
			gotStart, gotEnd := tt.filter.ApplyTimeFilter(tt.now)
			assert.Equal(t, tt.wantStart, gotStart, "Start time mismatch")
			assert.Equal(t, tt.wantEnd, gotEnd, "End time mismatch")
		})
	}
}

func TestQueryFilter_BuildWhereClause(t *testing.T) {
	tests := []struct {
		name         string
		filter       QueryFilter
		wantClause   string
		wantArgCount int
	}{
		{
			name:         "Empty filter",
			filter:       QueryFilter{},
			wantClause:   "",
			wantArgCount: 0,
		},
		{
			name: "Tool filter only",
			filter: QueryFilter{
				Tool: "bash",
			},
			wantClause:   "tool_name = ?",
			wantArgCount: 1,
		},
		{
			name: "Category and tool filter",
			filter: QueryFilter{
				Tool:     "git",
				Category: "success",
			},
			wantClause:   "tool_name = ? AND " + categorySQL("context") + " = ?",
			wantArgCount: 2,
		},
		{
			name: "Time range filter with start and end",
			filter: QueryFilter{
				StartTime: timePtr(time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC)),
				EndTime:   timePtr(time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)),
			},
			wantClause:   "timestamp >= ? AND timestamp <= ?",
			wantArgCount: 2,
		},
		{
			name: "Session filter",
			filter: QueryFilter{
				SessionID: "test-session-123",
			},
			wantClause:   "session_id = ?",
			wantArgCount: 1,
		},
		{
			name: "All filters combined",
			filter: QueryFilter{
				StartTime: timePtr(time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC)),
				EndTime:   timePtr(time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)),
				Tool:      "edit",
				Category:  "error",
				SessionID: "session-456",
			},
			wantClause:   "timestamp >= ? AND timestamp <= ? AND tool_name = ? AND " + categorySQL("context") + " = ? AND session_id = ?",
			wantArgCount: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// TDD RED: This method doesn't exist yet
			gotClause, gotArgs, err := tt.filter.BuildWhereClause()
			assert.NoError(t, err)
			assert.Equal(t, tt.wantClause, gotClause, "WHERE clause mismatch")
			assert.Len(t, gotArgs, tt.wantArgCount, "Argument count mismatch")
		})
	}
}

func TestParseDatePreset(t *testing.T) {
	baseTime := time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC) // Monday

	tests := []struct {
		name      string
		preset    string
		wantStart time.Time
		wantEnd   time.Time
		wantError bool
	}{
		{
			name:      "today",
			preset:    "today",
			wantStart: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			wantEnd:   baseTime,
			wantError: false,
		},
		{
			name:      "yesterday",
			preset:    "yesterday",
			wantStart: time.Date(2024, 1, 14, 0, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			wantError: false,
		},
		{
			name:      "this-week",
			preset:    "this-week",
			wantStart: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC), // Monday - start of week
			wantEnd:   baseTime,
			wantError: false,
		},
		{
			name:      "last-week",
			preset:    "last-week",
			wantStart: time.Date(2024, 1, 8, 0, 0, 0, 0, time.UTC),  // Previous Monday
			wantEnd:   time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC), // Current Monday
			wantError: false,
		},
		{
			name:      "this-month",
			preset:    "this-month",
			wantStart: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			wantEnd:   baseTime,
			wantError: false,
		},
		{
			name:      "all-time",
			preset:    "all-time",
			wantStart: time.Time{}, // Zero value = no lower bound
			wantEnd:   baseTime,
			wantError: false,
		},
		{
			name:      "invalid preset",
			preset:    "invalid-preset",
			wantStart: time.Time{},
			wantEnd:   time.Time{},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// TDD RED: This function doesn't exist yet
			gotStart, gotEnd, err := ParseDatePreset(tt.preset, baseTime)

			if tt.wantError {
				assert.Error(t, err, "Expected error for invalid preset")
			} else {
				assert.NoError(t, err, "Unexpected error")
				assert.Equal(t, tt.wantStart, gotStart, "Start time mismatch")
				assert.Equal(t, tt.wantEnd, gotEnd, "End time mismatch")
			}
		})
	}
}

// Helper functions for tests

// timePtr returns a pointer to a time.Time
func timePtr(t time.Time) *time.Time {
	return &t
}
