package application

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// TestAppErrSentinelFileLayout 约束 apperr 哨兵的声明位置：
// application 层集中在 errs.go，transport/http/<模块> 层集中在 errors.go。
func TestAppErrSentinelFileLayout(t *testing.T) {
	root := filepath.Clean("..")
	checks := []struct {
		dir      string
		fileName string
		// skipRoot 为 true 时跳过 dir 根目录下的文件，只检查子目录（模块）。
		skipRoot bool
	}{
		{dir: "application", fileName: "errs.go"},
		{dir: filepath.Join("transport", "http"), fileName: "errors.go", skipRoot: true},
	}

	for _, check := range checks {
		base := filepath.Join(root, check.dir)
		err := filepath.WalkDir(base, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			if check.skipRoot && filepath.Dir(path) == base {
				return nil
			}
			if filepath.Base(path) == check.fileName {
				return nil
			}
			for _, name := range packageLevelAppErrDeclarations(t, path) {
				t.Errorf("%s: package-level apperr sentinel %s must be declared in %s", path, name, check.fileName)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", base, err)
		}
	}
}

// packageLevelAppErrDeclarations 返回文件中以 apperr.New/apperr.NewMasked 初始化的包级变量名。
func packageLevelAppErrDeclarations(t *testing.T, path string) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	var names []string
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			continue
		}
		for _, spec := range gen.Specs {
			valueSpec, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for index, value := range valueSpec.Values {
				if !containsAppErrConstructor(value) {
					continue
				}
				name := "_"
				if index < len(valueSpec.Names) {
					name = valueSpec.Names[index].Name
				}
				names = append(names, name)
			}
		}
	}
	return names
}

func containsAppErrConstructor(expr ast.Expr) bool {
	found := false
	ast.Inspect(expr, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return !found
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return !found
		}
		pkg, ok := selector.X.(*ast.Ident)
		if ok && pkg.Name == "apperr" && (selector.Sel.Name == "New" || selector.Sel.Name == "NewMasked") {
			found = true
		}
		return !found
	})
	return found
}
