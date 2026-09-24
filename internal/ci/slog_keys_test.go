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
	if !ok || len(call.Args) == 0 {
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

// TestNoLogAndReturn: an error is logged once, where it is handled. A
// function that returns an error must not also log it on the way out:
// slog.Error right before returning an error, or any slog call that logs
// the very error variable it then returns, reports one failure twice.
func TestNoLogAndReturn(t *testing.T) {
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
				block, ok := n.(*ast.BlockStmt)
				if !ok {
					return true
				}
				for i := 0; i+1 < len(block.List); i++ {
					call := slogCallStmt(block.List[i])
					ret, ok := block.List[i+1].(*ast.ReturnStmt)
					if call == nil || !ok || len(ret.Results) == 0 {
						continue
					}
					last := ret.Results[len(ret.Results)-1]
					if !isErrorExpr(last) {
						continue
					}
					level := call.Fun.(*ast.SelectorExpr).Sel.Name
					if strings.HasPrefix(level, "Error") || sharesErrIdent(call, last) {
						bad = append(bad, fset.Position(call.Pos()).String())
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
		t.Errorf("error logged and then returned (log it where it is handled): %s", b)
	}
}

func slogCallStmt(stmt ast.Stmt) *ast.CallExpr {
	expr, ok := stmt.(*ast.ExprStmt)
	if !ok {
		return nil
	}
	call, ok := expr.X.(*ast.CallExpr)
	if !ok || !isSlogLogCall(call) {
		return nil
	}
	return call
}

// sharesErrIdent reports whether an err-like identifier (err, lastErr,
// closeErr, ...) appears both in the slog call and in the returned value.
func sharesErrIdent(call *ast.CallExpr, ret ast.Expr) bool {
	logged := map[string]bool{}
	for _, arg := range call.Args {
		ast.Inspect(arg, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok {
				logged[id.Name] = true
			}
			return true
		})
	}
	found := false
	ast.Inspect(ret, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && logged[id.Name] && (id.Name == "err" || strings.HasSuffix(id.Name, "Err")) {
			found = true
		}
		return true
	})
	return found
}

// isErrorExpr reports whether e syntactically looks like a non-nil error:
// fmt.Errorf/errors.New/errors.Join, an err-named variable, or an Err*
// sentinel.
func isErrorExpr(e ast.Expr) bool {
	switch v := e.(type) {
	case *ast.CallExpr:
		if sel, ok := v.Fun.(*ast.SelectorExpr); ok {
			if pkg, ok := sel.X.(*ast.Ident); ok {
				return (pkg.Name == "fmt" && sel.Sel.Name == "Errorf") ||
					(pkg.Name == "errors" && (sel.Sel.Name == "New" || sel.Sel.Name == "Join"))
			}
		}
	case *ast.Ident:
		return v.Name == "err" || strings.HasSuffix(v.Name, "Err") || strings.HasPrefix(v.Name, "Err")
	case *ast.SelectorExpr:
		return strings.HasPrefix(v.Sel.Name, "Err")
	}
	return false
}
