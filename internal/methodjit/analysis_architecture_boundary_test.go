//go:build darwin && arm64

package methodjit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

func TestProductionCodeUsesTableShapeFactsBoundary(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob methodjit files: %v", err)
	}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") || file == "analysis_result.go" {
			continue
		}
		fset := token.NewFileSet()
		parsed, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", file, err)
		}
		ast.Inspect(parsed, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok || !legacyTableShapeField(sel.Sel.Name) {
				return true
			}
			if isSelectorNamed(sel.X, "Analysis") {
				pos := fset.Position(sel.Pos())
				t.Fatalf("%s directly accesses Analysis.%s; use TableShapeFacts helpers", pos, sel.Sel.Name)
			}
			return true
		})
	}
}

func legacyTableShapeField(name string) bool {
	switch name {
	case "FieldPolyShapeFacts", "FieldPolyShapeReceivers", "FieldPolyShapeCatalog", "FieldCallPolyLenFusions":
		return true
	default:
		return false
	}
}

func isSelectorNamed(expr ast.Expr, name string) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	return ok && sel.Sel.Name == name
}
