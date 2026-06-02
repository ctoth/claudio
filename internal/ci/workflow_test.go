package ci_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkflowsUseModuleGoVersion(t *testing.T) {
	root := repoRoot(t)
	for _, rel := range []string{
		".github/workflows/ci.yml",
		".github/workflows/release.yml",
	} {
		t.Run(rel, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(root, rel))
			if err != nil {
				t.Fatalf("read workflow: %v", err)
			}

			text := string(data)
			if strings.Contains(text, `go-version: "1.23"`) {
				t.Fatalf("workflow pins stale Go 1.23 instead of reading go.mod")
			}
			if !strings.Contains(text, "go-version-file: go.mod") {
				t.Fatalf("workflow does not read Go version from go.mod")
			}
		})
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root")
		}
		dir = parent
	}
}
