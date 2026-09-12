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

func TestLintVersionIsConsistentAcrossDeveloperAndCIConfig(t *testing.T) {
	root := repoRoot(t)

	precommit := readRepoFile(t, root, ".pre-commit-config.yaml")
	workflow := readRepoFile(t, root, ".github/workflows/ci.yml")
	const lintVersion = "v2.12.2"

	if !strings.Contains(precommit, "golangci-lint@"+lintVersion) {
		t.Fatalf("pre-commit does not run golangci-lint %s", lintVersion)
	}
	if !strings.Contains(workflow, "version: "+lintVersion) {
		t.Fatalf("CI does not run golangci-lint %s", lintVersion)
	}
	if strings.Contains(precommit, "files: \\.go$") {
		t.Fatal("Go pre-commit hooks ignore go.mod, go.sum, and lint config changes")
	}
}

func TestReleaseRunsVetBeforePublishing(t *testing.T) {
	root := repoRoot(t)
	release := readRepoFile(t, root, ".github/workflows/release.yml")
	if !strings.Contains(release, "run: go vet ./...") {
		t.Fatal("release workflow does not run go vet")
	}
	if !strings.Contains(release, `expected="${GITHUB_REF_NAME#v}"`) ||
		!strings.Contains(release, `claudio version`) {
		t.Fatal("release workflow does not verify that the tag matches the binary version")
	}
}

func TestSmokeTargetsDisableTracking(t *testing.T) {
	root := repoRoot(t)
	makefile := readRepoFile(t, root, "Makefile")
	if strings.Count(makefile, "CLAUDIO_SOUND_TRACKING") < 2 {
		t.Fatal("smoke targets can write test events to the user's tracking database")
	}
}

func TestCIBuildsDocumentation(t *testing.T) {
	root := repoRoot(t)
	workflow := readRepoFile(t, root, ".github/workflows/ci.yml")
	if !strings.Contains(workflow, "uses: actions/jekyll-build-pages@v1") ||
		!strings.Contains(workflow, "source: ./docs") {
		t.Fatal("CI does not build the documentation site")
	}
}

func readRepoFile(t *testing.T, root, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(data)
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
