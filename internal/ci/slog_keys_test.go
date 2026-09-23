package ci_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// errorishKey matches attribute keys that name an error ("err",
// "primary_error", "close_err", ...) without being the canonical "error".
// Boolean flags such as "has_error" are not error values and are allowed.
var errorishKey = regexp.MustCompile(`^([a-z0-9]+_)*err(or)?$`)

func isErrorFlag(key string) bool {
	return strings.HasPrefix(key, "has_") || strings.HasPrefix(key, "is_")
}

// TestSlogErrorKeyIsError keeps one attribute key for errors across the
// codebase, so `grep error=` over the log file finds every failure.
func TestSlogErrorKeyIsError(t *testing.T) {
	root := repoRoot(t)
	fset := token.NewFileSet()
	var bad []string
	for _, dir := range []string{"cmd", "internal"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			file, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return err
			}
			ast.Inspect(file, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok || !isSlogLogCall(call) {
					return true
				}
				for _, arg := range call.Args[1:] {
					lit, ok := arg.(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING {
						continue
					}
					key, _ := strconv.Unquote(lit.Value)
					if key != "error" && errorishKey.MatchString(key) && !isErrorFlag(key) {
						bad = append(bad, fset.Position(lit.Pos()).String()+": "+key)
					}
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, b := range bad {
		t.Errorf("slog error attribute must use key \"error\": %s", b)
	}
}

func isSlogLogCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || len(call.Args) < 2 {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok || pkg.Name != "slog" {
		return false
	}
	switch strings.TrimSuffix(sel.Sel.Name, "Context") {
	case "Debug", "Info", "Warn", "Error":
		return true
	}
	return false
}
