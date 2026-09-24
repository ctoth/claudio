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

// The embedded default WAVs are generator output: config.go carries a
// generate directive, and CI regenerates them and fails on any drift.
func TestEmbeddedSoundsAreRegeneratedInCI(t *testing.T) {
	root := repoRoot(t)
	config := readRepoFile(t, root, "internal/config/config.go")
	if !strings.Contains(config, "//go:generate go -C embedded_sounds run generate.go") {
		t.Fatal("internal/config/config.go has no go:generate line for embedded_sounds/generate.go")
	}
	workflow := readRepoFile(t, root, ".github/workflows/ci.yml")
	gen := strings.Index(workflow, "go generate ./internal/config/...")
	diff := strings.Index(workflow, "git diff --exit-code")
	if gen < 0 || diff < gen {
		t.Fatal("CI does not run go generate ./internal/config/... followed by git diff --exit-code")
	}
}
