//go:build darwin && arm64

package methodjit

import (
	"testing"

	"github.com/never-labs/leia/internal/runtime"
	qbind "github.com/never-labs/leia/internal/stdlib/bind"
	stdq "github.com/never-labs/leia/internal/stdlib/lib/q"
	"github.com/never-labs/leia/internal/vm"
)

func qSessionEvalPlannedTestSession(t *testing.T) runtime.Value {
	t.Helper()
	sessionFn := qbind.BuildQ().RawGetString("session").GoFunction()
	if sessionFn == nil {
		t.Fatal("q.session function missing")
	}
	out, err := sessionFn.Fn(nil)
	if err != nil {
		t.Fatalf("q.session: %v", err)
	}
	if len(out) != 1 || !out[0].IsTable() {
		t.Fatalf("q.session returned %#v, want one table", out)
	}
	return out[0]
}

// TestExecuteQEvalSessionEvalPlannedRoute pins the op-exit handler contract:
// the first execution per (site, receiver session) resolves the session's
// planned-eval handle and every execution — including the first — runs
// through the planned route (no string-eval shell), with per-execution
// results identical to the host eval function and per-route counters
// recorded on the compiled function.
func TestExecuteQEvalSessionEvalPlannedRoute(t *testing.T) {
	receiver := qSessionEvalPlannedTestSession(t)
	const source = "x:til 64;+/x"
	cf := &CompiledFunction{
		Proto:                 &vm.FuncProto{Constants: []runtime.Value{runtime.StringValue(source)}},
		QEvalSessionEvalSites: []*qEvalSessionEvalSite{7: {}},
	}

	const want = int64(2016)
	for i := 0; i < 3; i++ {
		out, err := cf.executeQEvalSessionEval(7, 0, receiver)
		if err != nil || !out.IsInt() || out.Int() != want {
			t.Fatalf("executeQEvalSessionEval #%d = %s,%v; want %d,nil", i, out.String(), err, want)
		}
	}
	if got := cf.QEvalSessionEvalStats.plannedSuccess.Load(); got != 3 {
		t.Fatalf("planned success counter = %d, want 3", got)
	}
	if got := cf.QEvalSessionEvalStats.success.Load(); got != 0 {
		t.Fatalf("shell success counter = %d, want 0 (all executions must take the planned route)", got)
	}
	site := cf.QEvalSessionEvalSites[7]
	planned := site.planned.Load()
	if planned == nil || planned.receiver != receiver.Table() {
		t.Fatalf("site cache = %#v, want pinned executor for the receiver session", planned)
	}

	// A different receiver session at the same site re-resolves and keeps its
	// own state (fresh session: x is rebound by the source itself, same sum).
	other := qSessionEvalPlannedTestSession(t)
	out, err := cf.executeQEvalSessionEval(7, 0, other)
	if err != nil || !out.IsInt() || out.Int() != want {
		t.Fatalf("executeQEvalSessionEval(other session) = %s,%v; want %d,nil", out.String(), err, want)
	}
	if replaced := site.planned.Load(); replaced == nil || replaced.receiver != other.Table() {
		t.Fatalf("site cache after receiver switch = %#v, want executor pinned to the new session", replaced)
	}
	if got := cf.QEvalSessionEvalStats.plannedSuccess.Load(); got != 4 {
		t.Fatalf("planned success counter after receiver switch = %d, want 4", got)
	}

	// Stats fold into the public rows under the planned route label.
	var plannedRow bool
	for _, stat := range cf.QKernelExecutionStats() {
		if stat.Kernel == "QEvalSessionEval" && stat.Route == "session_planned_op_exit" && stat.Outcome == "success" && stat.Count == 4 {
			plannedRow = true
		}
	}
	if !plannedRow {
		t.Fatalf("QKernelExecutionStats missing planned-route success row: %#v", cf.QKernelExecutionStats())
	}
}

// TestExecuteQEvalSessionEvalShellFallback pins the fallback: receivers
// without the reserved planned-eval resolver (plain tables exposing only an
// eval function) keep working through the host-eval shell, and sites missing
// from the compile-time table do not panic.
func TestExecuteQEvalSessionEvalShellFallback(t *testing.T) {
	const source = "1+2"
	tbl := runtime.NewTable()
	tbl.RawSetString("eval", runtime.FunctionValue(&runtime.GoFunction{
		Name: "test.eval",
		FastArg1: func(src runtime.Value) (runtime.Value, error) {
			if !src.IsString() || src.Str() != source {
				t.Fatalf("shell eval received %s, want constant source %q", src.String(), source)
			}
			return runtime.IntValue(3), nil
		},
	}))
	receiver := runtime.TableValue(tbl)
	cf := &CompiledFunction{
		Proto:                 &vm.FuncProto{Constants: []runtime.Value{runtime.StringValue(source)}},
		QEvalSessionEvalSites: []*qEvalSessionEvalSite{7: {}},
	}
	out, err := cf.executeQEvalSessionEval(7, 0, receiver)
	if err != nil || !out.IsInt() || out.Int() != 3 {
		t.Fatalf("executeQEvalSessionEval(shell receiver) = %s,%v; want 3,nil", out.String(), err)
	}
	if got := cf.QEvalSessionEvalStats.success.Load(); got != 1 {
		t.Fatalf("shell success counter = %d, want 1", got)
	}
	if got := cf.QEvalSessionEvalStats.plannedSuccess.Load(); got != 0 {
		t.Fatalf("planned success counter = %d, want 0", got)
	}

	// Unknown site ID (no compile-time site entry) still executes via shell.
	out, err = cf.executeQEvalSessionEval(99, 0, receiver)
	if err != nil || !out.IsInt() || out.Int() != 3 {
		t.Fatalf("executeQEvalSessionEval(unknown site) = %s,%v; want 3,nil", out.String(), err)
	}

	// The planned route resolver must come from the q bind layer contract.
	if _, ok := resolveQEvalSessionPlannedExec(tbl, cf.Proto.Constants, 0); ok {
		t.Fatal("resolveQEvalSessionPlannedExec resolved a table without the reserved resolver field")
	}
	session := qSessionEvalPlannedTestSession(t)
	if _, ok := resolveQEvalSessionPlannedExec(session.Table(), cf.Proto.Constants, 0); !ok {
		t.Fatalf("resolveQEvalSessionPlannedExec failed for a real q session table (field %q)", stdq.SessionPlannedEvalField)
	}
}
