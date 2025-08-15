package cli

import (
	"database/sql"
	"fmt"
	"io"
	"log/slog"

	"claudio.click/internal/tracking"
	"github.com/spf13/cobra"
)

// TDD Step 3 GREEN: Data structures for tool-first grouping moved from test file

// ToolGroup represents a tool with its missing sounds grouped by category
type ToolGroup struct {
	Name       string          `json:"name"`
	Total      int             `json:"total"`       // Total requests across all categories
	Count      int             `json:"count"`       // Total missing sounds count
	Categories []CategoryGroup `json:"categories"`
}

// CategoryGroup represents a category of missing sounds within a tool
type CategoryGroup struct {
	Name   string                     `json:"name"`
	Total  int                        `json:"total"`  // Total requests for this category
	Count  int                        `json:"count"`  // Number of missing sounds
	Sounds []tracking.MissingSound    `json:"sounds"`
}

// Analysis represents the complete analysis of missing sounds grouped by tool
type Analysis struct {
	Tools []ToolGroup     `json:"tools"`  // Tool-specific missing sounds
	Other []CategoryGroup `json:"other"`  // Non-tool-specific missing sounds (interactive, system, etc.)
}

// newAnalyzeCommand creates the analyze command with subcommands
func newAnalyzeCommand() *cobra.Command {
	analyzeCmd := &cobra.Command{
		Use:   "analyze",
		Short: "Analyze sound tracking data",
		Long:  "Analyze sound tracking data to understand usage patterns and missing sounds",
	}

	// Add missing subcommand
	analyzeCmd.AddCommand(newAnalyzeMissingCommand())
	
	// Add usage subcommand
	analyzeCmd.AddCommand(newAnalyzeUsageCommand())

	return analyzeCmd
}

// newAnalyzeMissingCommand creates the analyze missing subcommand
func newAnalyzeMissingCommand() *cobra.Command {
	var days int
	var tool string
	var category string
	var limit int
	var preset string

	missingCmd := &cobra.Command{
		Use:   "missing",
		Short: "Show missing sounds that were requested but not found",
		Long: `Show missing sounds that were requested but not found.

This command analyzes the sound tracking database to identify which sound files
were requested but didn't exist in your soundpack. This helps you understand
what sounds you could create to improve your audio experience.

The results are ordered by frequency (most requested first) to help you
prioritize which sounds to create.

Examples:
  claudio analyze missing                    # Show recent missing sounds
  claudio analyze missing --days 30         # Last 30 days
  claudio analyze missing --preset today    # Today only
  claudio analyze missing --tool Edit       # Edit tool only
  claudio analyze missing --category error  # Error sounds only`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAnalyzeMissing(cmd, days, tool, category, limit, preset)
		},
	}

	// Add flags - now consistent with analyze usage
	missingCmd.Flags().IntVar(&days, "days", 7, "Number of days to analyze (0 = all time)")
	missingCmd.Flags().StringVar(&tool, "tool", "", "Filter by specific tool name")
	missingCmd.Flags().StringVar(&category, "category", "", "Filter by category (success, error, loading, interactive)")
	missingCmd.Flags().IntVar(&limit, "limit", 20, "Maximum number of results to show")
	missingCmd.Flags().StringVar(&preset, "preset", "", "Date preset (today, yesterday, last-week, this-month, all-time)")

	return missingCmd
}

// runAnalyzeMissing executes the analyze missing command
func runAnalyzeMissing(cmd *cobra.Command, days int, tool, category string, limit int, preset string) error {
	slog.Debug("running analyze missing command", "days", days, "tool", tool, "category", category, "limit", limit, "preset", preset)

	// Extract CLI instance from context
	cli := cliFromContext(cmd.Context())
	if cli == nil {
		return fmt.Errorf("CLI instance not found in context")
	}

	// Ensure tracking database is initialized
	cli.initializeTracking()

	// Check if tracking database is available
	if cli.trackingDB == nil {
		return fmt.Errorf("sound tracking is not enabled or database is not available")
	}

	// Build query filter using new common infrastructure
	filter := tracking.QueryFilter{
		Days:      days,
		Tool:      tool,
		Category:  category,
		Limit:     limit,
		DatePreset: preset,
		OrderBy:   "frequency",
		OrderDesc: true,
	}

	// Get missing sounds data
	missingSounds, err := tracking.GetMissingSounds(cli.trackingDB, filter)
	if err != nil {
		slog.Error("failed to get missing sounds", "error", err)
		return fmt.Errorf("failed to analyze missing sounds: %w", err)
	}

	// Get summary statistics
	summary, err := tracking.GetMissingSoundsSummary(cli.trackingDB, filter)
	if err != nil {
		slog.Warn("failed to get missing sounds summary", "error", err)
		// Continue without summary - not critical
	}

	// TDD Step 3 GREEN: Replace flat output with hierarchical tool-grouped output
	return outputMissingSoundsHierarchical(cmd.OutOrStdout(), missingSounds, summary, filter)
}

// TDD Step 3 GREEN: groupByTool groups missing sounds by tool and category
func groupByTool(missingSounds []tracking.MissingSound) Analysis {
	toolMap := make(map[string]map[string][]tracking.MissingSound) // tool -> category -> sounds
	otherMap := make(map[string][]tracking.MissingSound)           // category -> sounds (for non-tool sounds)
	
	// Group sounds by tool and category
	for _, sound := range missingSounds {
		if sound.ToolName != "" {
			// Tool-specific sound
			if toolMap[sound.ToolName] == nil {
				toolMap[sound.ToolName] = make(map[string][]tracking.MissingSound)
			}
			toolMap[sound.ToolName][sound.Category] = append(toolMap[sound.ToolName][sound.Category], sound)
		} else {
			// Non-tool sound (goes to Other section)
			otherMap[sound.Category] = append(otherMap[sound.Category], sound)
		}
	}
	
	// Build tool groups
	var tools []ToolGroup
	for toolName, categoryMap := range toolMap {
		var categories []CategoryGroup
		toolTotal := 0
		toolCount := 0
		
		for categoryName, sounds := range categoryMap {
			categoryTotal := 0
			for _, sound := range sounds {
				categoryTotal += sound.RequestCount
			}
			
			categories = append(categories, CategoryGroup{
				Name:   categoryName,
				Total:  categoryTotal,
				Count:  len(sounds),
				Sounds: sounds,
			})
			
			toolTotal += categoryTotal
			toolCount += len(sounds)
		}
		
		tools = append(tools, ToolGroup{
			Name:       toolName,
			Total:      toolTotal,
			Count:      toolCount,
			Categories: categories,
		})
	}
	
	// Build other groups
	var other []CategoryGroup
	for categoryName, sounds := range otherMap {
		categoryTotal := 0
		for _, sound := range sounds {
			categoryTotal += sound.RequestCount
		}
		
		other = append(other, CategoryGroup{
			Name:   categoryName,
			Total:  categoryTotal,
			Count:  len(sounds),
			Sounds: sounds,
		})
	}
	
	// Sort tools by total requests (descending)
	for i := 0; i < len(tools); i++ {
		for j := i + 1; j < len(tools); j++ {
			if tools[j].Total > tools[i].Total {
				tools[i], tools[j] = tools[j], tools[i]
			}
		}
	}
	
	return Analysis{
		Tools: tools,
		Other: other,
	}
}

// TDD Step 3 GREEN: outputMissingSoundsHierarchical displays missing sounds grouped by tool
func outputMissingSoundsHierarchical(w io.Writer, sounds []tracking.MissingSound, summary map[string]interface{}, filter tracking.QueryFilter) error {
	if len(sounds) == 0 {
		// No missing sounds found
		if filter.Days > 0 {
			fmt.Fprintf(w, "No missing sounds found in the last %d days", filter.Days)
		} else {
			fmt.Fprint(w, "No missing sounds found")
		}
		if filter.Tool != "" {
			fmt.Fprintf(w, " for tool '%s'", filter.Tool)
		}
		fmt.Fprintln(w, ".")
		fmt.Fprintln(w, "\nThis means either:")
		fmt.Fprintln(w, "  • Your soundpack has excellent coverage")
		fmt.Fprintln(w, "  • Sound tracking hasn't been running long enough to collect data")
		fmt.Fprintln(w, "  • No tools have been used that would generate missing sounds")
		return nil
	}

	// Group sounds by tool
	analysis := groupByTool(sounds)

	// Header with hierarchical context
	var timeContext string
	if filter.DatePreset != "" {
		timeContext = filter.DatePreset
	} else if filter.Days > 0 {
		timeContext = fmt.Sprintf("last %d days", filter.Days)
	} else {
		timeContext = "all time"
	}

	fmt.Fprintf(w, "Missing Sounds by Tool (%s):\n\n", timeContext)

	// Summary statistics if available
	if summary != nil {
		if uniqueCount, ok := summary["unique_missing_sounds"].(int); ok && uniqueCount > 0 {
			totalRequests := summary["total_missing_requests"].(int)
			fmt.Fprintf(w, "Found %d unique missing sounds with %d total requests\n", uniqueCount, totalRequests)
			
			if toolCount, ok := summary["tools_with_missing_sounds"].(int); ok && toolCount > 0 {
				fmt.Fprintf(w, "Across %d different tools\n", toolCount)
			}
			fmt.Fprintln(w)
		}
	}

	// Display tools grouped hierarchically with improved formatting
	for _, tool := range analysis.Tools {
		// Handle edge case: skip tools with no sounds (shouldn't happen, but defensive)
		if tool.Count == 0 {
			continue
		}
		
		fmt.Fprintf(w, "%s (total: %d requests, %d sounds):\n", tool.Name, tool.Total, tool.Count)
		
		// Sort categories for consistent output (success, error, loading, etc.)
		sortedCategories := sortCategories(tool.Categories)
		
		for _, category := range sortedCategories {
			fmt.Fprintf(w, "  %s (%d requests):\n", category.Name, category.Total)
			
			// Sort sounds by request count (descending)
			sortedSounds := sortSoundsByRequestCount(category.Sounds)
			
			for _, sound := range sortedSounds {
				// Handle edge case: truncate very long paths for better formatting
				displayPath := sound.Path
				if len(displayPath) > 35 {
					displayPath = "..." + displayPath[len(displayPath)-32:]
				}
				
				// Better alignment: path padded to 35 chars, right-aligned request count
				fmt.Fprintf(w, "    %-35s %3d requests\n", displayPath, sound.RequestCount)
			}
			
			if len(sortedCategories) > 1 {
				fmt.Fprintln(w) // Space between categories only if multiple categories
			}
		}
		fmt.Fprintln(w) // Space between tools
	}

	// Display Other section if present with consistent formatting
	if len(analysis.Other) > 0 {
		fmt.Fprintln(w, "Other (non-tool sounds):")
		
		sortedOtherCategories := sortCategories(analysis.Other)
		
		for _, category := range sortedOtherCategories {
			fmt.Fprintf(w, "  %s (%d requests):\n", category.Name, category.Total)
			
			sortedOtherSounds := sortSoundsByRequestCount(category.Sounds)
			
			for _, sound := range sortedOtherSounds {
				// Handle edge case: truncate very long paths for better formatting
				displayPath := sound.Path
				if len(displayPath) > 35 {
					displayPath = "..." + displayPath[len(displayPath)-32:]
				}
				
				fmt.Fprintf(w, "    %-35s %3d requests\n", displayPath, sound.RequestCount)
			}
			
			if len(sortedOtherCategories) > 1 {
				fmt.Fprintln(w) // Space between categories only if multiple
			}
		}
		fmt.Fprintln(w) // Space after Other section
	}

	// Footer with actionable advice
	fmt.Fprintln(w, "To improve your sound experience:")
	fmt.Fprintln(w, "  1. Create the most frequently requested sounds first")
	fmt.Fprintln(w, "  2. Add them to your soundpack directory")
	fmt.Fprintln(w, "  3. Use the exact file names shown above")

	if filter.Tool == "" && len(sounds) > 3 {
		fmt.Fprintln(w, "  4. Use --tool <name> to focus on specific tools")
	}

	return nil
}

// TDD Step 3 REFACTOR: Helper functions for consistent sorting and formatting

// sortCategories sorts categories in a logical order for display
func sortCategories(categories []CategoryGroup) []CategoryGroup {
	// Create a copy to avoid modifying original
	sorted := make([]CategoryGroup, len(categories))
	copy(sorted, categories)
	
	// Define preferred order: success, error, loading, interactive, completion, system, others
	categoryOrder := map[string]int{
		"success":     1,
		"error":       2, 
		"loading":     3,
		"interactive": 4,
		"completion":  5,
		"system":      6,
	}
	
	// Sort by preferred order, then by total requests (descending), then by name
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			orderI := categoryOrder[sorted[i].Name]
			orderJ := categoryOrder[sorted[j].Name]
			
			// If both have defined order, use it
			if orderI > 0 && orderJ > 0 {
				if orderI > orderJ {
					sorted[i], sorted[j] = sorted[j], sorted[i]
				}
			} else if orderI > 0 && orderJ == 0 {
				// I has order, J doesn't - I comes first
				continue
			} else if orderI == 0 && orderJ > 0 {
				// J has order, I doesn't - swap
				sorted[i], sorted[j] = sorted[j], sorted[i]
			} else {
				// Neither has defined order - sort by total requests (desc), then by name
				if sorted[j].Total > sorted[i].Total {
					sorted[i], sorted[j] = sorted[j], sorted[i]
				} else if sorted[j].Total == sorted[i].Total && sorted[j].Name < sorted[i].Name {
					sorted[i], sorted[j] = sorted[j], sorted[i]
				}
			}
		}
	}
	
	return sorted
}

// sortSoundsByRequestCount sorts sounds by request count (descending), then by path
func sortSoundsByRequestCount(sounds []tracking.MissingSound) []tracking.MissingSound {
	// Create a copy to avoid modifying original
	sorted := make([]tracking.MissingSound, len(sounds))
	copy(sorted, sounds)
	
	// Sort by request count (descending), then by path (ascending) for tie-breaking
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].RequestCount > sorted[i].RequestCount {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			} else if sorted[j].RequestCount == sorted[i].RequestCount && sorted[j].Path < sorted[i].Path {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	
	return sorted
}

// outputMissingSounds formats and displays the missing sounds analysis
func outputMissingSounds(w io.Writer, sounds []tracking.MissingSound, summary map[string]interface{}, filter tracking.QueryFilter) error {
	if len(sounds) == 0 {
		// No missing sounds found
		if filter.Days > 0 {
			fmt.Fprintf(w, "No missing sounds found in the last %d days", filter.Days)
		} else {
			fmt.Fprint(w, "No missing sounds found")
		}
		if filter.Tool != "" {
			fmt.Fprintf(w, " for tool '%s'", filter.Tool)
		}
		fmt.Fprintln(w, ".")
		fmt.Fprintln(w, "\nThis means either:")
		fmt.Fprintln(w, "  • Your soundpack has excellent coverage")
		fmt.Fprintln(w, "  • Sound tracking hasn't been running long enough to collect data")
		fmt.Fprintln(w, "  • No tools have been used that would generate missing sounds")
		return nil
	}

	// Header with context
	var timeContext string
	if filter.DatePreset != "" {
		timeContext = filter.DatePreset
	} else if filter.Days > 0 {
		timeContext = fmt.Sprintf("last %d days", filter.Days)
	} else {
		timeContext = "all time"
	}

	fmt.Fprintf(w, "Missing Sounds (%s):\n\n", timeContext)

	// Summary statistics if available
	if summary != nil {
		if uniqueCount, ok := summary["unique_missing_sounds"].(int); ok && uniqueCount > 0 {
			totalRequests := summary["total_missing_requests"].(int)
			fmt.Fprintf(w, "Found %d unique missing sounds with %d total requests\n", uniqueCount, totalRequests)
			
			if toolCount, ok := summary["tools_with_missing_sounds"].(int); ok && toolCount > 0 {
				fmt.Fprintf(w, "Across %d different tools\n", toolCount)
			}
			fmt.Fprintln(w)
		}
	}

	// List missing sounds by frequency
	fmt.Fprintln(w, "Most Requested:")
	for i, sound := range sounds {
		// Format: "  success/edit-success.wav    12 requests"
		fmt.Fprintf(w, "  %-30s %d requests", sound.Path, sound.RequestCount)
		
		// Add tool information if available and not filtering by tool
		if len(sound.Tools) > 0 && filter.Tool == "" {
			fmt.Fprintf(w, " (%s)", formatTools(sound.Tools))
		}
		fmt.Fprintln(w)

		// Add spacing after every 5 items for readability
		if i > 0 && i%5 == 4 && i < len(sounds)-1 {
			fmt.Fprintln(w)
		}
	}

	// Footer with actionable advice
	fmt.Fprintln(w, "\nTo improve your sound experience:")
	fmt.Fprintln(w, "  1. Create the most frequently requested sounds first")
	fmt.Fprintln(w, "  2. Add them to your soundpack directory")
	fmt.Fprintln(w, "  3. Use the exact file names shown above")

	if filter.Tool == "" && len(sounds) > 3 {
		fmt.Fprintln(w, "  4. Use --tool <name> to focus on specific tools")
	}

	return nil
}

// formatTools formats a list of tool names for display
func formatTools(tools []string) string {
	if len(tools) == 0 {
		return ""
	}
	if len(tools) == 1 {
		return tools[0]
	}
	if len(tools) == 2 {
		return tools[0] + ", " + tools[1]
	}
	// For 3+ tools, show first two and count
	return fmt.Sprintf("%s, %s + %d more", tools[0], tools[1], len(tools)-2)
}

// TDD RED: New analyze usage command implementation

// newAnalyzeUsageCommand creates the analyze usage subcommand
func newAnalyzeUsageCommand() *cobra.Command {
	var days int
	var tool string
	var category string
	var limit int
	var preset string
	var showFallbacks bool
	var showSummary bool

	usageCmd := &cobra.Command{
		Use:   "usage",
		Short: "Show actual sound usage patterns and statistics",
		Long: `Show actual sound usage patterns and statistics from the tracking database.

This command analyzes which sounds were actually played, how often they were used,
and what fallback levels were reached. This helps you understand your soundpack
effectiveness and identify optimization opportunities.

The results show:
- Most frequently played sounds
- Fallback level statistics (lower levels = better sound coverage)  
- Tool usage patterns
- Category distribution

Examples:
  claudio analyze usage                    # Show recent usage
  claudio analyze usage --days 30         # Last 30 days
  claudio analyze usage --preset today    # Today only
  claudio analyze usage --tool Edit       # Edit tool only
  claudio analyze usage --category success # Success sounds only
  claudio analyze usage --show-fallbacks  # Include fallback statistics
  claudio analyze usage --show-summary    # Show summary statistics`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAnalyzeUsage(cmd, days, tool, category, limit, preset, showFallbacks, showSummary)
		},
	}

	// Add flags
	usageCmd.Flags().IntVar(&days, "days", 7, "Number of days to analyze (0 = all time)")
	usageCmd.Flags().StringVar(&tool, "tool", "", "Filter by specific tool name")
	usageCmd.Flags().StringVar(&category, "category", "", "Filter by category (success, error, loading, interactive)")
	usageCmd.Flags().IntVar(&limit, "limit", 20, "Maximum number of results to show")
	usageCmd.Flags().StringVar(&preset, "preset", "", "Date preset (today, yesterday, last-week, this-month, all-time)")
	usageCmd.Flags().BoolVar(&showFallbacks, "show-fallbacks", false, "Show fallback level statistics")
	usageCmd.Flags().BoolVar(&showSummary, "show-summary", false, "Show usage summary statistics")

	return usageCmd
}

// runAnalyzeUsage executes the analyze usage command
func runAnalyzeUsage(cmd *cobra.Command, days int, tool, category string, limit int, preset string, showFallbacks, showSummary bool) error {
	slog.Debug("running analyze usage command", "days", days, "tool", tool, "category", category, "limit", limit, "preset", preset)

	// Extract CLI instance from context
	cli := cliFromContext(cmd.Context())
	if cli == nil {
		return fmt.Errorf("CLI instance not found in context")
	}

	// Ensure tracking database is initialized
	cli.initializeTracking()

	// Check if tracking database is available
	if cli.trackingDB == nil {
		fmt.Fprintln(cmd.OutOrStdout(), "Sound tracking is not enabled or database not available.")
		fmt.Fprintln(cmd.OutOrStdout(), "Enable tracking with CLAUDIO_SOUND_TRACKING=true")
		return nil
	}

	// Build query filter
	filter := tracking.QueryFilter{
		Days:      days,
		Tool:      tool,
		Category:  category,
		Limit:     limit,
		DatePreset: preset,
		OrderBy:   "frequency",
		OrderDesc: true,
	}

	// Get sound usage statistics
	usage, err := tracking.GetSoundUsage(cli.trackingDB, filter)
	if err != nil {
		return fmt.Errorf("failed to get sound usage: %w", err)
	}

	// Output results
	if err := outputUsageStatistics(cmd.OutOrStdout(), usage, filter, showFallbacks, showSummary, cli.trackingDB); err != nil {
		return fmt.Errorf("failed to output usage statistics: %w", err)
	}

	return nil
}

// outputUsageStatistics formats and outputs usage statistics
func outputUsageStatistics(w io.Writer, usage []tracking.SoundUsage, filter tracking.QueryFilter, showFallbacks, showSummary bool, db interface{}) error {
	if len(usage) == 0 {
		fmt.Fprintln(w, "No sound usage data found for the specified criteria.")
		
		// Provide helpful suggestions
		if filter.Days > 0 {
			fmt.Fprintf(w, "Try expanding the time range with --days 0 (all time) or --preset all-time\n")
		}
		if filter.Tool != "" {
			fmt.Fprintf(w, "Try removing the --tool filter to see all tools\n")
		}
		if filter.Category != "" {
			fmt.Fprintf(w, "Try removing the --category filter to see all categories\n")
		}
		
		return nil
	}

	// Show header with filter info
	fmt.Fprintln(w, "Sound Usage Statistics")
	fmt.Fprintln(w, "=====================")
	
	// Show filter details
	if filter.DatePreset != "" {
		fmt.Fprintf(w, "Time Range: %s\n", filter.DatePreset)
	} else if filter.Days > 0 {
		fmt.Fprintf(w, "Time Range: Last %d days\n", filter.Days)
	} else {
		fmt.Fprintln(w, "Time Range: All time")
	}
	
	if filter.Tool != "" {
		fmt.Fprintf(w, "Tool Filter: %s\n", filter.Tool)
	}
	if filter.Category != "" {
		fmt.Fprintf(w, "Category Filter: %s\n", filter.Category)
	}
	fmt.Fprintln(w)

	// Show summary if requested
	if showSummary {
		if dbConn, ok := db.(*sql.DB); ok {
			summary, err := tracking.GetUsageSummary(dbConn, filter)
			if err == nil {
				fmt.Fprintf(w, "Summary: %d total events, %d unique sounds, avg fallback %.1f\n\n", 
					summary.TotalEvents, summary.UniqueSounds, summary.AvgFallbackLevel)
			}
		}
	}

	// Show most used sounds
	fmt.Fprintln(w, "Most Frequently Used Sounds:")
	fmt.Fprintln(w, "----------------------------")
	
	for i, sound := range usage {
		if i >= filter.Limit {
			break
		}

		// Format: rank. path (play_count times, fallback level X) - tool/category
		rank := i + 1
		fallbackDesc := getFallbackLevelDescription(sound.FallbackLevel)
		
		fmt.Fprintf(w, "%2d. %s (%d times, %s)",
			rank, sound.Path, sound.PlayCount, fallbackDesc)
		
		// Add tool/category info if available
		if sound.ToolName != "" || sound.Category != "" {
			fmt.Fprintf(w, " - ")
			if sound.ToolName != "" {
				fmt.Fprintf(w, "%s", sound.ToolName)
			}
			if sound.Category != "" {
				if sound.ToolName != "" {
					fmt.Fprintf(w, "/%s", sound.Category)
				} else {
					fmt.Fprintf(w, "%s", sound.Category)
				}
			}
		}
		fmt.Fprintln(w)
	}

	// Show fallback statistics if requested
	if showFallbacks {
		if dbConn, ok := db.(*sql.DB); ok {
			fmt.Fprintln(w, "\nFallback Level Statistics:")
			fmt.Fprintln(w, "--------------------------")
			
			fallbackStats, err := tracking.GetFallbackStatistics(dbConn, filter)
			if err == nil {
				for _, stat := range fallbackStats {
					fmt.Fprintf(w, "Level %d: %d events (%.1f%%) - %s\n",
						stat.FallbackLevel, stat.Count, stat.Percentage, stat.Description)
				}
			}
		}
	}

	// Footer with actionable advice
	fmt.Fprintln(w, "\nTo improve your sound coverage:")
	fmt.Fprintln(w, "  1. Focus on sounds with higher fallback levels")
	fmt.Fprintln(w, "  2. Create specific sounds for frequently used tools")
	fmt.Fprintln(w, "  3. Use --show-fallbacks to see detailed fallback statistics")
	
	if !showSummary {
		fmt.Fprintln(w, "  4. Use --show-summary to see overall statistics")
	}

	return nil
}

// getFallbackLevelDescription returns a short description for fallback levels
func getFallbackLevelDescription(level int) string {
	switch level {
	case 1:
		return "exact match"
	case 2:
		return "tool-specific"
	case 3:
		return "operation-specific" 
	case 4:
		return "category-specific"
	case 5:
		return "default fallback"
	default:
		return fmt.Sprintf("level %d", level)
	}
}