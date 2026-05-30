package cli

import (
	"os"
	"os/exec"
	"testing"

	"github.com/spf13/cobra"
)

// detachTestCmd builds a fresh cobra.Command with the flags
// buildDetachedWorkerArgs inspects (config/volume/soundpack/silent). The
// flags are registered directly on Flags() (not PersistentFlags) so
// cmd.Flags().GetString reads them back without requiring a full
// ParseFlags pass — buildDetachedWorkerArgs runs against an *already
// parsed* root command in production, where the persistent flags have
// been merged into the local set.
func detachTestCmd(t *testing.T) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "claudio"}
	cmd.Flags().String("config", "", "")
	cmd.Flags().String("volume", "", "")
	cmd.Flags().String("soundpack", "", "")
	cmd.Flags().Bool("silent", false, "")
	return cmd
}

// indexOf returns the first index of value in args, or -1.
func indexOfArg(args []string, value string) int {
	for i, a := range args {
		if a == value {
			return i
		}
	}
	return -1
}

// TestBuildDetachedWorkerArgs_StandardShape (finding #59) verifies the
// minimum argv: --daemon-child plus --hook-input-file with the supplied
// path, and nothing else when no other flags are set.
func TestBuildDetachedWorkerArgs_StandardShape(t *testing.T) {
	cmd := detachTestCmd(t)
	got := buildDetachedWorkerArgs(cmd, "/tmp/hook.json")

	want := []string{"--daemon-child", "--hook-input-file", "/tmp/hook.json"}
	if len(got) != len(want) {
		t.Fatalf("expected %d args (%v), got %d (%v)", len(want), want, len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("arg[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestBuildDetachedWorkerArgs_PropagatesHookInputFile ensures the input
// path is passed through verbatim, including paths with spaces.
func TestBuildDetachedWorkerArgs_PropagatesHookInputFile(t *testing.T) {
	cmd := detachTestCmd(t)
	path := "/tmp/dir with spaces/hook-123.json"
	got := buildDetachedWorkerArgs(cmd, path)
	idx := indexOfArg(got, "--hook-input-file")
	if idx < 0 {
		t.Fatalf("expected --hook-input-file flag in args: %v", got)
	}
	if idx+1 >= len(got) || got[idx+1] != path {
		t.Errorf("expected hook input path %q after --hook-input-file, got args %v", path, got)
	}
}

// TestBuildDetachedWorkerArgs_PropagatesConfigOverride covers the case where
// the user passes --config; the detached worker must receive the same flag
// or the user-supplied config is silently lost.
func TestBuildDetachedWorkerArgs_PropagatesConfigOverride(t *testing.T) {
	cmd := detachTestCmd(t)
	if err := cmd.Flags().Set("config", "/etc/claudio.json"); err != nil {
		t.Fatalf("set config flag: %v", err)
	}
	got := buildDetachedWorkerArgs(cmd, "/tmp/x.json")

	idx := indexOfArg(got, "--config")
	if idx < 0 {
		t.Fatalf("expected --config flag in args: %v", got)
	}
	if idx+1 >= len(got) || got[idx+1] != "/etc/claudio.json" {
		t.Errorf("expected --config value to propagate, got args %v", got)
	}
}

// TestBuildDetachedWorkerArgs_PropagatesVolumeAndSoundpack covers the
// --volume and --soundpack overrides reaching the worker.
func TestBuildDetachedWorkerArgs_PropagatesVolumeAndSoundpack(t *testing.T) {
	cmd := detachTestCmd(t)
	if err := cmd.Flags().Set("volume", "0.42"); err != nil {
		t.Fatalf("set volume flag: %v", err)
	}
	if err := cmd.Flags().Set("soundpack", "mechanical"); err != nil {
		t.Fatalf("set soundpack flag: %v", err)
	}
	got := buildDetachedWorkerArgs(cmd, "/tmp/x.json")

	if idx := indexOfArg(got, "--volume"); idx < 0 || got[idx+1] != "0.42" {
		t.Errorf("expected --volume 0.42 in args, got %v", got)
	}
	if idx := indexOfArg(got, "--soundpack"); idx < 0 || got[idx+1] != "mechanical" {
		t.Errorf("expected --soundpack mechanical in args, got %v", got)
	}
}

// TestBuildDetachedWorkerArgs_PropagatesSilent verifies the bool --silent
// flag reaches the worker as a presence-only flag (no value).
func TestBuildDetachedWorkerArgs_PropagatesSilent(t *testing.T) {
	cmd := detachTestCmd(t)
	if err := cmd.Flags().Set("silent", "true"); err != nil {
		t.Fatalf("set silent flag: %v", err)
	}
	got := buildDetachedWorkerArgs(cmd, "/tmp/x.json")
	if indexOfArg(got, "--silent") < 0 {
		t.Errorf("expected --silent in args, got %v", got)
	}
	// --silent must NOT be followed by a value (it's a bool flag).
	idx := indexOfArg(got, "--silent")
	if idx+1 < len(got) && (got[idx+1] == "true" || got[idx+1] == "false") {
		t.Errorf("--silent should not carry a value argument, got %v", got)
	}
}

// TestBuildDetachedWorkerArgs_DaemonChildFirst asserts the daemon-child
// marker appears before any other flag — easier to spot in process
// listings (ps / Get-Process / Task Manager).
func TestBuildDetachedWorkerArgs_DaemonChildFirst(t *testing.T) {
	cmd := detachTestCmd(t)
	if err := cmd.Flags().Set("config", "/x.json"); err != nil {
		t.Fatalf("set config flag: %v", err)
	}
	if err := cmd.Flags().Set("silent", "true"); err != nil {
		t.Fatalf("set silent flag: %v", err)
	}
	got := buildDetachedWorkerArgs(cmd, "/tmp/x.json")
	if len(got) == 0 || got[0] != "--daemon-child" {
		t.Errorf("expected first arg to be --daemon-child, got %v", got)
	}
}


// TestDetachStdio_PrefersDevNull verifies the happy path attaches all three
// streams to the same DevNull file. Covers finding #48 by exercising the
// preferred code path.
func TestDetachStdio_PrefersDevNull(t *testing.T) {
	cmd := exec.Command("noop")
	detachStdio(cmd)

	if cmd.Stdin == nil {
		t.Fatal("expected Stdin to be non-nil after detachStdio")
	}
	if cmd.Stdout == nil {
		t.Fatal("expected Stdout to be non-nil after detachStdio")
	}
	if cmd.Stderr == nil {
		t.Fatal("expected Stderr to be non-nil after detachStdio")
	}
}

// TestDetachStdio_NeverInheritsParentStdio is the load-bearing assertion for
// #48: after detachStdio, neither Stdout nor Stderr may equal os.Stdout /
// os.Stderr (the default cobra-exec behavior that leaks the child to the
// user's terminal).
func TestDetachStdio_NeverInheritsParentStdio(t *testing.T) {
	cmd := exec.Command("noop")
	// Pre-seed the fields to parent stdio to simulate what would happen
	// if detachStdio silently fell through — the fix must overwrite them.
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	detachStdio(cmd)

	if cmd.Stdout == os.Stdout {
		t.Error("detachStdio left child Stdout pointing at parent os.Stdout")
	}
	if cmd.Stderr == os.Stderr {
		t.Error("detachStdio left child Stderr pointing at parent os.Stderr")
	}
}
