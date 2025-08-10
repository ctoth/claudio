package cli

import (
	"fmt"
	"io"
	"log/slog"

	"github.com/ctoth/claudio/internal/tracking"
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

	return analyzeCmd
}

// newAnalyzeMissingCommand creates the analyze missing subcommand
func newAnalyzeMissingCommand() *cobra.Command {
	var days int
	var tool string
	var limit int

	missingCmd := &cobra.Command{
		Use:   "missing",
		Short: "Show missing sounds that were requested but not found",
		Long: `Show missing sounds that were requested but not found.

This command analyzes the sound tracking database to identify which sound files
were requested but didn't exist in your soundpack. This helps you understand
what sounds you could create to improve your audio experience.

The results are ordered by frequency (most requested first) to help you
prioritize which sounds to create.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAnalyzeMissing(cmd, days, tool, limit)
		},
	}

	// Add flags
	missingCmd.Flags().IntVar(&days, "days", 7, "Number of days to analyze (0 = all time)")
	missingCmd.Flags().StringVar(&tool, "tool", "", "Filter by specific tool name")
	missingCmd.Flags().IntVar(&limit, "limit", 20, "Maximum number of results to show")

	return missingCmd
}

// runAnalyzeMissing executes the analyze missing command
func runAnalyzeMissing(cmd *cobra.Command, days int, tool string, limit int) error {
	slog.Debug("running analyze missing command", "days", days, "tool", tool, "limit", limit)

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

	// Build query parameters
	query := tracking.MissingSoundQuery{
		Days:  days,
		Tool:  tool,
		Limit: limit,
	}

	// Get missing sounds data
	missingSounds, err := tracking.GetMissingSounds(cli.trackingDB, query)
	if err != nil {
		slog.Error("failed to get missing sounds", "error", err)
		return fmt.Errorf("failed to analyze missing sounds: %w", err)
	}

	// Get summary statistics
	summary, err := tracking.GetMissingSoundsSummary(cli.trackingDB, query)
	if err != nil {
		slog.Warn("failed to get missing sounds summary", "error", err)
		// Continue without summary - not critical
	}

	// TDD Step 3 GREEN: Replace flat output with hierarchical tool-grouped output
	return outputMissingSoundsHierarchical(cmd.OutOrStdout(), missingSounds, summary, query)
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
func outputMissingSoundsHierarchical(w io.Writer, sounds []tracking.MissingSound, summary map[string]interface{}, query tracking.MissingSoundQuery) error {
	if len(sounds) == 0 {
		// No missing sounds found
		if query.Days > 0 {
			fmt.Fprintf(w, "No missing sounds found in the last %d days", query.Days)
		} else {
			fmt.Fprint(w, "No missing sounds found")
		}
		if query.Tool != "" {
			fmt.Fprintf(w, " for tool '%s'", query.Tool)
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
	timeContext := fmt.Sprintf("last %d days", query.Days)
	if query.Days == 0 {
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

	if query.Tool == "" && len(sounds) > 3 {
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
func outputMissingSounds(w io.Writer, sounds []tracking.MissingSound, summary map[string]interface{}, query tracking.MissingSoundQuery) error {
	if len(sounds) == 0 {
		// No missing sounds found
		if query.Days > 0 {
			fmt.Fprintf(w, "No missing sounds found in the last %d days", query.Days)
		} else {
			fmt.Fprint(w, "No missing sounds found")
		}
		if query.Tool != "" {
			fmt.Fprintf(w, " for tool '%s'", query.Tool)
		}
		fmt.Fprintln(w, ".")
		fmt.Fprintln(w, "\nThis means either:")
		fmt.Fprintln(w, "  • Your soundpack has excellent coverage")
		fmt.Fprintln(w, "  • Sound tracking hasn't been running long enough to collect data")
		fmt.Fprintln(w, "  • No tools have been used that would generate missing sounds")
		return nil
	}

	// Header with context
	timeContext := fmt.Sprintf("last %d days", query.Days)
	if query.Days == 0 {
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
		if len(sound.Tools) > 0 && query.Tool == "" {
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

	if query.Tool == "" && len(sounds) > 3 {
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