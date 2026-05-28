package methodjit

import (
	"errors"
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

func TestOpSpecOracleUnsupportedOpsHaveReasons(t *testing.T) {
	for op := Op(0); op < OpMax; op++ {
		spec, ok := op.Spec()
		if !ok {
			t.Fatalf("%d has no OpSpec", op)
		}
		if spec.OracleSupport == OpOracleUnsupported && spec.OracleUnsupportedReason == "" {
			t.Fatalf("%s is oracle-unsupported without a reason", spec.Name)
		}
		if spec.OracleSupport != OpOracleUnsupported && spec.OracleUnsupportedReason != "" {
			t.Fatalf("%s has oracle unsupported reason %q but support is %s", spec.Name, spec.OracleUnsupportedReason, spec.OracleSupport)
		}
	}
}

func TestValidateOracleSupportRejectsUnsupportedOpsWithReasons(t *testing.T) {
	fn := &Function{
		Blocks: []*Block{{
			Instrs: []*Instr{{Op: OpYield}},
		}},
	}
	err := ValidateOracleSupport(fn)
	if !errors.Is(err, ErrOracleUnsupported) {
		t.Fatalf("ValidateOracleSupport error = %v, want ErrOracleUnsupported", err)
	}
	var unsupported *OracleUnsupportedError
	if !errors.As(err, &unsupported) {
		t.Fatalf("ValidateOracleSupport error type = %T, want *OracleUnsupportedError", err)
	}
	if got := unsupported.Reasons[OpYield]; got != "coroutine" {
		t.Fatalf("OpYield unsupported reason = %q, want coroutine", got)
	}
}

func TestClassifyOracleSupportGroupsOpsFromOpSpec(t *testing.T) {
	fn := &Function{
		Blocks: []*Block{
			{
				Instrs: []*Instr{
					{Op: OpConstInt},
					{Op: OpPhi},
					{Op: OpYield},
					{Op: OpYield},
				},
			},
			{
				Instrs: []*Instr{
					{Op: OpReturn},
				},
			},
		},
	}
	summary, err := ClassifyOracleSupport(fn)
	if err != nil {
		t.Fatalf("ClassifyOracleSupport: %v", err)
	}
	assertOracleSummaryHasOp(t, summary, OpOracleExecutable, OpConstInt)
	assertOracleSummaryHasOp(t, summary, OpOraclePseudo, OpPhi)
	assertOracleSummaryHasOp(t, summary, OpOracleUnsupported, OpYield)
	assertOracleSummaryHasOp(t, summary, OpOracleTerminator, OpReturn)
	if got := countOracleSummaryOp(summary, OpYield); got != 1 {
		t.Fatalf("OpYield classified %d times, want once", got)
	}
	if got := summary.Reasons[OpYield]; got != "coroutine" {
		t.Fatalf("OpYield reason = %q, want coroutine", got)
	}
}

func assertOracleSummaryHasOp(t *testing.T, summary OracleSupportSummary, support OpOracleSupport, op Op) {
	t.Helper()
	for _, got := range summary.BySupport[support] {
		if got == op {
			return
		}
	}
	t.Fatalf("oracle summary missing %s in %s: %v", op, support, summary.BySupport[support])
}

func countOracleSummaryOp(summary OracleSupportSummary, op Op) int {
	count := 0
	for _, ops := range summary.BySupport {
		for _, got := range ops {
			if got == op {
				count++
			}
		}
	}
	return count
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
