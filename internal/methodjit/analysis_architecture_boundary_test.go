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

func TestProductionCodeUsesCallFactsBoundary(t *testing.T) {
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
			if !ok || !legacyCallFactField(sel.Sel.Name) {
				return true
			}
			if isSelectorNamed(sel.X, "Analysis") {
				pos := fset.Position(sel.Pos())
				t.Fatalf("%s directly accesses Analysis.%s; use CallFacts helpers", pos, sel.Sel.Name)
			}
			return true
		})
	}
}

func TestProductionCodeUsesLoopSpecializationFactsBoundary(t *testing.T) {
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
			if !ok || !legacyLoopSpecializationFactField(sel.Sel.Name) {
				return true
			}
			if isSelectorNamed(sel.X, "Analysis") {
				pos := fset.Position(sel.Pos())
				t.Fatalf("%s directly accesses Analysis.%s; use LoopSpecializationFacts helpers", pos, sel.Sel.Name)
			}
			return true
		})
	}
}

func TestProductionCodeUsesNumericFactsBoundary(t *testing.T) {
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
			if !ok || !legacyNumericFactField(sel.Sel.Name) {
				return true
			}
			if isSelectorNamed(sel.X, "Analysis") {
				pos := fset.Position(sel.Pos())
				t.Fatalf("%s directly accesses Analysis.%s; use NumericFacts helpers", pos, sel.Sel.Name)
			}
			return true
		})
	}
}

func TestProductionCodeKeepsAnalysisFactFallbacksAtCompatibilityBoundaries(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob methodjit files: %v", err)
	}
	allowedFuncs := map[string]bool{
		"callABIAnnotationConfigWithDefaults":            true,
		"collectStableGlobalArrayElementFacts":           true,
		"callABIFeedbackCalleeProto":                     true,
		"fieldShapeCalleeProtos":                         true,
		"fieldShapeCalleeCases":                          true,
		"fieldShapeCalleeSummary":                        true,
		"fieldShapeCalleeABISummary":                     true,
		"specGuardKindSuppressed":                        true,
		"getFieldReceiverFixedShapeFact":                 true,
		"inlineConfigWithDefaults":                       true,
		"inlineFeedbackCallee":                           true,
		"inlineFeedbackFieldShapeCase":                   true,
		"fieldShapeInlineSplitEligibilitySummary":        true,
		"functionNumericFacts":                           true,
		"functionCallFacts":                              true,
		"functionSpeculationFacts":                       true,
		"functionTableShapeFacts":                        true,
		"functionLoopSpecializationFacts":                true,
		"functionGlobalFacts":                            true,
		"newPassContext":                                 true,
		"newPassContextWithAllowedDomains":               true,
		"newPassContextWithAllowedDomainsAndEnforcement": true,
	}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") || file == "analysis_result.go" || file == "pass_context.go" ||
			!analysisFactFallbackConstrainedFile(file) ||
			analysisFactFallbackBoundaryFile(file) {
			continue
		}
		fset := token.NewFileSet()
		parsed, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", file, err)
		}
		for _, decl := range parsed.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok || !isFunctionFactAccessorCall(call) {
					return true
				}
				if allowedFuncs[fn.Name.Name] {
					return true
				}
				pos := fset.Position(call.Pos())
				t.Fatalf("%s calls %s outside a compatibility boundary; pass facts explicitly through PassContext/config", pos, functionFactAccessorName(call))
				return true
			})
		}
	}
}

func TestCompilerRuntimeUsesFactAccessBoundaries(t *testing.T) {
	files, err := filepath.Glob("emit_*.go")
	if err != nil {
		t.Fatalf("glob compiler runtime files: %v", err)
	}
	files = append(files, "graph_builder.go", "interp_ops.go")
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		fset := token.NewFileSet()
		parsed, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", file, err)
		}
		ast.Inspect(parsed, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || !analysisFactAccessorName(sel.Sel.Name) {
				return true
			}
			if isSelectorNamed(sel.X, "Analysis") {
				pos := fset.Position(sel.Pos())
				t.Fatalf("%s directly calls Analysis.%s in compiler/runtime code; use function*Facts helpers", pos, sel.Sel.Name)
			}
			return true
		})
	}
}

func analysisFactFallbackConstrainedFile(file string) bool {
	base := filepath.Base(file)
	return strings.HasPrefix(base, "pass_") ||
		strings.HasPrefix(base, "tier2_optimizer_modules_")
}

func analysisFactFallbackBoundaryFile(file string) bool {
	base := filepath.Base(file)
	return strings.HasPrefix(base, "emit_") ||
		strings.HasPrefix(base, "tiering_") ||
		strings.HasPrefix(base, "facts_") ||
		base == "interp.go"
}

func legacyTableShapeField(name string) bool {
	switch name {
	case "FieldPolyShapeFacts", "FieldPolyShapeReceivers", "FieldPolyShapeCatalog", "FieldCallPolyLenFusions":
		return true
	default:
		return false
	}
}

func isFunctionFactAccessorCall(call *ast.CallExpr) bool {
	return functionFactAccessorName(call) != ""
}

func functionFactAccessorName(call *ast.CallExpr) string {
	ident, ok := call.Fun.(*ast.Ident)
	if !ok {
		return ""
	}
	switch ident.Name {
	case "functionNumericFacts",
		"functionCallFacts",
		"functionSpeculationFacts",
		"functionTableShapeFacts",
		"functionLoopSpecializationFacts",
		"functionGlobalFacts":
		return ident.Name
	default:
		return ""
	}
}

func analysisFactAccessorName(name string) bool {
	switch name {
	case "NumericFacts",
		"CallFacts",
		"SpeculationFacts",
		"TableShapeFacts",
		"LoopSpecializationFacts",
		"GlobalFacts":
		return true
	default:
		return false
	}
}

func legacyCallFactField(name string) bool {
	switch name {
	case "CallABIs":
		return true
	default:
		return false
	}
}

func legacyLoopSpecializationFactField(name string) bool {
	switch name {
	case "RecordArrayLoopSpecializations", "TableArrayUpperBoundSafe", "TableArrayLowerBoundSafe", "LoopTableArrayFacts":
		return true
	default:
		return false
	}
}

func legacyNumericFactField(name string) bool {
	switch name {
	case "Int48Safe", "IntModNonZeroDivisor", "IntModNoSignAdjust", "IntRanges", "ProfiledIntRanges", "ProfiledLenRanges", "IntNonNegative":
		return true
	default:
		return false
	}
}

func isSelectorNamed(expr ast.Expr, name string) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	return ok && sel.Sel.Name == name
}
