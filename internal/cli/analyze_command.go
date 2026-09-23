package cli

import (
	"cmp"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"strings"
	"time"

	"claudio.click/internal/hooks"
	"claudio.click/internal/tracking"
	"github.com/spf13/cobra"
)

// TDD Step 3 GREEN: Data structures for tool-first grouping moved from test file

// ToolGroup represents a tool with its missing sounds grouped by category
type ToolGroup struct {
	Name       string          `json:"name"`
	Total      int             `json:"total"` // Total requests across all categories
	Count      int             `json:"count"` // Total missing sounds count
	Categories []CategoryGroup `json:"categories"`
}

// CategoryGroup represents a category of missing sounds within a tool
type CategoryGroup struct {
	Name   string                  `json:"name"`
	Total  int                     `json:"total"` // Total requests for this category
	Count  int                     `json:"count"` // Number of missing sounds
	Sounds []tracking.MissingSound `json:"sounds"`
}

// Analysis represents the complete analysis of missing sounds grouped by tool
type Analysis struct {
	Tools []ToolGroup     `json:"tools"` // Tool-specific missing sounds
	Other []CategoryGroup `json:"other"` // Non-tool-specific missing sounds (interactive, system, etc.)
}

// newAnalyzeCommand creates the analyze command with subcommands
func newAnalyzeCommand(c *CLI) *cobra.Command {
	analyzeCmd := &cobra.Command{
		Use:   "analyze",
		Short: "Analyze sound tracking data",
		Long:  "Analyze sound tracking data to understand usage patterns and missing sounds",
	}

	// Add missing subcommand
	analyzeCmd.AddCommand(newAnalyzeMissingCommand(c))

	// Add usage subcommand
	analyzeCmd.AddCommand(newAnalyzeUsageCommand(c))

	return analyzeCmd
}

// analyzeFilterFlags are the query flags both analyze subcommands take.
type analyzeFilterFlags struct {
	days     int
	tool     string
	category string
	limit    int
	preset   string
}

func (f *analyzeFilterFlags) register(cmd *cobra.Command) {
	cmd.Flags().IntVar(&f.days, "days", 7, "Number of days to analyze (0 = all time)")
	cmd.Flags().StringVar(&f.tool, "tool", "", "Filter by specific tool name")
	cmd.Flags().StringVar(&f.category, "category", "", "Filter by category ("+strings.Join(analyzeCategories, ", ")+")")
	cmd.Flags().IntVar(&f.limit, "limit", 20, "Maximum number of results to show")
	cmd.Flags().StringVar(&f.preset, "preset", "", "Date preset ("+strings.Join(tracking.DatePresets, ", ")+")")
}

// filter validates the flag values and returns the query they select,
// most frequent first.
func (f *analyzeFilterFlags) filter() (tracking.QueryFilter, error) {
	if err := validateAnalyzeFilterValues(f.category, f.preset); err != nil {
		return tracking.QueryFilter{}, err
	}
	return tracking.QueryFilter{
		Days:       f.days,
		Tool:       f.tool,
		Category:   f.category,
		Limit:      f.limit,
		DatePreset: f.preset,
		OrderBy:    "frequency",
		OrderDesc:  true,
	}, nil
}

// openAnalyzeDB validates the filter flags, loads the config (honoring
// --config, so an override reaches the tracking database path) and opens
// the tracking database. With tracking off it prints a hint and returns a
// nil db: there is nothing to analyze, which is not an error.
func (c *CLI) openAnalyzeDB(cmd *cobra.Command, flags *analyzeFilterFlags) (*sql.DB, tracking.QueryFilter, error) {
	filter, err := flags.filter()
	if err != nil {
		return nil, filter, err
	}
	slog.Debug("running analyze command", "command", cmd.Name(), "filter", filter)

	cfg, err := c.loadAndValidateConfig(cmd)
	if err != nil {
		return nil, filter, err
	}
	c.initializeTracking(cfg)
	if c.trackingDB == nil {
		fmt.Fprintln(cmd.OutOrStdout(), "Sound tracking is not enabled or database not available.")
		fmt.Fprintln(cmd.OutOrStdout(), "Enable tracking with CLAUDIO_SOUND_TRACKING=true")
	}
	return c.trackingDB, filter, nil
}

// newAnalyzeMissingCommand creates the analyze missing subcommand
func newAnalyzeMissingCommand(c *CLI) *cobra.Command {
	var flags analyzeFilterFlags
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
		RunE: func(cmd *cobra.Command, _ []string) error {
			return c.runAnalyzeMissing(cmd, &flags)
		},
	}
	flags.register(missingCmd)
	return missingCmd
}

// runAnalyzeMissing executes the analyze missing command
func (c *CLI) runAnalyzeMissing(cmd *cobra.Command, flags *analyzeFilterFlags) error {
	db, filter, err := c.openAnalyzeDB(cmd, flags)
	if err != nil || db == nil {
		return err
	}

	missingSounds, err := tracking.GetMissingSounds(db, filter)
	if err != nil {
		return fmt.Errorf("failed to analyze missing sounds: %w", err)
	}

	// Get summary statistics
	summary, err := tracking.GetMissingSoundsSummary(db, filter)
	if err != nil {
		slog.Warn("failed to get missing sounds summary", "error", err)
		// Continue without summary - not critical
	}

	return outputMissingSoundsHierarchical(cmd.OutOrStdout(), missingSounds, summary, filter)
}

// groupByTool groups missing sounds by tool and category. Everything comes
// back in display order: tools by total requests (descending) then name,
// categories as sortCategories orders them, sounds by sortSoundsByRequestCount.
func groupByTool(missingSounds []tracking.MissingSound) Analysis {
	toolMap := make(map[string]map[string][]tracking.MissingSound) // tool -> category -> sounds
	otherMap := make(map[string][]tracking.MissingSound)           // category -> sounds (for non-tool sounds)

	for _, sound := range missingSounds {
		if sound.ToolName == "" {
			otherMap[sound.Category] = append(otherMap[sound.Category], sound)
			continue
		}
		if toolMap[sound.ToolName] == nil {
			toolMap[sound.ToolName] = make(map[string][]tracking.MissingSound)
		}
		toolMap[sound.ToolName][sound.Category] = append(toolMap[sound.ToolName][sound.Category], sound)
	}

	tools := make([]ToolGroup, 0, len(toolMap))
	for toolName, categoryMap := range toolMap {
		tool := ToolGroup{Name: toolName, Categories: categoryGroups(categoryMap)}
		for _, category := range tool.Categories {
			tool.Total += category.Total
			tool.Count += category.Count
		}
		tools = append(tools, tool)
	}
	slices.SortFunc(tools, func(a, b ToolGroup) int {
		return cmp.Or(cmp.Compare(b.Total, a.Total), cmp.Compare(a.Name, b.Name))
	})

	return Analysis{Tools: tools, Other: categoryGroups(otherMap)}
}

// categoryGroups turns category -> sounds into sorted CategoryGroups.
func categoryGroups(byCategory map[string][]tracking.MissingSound) []CategoryGroup {
	groups := make([]CategoryGroup, 0, len(byCategory))
	for name, sounds := range byCategory {
		group := CategoryGroup{Name: name, Count: len(sounds), Sounds: sortSoundsByRequestCount(sounds)}
		for _, sound := range sounds {
			group.Total += sound.RequestCount
		}
		groups = append(groups, group)
	}
	return sortCategories(groups)
}

// outputMissingSoundsHierarchical displays missing sounds grouped by tool
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

	// Summary statistics if available. Each key is checked: a missing or
	// mistyped one skips its line instead of panicking.
	if uniqueCount, ok := summary["unique_missing_sounds"].(int); ok && uniqueCount > 0 {
		if totalRequests, ok := summary["total_missing_requests"].(int); ok {
			fmt.Fprintf(w, "Found %d unique missing sounds with %d total requests\n", uniqueCount, totalRequests)
		}
		if toolCount, ok := summary["tools_with_missing_sounds"].(int); ok && toolCount > 0 {
			fmt.Fprintf(w, "Across %d different tools\n", toolCount)
		}
		fmt.Fprintln(w)
	}

	for _, tool := range analysis.Tools {
		fmt.Fprintf(w, "%s (total: %d requests, %d sounds):\n", tool.Name, tool.Total, tool.Count)
		printCategoryGroups(w, tool.Categories)
		fmt.Fprintln(w) // Space between tools
	}

	if len(analysis.Other) > 0 {
		fmt.Fprintln(w, "Other (non-tool sounds):")
		printCategoryGroups(w, analysis.Other)
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

// printCategoryGroups prints each category with its sounds, paths
// truncated to 35 characters so the request counts line up.
func printCategoryGroups(w io.Writer, categories []CategoryGroup) {
	for _, category := range categories {
		fmt.Fprintf(w, "  %s (%d requests):\n", category.Name, category.Total)
		for _, sound := range category.Sounds {
			displayPath := sound.Path
			if len(displayPath) > 35 {
				displayPath = "..." + displayPath[len(displayPath)-32:]
			}
			fmt.Fprintf(w, "    %-35s %3d requests\n", displayPath, sound.RequestCount)
		}
		if len(categories) > 1 {
			fmt.Fprintln(w) // Space between categories only if multiple
		}
	}
}

// categoryDisplayOrder is the preferred category order; categories not
// listed follow it.
var categoryDisplayOrder = []string{"success", "error", "loading", "interactive", "completion", "system"}

// sortCategories returns categories in display order: known categories in
// categoryDisplayOrder, then the rest by total requests (descending) and
// name.
func sortCategories(categories []CategoryGroup) []CategoryGroup {
	rank := func(name string) int {
		if i := slices.Index(categoryDisplayOrder, name); i >= 0 {
			return i
		}
		return len(categoryDisplayOrder)
	}
	sorted := slices.Clone(categories)
	slices.SortFunc(sorted, func(a, b CategoryGroup) int {
		return cmp.Or(
			cmp.Compare(rank(a.Name), rank(b.Name)),
			cmp.Compare(b.Total, a.Total),
			cmp.Compare(a.Name, b.Name),
		)
	})
	return sorted
}

// sortSoundsByRequestCount returns sounds by request count (descending),
// then by path.
func sortSoundsByRequestCount(sounds []tracking.MissingSound) []tracking.MissingSound {
	sorted := slices.Clone(sounds)
	slices.SortFunc(sorted, func(a, b tracking.MissingSound) int {
		return cmp.Or(cmp.Compare(b.RequestCount, a.RequestCount), cmp.Compare(a.Path, b.Path))
	})
	return sorted
}

// newAnalyzeUsageCommand creates the analyze usage subcommand
func newAnalyzeUsageCommand(c *CLI) *cobra.Command {
	var flags analyzeFilterFlags
	var showChains, showSummary bool
	usageCmd := &cobra.Command{
		Use:   "usage",
		Short: "Show actual sound usage patterns and statistics",
		Long: `Show actual sound usage patterns and statistics from the tracking database.

This command analyzes which sounds were actually played, how often they were used,
and which fallback chain (Enhanced/PostTool/Simple) Claudio walked. This helps you
understand your soundpack effectiveness and identify optimization opportunities.

The results show:
- Most frequently played sounds
- Per-chain-type statistics (event count and average depth into the chain)
- Tool usage patterns
- Category distribution

Examples:
  claudio analyze usage                    # Show recent usage
  claudio analyze usage --days 30         # Last 30 days
  claudio analyze usage --preset today    # Today only
  claudio analyze usage --tool Edit       # Edit tool only
  claudio analyze usage --category success # Success sounds only
  claudio analyze usage --show-chains     # Include chain-type statistics
  claudio analyze usage --show-summary    # Show summary statistics`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return c.runAnalyzeUsage(cmd, &flags, showChains, showSummary)
		},
	}
	flags.register(usageCmd)
	usageCmd.Flags().BoolVar(&showChains, "show-chains", false, "Show per-chain-type statistics")
	usageCmd.Flags().BoolVar(&showSummary, "show-summary", false, "Show usage summary statistics")
	return usageCmd
}

// runAnalyzeUsage executes the analyze usage command
func (c *CLI) runAnalyzeUsage(cmd *cobra.Command, flags *analyzeFilterFlags, showChains, showSummary bool) error {
	db, filter, err := c.openAnalyzeDB(cmd, flags)
	if err != nil || db == nil {
		return err
	}

	usage, err := tracking.GetSoundUsage(db, filter)
	if err != nil {
		return fmt.Errorf("failed to get sound usage: %w", err)
	}
	if err := outputUsageStatistics(cmd.OutOrStdout(), usage, filter, showChains, showSummary, db); err != nil {
		return fmt.Errorf("failed to output usage statistics: %w", err)
	}
	return nil
}

// outputUsageStatistics formats and outputs usage statistics
func outputUsageStatistics(w io.Writer, usage []tracking.SoundUsage, filter tracking.QueryFilter, showChains, showSummary bool, db *sql.DB) error {
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
		summary, err := tracking.GetUsageSummary(db, filter)
		if err != nil {
			return fmt.Errorf("failed to get usage summary: %w", err)
		}
		fmt.Fprintf(w, "Summary: %d total events, %d unique sounds\n\n",
			summary.TotalEvents, summary.UniqueSounds)
	}

	// Show most used sounds
	fmt.Fprintln(w, "Most Frequently Used Sounds:")
	fmt.Fprintln(w, "----------------------------")

	for i, sound := range usage {
		if i >= filter.Limit {
			break
		}

		// Format: rank. path (play_count times) - tool/category
		rank := i + 1

		fmt.Fprintf(w, "%2d. %s (%d times)",
			rank, sound.Path, sound.PlayCount)

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

	// Show per-chain-type statistics if requested
	if showChains {
		chainStats, err := tracking.GetChainTypeStatistics(db, filter)
		if err != nil {
			return fmt.Errorf("failed to get chain type statistics: %w", err)
		}
		fmt.Fprintln(w, "\nChain Type Statistics:")
		fmt.Fprintln(w, "----------------------")
		for _, stat := range chainStats {
			label := stat.ChainType
			if label == "" {
				label = "(unrecorded)"
			}
			fmt.Fprintf(w, "%s: %d events (%.1f%%), avg depth %.1f\n",
				label, stat.EventCount, stat.Percentage, stat.AvgDepth)
		}
	}

	// Footer with actionable advice
	fmt.Fprintln(w, "\nTo improve your sound coverage:")
	fmt.Fprintln(w, "  1. Focus on sounds at deeper positions in their fallback chain")
	fmt.Fprintln(w, "  2. Create specific sounds for frequently used tools")
	fmt.Fprintln(w, "  3. Use --show-chains to see per-chain-type statistics")

	if !showSummary {
		fmt.Fprintln(w, "  4. Use --show-summary to see overall statistics")
	}

	return nil
}

// analyzeCategories lists the sound categories --category accepts: every
// hooks category except Silent, whose events play nothing and are never
// recorded, so filtering on it could only ever match nothing.
var analyzeCategories = func() []string {
	var names []string
	for _, c := range hooks.Categories() {
		if c != hooks.Silent {
			names = append(names, c.String())
		}
	}
	return names
}()

func validateAnalyzeFilterValues(category, preset string) error {
	if category != "" && !slices.Contains(analyzeCategories, category) {
		return fmt.Errorf("invalid category %q: must be one of %s", category, strings.Join(analyzeCategories, ", "))
	}
	if preset != "" {
		if _, _, err := tracking.ParseDatePreset(preset, time.Now()); err != nil {
			return fmt.Errorf("invalid date preset %q: %w", preset, err)
		}
	}
	return nil
}
