package testenv

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adrg/xdg"
)

func TestIsolateXDG_SetsAllVars(t *testing.T) {
	root := IsolateXDG(t)

	if root == "" {
		t.Fatal("IsolateXDG returned empty root")
	}

	checks := []struct {
		name  string
		got   string
		want  string
	}{
		{"HOME", os.Getenv("HOME"), root},
		{"USERPROFILE", os.Getenv("USERPROFILE"), root},
		{"XDG_CACHE_HOME", os.Getenv("XDG_CACHE_HOME"), filepath.Join(root, ".cache")},
		{"XDG_DATA_HOME", os.Getenv("XDG_DATA_HOME"), filepath.Join(root, ".local", "share")},
		{"XDG_CONFIG_HOME", os.Getenv("XDG_CONFIG_HOME"), filepath.Join(root, ".config")},
	}

	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.name, c.got, c.want)
		}
	}
}

func TestIsolateXDG_ReloadsXDG(t *testing.T) {
	root := IsolateXDG(t)

	// After IsolateXDG, xdg.CacheHome should point under the sandbox root,
	// not the developer's real cache directory.
	if !strings.HasPrefix(xdg.CacheHome, root) {
		t.Errorf("xdg.CacheHome=%q is not under sandbox root=%q", xdg.CacheHome, root)
	}
	if !strings.HasPrefix(xdg.DataHome, root) {
		t.Errorf("xdg.DataHome=%q is not under sandbox root=%q", xdg.DataHome, root)
	}
	if !strings.HasPrefix(xdg.ConfigHome, root) {
		t.Errorf("xdg.ConfigHome=%q is not under sandbox root=%q", xdg.ConfigHome, root)
	}
}

func TestIsolateXDG_TwoCallsGetSeparateRoots(t *testing.T) {
	// Each subtest gets its own t.TempDir() so its sandbox is independent.
	var root1, root2 string

	t.Run("first", func(t *testing.T) {
		root1 = IsolateXDG(t)
	})

	t.Run("second", func(t *testing.T) {
		root2 = IsolateXDG(t)
	})

	if root1 == root2 {
		t.Errorf("expected distinct sandbox roots, both = %q", root1)
	}
}
