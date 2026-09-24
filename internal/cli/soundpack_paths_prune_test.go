package cli

import (
	"bytes"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"claudio.click/internal/config"
	"claudio.click/internal/testutil/wavfixture"
	"github.com/spf13/afero"
)

// writeSoundpackPathsConfig writes a config whose soundpack_paths lists a
// deleted pack ahead of a live directory pack, as reported in #87, and
// returns the stale and live entries.
func writeSoundpackPathsConfig(t *testing.T, dataDir, configDir string) (stale, live string) {
	t.Helper()
	stale = filepath.Join(dataDir, "claudio", "windows-custom.json")
	live = filepath.Join(dataDir, "claudio", "soundpacks", "pack")
	wavfixture.Write(t, filepath.Join(live, "default.wav"))

	cfg := config.NewConfigManager().GetDefaultConfig()
	cfg.DefaultSoundpack = "pack"
	cfg.SoundpackPaths = []string{stale, live}
	configPath := filepath.Join(configDir, "claudio", "config.json")
	if err := config.WriteConfigFile(afero.NewOsFs(), configPath, cfg); err != nil {
		t.Fatal(err)
	}
	return stale, live
}

func TestSoundpackInstallPrunesMissingSoundpackPaths(t *testing.T) {
	dataDir, configDir, _ := setupInstallTestEnv(t)
	stale, live := writeSoundpackPathsConfig(t, dataDir, configDir)
	jsonPath := createTestJSONSoundpack(t, t.TempDir(), "fresh-pack")

	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	if code := NewCLI().Run([]string{"claudio", "soundpack", "install", jsonPath}, nil, stdout, stderr); code != 0 {
		t.Fatalf("install failed: stdout=%q stderr=%q", stdout, stderr)
	}

	cfg := loadTestConfig(t, configDir)
	installed := filepath.Join(dataDir, "claudio", "soundpacks", "fresh-pack", "soundpack.json")
	if want := []string{live, installed}; !slices.Equal(cfg.SoundpackPaths, want) {
		t.Fatalf("soundpack_paths = %v, want %v", cfg.SoundpackPaths, want)
	}
	if !strings.Contains(stdout.String(), stale) {
		t.Fatalf("install did not report pruning %s: stdout=%q", stale, stdout)
	}
}

func TestSoundpackUsePrunesMissingSoundpackPaths(t *testing.T) {
	dataDir, configDir, _ := setupInstallTestEnv(t)
	stale, live := writeSoundpackPathsConfig(t, dataDir, configDir)

	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	if code := NewCLI().Run([]string{"claudio", "soundpack", "use", "pack"}, nil, stdout, stderr); code != 0 {
		t.Fatalf("use failed: stdout=%q stderr=%q", stdout, stderr)
	}

	cfg := loadTestConfig(t, configDir)
	if want := []string{live}; !slices.Equal(cfg.SoundpackPaths, want) {
		t.Fatalf("soundpack_paths = %v, want %v", cfg.SoundpackPaths, want)
	}
	if !strings.Contains(stdout.String(), stale) {
		t.Fatalf("use did not report pruning %s: stdout=%q", stale, stdout)
	}
}

func TestSoundpackAddPrunesMissingSoundpackPaths(t *testing.T) {
	dataDir, configDir, _ := setupInstallTestEnv(t)
	stale, live := writeSoundpackPathsConfig(t, dataDir, configDir)
	repoPath := createTestGitSoundpackRepo(t)

	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	if code := NewCLI().Run([]string{"claudio", "soundpack", "add", repoPath, "--name", "git-pack"}, nil, stdout, stderr); code != 0 {
		t.Fatalf("add failed: stdout=%q stderr=%q", stdout, stderr)
	}

	cfg := loadTestConfig(t, configDir)
	clone := filepath.Join(dataDir, "claudio", "soundpack-repos", "git-pack")
	if containsPath(cfg.SoundpackPaths, stale) || !containsPath(cfg.SoundpackPaths, live) || !containsPath(cfg.SoundpackPaths, clone) {
		t.Fatalf("soundpack_paths = %v, want %s and %s without %s", cfg.SoundpackPaths, live, clone, stale)
	}
}

// TestStaleSoundpackPathIsQuietAtHookTime guards the hook path: a missing
// soundpack_paths entry is skipped without a warning or error, and the hook
// never rewrites the config to drop it.
func TestStaleSoundpackPathIsQuietAtHookTime(t *testing.T) {
	dataDir, configDir, _ := setupInstallTestEnv(t)
	stale, _ := writeSoundpackPathsConfig(t, dataDir, configDir)

	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelWarn})))
	defer slog.SetDefault(previous)

	cfg := loadTestConfig(t, configDir)
	cfg.Enabled = false
	c := NewCLI()
	if err := c.initializeAudioSystem(cfg); err != nil {
		t.Fatalf("initializeAudioSystem: %v", err)
	}
	if _, err := c.soundpackResolver.ResolveSound("default.wav"); err != nil {
		t.Fatalf("live pack did not resolve: %v", err)
	}
	if strings.Contains(logs.String(), filepath.Base(stale)) {
		t.Fatalf("stale soundpack_paths entry logged at warn or above:\n%s", logs.String())
	}
	if !containsPath(loadTestConfig(t, configDir).SoundpackPaths, stale) {
		t.Fatal("hook path rewrote config soundpack_paths")
	}
}

func TestSoundpackRemovePrunesMissingSoundpackPaths(t *testing.T) {
	dataDir, configDir, _ := setupInstallTestEnv(t)
	repoPath := createTestGitSoundpackRepo(t)
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	if code := NewCLI().Run([]string{"claudio", "soundpack", "add", repoPath, "--name", "git-pack"}, nil, stdout, stderr); code != 0 {
		t.Fatalf("add failed: stdout=%q stderr=%q", stdout, stderr)
	}
	stale := filepath.Join(dataDir, "claudio", "windows-custom.json")
	cfg := loadTestConfig(t, configDir)
	cfg.SoundpackPaths = append([]string{stale}, cfg.SoundpackPaths...)
	if err := config.WriteConfigFile(afero.NewOsFs(), filepath.Join(configDir, "claudio", "config.json"), cfg); err != nil {
		t.Fatal(err)
	}

	stdout.Reset()
	stderr.Reset()
	if code := NewCLI().Run([]string{"claudio", "soundpack", "remove", "git-pack"}, nil, stdout, stderr); code != 0 {
		t.Fatalf("remove failed: stdout=%q stderr=%q", stdout, stderr)
	}

	if paths := loadTestConfig(t, configDir).SoundpackPaths; len(paths) != 0 {
		t.Fatalf("soundpack_paths = %v, want none", paths)
	}
	if !strings.Contains(stdout.String(), stale) {
		t.Fatalf("remove did not report pruning %s: stdout=%q", stale, stdout)
	}
}
