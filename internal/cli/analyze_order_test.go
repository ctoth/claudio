package cli

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"claudio.click/internal/tracking"
)

// Tools and categories with equal totals print in name order; before, the
// order came from Go map iteration and changed between runs.
func TestGroupByToolOrderIsDeterministic(t *testing.T) {
	var sounds []tracking.MissingSound
	for _, tool := range []string{"Write", "Bash", "Read", "Edit", "Grep"} {
		for _, category := range []string{"zeta", "alpha", "success"} {
			sounds = append(sounds, tracking.MissingSound{
				Path: tool + "-" + category + ".wav", RequestCount: 3, Category: category, ToolName: tool,
			})
		}
	}
	for _, category := range []string{"omega", "beta"} {
		sounds = append(sounds, tracking.MissingSound{Path: category + ".wav", RequestCount: 1, Category: category})
	}

	wantTools := []string{"Bash", "Edit", "Grep", "Read", "Write"}
	for range 30 {
		analysis := groupByTool(sounds)
		var tools []string
		for _, tool := range analysis.Tools {
			tools = append(tools, tool.Name)
			if got := categoryNames(tool.Categories); !slices.Equal(got, []string{"success", "alpha", "zeta"}) {
				t.Fatalf("%s categories = %v, want [success alpha zeta]", tool.Name, got)
			}
		}
		if !slices.Equal(tools, wantTools) {
			t.Fatalf("tools = %v, want %v", tools, wantTools)
		}
		if got := categoryNames(analysis.Other); !slices.Equal(got, []string{"beta", "omega"}) {
			t.Fatalf("other = %v, want [beta omega]", got)
		}
	}
}

func categoryNames(groups []CategoryGroup) []string {
	var names []string
	for _, g := range groups {
		names = append(names, g.Name)
	}
	return names
}

// A summary missing a key is skipped, not a panic.
func TestOutputMissingSoundsToleratesPartialSummary(t *testing.T) {
	sounds := []tracking.MissingSound{{Path: "x.wav", RequestCount: 1, Category: "success", ToolName: "Bash"}}
	var out bytes.Buffer
	summary := map[string]interface{}{"unique_missing_sounds": 1}
	if err := outputMissingSoundsHierarchical(&out, sounds, summary, tracking.QueryFilter{}); err != nil {
		t.Fatalf("outputMissingSoundsHierarchical: %v", err)
	}
	if !strings.Contains(out.String(), "x.wav") {
		t.Errorf("output lacks the sound:\n%s", out.String())
	}
}

// A failing summary or chain-statistics query is an error, not a silently
// missing section.
func TestOutputUsageStatisticsReportsQueryErrors(t *testing.T) {
	usage := []tracking.SoundUsage{{Path: "x.wav", PlayCount: 1}}
	for name, flags := range map[string][2]bool{"summary": {false, true}, "chains": {true, false}} {
		t.Run(name, func(t *testing.T) {
			db, err := tracking.NewDatabase(":memory:")
			if err != nil {
				t.Fatalf("NewDatabase: %v", err)
			}
			db.Close()
			var out bytes.Buffer
			if err := outputUsageStatistics(&out, usage, tracking.QueryFilter{Limit: 5}, flags[0], flags[1], db); err == nil {
				t.Errorf("want an error from the closed database, got nil; output:\n%s", out.String())
			}
		})
	}
}
