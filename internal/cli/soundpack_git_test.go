package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"claudio.click/internal/config"
)

func TestExpandGitSoundpackSource_GitHubAlias(t *testing.T) {
	got, err := expandGitSoundpackSource("gh:ctoth/whatever")
	if err != nil {
		t.Fatalf("expandGitSoundpackSource returned error: %v", err)
	}

	want := "https://github.com/ctoth/whatever.git"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestExpandGitSoundpackSource_RejectsInvalidGitHubAlias(t *testing.T) {
	_, err := expandGitSoundpackSource("gh:ctoth")
	if err == nil {
		t.Fatal("expected invalid gh alias to fail")
	}
}

func TestSoundpackAdd_ClonesGitRepositoryAndUpdatesConfig(t *testing.T) {
	dataDir, configDir, cleanup := setupInstallTestEnv(t)
	defer cleanup()

	repoPath := createTestGitSoundpackRepo(t)

	cli := NewCLI()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := cli.Run([]string{"claudio", "soundpack", "add", repoPath, "--name", "git-pack"}, nil, stdout, stderr)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stdout: %s, stderr: %s", exitCode, stdout.String(), stderr.String())
	}

	expectedClone := filepath.Join(dataDir, "claudio", "soundpack-repos", "git-pack")
	if _, err := os.Stat(filepath.Join(expectedClone, "success", "success.wav")); err != nil {
		t.Fatalf("expected cloned sound file, got error: %v", err)
	}

	registry, err := loadSoundpackRegistry()
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}
	record, exists := registry.Packs["git-pack"]
	if !exists {
		t.Fatalf("expected registry to contain git-pack: %#v", registry.Packs)
	}
	if record.URL != repoPath {
		t.Fatalf("expected registry URL %q, got %q", repoPath, record.URL)
	}

	cfg := loadTestConfig(t, configDir)
	if !containsPath(cfg.SoundpackPaths, expectedClone) {
		t.Fatalf("expected soundpack_paths to contain %s, got %v", expectedClone, cfg.SoundpackPaths)
	}
}

func TestSoundpackUpdate_PullsLatestCommit(t *testing.T) {
	dataDir, _, cleanup := setupInstallTestEnv(t)
	defer cleanup()

	repoPath := createTestGitSoundpackRepo(t)

	cli := NewCLI()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := cli.Run([]string{"claudio", "soundpack", "add", repoPath, "--name", "git-pack"}, nil, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("add: expected exit code 0, got %d, stdout: %s, stderr: %s", exitCode, stdout.String(), stderr.String())
	}

	createDummyWAV(t, filepath.Join(repoPath, "error", "error.wav"))
	commitTestGitRepo(t, repoPath, "add error sound")

	stdout.Reset()
	stderr.Reset()
	exitCode = cli.Run([]string{"claudio", "soundpack", "update", "git-pack"}, nil, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("update: expected exit code 0, got %d, stdout: %s, stderr: %s", exitCode, stdout.String(), stderr.String())
	}

	expectedFile := filepath.Join(dataDir, "claudio", "soundpack-repos", "git-pack", "error", "error.wav")
	if _, err := os.Stat(expectedFile); err != nil {
		t.Fatalf("expected updated clone to contain %s, got error: %v", expectedFile, err)
	}
}

func TestSoundpackRemove_DeletesManagedCloneAndConfigEntries(t *testing.T) {
	dataDir, configDir, cleanup := setupInstallTestEnv(t)
	defer cleanup()

	repoPath := createTestGitSoundpackRepo(t)
	clonePath := filepath.Join(dataDir, "claudio", "soundpack-repos", "git-pack")

	cli := NewCLI()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := cli.Run([]string{"claudio", "soundpack", "add", repoPath, "--name", "git-pack", "--default"}, nil, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("add: expected exit code 0, got %d, stdout: %s, stderr: %s", exitCode, stdout.String(), stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	exitCode = cli.Run([]string{"claudio", "soundpack", "remove", "git-pack"}, nil, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("remove: expected exit code 0, got %d, stdout: %s, stderr: %s", exitCode, stdout.String(), stderr.String())
	}

	if _, err := os.Stat(clonePath); !os.IsNotExist(err) {
		t.Fatalf("expected clone path to be removed, stat error: %v", err)
	}

	registry, err := loadSoundpackRegistry()
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}
	if _, exists := registry.Packs["git-pack"]; exists {
		t.Fatalf("expected git-pack to be removed from registry")
	}

	cfg := loadTestConfig(t, configDir)
	if containsPath(cfg.SoundpackPaths, clonePath) {
		t.Fatalf("expected soundpack_paths not to contain %s, got %v", clonePath, cfg.SoundpackPaths)
	}
	if cfg.DefaultSoundpack == "git-pack" {
		t.Fatalf("expected default_soundpack to be reset after removal")
	}
}

func TestSoundpackStatus_ShowsManagedGitPack(t *testing.T) {
	_, _, cleanup := setupInstallTestEnv(t)
	defer cleanup()

	repoPath := createTestGitSoundpackRepo(t)

	cli := NewCLI()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := cli.Run([]string{"claudio", "soundpack", "add", repoPath, "--name", "git-pack"}, nil, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("add: expected exit code 0, got %d, stdout: %s, stderr: %s", exitCode, stdout.String(), stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	exitCode = cli.Run([]string{"claudio", "soundpack", "status", "git-pack"}, nil, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("status: expected exit code 0, got %d, stdout: %s, stderr: %s", exitCode, stdout.String(), stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "git-pack") || !strings.Contains(output, "clean") {
		t.Fatalf("expected status output to mention git-pack and clean state, got: %s", output)
	}
}

func createTestGitSoundpackRepo(t *testing.T) string {
	t.Helper()
	if err := requireGit(); err != nil {
		t.Skipf("git is required for git soundpack tests: %v", err)
	}

	repoPath := filepath.Join(t.TempDir(), "repo")
	createDummyWAV(t, filepath.Join(repoPath, "success", "success.wav"))

	if _, err := runGit(repoPath, "init"); err != nil {
		t.Fatalf("git init failed: %v", err)
	}
	if _, err := runGit(repoPath, "config", "user.email", "test@example.com"); err != nil {
		t.Fatalf("git config user.email failed: %v", err)
	}
	if _, err := runGit(repoPath, "config", "user.name", "Test User"); err != nil {
		t.Fatalf("git config user.name failed: %v", err)
	}
	commitTestGitRepo(t, repoPath, "initial soundpack")
	return repoPath
}

func commitTestGitRepo(t *testing.T, repoPath, message string) {
	t.Helper()
	if _, err := runGit(repoPath, "add", "."); err != nil {
		t.Fatalf("git add failed: %v", err)
	}
	if _, err := runGit(repoPath, "commit", "-m", message); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}
}

func loadTestConfig(t *testing.T, configDir string) *config.Config {
	t.Helper()
	configPath := filepath.Join(configDir, "claudio", "config.json")
	cm := config.NewConfigManager()
	cfg, err := cm.LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("failed to load config from %s: %v", configPath, err)
	}
	return cfg
}

func containsPath(paths []string, want string) bool {
	for _, p := range paths {
		if filepath.Clean(p) == filepath.Clean(want) {
			return true
		}
	}
	return false
}
