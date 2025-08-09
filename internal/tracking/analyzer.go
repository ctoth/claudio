package tracking

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// MissingSound represents a sound that was requested but not found
type MissingSound struct {
	Path         string   `json:"path"`
	RequestCount int      `json:"request_count"`
	Tools        []string `json:"tools,omitempty"` // Which tools requested this sound
	Category     string   `json:"category,omitempty"` // Category from context JSON (loading, success, error, etc.)
	ToolName     string   `json:"tool_name,omitempty"` // Tool name from context JSON
}

// MissingSoundQuery holds parameters for querying missing sounds
type MissingSoundQuery struct {
	Days  int    // Number of days to look back (0 = all time)
	Tool  string // Filter by specific tool (empty = all tools)
	Limit int    // Maximum number of results (0 = no limit)
}

// GetMissingSounds queries the database for sounds that were requested but not found
func GetMissingSounds(db *sql.DB, query MissingSoundQuery) ([]MissingSound, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection is nil")
	}

	// Build the SQL query with optional filters
	baseQuery := `
		SELECT 
			pl.path,
			COUNT(*) as request_count,
			GROUP_CONCAT(DISTINCT he.tool_name) as tools,
			he.context
		FROM path_lookups pl
		JOIN hook_events he ON pl.event_id = he.id
		WHERE pl.found = 0`

	args := []interface{}{}

	// Add time filter if specified
	if query.Days > 0 {
		cutoff := time.Now().Unix() - int64(query.Days*24*60*60)
		baseQuery += " AND he.timestamp >= ?"
		args = append(args, cutoff)
	}

	// Add tool filter if specified
	if query.Tool != "" {
		baseQuery += " AND he.tool_name = ?"
		args = append(args, query.Tool)
	}

	// Group by path and order by frequency
	// We group by path and take any context (since all requests for same path should have similar context)
	baseQuery += `
		GROUP BY pl.path
		ORDER BY request_count DESC`

	// Add limit if specified  
	if query.Limit > 0 {
		baseQuery += fmt.Sprintf(" LIMIT %d", query.Limit)
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
func GetMissingSoundsSummary(db *sql.DB, query MissingSoundQuery) (map[string]interface{}, error) {
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

	args := []interface{}{}

	// Add time filter if specified
	if query.Days > 0 {
		cutoff := time.Now().Unix() - int64(query.Days*24*60*60)
		summaryQuery += " AND he.timestamp >= ?"
		args = append(args, cutoff)
	}

	// Add tool filter if specified
	if query.Tool != "" {
		summaryQuery += " AND he.tool_name = ?"
		args = append(args, query.Tool)
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
		"query_days":                 query.Days,
		"query_tool_filter":          query.Tool,
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