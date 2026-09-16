package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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

	databasePath, err := tracking.GetDatabasePath()
	if err != nil {
		t.Fatalf("GetDatabasePath: %v", err)
	}
	if !integrationPathWithin(cacheRoot, databasePath) {
		t.Fatalf("tracking database path %q is outside isolated cache %q", databasePath, cacheRoot)
	}
}

func integrationPathWithin(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." &&
		!strings.HasPrefix(relative, ".."+string(filepath.Separator)) &&
		!filepath.IsAbs(relative)
}
