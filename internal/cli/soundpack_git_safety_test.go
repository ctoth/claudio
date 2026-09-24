package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestValidateGitRefRejectsOptionLikeRefs(t *testing.T) {
	for _, ref := range []string{"-", "--orphan=evil", "-b", "--upload-pack=touch pwned"} {
		if err := validateGitRef(ref); err == nil {
			t.Errorf("validateGitRef(%q) accepted an option-like ref", ref)
		}
	}
	for _, ref := range []string{"", "main", "v1.2.3", "feature/x", "0123abcd"} {
		if err := validateGitRef(ref); err != nil {
			t.Errorf("validateGitRef(%q) rejected a valid ref: %v", ref, err)
		}
	}
}

func TestGitCommandDisablesTerminalPrompt(t *testing.T) {
	cmd := gitCommand(context.Background(), "", "status")
	if !slices.Contains(cmd.Env, "GIT_TERMINAL_PROMPT=0") {
		t.Fatalf("git command env lacks GIT_TERMINAL_PROMPT=0: %v", cmd.Env)
	}
}

func TestSoundpackAddRejectsOptionLikeRefBeforeCloning(t *testing.T) {
	dataDir, _, cleanup := setupInstallTestEnv(t)
	defer cleanup()
	repo := createTestGitSoundpackRepo(t)

	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	code := NewCLI().Run([]string{"claudio", "soundpack", "add", repo, "--name", "dash-ref", "--ref=--orphan=evil"}, nil, stdout, stderr)
	if code == 0 {
		t.Fatalf("expected option-like ref to be rejected, stdout=%q", stdout)
	}
	if !strings.Contains(stderr.String()+stdout.String(), "ref") {
		t.Fatalf("expected ref error, stdout=%q stderr=%q", stdout, stderr)
	}
	entries, _ := os.ReadDir(filepath.Join(dataDir, "claudio", "soundpack-repos"))
	for _, e := range entries {
		if strings.Contains(e.Name(), "dash-ref") && !strings.HasSuffix(e.Name(), ".lock") {
			t.Fatalf("clone artifact left behind: %s", e.Name())
		}
	}
}

func TestSoundpackUpdateRejectsOptionLikeRefFromRegistry(t *testing.T) {
	_, _, cleanup := setupInstallTestEnv(t)
	defer cleanup()
	repo := createTestGitSoundpackRepo(t)
	cli := NewCLI()
	if code := cli.Run([]string{"claudio", "soundpack", "add", repo, "--name", "tampered"}, nil, &bytes.Buffer{}, &bytes.Buffer{}); code != 0 {
		t.Fatalf("add exited %d", code)
	}
	registry, err := loadSoundpackRegistry()
	if err != nil {
		t.Fatal(err)
	}
	record := registry.Packs["tampered"]
	record.Ref = "--orphan=evil"
	registry.Packs["tampered"] = record
	if err := saveSoundpackRegistry(registry); err != nil {
		t.Fatal(err)
	}

	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	if code := cli.Run([]string{"claudio", "soundpack", "update", "tampered"}, nil, stdout, stderr); code == 0 {
		t.Fatalf("expected tampered ref to be rejected, stdout=%q", stdout)
	}
	branch, err := currentGitBranch(context.Background(), record.Path)
	if err != nil || branch == "evil" {
		t.Fatalf("tampered ref reached git: branch=%q err=%v", branch, err)
	}
}

func TestSoundpackAddTreatsDashPrefixedSourceAsURL(t *testing.T) {
	dataDir, _, cleanup := setupInstallTestEnv(t)
	defer cleanup()
	repo := createTestGitSoundpackRepo(t)
	parent := filepath.Dir(repo)
	dashRepo := filepath.Join(parent, "-dash-repo")
	if err := os.Rename(repo, dashRepo); err != nil {
		t.Fatal(err)
	}
	t.Chdir(parent)

	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	code := NewCLI().Run([]string{"claudio", "soundpack", "add", "--name", "dash-src", "--", "-dash-repo"}, nil, stdout, stderr)
	if code != 0 {
		t.Fatalf("expected dash-prefixed local source to clone, code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "claudio", "soundpack-repos", "dash-src", "success", "success.wav")); err != nil {
		t.Fatalf("clone incomplete: %v", err)
	}
}

func TestSoundpackBranchRefTracksRemoteOnUpdate(t *testing.T) {
	dataDir, _, cleanup := setupInstallTestEnv(t)
	defer cleanup()
	ctx := context.Background()
	repo := createTestGitSoundpackRepo(t)
	defaultBranch, err := currentGitBranch(ctx, repo)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runGit(ctx, repo, "checkout", "-b", "dev"); err != nil {
		t.Fatal(err)
	}
	createDummyWAV(t, filepath.Join(repo, "error", "error.wav"))
	commitTestGitRepo(t, repo, "dev sound")
	if _, err := runGit(ctx, repo, "checkout", defaultBranch); err != nil {
		t.Fatal(err)
	}

	cli := NewCLI()
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	if code := cli.Run([]string{"claudio", "soundpack", "add", repo, "--name", "branch-pack", "--ref", "dev"}, nil, stdout, stderr); code != 0 {
		t.Fatalf("add exited %d stdout=%q stderr=%q", code, stdout, stderr)
	}
	clone := filepath.Join(dataDir, "claudio", "soundpack-repos", "branch-pack")
	if _, err := os.Stat(filepath.Join(clone, "error", "error.wav")); err != nil {
		t.Fatalf("dev branch not checked out: %v", err)
	}

	if _, err := runGit(ctx, repo, "checkout", "dev"); err != nil {
		t.Fatal(err)
	}
	createDummyWAV(t, filepath.Join(repo, "interactive", "interactive.wav"))
	commitTestGitRepo(t, repo, "second dev sound")

	stdout.Reset()
	stderr.Reset()
	if code := cli.Run([]string{"claudio", "soundpack", "update", "branch-pack"}, nil, stdout, stderr); code != 0 {
		t.Fatalf("update exited %d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(clone, "interactive", "interactive.wav")); err != nil {
		t.Fatalf("update did not advance dev branch ref: %v", err)
	}
}

func TestCurrentGitBranchDetachedHeadIsNotAnError(t *testing.T) {
	ctx := context.Background()
	repo := createTestGitSoundpackRepo(t)
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
