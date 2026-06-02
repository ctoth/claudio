package install

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestFindCodexHooksPathsUserScope(t *testing.T) {
	t.Setenv("CODEX_HOME", "")

	paths, err := FindCodexHooksPaths("global")
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

	paths, err := FindCodexHooksPaths("user")
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

	paths, err := FindCodexHooksPaths("user")
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
	paths, err := FindCodexHooksPaths("project")
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
	if _, err := FindCodexHooksPaths("bogus"); err == nil {
		t.Error("expected error for invalid scope, got nil")
	}
}

func TestFindBestCodexPathReturnsFirstWhenNoneExist(t *testing.T) {
	got, err := FindBestCodexPath("user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == "" {
		t.Fatal("expected a non-empty path")
	}
}

func TestFindBestCodexPathInvalidScope(t *testing.T) {
	if _, err := FindBestCodexPath("bogus"); err == nil {
		t.Error("expected error for invalid scope")
	}
}
