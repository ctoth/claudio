package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

func TestSoundpackAddHonorsExplicitConfig(t *testing.T) {
	_, configDir, cleanup := setupInstallTestEnv(t)
	defer cleanup()
	repoPath := createTestGitSoundpackRepo(t)
	customConfig := filepath.Join(t.TempDir(), "custom.json")

	cli := NewCLI()
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	if code := cli.Run([]string{"claudio", "soundpack", "add", repoPath, "--name", "custom-config-pack", "--config", customConfig}, nil, stdout, stderr); code != 0 {
		t.Fatalf("add failed: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	cfg, err := config.NewConfigManager().LoadFromFile(customConfig)
	if err != nil {
		t.Fatalf("explicit config was not written: %v", err)
	}
	if len(cfg.SoundpackPaths) != 1 || !strings.Contains(cfg.SoundpackPaths[0], "custom-config-pack") {
		t.Fatalf("unexpected explicit config paths: %v", cfg.SoundpackPaths)
	}
	if _, err := os.Stat(filepath.Join(configDir, "claudio", "config.json")); !os.IsNotExist(err) {
		t.Fatalf("default config was written despite --config: %v", err)
	}
}

func TestSoundpackAddValidatesConfigBeforeCloning(t *testing.T) {
	dataDir, _, cleanup := setupInstallTestEnv(t)
	defer cleanup()
	repoPath := createTestGitSoundpackRepo(t)
	customConfig := filepath.Join(t.TempDir(), "custom.json")
	want := []byte("{recover me")
	if err := os.WriteFile(customConfig, want, 0600); err != nil {
		t.Fatal(err)
	}

	cli := NewCLI()
	if code := cli.Run([]string{"claudio", "soundpack", "add", repoPath, "--name", "preflight-pack", "--config", customConfig}, nil, &bytes.Buffer{}, &bytes.Buffer{}); code == 0 {
		t.Fatal("expected malformed config error")
	}
	clone := filepath.Join(dataDir, "claudio", "soundpack-repos", "preflight-pack")
	if _, err := os.Stat(clone); !os.IsNotExist(err) {
		t.Fatalf("repo was cloned before config validation: %v", err)
	}
	registry, err := loadSoundpackRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := registry.Packs["preflight-pack"]; ok {
		t.Fatal("registry changed before config validation")
	}
	got, err := os.ReadFile(customConfig)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("malformed config changed: got=%q err=%v", got, err)
	}
}

func TestSoundpackAddRetryRepairsConfigForMatchingManagedRecord(t *testing.T) {
	_, configDir, cleanup := setupInstallTestEnv(t)
	defer cleanup()
	repo := createTestGitSoundpackRepo(t)
	cli := NewCLI()
	args := []string{"claudio", "soundpack", "add", repo, "--name", "retry-add"}
	if code := cli.Run(args, nil, &bytes.Buffer{}, &bytes.Buffer{}); code != 0 {
		t.Fatalf("initial add exited %d", code)
	}
	configPath := filepath.Join(configDir, "claudio", "config.json")
	registry, err := loadSoundpackRegistry()
	if err != nil {
		t.Fatal(err)
	}
	clonePath := registry.Packs["retry-add"].Path
	cfg := config.NewConfigManager().GetDefaultConfig()
	cfg.SoundpackPaths = []string{filepath.Join(clonePath, "obsolete-layout.json")}
	if err := config.NewConfigManager().SaveToFile(cfg, configPath); err != nil {
		t.Fatal(err)
	}

	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	if code := cli.Run(args, nil, stdout, stderr); code != 0 {
		t.Fatalf("retry failed: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	got := loadTestConfig(t, configDir)
	if len(got.SoundpackPaths) != 1 || filepath.Clean(got.SoundpackPaths[0]) != filepath.Clean(clonePath) {
		t.Fatalf("retry did not repair config: %v", got.SoundpackPaths)
	}
}

func TestSoundpackReplaceFailurePreservesExistingManagedPack(t *testing.T) {
	dataDir, _, cleanup := setupInstallTestEnv(t)
	defer cleanup()
	repo := createTestGitSoundpackRepo(t)
	cli := NewCLI()
	if code := cli.Run([]string{"claudio", "soundpack", "add", repo, "--name", "replace-safe"}, nil, &bytes.Buffer{}, &bytes.Buffer{}); code != 0 {
		t.Fatalf("initial add exited %d", code)
	}
	clone := filepath.Join(dataDir, "claudio", "soundpack-repos", "replace-safe")
	before, err := currentGitCommit(clone)
	if err != nil {
		t.Fatal(err)
	}

	if code := cli.Run([]string{"claudio", "soundpack", "add", repo, "--name", "replace-safe", "--replace", "--ref", "missing-ref"}, nil, &bytes.Buffer{}, &bytes.Buffer{}); code == 0 {
		t.Fatal("expected invalid replacement ref to fail")
	}
	after, err := currentGitCommit(clone)
	if err != nil {
		t.Fatalf("existing clone was lost: %v", err)
	}
	if after != before {
		t.Fatalf("existing clone changed after failed replacement: before=%s after=%s", before, after)
	}
	registry, err := loadSoundpackRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if record := registry.Packs["replace-safe"]; record.ResolvedCommit != before || record.Path != clone {
		t.Fatalf("existing registry record changed: %#v", record)
	}
}

func TestConcurrentSoundpackAddsPreserveBothRegistryEntries(t *testing.T) {
	_, _, cleanup := setupInstallTestEnv(t)
	defer cleanup()
	repos := []string{createTestGitSoundpackRepo(t), createTestGitSoundpackRepo(t)}
	names := []string{"concurrent-a", "concurrent-b"}
	start := make(chan struct{})
	results := make(chan int, len(repos))
	var wg sync.WaitGroup
	for i := range repos {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			results <- NewCLI().Run(
				[]string{"claudio", "soundpack", "add", repos[i], "--name", names[i]},
				nil, &bytes.Buffer{}, &bytes.Buffer{},
			)
		}(i)
	}
	close(start)
	wg.Wait()
	close(results)
	for code := range results {
		if code != 0 {
			t.Fatalf("concurrent add exited %d", code)
		}
	}
	registry, err := loadSoundpackRegistry()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		if _, ok := registry.Packs[name]; !ok {
			t.Fatalf("registry lost concurrent add %q: %#v", name, registry.Packs)
		}
	}
}

func TestConcurrentSameNameAddsConvergeOnOneValidInstall(t *testing.T) {
	dataDir, _, cleanup := setupInstallTestEnv(t)
	defer cleanup()
	repo := createTestGitSoundpackRepo(t)
	start := make(chan struct{})
	results := make(chan int, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results <- NewCLI().Run(
				[]string{"claudio", "soundpack", "add", repo, "--name", "same-name"},
				nil, &bytes.Buffer{}, &bytes.Buffer{},
			)
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	successes := 0
	for code := range results {
		if code == 0 {
			successes++
		}
	}
	if successes == 0 {
		t.Fatal("both concurrent same-name adds failed")
	}
	clone := filepath.Join(dataDir, "claudio", "soundpack-repos", "same-name")
	if _, err := os.Stat(filepath.Join(clone, "success", "success.wav")); err != nil {
		t.Fatalf("winning clone is incomplete: %v", err)
	}
	registry, err := loadSoundpackRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if record, ok := registry.Packs["same-name"]; !ok || record.Path != clone {
		t.Fatalf("winning registry entry is missing or invalid: %#v", record)
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

func TestSoundpackRemovePreservesMalformedExplicitConfigAndManagedClone(t *testing.T) {
	dataDir, _, cleanup := setupInstallTestEnv(t)
	defer cleanup()
	repoPath := createTestGitSoundpackRepo(t)
	customConfig := filepath.Join(t.TempDir(), "custom.json")

	cli := NewCLI()
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	if code := cli.Run([]string{"claudio", "soundpack", "add", repoPath, "--name", "preserve-pack", "--config", customConfig}, nil, stdout, stderr); code != 0 {
		t.Fatalf("add failed: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	want := []byte("{recover me")
	if err := os.WriteFile(customConfig, want, 0600); err != nil {
		t.Fatal(err)
	}
	clonePath := filepath.Join(dataDir, "claudio", "soundpack-repos", "preserve-pack")
	stdout.Reset()
	stderr.Reset()
	if code := cli.Run([]string{"claudio", "soundpack", "remove", "preserve-pack", "--config", customConfig}, nil, stdout, stderr); code == 0 {
		t.Fatalf("expected malformed config error, stdout=%q stderr=%q", stdout, stderr)
	}
	got, err := os.ReadFile(customConfig)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("malformed config changed: got=%q err=%v", got, err)
	}
	if _, err := os.Stat(clonePath); err != nil {
		t.Fatalf("clone was removed before config validation: %v", err)
	}
	registry, err := loadSoundpackRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := registry.Packs["preserve-pack"]; !ok {
		t.Fatal("registry entry was removed before config validation")
	}
}

func TestSoundpackRemoveHonorsPerNameOperationLock(t *testing.T) {
	dataDir, _, cleanup := setupInstallTestEnv(t)
	defer cleanup()
	repo := createTestGitSoundpackRepo(t)
	cli := NewCLI()
	if code := cli.Run([]string{"claudio", "soundpack", "add", repo, "--name", "locked-pack"}, nil, &bytes.Buffer{}, &bytes.Buffer{}); code != 0 {
		t.Fatalf("initial add exited %d", code)
	}
	lock, err := lockSoundpackName("locked-pack")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = lock.Unlock() }()

	if code := cli.Run([]string{"claudio", "soundpack", "remove", "locked-pack"}, nil, &bytes.Buffer{}, &bytes.Buffer{}); code == 0 {
		t.Fatal("remove ignored the held per-name operation lock")
	}
	clone := filepath.Join(dataDir, "claudio", "soundpack-repos", "locked-pack")
	if _, err := os.Stat(filepath.Join(clone, "success", "success.wav")); err != nil {
		t.Fatalf("remove mutated clone while name lock was held: %v", err)
	}
	registry, err := loadSoundpackRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := registry.Packs["locked-pack"]; !ok {
		t.Fatal("remove mutated registry while name lock was held")
	}
}

func TestSoundpackRemoveRetryRepairsConfigAfterRegistryRemoval(t *testing.T) {
	dataDir, configDir, cleanup := setupInstallTestEnv(t)
	defer cleanup()
	repo := createTestGitSoundpackRepo(t)
	cli := NewCLI()
	if code := cli.Run([]string{"claudio", "soundpack", "add", repo, "--name", "retry-remove", "--default"}, nil, &bytes.Buffer{}, &bytes.Buffer{}); code != 0 {
		t.Fatalf("initial add exited %d", code)
	}
	registry, err := loadSoundpackRegistry()
	if err != nil {
		t.Fatal(err)
	}
	delete(registry.Packs, "retry-remove")
	if err := saveSoundpackRegistry(registry); err != nil {
		t.Fatal(err)
	}
	clone := filepath.Join(dataDir, "claudio", "soundpack-repos", "retry-remove")
	if err := removeManagedGitClone(clone); err != nil {
		t.Fatal(err)
	}

	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	if code := cli.Run([]string{"claudio", "soundpack", "remove", "retry-remove"}, nil, stdout, stderr); code != 0 {
		t.Fatalf("retry failed: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	cfg := loadTestConfig(t, configDir)
	if cfg.DefaultSoundpack == "retry-remove" || containsPath(cfg.SoundpackPaths, clone) {
		t.Fatalf("retry did not repair config: %+v", cfg)
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
