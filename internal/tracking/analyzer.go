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

// SoundUsage represents actual sound playback statistics
type SoundUsage struct {
	Path          string  `json:"path"`
	PlayCount     int     `json:"play_count"`
	FallbackLevel int     `json:"fallback_level"`
	Category      string  `json:"category,omitempty"`
	ToolName      string  `json:"tool_name,omitempty"`
	LastPlayed    int64   `json:"last_played"`           // Unix timestamp
	AvgFallback   float64 `json:"avg_fallback"`          // Average fallback level
}

// UsageSummary provides overall usage statistics
type UsageSummary struct {
	TotalEvents          int            `json:"total_events"`
	UniqueSounds         int            `json:"unique_sounds"`
	AvgFallbackLevel     float64        `json:"avg_fallback_level"`
	FallbackDistribution map[int]int    `json:"fallback_distribution"` // Level -> count
	TimeRange            string         `json:"time_range,omitempty"`  // Human readable
}

// ToolUsageStats represents tool-specific usage statistics
type ToolUsageStats struct {
	ToolName         string  `json:"tool_name"`
	UsageCount       int     `json:"usage_count"`
	AvgFallbackLevel float64 `json:"avg_fallback_level"`
	LastUsed         int64   `json:"last_used"`
	Categories       []string `json:"categories,omitempty"` // Categories this tool uses
}

// CategoryDistribution represents category usage statistics
type CategoryDistribution struct {
	Category   string  `json:"category"`
	Count      int     `json:"count"`
	Percentage float64 `json:"percentage"`
}

// FallbackStatistic represents fallback level statistics
type FallbackStatistic struct {
	FallbackLevel int     `json:"fallback_level"`
	Count         int     `json:"count"`
	Percentage    float64 `json:"percentage"`
	Description   string  `json:"description"`
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
			AVG(he.fallback_level) as avg_fallback,
			MIN(he.fallback_level) as min_fallback, 
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
		var avgFallback float64

		err := rows.Scan(&usage.Path, &usage.PlayCount, &avgFallback, &usage.FallbackLevel, &usage.LastPlayed, &contextStr)
		if err != nil {
			return nil, fmt.Errorf("failed to scan sound usage row: %w", err)
		}

		usage.AvgFallback = avgFallback

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
			COUNT(DISTINCT he.selected_path) as unique_sounds,
			AVG(he.fallback_level) as avg_fallback_level
		FROM hook_events he
		WHERE he.selected_path != ''`

	// Apply filters using common QueryFilter
	whereClause, args := filter.BuildWhereClause()
	if whereClause != "" {
		summaryQuery += " AND " + whereClause
	}

	var summary UsageSummary
	err := db.QueryRow(summaryQuery, args...).Scan(&summary.TotalEvents, &summary.UniqueSounds, &summary.AvgFallbackLevel)
	if err != nil {
		return nil, fmt.Errorf("failed to query usage summary: %w", err)
	}

	// Get fallback distribution
	distributionQuery := `
		SELECT 
			he.fallback_level,
			COUNT(*) as count
		FROM hook_events he
		WHERE he.selected_path != ''`

	if whereClause != "" {
		distributionQuery += " AND " + whereClause
	}

	distributionQuery += `
		GROUP BY he.fallback_level
		ORDER BY he.fallback_level`

	rows, err := db.Query(distributionQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query fallback distribution: %w", err)
	}
	defer rows.Close()

	summary.FallbackDistribution = make(map[int]int)
	for rows.Next() {
		var level, count int
		if err := rows.Scan(&level, &count); err != nil {
			return nil, fmt.Errorf("failed to scan fallback distribution: %w", err)
		}
		summary.FallbackDistribution[level] = count
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating fallback distribution rows: %w", err)
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
			AVG(he.fallback_level) as avg_fallback_level,
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

		err := rows.Scan(&toolName, &stats.UsageCount, &stats.AvgFallbackLevel, &stats.LastUsed, &categoriesStr)
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

// GetFallbackStatistics returns fallback level statistics
func GetFallbackStatistics(db *sql.DB, filter QueryFilter) ([]FallbackStatistic, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection is nil")
	}

	// Build query to get fallback statistics
	baseQuery := `
		SELECT 
			he.fallback_level,
			COUNT(*) as count
		FROM hook_events he
		WHERE he.selected_path != ''`

	// Apply filters using common QueryFilter
	whereClause, args := filter.BuildWhereClause()
	if whereClause != "" {
		baseQuery += " AND " + whereClause
	}

	baseQuery += `
		GROUP BY he.fallback_level
		ORDER BY he.fallback_level`

	rows, err := db.Query(baseQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query fallback statistics: %w", err)
	}
	defer rows.Close()

	var results []FallbackStatistic
	var totalCount int

	// First pass: collect data and calculate total
	for rows.Next() {
		var level, count int

		if err := rows.Scan(&level, &count); err != nil {
			return nil, fmt.Errorf("failed to scan fallback statistics row: %w", err)
		}

		results = append(results, FallbackStatistic{
			FallbackLevel: level,
			Count:         count,
			Description:   getFallbackDescription(level),
		})
		totalCount += count
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating fallback statistics rows: %w", err)
	}

	// Second pass: calculate percentages
	for i := range results {
		if totalCount > 0 {
			results[i].Percentage = float64(results[i].Count) / float64(totalCount) * 100.0
		}
	}

	return results, nil
}

// getFallbackDescription returns human-readable description for fallback levels
func getFallbackDescription(level int) string {
	switch level {
	case 1:
		return "Exact hint match"
	case 2:
		return "Tool-specific sound"
	case 3:
		return "Operation-specific sound"
	case 4:
		return "Category-specific sound"
	case 5:
		return "Default fallback sound"
	default:
		return "Unknown fallback level"
	}
}