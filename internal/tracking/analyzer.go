package tracking

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

// MissingSound represents a sound that was requested but not found
type MissingSound struct {
	Path         string   `json:"path"`
	RequestCount int      `json:"request_count"`
	Tools        []string `json:"tools,omitempty"` // Which tools requested this sound
	Category     string   `json:"category,omitempty"` // Category from context JSON (loading, success, error, etc.)
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
			he.context
		FROM path_lookups pl
		JOIN hook_events he ON pl.event_id = he.id
		WHERE pl.found = 0`

	// Use QueryFilter to build WHERE clause
	whereClause, args := filter.BuildWhereClause()
	if whereClause != "" {
		baseQuery += " AND " + whereClause
	}

	// Group by path and order by frequency
	// We group by path and take any context (since all requests for same path should have similar context)
	baseQuery += `
		GROUP BY pl.path
		ORDER BY request_count DESC`

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
		var contextStr sql.NullString

		err := rows.Scan(&sound.Path, &sound.RequestCount, &toolsStr, &contextStr)
		if err != nil {
			return nil, fmt.Errorf("failed to scan missing sound row: %w", err)
		}

		// Parse tools string (comma-separated)
		if toolsStr.Valid && toolsStr.String != "" {
			// Split the comma-separated tools and deduplicate
			toolMap := make(map[string]bool)
			for _, tool := range parseCommaSeparated(toolsStr.String) {
				if tool != "" {
					toolMap[tool] = true
				}
			}
			
			// Convert back to slice
			for tool := range toolMap {
				sound.Tools = append(sound.Tools, tool)
			}
		}

		// Parse context JSON to extract Category and ToolName
		if contextStr.Valid && contextStr.String != "" {
			category, toolName := extractFromContextJSON(contextStr.String)
			sound.Category = category
			sound.ToolName = toolName
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
	whereClause, args := filter.BuildWhereClause()
	if whereClause != "" {
		summaryQuery += " AND " + whereClause
	}

	var uniqueSounds, totalRequests, toolsWithMissing int
	err := db.QueryRow(summaryQuery, args...).Scan(&uniqueSounds, &totalRequests, &toolsWithMissing)
	if err != nil {
		return nil, fmt.Errorf("failed to query missing sounds summary: %w", err)
	}

	summary := map[string]interface{}{
		"unique_missing_sounds":      uniqueSounds,
		"total_missing_requests":     totalRequests,
		"tools_with_missing_sounds":  toolsWithMissing,
		"query_days":                 filter.Days,
		"query_tool_filter":          filter.Tool,
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

// extractFromContextJSON parses context JSON to extract Category and ToolName
func extractFromContextJSON(contextJSON string) (category, toolName string) {
	if contextJSON == "" {
		return "", ""
	}
	
	// Parse the JSON context to extract Category and ToolName
	var context map[string]interface{}
	err := json.Unmarshal([]byte(contextJSON), &context)
	if err != nil {
		// If JSON parsing fails, return empty strings (graceful degradation)
		return "", ""
	}
	
	// Extract Category (as integer) and convert to string
	if categoryVal, exists := context["Category"]; exists {
		if categoryInt, ok := categoryVal.(float64); ok { // JSON numbers are float64
			category = categoryToString(int(categoryInt))
		}
	}
	
	// Extract ToolName (as string)
	if toolNameVal, exists := context["ToolName"]; exists {
		if toolNameStr, ok := toolNameVal.(string); ok {
			toolName = toolNameStr
		}
	}
	
	return category, toolName
}

// categoryToString converts category integer to string representation
func categoryToString(categoryInt int) string {
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

// ToolUsageStats represents tool-specific usage statistics.
type ToolUsageStats struct {
	ToolName   string   `json:"tool_name"`
	UsageCount int      `json:"usage_count"`
	LastUsed   int64    `json:"last_used"`
	Categories []string `json:"categories,omitempty"` // Categories this tool uses
}

// CategoryDistribution represents category usage statistics
type CategoryDistribution struct {
	Category   string  `json:"category"`
	Count      int     `json:"count"`
	Percentage float64 `json:"percentage"`
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
			(SELECT context FROM hook_events he2 WHERE he2.selected_path = he.selected_path LIMIT 1) as context
		FROM hook_events he
		WHERE he.selected_path != ''`

	// Apply filters using common QueryFilter
	whereClause, args := filter.BuildWhereClause()
	if whereClause != "" {
		baseQuery += " AND " + whereClause
	}

	baseQuery += `
		GROUP BY he.selected_path
		ORDER BY play_count DESC`

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
		var contextStr sql.NullString

		err := rows.Scan(&usage.Path, &usage.PlayCount, &usage.LastPlayed, &contextStr)
		if err != nil {
			return nil, fmt.Errorf("failed to scan sound usage row: %w", err)
		}

		// Extract category and tool name from context JSON
		if contextStr.Valid && contextStr.String != "" {
			category, toolName := extractFromContextJSON(contextStr.String)
			usage.Category = category
			usage.ToolName = toolName
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
	whereClause, args := filter.BuildWhereClause()
	if whereClause != "" {
		summaryQuery += " AND " + whereClause
	}

	var summary UsageSummary
	err := db.QueryRow(summaryQuery, args...).Scan(&summary.TotalEvents, &summary.UniqueSounds)
	if err != nil {
		return nil, fmt.Errorf("failed to query usage summary: %w", err)
	}

	return &summary, nil
}

// GetToolUsageStats returns tool-specific usage statistics
func GetToolUsageStats(db *sql.DB, filter QueryFilter) ([]ToolUsageStats, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection is nil")
	}

	// Build query to get tool usage statistics
	baseQuery := `
		SELECT
			JSON_EXTRACT(he.context, '$.ToolName') as tool_name,
			COUNT(*) as usage_count,
			MAX(he.timestamp) as last_used,
			GROUP_CONCAT(DISTINCT JSON_EXTRACT(he.context, '$.Category')) as categories
		FROM hook_events he
		WHERE he.context != '' AND JSON_EXTRACT(he.context, '$.ToolName') IS NOT NULL`

	// Apply filters using common QueryFilter
	whereClause, args := filter.BuildWhereClause()
	if whereClause != "" {
		baseQuery += " AND " + whereClause
	}

	baseQuery += `
		GROUP BY JSON_EXTRACT(he.context, '$.ToolName')
		ORDER BY usage_count DESC`

	// Apply limit
	if filter.Limit > 0 {
		baseQuery += fmt.Sprintf(" LIMIT %d", filter.Limit)
	}

	rows, err := db.Query(baseQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query tool usage stats: %w", err)
	}
	defer rows.Close()

	var results []ToolUsageStats
	for rows.Next() {
		var stats ToolUsageStats
		var toolName, categoriesStr sql.NullString

		err := rows.Scan(&toolName, &stats.UsageCount, &stats.LastUsed, &categoriesStr)
		if err != nil {
			return nil, fmt.Errorf("failed to scan tool usage stats row: %w", err)
		}

		if toolName.Valid {
			stats.ToolName = toolName.String
		}

		// Parse categories (comma-separated category integers)
		if categoriesStr.Valid && categoriesStr.String != "" {
			categoryMap := make(map[string]bool)
			for _, categoryStr := range parseCommaSeparated(categoriesStr.String) {
				// Convert category integer strings to category names
				if categoryStr != "" && categoryStr != "null" {
					// Parse the integer string to int
					var categoryInt int
					_, err := fmt.Sscanf(categoryStr, "%d", &categoryInt)
					if err == nil {
						categoryName := categoryToString(categoryInt)
						if categoryName != "unknown" {
							categoryMap[categoryName] = true
						}
					}
				}
			}

			// Convert map to slice
			for category := range categoryMap {
				stats.Categories = append(stats.Categories, category)
			}
		}

		results = append(results, stats)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating tool usage stats rows: %w", err)
	}

	return results, nil
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

	whereClause, args := filter.BuildWhereClause()
	if whereClause != "" {
		baseQuery += " AND " + whereClause
	}

	baseQuery += `
		GROUP BY COALESCE(he.chain_type, '')
		ORDER BY event_count DESC`

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

// GetCategoryDistribution returns category usage distribution
func GetCategoryDistribution(db *sql.DB, filter QueryFilter) ([]CategoryDistribution, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection is nil")
	}

	// Build query to get category distribution
	baseQuery := `
		SELECT 
			JSON_EXTRACT(he.context, '$.Category') as category_int,
			COUNT(*) as count
		FROM hook_events he
		WHERE he.context != '' AND JSON_EXTRACT(he.context, '$.Category') IS NOT NULL`

	// Apply filters using common QueryFilter
	whereClause, args := filter.BuildWhereClause()
	if whereClause != "" {
		baseQuery += " AND " + whereClause
	}

	baseQuery += `
		GROUP BY JSON_EXTRACT(he.context, '$.Category')
		ORDER BY count DESC`

	rows, err := db.Query(baseQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query category distribution: %w", err)
	}
	defer rows.Close()

	var results []CategoryDistribution
	var totalCount int

	// First pass: collect data and calculate total
	for rows.Next() {
		var categoryInt sql.NullFloat64
		var count int

		if err := rows.Scan(&categoryInt, &count); err != nil {
			return nil, fmt.Errorf("failed to scan category distribution row: %w", err)
		}

		if categoryInt.Valid {
			categoryName := categoryToString(int(categoryInt.Float64))
			results = append(results, CategoryDistribution{
				Category: categoryName,
				Count:    count,
			})
			totalCount += count
		}
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating category distribution rows: %w", err)
	}

	// Second pass: calculate percentages
	for i := range results {
		if totalCount > 0 {
			results[i].Percentage = float64(results[i].Count) / float64(totalCount) * 100.0
		}
	}

	return results, nil
}

