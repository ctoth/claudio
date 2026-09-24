package cli

import (
	"slices"
	"testing"
)

// Every flag the hook invocation set reaches the detached worker, including
// one added later: there is no whitelist to keep in sync.
func TestBuildDetachedWorkerArgs_ForwardsEverySetFlag(t *testing.T) {
	root := NewCLI().rootCmd
	root.PersistentFlags().String("brand-new", "", "a flag added after the worker args were written")
	root.PersistentFlags().Bool("brand-new-bool", true, "")
	if err := root.ParseFlags([]string{
		"--brand-new", "x y", "--brand-new-bool=false", "--volume", "0.3", "--silent", "--hook-agent", "gemini",
		"--hook-input-file", "/stale.json", "--daemon-child",
	}); err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}

	got := buildDetachedWorkerArgs(root, "/tmp/hook.json")

	want := []string{
		"--daemon-child", "--hook-input-file", "/tmp/hook.json",
		"--brand-new", "x y", "--brand-new-bool=false", "--hook-agent", "gemini", "--silent", "--volume", "0.3",
	}
	if !slices.Equal(got, want) {
		t.Errorf("worker args =\n  %q\nwant\n  %q", got, want)
	}
}
