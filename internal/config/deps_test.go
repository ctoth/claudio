package config

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// config sits below the audio stack: it must not import internal/audio (or
// anything under it). Shared rules such as volume validation live in leaf
// packages both can import.
func TestConfigDoesNotImportAudio(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		f, err := parser.ParseFile(fset, name, src, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range f.Imports {
			path, _ := strconv.Unquote(imp.Path.Value)
			if path == "claudio.click/internal/audio" || strings.HasPrefix(path, "claudio.click/internal/audio/") {
				t.Errorf("%s imports %s; config must not depend on the audio stack", name, path)
			}
		}
	}
}
