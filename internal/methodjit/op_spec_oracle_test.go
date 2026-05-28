package methodjit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

func TestOpSpecOracleSupportMatchesIRInterpreterCases(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	handled, err := interpExecInstrHandledOps(filepath.Join(filepath.Dir(file), "interp.go"))
	if err != nil {
		t.Fatalf("scan interp.go: %v", err)
	}

	var missing []string
	for op := Op(0); op < OpMax; op++ {
		spec, ok := op.Spec()
		if !ok {
			t.Fatalf("%d has no OpSpec", op)
		}
		switch spec.OracleSupport {
		case OpOracleExecutable, OpOracleTerminator:
			if !handled[op] {
				missing = append(missing, spec.Name)
			}
		case OpOracleUnsupported, OpOraclePseudo:
		default:
			t.Fatalf("%s has invalid oracle support %d", spec.Name, spec.OracleSupport)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Fatalf("OpSpec marks op(s) oracle-supported but interp.execInstr lacks case: %s", strings.Join(missing, ", "))
	}
}

func interpExecInstrHandledOps(path string) (map[Op]bool, error) {
	fileSet := token.NewFileSet()
	parsed, err := parser.ParseFile(fileSet, path, nil, 0)
	if err != nil {
		return nil, err
	}
	ops := make(map[Op]bool)
	for _, decl := range parsed.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "execInstr" {
			continue
		}
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			sw, ok := node.(*ast.SwitchStmt)
			if !ok {
				return true
			}
			for _, stmt := range sw.Body.List {
				cc, ok := stmt.(*ast.CaseClause)
				if !ok {
					continue
				}
				for _, expr := range cc.List {
					if ident, ok := expr.(*ast.Ident); ok {
						if ident.Name == "OpTableArrayNestedLoad" {
							ops[OpTableArrayNestedLoad] = true
							continue
						}
						if op, ok := OpByName(strings.TrimPrefix(ident.Name, "Op")); ok {
							ops[op] = true
						}
					}
				}
			}
			return false
		})
		return ops, nil
	}
	return nil, errExecInstrNotFound{}
}

type errExecInstrNotFound struct{}

func (errExecInstrNotFound) Error() string { return "execInstr not found" }
