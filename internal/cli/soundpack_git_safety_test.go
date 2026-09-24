package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"claudio.click/internal/soundpack/gitpack"
	"claudio.click/internal/testutil/wavfixture"
)

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
	registry, err := gitpack.LoadRegistry()
	if err != nil {
		t.Fatal(err)
	}
	record := registry.Packs["tampered"]
	record.Ref = "--orphan=evil"
	registry.Packs["tampered"] = record
	if err := gitpack.SaveRegistry(registry); err != nil {
		t.Fatal(err)
	}

	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	if code := cli.Run([]string{"claudio", "soundpack", "update", "tampered"}, nil, stdout, stderr); code == 0 {
		t.Fatalf("expected tampered ref to be rejected, stdout=%q", stdout)
	}
	branch, err := testGitBranch(context.Background(), record.Path)
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
	defaultBranch, err := testGitBranch(ctx, repo)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := testGit(ctx, repo, "checkout", "-b", "dev"); err != nil {
		t.Fatal(err)
	}
	wavfixture.Write(t, filepath.Join(repo, "error", "error.wav"))
	commitTestGitRepo(t, repo, "dev sound")
	if _, err := testGit(ctx, repo, "checkout", defaultBranch); err != nil {
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

	if _, err := testGit(ctx, repo, "checkout", "dev"); err != nil {
		t.Fatal(err)
	}
	wavfixture.Write(t, filepath.Join(repo, "interactive", "interactive.wav"))
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
