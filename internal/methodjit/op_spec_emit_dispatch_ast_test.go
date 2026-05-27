//go:build darwin && arm64

package methodjit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestScratchFPRCachePreserveHelperUsesBackendPolicy(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "emit_dispatch.go", nil, 0)
	if err != nil {
		t.Fatalf("parse emit_dispatch.go: %v", err)
	}

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "instrPreservesScratchFPRCache" {
			continue
		}

		foundGeneralPolicy := false
		foundFloatResultPolicy := false
		foundSwitch := false
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.SwitchStmt:
				foundSwitch = true
			case *ast.Ident:
				switch node.Name {
				case "OpBackendPreservesScratchFPRCache":
					foundGeneralPolicy = true
				case "OpBackendPreservesScratchFPRCacheForFloatResult":
					foundFloatResultPolicy = true
				}
			}
			return true
		})
		if foundSwitch {
			t.Fatalf("instrPreservesScratchFPRCache should be policy-based, not an op switch")
		}
		if !foundGeneralPolicy || !foundFloatResultPolicy {
			t.Fatalf("instrPreservesScratchFPRCache does not reference scratch FPR backend policies")
		}
		return
	}

	t.Fatalf("instrPreservesScratchFPRCache not found")
}
