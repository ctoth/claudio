package tracking

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/adrg/xdg"
)

// isolateCache points both the XDG cache home and os.UserCacheDir at
// separate temp directories and returns them.
func isolateCache(t *testing.T) (xdgCache, legacyCache string) {
	t.Helper()
	root := t.TempDir()
	// Registered before t.Setenv so it runs after the env is restored (#92).
	t.Cleanup(xdg.Reload)
	xdgCache = filepath.Join(root, "xdg-cache")
	t.Setenv("XDG_CACHE_HOME", xdgCache)
	if runtime.GOOS == "windows" {
		lad := filepath.Join(root, "localappdata")
		t.Setenv("LOCALAPPDATA", lad)
		legacyCache = lad
	} else {
		legacyCache = xdgCache
	}
	xdg.Reload()
	return xdgCache, legacyCache
}

// TestIsolateCacheRestoresXDGAfterTest pins issue #92: a stale xdg.CacheHome
// after isolateCache made a later GetDatabasePath migrate the developer's
// real sounds.db into the deleted temp dir.
func TestIsolateCacheRestoresXDGAfterTest(t *testing.T) {
	before := xdg.CacheHome
	t.Run("isolated", func(t *testing.T) {
		isolateCache(t)
	})
	if xdg.CacheHome != before {
		t.Errorf("xdg.CacheHome = %q after the isolated test, want %q", xdg.CacheHome, before)
	}
}

// The database must live under the same cache root as the log file
// (adrg/xdg CacheHome). os.UserCacheDir differs from it on Windows.
func TestGetDatabasePathUsesXDGCacheHome(t *testing.T) {
	xdgCache, _ := isolateCache(t)

	got, err := GetDatabasePath()
	if err != nil {
		t.Fatalf("GetDatabasePath: %v", err)
	}
	want := filepath.Join(xdgCache, "claudio", "sounds.db")
	if got != want {
		t.Errorf("GetDatabasePath = %q, want %q", got, want)
	}
}

// Existing history at the old os.UserCacheDir location is moved, with its
// WAL sidecars, the first time the new path is resolved.
func TestGetDatabasePathMigratesLegacyDatabase(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("isolateCache models the Windows legacy layout")
	}
	xdgCache, legacyCache := isolateCache(t)

	legacyDir := filepath.Join(legacyCache, "claudio")
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"sounds.db", "sounds.db-wal"} {
		if err := os.WriteFile(filepath.Join(legacyDir, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	got, err := GetDatabasePath()
	if err != nil {
		t.Fatalf("GetDatabasePath: %v", err)
	}
	if want := filepath.Join(xdgCache, "claudio", "sounds.db"); got != want {
		t.Fatalf("GetDatabasePath = %q, want %q", got, want)
	}
	for _, name := range []string{"sounds.db", "sounds.db-wal"} {
		data, err := os.ReadFile(filepath.Join(xdgCache, "claudio", name))
		if err != nil || string(data) != name {
			t.Errorf("%s not migrated: data=%q err=%v", name, data, err)
		}
		if _, err := os.Stat(filepath.Join(legacyDir, name)); !os.IsNotExist(err) {
			t.Errorf("legacy %s still present (err=%v)", name, err)
		}
	}
}

// An existing database at the new location is never overwritten.
func TestGetDatabasePathKeepsExistingDatabase(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("isolateCache models the Windows legacy layout")
	}
	xdgCache, legacyCache := isolateCache(t)
	for dir, body := range map[string]string{
		filepath.Join(legacyCache, "claudio"): "legacy",
		filepath.Join(xdgCache, "claudio"):    "current",
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "sounds.db"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	got, err := GetDatabasePath()
	if err != nil {
		t.Fatalf("GetDatabasePath: %v", err)
	}
	if data, _ := os.ReadFile(got); string(data) != "current" {
		t.Errorf("existing database at %q was replaced: %q", got, data)
	}
}

// A zero-length file at the new path holds no data (SQLite treats it as an
// empty database), so it must not block migrating real legacy history.
func TestGetDatabasePathMigratesOverEmptyPlaceholder(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("isolateCache models the Windows legacy layout")
	}
	xdgCache, legacyCache := isolateCache(t)
	legacyDir := filepath.Join(legacyCache, "claudio")
	newDir := filepath.Join(xdgCache, "claudio")
	for _, dir := range []string{legacyDir, newDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(legacyDir, "sounds.db"), []byte("history"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(newDir, "sounds.db"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := GetDatabasePath()
	if err != nil {
		t.Fatalf("GetDatabasePath: %v", err)
	}
	if data, _ := os.ReadFile(got); string(data) != "history" {
		t.Errorf("GetDatabasePath = %q with contents %q, want migrated history", got, data)
	}
}
