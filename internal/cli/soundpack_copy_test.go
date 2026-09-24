package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func symlinkOrSkip(t *testing.T, target, link string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(link), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unsupported on this platform: %v", err)
	}
}

func TestCopyDirectoryRejectsSymlinkedFile(t *testing.T) {
	outside := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(t.TempDir(), "pack")
	createDummyWAV(t, filepath.Join(src, "default.wav"))
	symlinkOrSkip(t, outside, filepath.Join(src, "notes.txt"))
	dst := filepath.Join(t.TempDir(), "dst")

	err := copyDirectory(src, dst)
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("expected symlink rejection, got %v", err)
	}
	if data, readErr := os.ReadFile(filepath.Join(dst, "notes.txt")); readErr == nil {
		t.Fatalf("symlink target was copied: %q", data)
	}
}

func TestCopyDirectoryRejectsSymlinkedDirectory(t *testing.T) {
	outsideDir := t.TempDir()
	createDummyWAV(t, filepath.Join(outsideDir, "stolen.wav"))
	src := filepath.Join(t.TempDir(), "pack")
	createDummyWAV(t, filepath.Join(src, "default.wav"))
	symlinkOrSkip(t, outsideDir, filepath.Join(src, "success"))
	dst := filepath.Join(t.TempDir(), "dst")

	if err := copyDirectory(src, dst); err == nil {
		t.Fatal("expected symlinked directory to be rejected")
	}
	if _, err := os.Stat(filepath.Join(dst, "success", "stolen.wav")); err == nil {
		t.Fatal("symlinked directory was followed and copied")
	}
}

func TestCopyDirectorySkipsGitMetadata(t *testing.T) {
	src := filepath.Join(t.TempDir(), "pack")
	createDummyWAV(t, filepath.Join(src, "default.wav"))
	createDummyWAV(t, filepath.Join(src, ".git", "objects", "stray.wav"))
	if err := os.WriteFile(filepath.Join(src, ".git", "config"), []byte("[core]"), 0644); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(t.TempDir(), "dst")

	if err := copyDirectory(src, dst); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dst, "default.wav")); err != nil {
		t.Fatalf("regular file not copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, ".git")); !os.IsNotExist(err) {
		t.Fatalf(".git was copied: %v", err)
	}
}

func TestCopyDirectoryFollowsSymlinkedRoot(t *testing.T) {
	real := filepath.Join(t.TempDir(), "pack")
	createDummyWAV(t, filepath.Join(real, "default.wav"))
	link := filepath.Join(t.TempDir(), "link-pack")
	symlinkOrSkip(t, real, link)
	dst := filepath.Join(t.TempDir(), "dst")

	if err := copyDirectory(link, dst); err != nil {
		t.Fatalf("symlinked source root should be copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "default.wav")); err != nil {
		t.Fatalf("file under symlinked root not copied: %v", err)
	}
}

func TestCopyFileRejectsSymlink(t *testing.T) {
	outside := filepath.Join(t.TempDir(), "secret.wav")
	createDummyWAV(t, outside)
	link := filepath.Join(t.TempDir(), "tone.wav")
	symlinkOrSkip(t, outside, link)

	if err := copyFile(link, filepath.Join(t.TempDir(), "out.wav")); err == nil {
		t.Fatal("expected copyFile to reject a symlink")
	}
}

func TestSoundpackInstallDirectoryRejectsSymlink(t *testing.T) {
	dataDir, _, cleanup := setupInstallTestEnv(t)
	defer cleanup()
	outside := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(t.TempDir(), "linked-pack")
	createDummyWAV(t, filepath.Join(src, "default.wav"))
	symlinkOrSkip(t, outside, filepath.Join(src, "readme.txt"))

	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	if code := NewCLI().Run([]string{"claudio", "soundpack", "install", src}, nil, stdout, stderr); code == 0 {
		t.Fatalf("expected install of pack with symlink to fail, stdout=%q", stdout)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "claudio", "soundpacks", "linked-pack")); !os.IsNotExist(err) {
		t.Fatalf("pack with symlink was installed: %v", err)
	}
}

func TestSoundpackInstallJSONRejectsSymlinkedMappedFile(t *testing.T) {
	dataDir, _, cleanup := setupInstallTestEnv(t)
	defer cleanup()
	outside := filepath.Join(t.TempDir(), "secret.wav")
	createDummyWAV(t, outside)
	src := filepath.Join(t.TempDir(), "src")
	symlinkOrSkip(t, outside, filepath.Join(src, "tone.wav"))
	path := writeJSONPack(t, src, map[string]string{"default.wav": "tone.wav"})

	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	if code := NewCLI().Run([]string{"claudio", "soundpack", "install", path, "--skip-validate"}, nil, stdout, stderr); code == 0 {
		t.Fatalf("expected install with symlinked mapped file to fail, stdout=%q", stdout)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "claudio", "soundpacks", "json-pack")); !os.IsNotExist(err) {
		t.Fatalf("pack with symlinked file was installed: %v", err)
	}
}
