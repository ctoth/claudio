package cli

import (
	"fmt"
	"io"
	"log/slog"

	"github.com/ctoth/claudio/internal/tracking"
	"github.com/spf13/cobra"
)

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

	// Output results
	return outputMissingSounds(cmd.OutOrStdout(), missingSounds, summary, query)
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