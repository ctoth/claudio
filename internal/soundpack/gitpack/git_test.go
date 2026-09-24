package gitpack

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestGitCommandDisablesTerminalPrompt(t *testing.T) {
	cmd := gitCommand(context.Background(), "", "status")
	if !slices.Contains(cmd.Env, "GIT_TERMINAL_PROMPT=0") {
		t.Fatalf("git command env lacks GIT_TERMINAL_PROMPT=0: %v", cmd.Env)
	}
}

func TestCurrentGitBranchDetachedHeadIsNotAnError(t *testing.T) {
	ctx := context.Background()
	repo := createTestRepo(t)
	if branch, err := currentGitBranch(ctx, repo); err != nil || branch == "" {
		t.Fatalf("expected a branch name on a fresh repo, got %q err=%v", branch, err)
	}
	if _, err := runGit(ctx, repo, "checkout", "--detach", "HEAD"); err != nil {
		t.Fatal(err)
	}
	branch, err := currentGitBranch(ctx, repo)
	if err != nil || branch != "" {
		t.Fatalf("expected detached HEAD to report no branch and no error, got %q err=%v", branch, err)
	}
	if _, err := currentGitBranch(ctx, t.TempDir()); err == nil {
		t.Fatal("expected an error outside a git repository")
	}
}

// createTestRepo makes a one-commit git repository.
func createTestRepo(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping git test in -short mode")
	}
	if err := RequireGit(); err != nil {
		t.Skipf("git is required: %v", err)
	}
	ctx := context.Background()
	repo := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(repo, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "README"), []byte("pack\n"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "Test User"},
		{"add", "."},
		{"commit", "-m", "initial"},
	} {
		if _, err := runGit(ctx, repo, args...); err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
	}
	return repo
}
