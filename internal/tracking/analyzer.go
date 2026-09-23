package tracking

import (
	"database/sql"
	"fmt"
	"slices"

	"claudio.click/internal/hooks"
)

// MissingSound represents a sound that was requested but not found
type MissingSound struct {
	Path         string   `json:"path"`
	RequestCount int      `json:"request_count"`
	Tools        []string `json:"tools,omitempty"`     // Which tools requested this sound
	Category     string   `json:"category,omitempty"`  // Category from context JSON (loading, success, error, etc.)
	ToolName     string   `json:"tool_name,omitempty"` // Tool name from context JSON
}

// GetMissingSounds queries the database for sounds that were requested but not found
func GetMissingSounds(db *sql.DB, filter QueryFilter) ([]MissingSound, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection is nil")
	}

	// Build the base query for missing sounds
	baseQuery := `
		SELECT 
			pl.path,
			COUNT(*) as request_count,
			GROUP_CONCAT(DISTINCT he.tool_name) as tools,
			` + categorySQL("he.context") + ` as category,
			JSON_EXTRACT(he.context, '$.ToolName') as context_tool
		FROM path_lookups pl
		JOIN hook_events he ON pl.event_id = he.id
		WHERE pl.found = 0`

	// Use QueryFilter to build WHERE clause
	whereClause, args, err := filter.BuildWhereClause()
	if err != nil {
		return nil, err
	}
	if whereClause != "" {
		baseQuery += " AND " + whereClause
	}

	// Keep tool/category ownership explicit. Generic fallback paths can be
	// requested by several tools and must not inherit an arbitrary context.
	baseQuery += `
		GROUP BY pl.path, category, context_tool
		ORDER BY request_count DESC, pl.path, category, context_tool`

	// Add limit if specified
	if filter.Limit > 0 {
		baseQuery += fmt.Sprintf(" LIMIT %d", filter.Limit)
	}

	rows, err := db.Query(baseQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query missing sounds: %w", err)
	}
	defer rows.Close()

	var results []MissingSound
	for rows.Next() {
		var sound MissingSound
		var toolsStr sql.NullString
		var category sql.NullString
		var contextTool sql.NullString

		err := rows.Scan(&sound.Path, &sound.RequestCount, &toolsStr, &category, &contextTool)
		if err != nil {
			return nil, fmt.Errorf("failed to scan missing sound row: %w", err)
		}

		// Parse tools string (comma-separated)
		if toolsStr.Valid && toolsStr.String != "" {
			sound.Tools = sortedUnique(parseCommaSeparated(toolsStr.String))
		}

		sound.Category = categoryName(category)
		if contextTool.Valid {
			sound.ToolName = contextTool.String
		}

		results = append(results, sound)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating missing sound rows: %w", err)
	}

	return results, nil
}

// GetMissingSoundsSummary returns summary statistics about missing sounds
func GetMissingSoundsSummary(db *sql.DB, filter QueryFilter) (map[string]interface{}, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection is nil")
	}

	// Build summary query
	summaryQuery := `
		SELECT 
			COUNT(DISTINCT pl.path) as unique_missing_sounds,
			COUNT(*) as total_missing_requests,
			COUNT(DISTINCT he.tool_name) as tools_with_missing_sounds
		FROM path_lookups pl
		JOIN hook_events he ON pl.event_id = he.id
		WHERE pl.found = 0`

	// Use QueryFilter to build WHERE clause
	whereClause, args, err := filter.BuildWhereClause()
	if err != nil {
		return nil, err
	}
	if whereClause != "" {
		summaryQuery += " AND " + whereClause
	}

	var uniqueSounds, totalRequests, toolsWithMissing int
	err = db.QueryRow(summaryQuery, args...).Scan(&uniqueSounds, &totalRequests, &toolsWithMissing)
	if err != nil {
		return nil, fmt.Errorf("failed to query missing sounds summary: %w", err)
	}

	summary := map[string]interface{}{
		"unique_missing_sounds":     uniqueSounds,
		"total_missing_requests":    totalRequests,
		"tools_with_missing_sounds": toolsWithMissing,
		"query_days":                filter.Days,
		"query_tool_filter":         filter.Tool,
	}

	return summary, nil
}

// parseCommaSeparated splits a comma-separated string and trims whitespace
func parseCommaSeparated(s string) []string {
	if s == "" {
		return nil
	}

	var result []string
	for _, part := range splitString(s, ",") {
		trimmed := trimString(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// splitString splits a string by delimiter (avoiding strings package dependency)
func splitString(s, delimiter string) []string {
	if s == "" {
		return nil
	}

	var result []string
	start := 0

	for i := 0; i <= len(s)-len(delimiter); i++ {
		if s[i:i+len(delimiter)] == delimiter {
			result = append(result, s[start:i])
			start = i + len(delimiter)
		}
	}
	result = append(result, s[start:])

	return result
}

// trimString removes leading and trailing whitespace (avoiding strings package dependency)
func trimString(s string) string {
	start := 0
	end := len(s)

	// Trim leading whitespace
	for start < end && isWhitespace(s[start]) {
		start++
	}

	// Trim trailing whitespace
	for end > start && isWhitespace(s[end-1]) {
		end--
	}

	return s[start:end]
}

// isWhitespace checks if a character is whitespace
func isWhitespace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

// categoryName turns a scanned categorySQL value into a category name;
// NULL or unrecognised values read back as "unknown" (or "" for NULL).
func categoryName(v sql.NullString) string {
	if !v.Valid {
		return ""
	}
	if _, err := hooks.ParseEventCategory(v.String); err != nil {
		return "unknown"
	}
	return v.String
}

// TDD GREEN: New usage analysis data structures

// SoundUsage represents actual sound playback statistics.
//
// Note: prior to v2 this carried FallbackLevel/AvgFallback derived from
// the now-deleted hook_events.fallback_level column. Those values mixed
// semantics across the three chain shapes (enhanced/posttool/simple) and
// were meaningless in aggregate. See review finding #20.
type SoundUsage struct {
	Path       string `json:"path"`
	PlayCount  int    `json:"play_count"`
	Category   string `json:"category,omitempty"`
	ToolName   string `json:"tool_name,omitempty"`
	LastPlayed int64  `json:"last_played"` // Unix timestamp
}

// UsageSummary provides overall usage statistics.
type UsageSummary struct {
	TotalEvents  int    `json:"total_events"`
	UniqueSounds int    `json:"unique_sounds"`
	TimeRange    string `json:"time_range,omitempty"` // Human readable
}

// ChainTypeStatistic summarizes how often each chain type fires and how
// deep into its fallback chain it landed on average. Replaces the
// removed FallbackStatistic — chain-scoped sequence is the only honest
// version of "how often does Claudio fall back?" given that sequence
// numbering is not comparable across chains. See review finding #20.
type ChainTypeStatistic struct {
	ChainType  string  `json:"chain_type"`
	EventCount int     `json:"event_count"`
	AvgDepth   float64 `json:"avg_depth"` // Average selected_path sequence within the chain
	Percentage float64 `json:"percentage"`
}

// TDD GREEN: Usage analysis functions

// GetSoundUsage returns statistics about actual sound playback
func GetSoundUsage(db *sql.DB, filter QueryFilter) ([]SoundUsage, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection is nil")
	}

	// Build query to get sound usage statistics
	baseQuery := `
		SELECT
			he.selected_path,
			COUNT(*) as play_count,
			MAX(he.timestamp) as last_played,
			CASE
				WHEN COUNT(*) = COUNT(` + categorySQL("he.context") + `)
					AND COUNT(DISTINCT ` + categorySQL("he.context") + `) = 1
				THEN MAX(` + categorySQL("he.context") + `)
			END as category,
			CASE
				WHEN COUNT(*) = COUNT(JSON_EXTRACT(he.context, '$.ToolName'))
					AND COUNT(DISTINCT JSON_EXTRACT(he.context, '$.ToolName')) = 1
				THEN MAX(JSON_EXTRACT(he.context, '$.ToolName'))
			END as context_tool
		FROM hook_events he
		WHERE he.selected_path != ''`

	// Apply filters using common QueryFilter
	whereClause, args, err := filter.BuildWhereClause()
	if err != nil {
		return nil, err
	}
	if whereClause != "" {
		baseQuery += " AND " + whereClause
	}

	baseQuery += `
		GROUP BY he.selected_path
		ORDER BY play_count DESC, he.selected_path`

	// Apply limit
	if filter.Limit > 0 {
		baseQuery += fmt.Sprintf(" LIMIT %d", filter.Limit)
	}

	rows, err := db.Query(baseQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query sound usage: %w", err)
	}
	defer rows.Close()

	var results []SoundUsage
	for rows.Next() {
		var usage SoundUsage
		var category sql.NullString
		var contextTool sql.NullString

		err := rows.Scan(&usage.Path, &usage.PlayCount, &usage.LastPlayed, &category, &contextTool)
		if err != nil {
			return nil, fmt.Errorf("failed to scan sound usage row: %w", err)
		}

		usage.Category = categoryName(category)
		if contextTool.Valid {
			usage.ToolName = contextTool.String
		}

		results = append(results, usage)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating sound usage rows: %w", err)
	}

	return results, nil
}

// GetUsageSummary returns overall usage statistics
func GetUsageSummary(db *sql.DB, filter QueryFilter) (*UsageSummary, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection is nil")
	}

	// Build query for summary statistics
	summaryQuery := `
		SELECT
			COUNT(*) as total_events,
			COUNT(DISTINCT he.selected_path) as unique_sounds
		FROM hook_events he
		WHERE he.selected_path != ''`

	// Apply filters using common QueryFilter
	whereClause, args, err := filter.BuildWhereClause()
	if err != nil {
		return nil, err
	}
	if whereClause != "" {
		summaryQuery += " AND " + whereClause
	}

	var summary UsageSummary
	err = db.QueryRow(summaryQuery, args...).Scan(&summary.TotalEvents, &summary.UniqueSounds)
	if err != nil {
		return nil, fmt.Errorf("failed to query usage summary: %w", err)
	}

	return &summary, nil
}

// GetChainTypeStatistics returns per-chain-type event counts and average
// selected-path depth (joined from path_lookups). The depth reflects how
// far down the fallback chain Claudio had to walk before finding a
// playable sound — high depth = thin soundpack coverage for that chain.
//
// chain_type is nullable in schema v2 (pre-migration rows have NULL);
// rows with NULL chain_type are surfaced under the empty-string label
// so callers can still see them without lying about which chain they
// came from.
func GetChainTypeStatistics(db *sql.DB, filter QueryFilter) ([]ChainTypeStatistic, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection is nil")
	}

	baseQuery := `
		SELECT
			COALESCE(he.chain_type, '') AS chain_type,
			COUNT(*) AS event_count,
			AVG(COALESCE(pl.sequence, 0)) AS avg_depth
		FROM hook_events he
		LEFT JOIN path_lookups pl
			ON pl.event_id = he.id AND pl.path = he.selected_path
		WHERE he.selected_path != ''`

	whereClause, args, err := filter.BuildWhereClause()
	if err != nil {
		return nil, err
	}
	if whereClause != "" {
		baseQuery += " AND " + whereClause
	}

	baseQuery += `
		GROUP BY COALESCE(he.chain_type, '')
		ORDER BY event_count DESC, chain_type`

	rows, err := db.Query(baseQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query chain type statistics: %w", err)
	}
	defer rows.Close()

	var results []ChainTypeStatistic
	var total int

	for rows.Next() {
		var stat ChainTypeStatistic
		if err := rows.Scan(&stat.ChainType, &stat.EventCount, &stat.AvgDepth); err != nil {
			return nil, fmt.Errorf("failed to scan chain type statistics row: %w", err)
		}
		total += stat.EventCount
		results = append(results, stat)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating chain type statistics rows: %w", err)
	}

	for i := range results {
		if total > 0 {
			results[i].Percentage = float64(results[i].EventCount) / float64(total) * 100.0
		}
	}

	return results, nil
}

// sortedUnique returns the non-empty values of s, sorted and deduplicated.
// GROUP_CONCAT order is unspecified, so sorting keeps output stable.
func sortedUnique(s []string) []string {
	s = slices.DeleteFunc(s, func(v string) bool { return v == "" })
	if len(s) == 0 {
		return nil
	}
	slices.Sort(s)
	return slices.Compact(s)
}
