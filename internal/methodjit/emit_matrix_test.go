package methodjit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestEmitMatrixInstrCoversMatrixEmitterFamily(t *testing.T) {
	handled := emitMatrixInstrHandledOps(t)
	for _, op := range OpsByEmitterFamily(OpEmitterMatrix) {
		if !handled[op] {
			t.Fatalf("emitMatrixInstr does not handle %s", op)
		}
		delete(handled, op)
	}
	for op := range handled {
		spec, ok := op.Spec()
		if !ok {
			t.Fatalf("emitMatrixInstr handles op without spec: %d", op)
		}
		if spec.EmitterFamily != OpEmitterMatrix {
			t.Fatalf("emitMatrixInstr handles non-matrix op %s from family %d", op, spec.EmitterFamily)
		}
	}
}

func emitMatrixInstrHandledOps(t *testing.T) map[Op]bool {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "emit_matrix.go", nil, 0)
	if err != nil {
		t.Fatalf("parse emit_matrix.go: %v", err)
	}

	ops := make(map[Op]bool)
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "emitMatrixInstr" {
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
				op, ok := opByName(ident.Name)
				if !ok {
					t.Fatalf("emitMatrixInstr has unknown op case %s", ident.Name)
				}
				ops[op] = true
			}
			return true
		})
		return ops
	}

	t.Fatalf("emitMatrixInstr not found")
	return nil
}

func opByName(name string) (Op, bool) {
	for op := Op(0); op < OpMax; op++ {
		if "Op"+op.String() == name {
			return op, true
		}
	}
	return 0, false
}
