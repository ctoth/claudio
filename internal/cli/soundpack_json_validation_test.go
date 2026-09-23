package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"claudio.click/internal/soundpack"
	"claudio.click/internal/soundpack/gitpack"
)

func writeJSONPack(t *testing.T, dir string, mappings map[string]string) string {
	t.Helper()
	data, err := json.Marshal(soundpack.JSONSoundpackFile{Name: "json-pack", Version: "1", Mappings: mappings})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "soundpack.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func createTestGitJSONSoundpackRepo(t *testing.T, mappings map[string]string) string {
	t.Helper()
	repo := createTestGitSoundpackRepo(t)
	writeJSONPack(t, repo, mappings)
	commitTestGitRepo(t, repo, "add manifest")
	return repo
}

func TestSoundpackAddRejectsJSONPackWithMissingFile(t *testing.T) {
	_, _, cleanup := setupInstallTestEnv(t)
	defer cleanup()
	repo := createTestGitJSONSoundpackRepo(t, map[string]string{
		"success/success.wav": "success/success.wav",
		"default.wav":         "missing.wav",
	})

	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	if code := NewCLI().Run([]string{"claudio", "soundpack", "add", repo, "--name", "broken-json"}, nil, stdout, stderr); code == 0 {
		t.Fatalf("expected add of JSON pack with a missing file to fail, stdout=%q", stdout)
	}
	registry, err := gitpack.LoadRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := registry.Packs["broken-json"]; ok {
		t.Fatal("broken JSON pack was registered")
	}
}

func TestSoundpackAddRejectsJSONPackWithTraversal(t *testing.T) {
	_, _, cleanup := setupInstallTestEnv(t)
	defer cleanup()
	repo := createTestGitJSONSoundpackRepo(t, map[string]string{
		"default.wav": "../outside.wav",
	})
	createDummyWAV(t, filepath.Join(filepath.Dir(repo), "outside.wav"))

	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	if code := NewCLI().Run([]string{"claudio", "soundpack", "add", repo, "--name", "escape-json"}, nil, stdout, stderr); code == 0 {
		t.Fatalf("expected add of JSON pack with '..' mapping to fail, stdout=%q", stdout)
	}
}

func TestSoundpackInstallRejectsJSONPackWithMissingFile(t *testing.T) {
	dataDir, _, cleanup := setupInstallTestEnv(t)
	defer cleanup()
	src := filepath.Join(t.TempDir(), "src")
	createDummyWAV(t, filepath.Join(src, "ok.wav"))
	path := writeJSONPack(t, src, map[string]string{"default.wav": "ok.wav", "loading/x.wav": "gone.wav"})

	for _, args := range [][]string{
		{"claudio", "soundpack", "install", path},
		{"claudio", "soundpack", "install", path, "--skip-validate"},
	} {
		if code := NewCLI().Run(args, nil, &bytes.Buffer{}, &bytes.Buffer{}); code == 0 {
			t.Fatalf("%v: expected install of JSON pack with a missing file to fail", args)
		}
	}
	if _, err := os.Stat(filepath.Join(dataDir, "claudio", "soundpacks", "json-pack")); !os.IsNotExist(err) {
		t.Fatalf("broken pack was installed: %v", err)
	}
}

func TestSoundpackValidateRejectsTraversalAndAbsoluteMappings(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(root, "outside.wav")
	createDummyWAV(t, outside)
	for name, value := range map[string]string{"dotdot": "../outside.wav", "absolute": outside} {
		t.Run(name, func(t *testing.T) {
			path := writeJSONPack(t, filepath.Join(root, name), map[string]string{"default.wav": value})
			stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
			if code := NewCLI().Run([]string{"claudio", "soundpack", "validate", path}, nil, stdout, stderr); code == 0 {
				t.Fatalf("expected validate to reject %q, stdout=%q", value, stdout)
			}
			if !strings.Contains(stdout.String(), "Unsafe") {
				t.Errorf("expected report to list unsafe mappings, got %q", stdout)
			}
		})
	}
}
