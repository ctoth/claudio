package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"claudio.click/internal/tracking"
)

// The --category and --preset help text must list every value validation
// accepts; it used to name 4 of 6 categories and 5 of 7 presets.
func TestAnalyzeHelpListsEveryAcceptedValue(t *testing.T) {
	for _, sub := range []string{"missing", "usage"} {
		stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
		code := NewCLI().Run([]string{"claudio", "analyze", sub, "--help"}, strings.NewReader(""), stdout, stderr)
		if code != 0 {
			t.Fatalf("analyze %s --help exit %d: %s", sub, code, stderr.String())
		}
		help := stdout.String()
		for _, c := range analyzeCategories {
			if !strings.Contains(help, c) {
				t.Errorf("analyze %s --help missing category %q", sub, c)
			}
			if err := validateAnalyzeFilterValues(c, ""); err != nil {
				t.Errorf("listed category %q rejected: %v", c, err)
			}
		}
		for _, p := range tracking.DatePresets {
			if !strings.Contains(help, p) {
				t.Errorf("analyze %s --help missing preset %q", sub, p)
			}
			if _, _, err := tracking.ParseDatePreset(p, time.Now()); err != nil {
				t.Errorf("listed preset %q rejected: %v", p, err)
			}
		}
	}
	if len(analyzeCategories) != 6 || len(tracking.DatePresets) != 7 {
		t.Errorf("got %d categories and %d presets, want 6 and 7", len(analyzeCategories), len(tracking.DatePresets))
	}
}
