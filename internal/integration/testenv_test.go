package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adrg/xdg"

	clitestenv "claudio.click/internal/cli/testenv"
	"claudio.click/internal/tracking"
)

func isolateIntegrationXDG(t *testing.T) string {
	t.Helper()
	return clitestenv.IsolateXDG(t)
}

func TestIntegrationEnvironmentUsesTemporaryXDG(t *testing.T) {
	root := isolateIntegrationXDG(t)

	cacheRoot, err := os.UserCacheDir()
	if err != nil {
		t.Fatalf("UserCacheDir: %v", err)
	}
	if !integrationPathWithin(root, cacheRoot) {
		t.Fatalf("user cache directory %q is outside temporary root %q", cacheRoot, root)
	}

	// The tracking database shares the log file's cache root (adrg/xdg
	// CacheHome), which differs from os.UserCacheDir on Windows and on
	// macOS when XDG_CACHE_HOME is set.
	if !integrationPathWithin(root, xdg.CacheHome) {
		t.Fatalf("XDG cache home %q is outside temporary root %q", xdg.CacheHome, root)
	}
	databasePath, err := tracking.GetDatabasePath()
	if err != nil {
		t.Fatalf("GetDatabasePath: %v", err)
	}
	if !integrationPathWithin(xdg.CacheHome, databasePath) {
		t.Fatalf("tracking database path %q is outside isolated cache %q", databasePath, xdg.CacheHome)
	}
}

func integrationPathWithin(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." &&
		!strings.HasPrefix(relative, ".."+string(filepath.Separator)) &&
		!filepath.IsAbs(relative)
}
