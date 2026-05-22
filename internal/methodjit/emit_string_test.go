package methodjit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestEmitInstrDelegatesStringEmitterFamily(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "emit_dispatch.go", nil, 0)
	if err != nil {
		t.Fatalf("parse emit_dispatch.go: %v", err)
	}

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "emitInstr" {
			continue
		}
		found := false
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if ok && sel.Sel.Name == "emitStringInstr" {
				found = true
			}
			return true
		})
		if !found {
			t.Fatalf("emitInstr does not delegate to emitStringInstr")
		}
		return
	}

	t.Fatalf("emitInstr not found")
}

func TestEmitStringInstrCoversStringEmitterFamily(t *testing.T) {
	handled := emitStringInstrHandledOps(t)
	for _, op := range OpsByEmitterFamily(OpEmitterString) {
		if !handled[op] {
			t.Fatalf("emitStringInstr does not handle %s", op)
		}
		delete(handled, op)
	}
	for op := range handled {
		spec, ok := op.Spec()
		if !ok {
			t.Fatalf("emitStringInstr handles op without spec: %d", op)
		}
		if spec.EmitterFamily != OpEmitterString {
			t.Fatalf("emitStringInstr handles non-string op %s from family %d", op, spec.EmitterFamily)
		}
	}
}

func emitStringInstrHandledOps(t *testing.T) map[Op]bool {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "emit_string.go", nil, 0)
	if err != nil {
		t.Fatalf("parse emit_string.go: %v", err)
	}

	ops := make(map[Op]bool)
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "emitStringInstr" {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			clause, ok := n.(*ast.CaseClause)
			if !ok {
				return true
			}
			for _, expr := range clause.List {
				ident, ok := expr.(*ast.Ident)
				if !ok {
					continue
				}
				op, ok := stringTestOpByName(ident.Name)
				if !ok {
					t.Fatalf("emitStringInstr has unknown op case %s", ident.Name)
				}
				ops[op] = true
			}
			return true
		})
		return ops
	}

	t.Fatalf("emitStringInstr not found")
	return nil
}

func stringTestOpByName(name string) (Op, bool) {
	for op := Op(0); op < OpMax; op++ {
		if "Op"+op.String() == name {
			return op, true
		}
	}
	return 0, false
}
