//go:build windows

package cli

import (
	"os"
	"os/exec"
	"testing"

	"golang.org/x/sys/windows"
)

// TestStartDetachedProcessInsideJobForbiddingBreakaway (#18): Claude Code
// runs hooks inside a job object that does not set
// JOB_OBJECT_LIMIT_BREAKAWAY_OK, so a CREATE_BREAKAWAY_FROM_JOB spawn fails
// with "Access is denied". The worker must still start. The test re-runs
// itself in a child process, because a process cannot leave a job.
func TestStartDetachedProcessInsideJobForbiddingBreakaway(t *testing.T) {
	if os.Getenv("CLAUDIO_TEST_IN_JOB") == "1" {
		job, err := windows.CreateJobObject(nil, nil)
		if err != nil {
			t.Fatalf("CreateJobObject: %v", err)
		}
		if err := windows.AssignProcessToJobObject(job, windows.CurrentProcess()); err != nil {
			t.Fatalf("AssignProcessToJobObject: %v", err)
		}
		cmd, err := startDetachedProcess(func() *exec.Cmd {
			return exec.Command(os.Args[0], "-test.run=^$")
		})
		if err != nil {
			t.Fatalf("worker did not start inside a job that forbids breakaway: %v", err)
		}
		if err := cmd.Wait(); err != nil {
			t.Fatalf("worker failed: %v", err)
		}
		return
	}

	child := exec.Command(os.Args[0], "-test.run=^TestStartDetachedProcessInsideJobForbiddingBreakaway$", "-test.v")
	child.Env = append(os.Environ(), "CLAUDIO_TEST_IN_JOB=1")
	if out, err := child.CombinedOutput(); err != nil {
		t.Fatalf("in-job run failed: %v\n%s", err, out)
	}
}
