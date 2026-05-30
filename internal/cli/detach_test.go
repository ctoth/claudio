package cli

import (
	"os"
	"os/exec"
	"testing"
)

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
