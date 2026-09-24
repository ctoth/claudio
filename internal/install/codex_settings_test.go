package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindCodexHooksPathsUserScope(t *testing.T) {
	t.Setenv("CODEX_HOME", "")

	paths, err := AgentCodex.ConfigPaths("global")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("expected at least one global-scope path")
	}
	want := filepath.Join(".codex", "hooks.json")
	for _, p := range paths {
		if !strings.HasSuffix(p, want) {
			t.Errorf("path %q does not end with %q", p, want)
		}
	}
}

func TestFindCodexHooksPathsLegacyUserScope(t *testing.T) {
	t.Setenv("CODEX_HOME", "")

	paths, err := AgentCodex.ConfigPaths("user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("expected at least one legacy user-scope path")
	}
	want := filepath.Join(".codex", "hooks.json")
	for _, p := range paths {
		if !strings.HasSuffix(p, want) {
			t.Errorf("path %q does not end with %q", p, want)
		}
	}
}

func TestFindCodexHooksPathsUserScopeHonorsCODEXHOME(t *testing.T) {
	codexHome := t.TempDir()
	t.Setenv("CODEX_HOME", codexHome)

	paths, err := AgentCodex.ConfigPaths("user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("expected at least one user-scope path")
	}

	want := filepath.Join(codexHome, "hooks.json")
	if paths[0] != want {
		t.Fatalf("first path = %q, want CODEX_HOME path %q", paths[0], want)
	}
}

func TestFindCodexHooksPathsProjectScope(t *testing.T) {
	paths, err := AgentCodex.ConfigPaths("project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("expected at least one project-scope path")
	}
	if !strings.Contains(paths[0], filepath.Join(".codex", "hooks.json")) {
		t.Errorf("project path %q missing .codex/hooks.json", paths[0])
	}
}

func TestFindCodexHooksPathsInvalidScope(t *testing.T) {
	if _, err := AgentCodex.ConfigPaths("bogus"); err == nil {
		t.Error("expected error for invalid scope, got nil")
	}
}

func TestFindBestCodexPathReturnsFirstWhenNoneExist(t *testing.T) {
	got, err := AgentCodex.BestConfigPath("user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == "" {
		t.Fatal("expected a non-empty path")
	}
}

func TestFindBestCodexPathPrefersExistingFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODEX_HOME", "")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")

	hooksPath := filepath.Join(home, ".codex", "hooks.json")
	if err := os.MkdirAll(filepath.Dir(hooksPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hooksPath, []byte(`{"hooks":{}}`), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := AgentCodex.BestConfigPath("global")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != hooksPath {
		t.Fatalf("FindBestCodexPath = %q, want %q", got, hooksPath)
	}
}

func TestFindBestCodexPathInvalidScope(t *testing.T) {
	if _, err := AgentCodex.BestConfigPath("bogus"); err == nil {
		t.Error("expected error for invalid scope")
	}
}

// Codex reads its config only from CODEX_HOME when that is set, so an
// existing ~/.codex/hooks.json must not win over a not-yet-created
// $CODEX_HOME/hooks.json.
func TestFindBestCodexPathPrefersCodexHomeOverExistingDefault(t *testing.T) {
	home := t.TempDir()
	codexHome := filepath.Join(t.TempDir(), "codex-home")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("CODEX_HOME", codexHome)

	legacy := filepath.Join(home, ".codex", "hooks.json")
	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := AgentCodex.BestConfigPath("global")
	if err != nil {
		t.Fatalf("FindBestCodexPath: %v", err)
	}
	want := filepath.Join(codexHome, "hooks.json")
	if got != want {
		t.Errorf("FindBestCodexPath = %q, want %q (CODEX_HOME must be authoritative)", got, want)
	}
}
