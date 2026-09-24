package gitpack

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// RequireGit fails when no git executable is on PATH.
func RequireGit() error {
	if _, err := exec.LookPath("git"); err != nil {
		return errors.New("git executable not found in PATH")
	}
	return nil
}

// gitCommandTimeout bounds every git invocation so a hung remote cannot
// wedge the CLI.
const gitCommandTimeout = 2 * time.Minute

// gitCommand builds a non-interactive git invocation. GIT_TERMINAL_PROMPT=0
// makes git fail instead of blocking on a credential prompt nobody can
// answer (hooks and detached workers have no terminal).
func gitCommand(ctx context.Context, dir string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	return cmd
}

func runGit(ctx context.Context, dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, gitCommandTimeout)
	defer cancel()

	cmd := gitCommand(ctx, dir, args...)
	output, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(output))
	if ctx.Err() == context.DeadlineExceeded {
		return text, fmt.Errorf("git %s timed out", strings.Join(args, " "))
	}
	if err != nil {
		if text != "" {
			return text, fmt.Errorf("git %s failed: %s", strings.Join(args, " "), text)
		}
		return text, fmt.Errorf("git %s failed: %w", strings.Join(args, " "), err)
	}
	return text, nil
}

// checkoutGitRef detaches HEAD at ref. The remote-tracking branch
// (origin/<ref>) wins over a local name so branch refs advance after a
// fetch; tags and commit ids resolve directly. --detach makes git treat the
// argument as a commit, never as a pathspec.
func checkoutGitRef(ctx context.Context, repoPath, ref string) error {
	if err := ValidateRef(ref); err != nil {
		return err
	}
	commit, err := resolveGitRef(ctx, repoPath, ref)
	if err != nil {
		return err
	}
	_, err = runGit(ctx, repoPath, "checkout", "--detach", commit)
	return err
}

func resolveGitRef(ctx context.Context, repoPath, ref string) (string, error) {
	for _, candidate := range []string{"refs/remotes/origin/" + ref, ref} {
		commit, err := runGit(ctx, repoPath, "rev-parse", "--verify", "--quiet", candidate+"^{commit}")
		if err == nil && commit != "" {
			return commit, nil
		}
	}
	return "", fmt.Errorf("cannot resolve git ref %q", ref)
}

func currentGitCommit(ctx context.Context, repoPath string) (string, error) {
	commit, err := runGit(ctx, repoPath, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(commit), nil
}

func currentGitBranch(ctx context.Context, repoPath string) (string, error) {
	branch, err := runGit(ctx, repoPath, "symbolic-ref", "--short", "-q", "HEAD")
	if err != nil {
		// With -q a detached HEAD exits 1 with no output; that means "no
		// branch", not failure. Anything else is a real error.
		var exitErr *exec.ExitError
		if branch == "" && errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(branch), nil
}

func gitWorktreeDirty(ctx context.Context, repoPath string) (bool, error) {
	status, err := runGit(ctx, repoPath, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(status) != "", nil
}
