package i18n

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"testing"
)

// translationHelpers maps a helper name to the positions of its catalog keys:
// the key is the first argument, while count helpers take the noun forms after
// the number. Remaining arguments are format values, not keys.
var translationHelpers = map[string][]int{
	"t":       {0},
	"tf":      {0},
	"trKnown": {0},
	"T":       {0},
	"TF":      {0},
	"count":   {1, 2},
	"Count":   {1, 2},
}

// TestSourceKeysAreTranslated keeps the catalogs in sync with the UI: every
// literal passed to a translation helper has to be a known catalog key, so new
// screens cannot silently fall back to English.
func TestSourceKeysAreTranslated(t *testing.T) {
	required := make(map[string]bool, len(requiredCatalogKeys))
	for _, key := range requiredCatalogKeys {
		required[key] = true
	}

	dirs := []string{
		filepath.Join("..", "tui"),
		filepath.Join("..", "..", "cmd", "mclauncher"),
	}
	seen := 0
	var problems []string

	for _, dir := range dirs {
		paths, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err != nil {
			t.Fatalf("glob %s: %v", dir, err)
		}
		if len(paths) == 0 {
			t.Fatalf("no Go files found in %s", dir)
		}
		for _, path := range paths {
			if filepath.Ext(path) != ".go" || isTestFile(path) {
				continue
			}
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if err != nil {
				t.Fatalf("parse %s: %v", path, err)
			}
			ast.Inspect(file, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				positions, ok := translationHelpers[sel.Sel.Name]
				if !ok {
					return true
				}
				for _, position := range positions {
					if position >= len(call.Args) {
						continue
					}
					lit, ok := call.Args[position].(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING {
						continue
					}
					key, err := strconv.Unquote(lit.Value)
					if err != nil {
						continue
					}
					seen++
					if !required[key] {
						problems = append(problems, path+": "+strconv.Quote(key))
					}
				}
				return true
			})
		}
	}

	if seen == 0 {
		t.Fatal("no translation calls were found; the helper list is out of date")
	}
	for _, problem := range problems {
		t.Errorf("key is not in requiredCatalogKeys: %s", problem)
	}
}

func isTestFile(path string) bool {
	return len(path) > len("_test.go") && path[len(path)-len("_test.go"):] == "_test.go"
}
