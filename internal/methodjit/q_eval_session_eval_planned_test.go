//go:build darwin && arm64 && qextension && leia_q

package methodjit

import (
	"fmt"
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

func TestQEvalSessionEvalLoweringRequiresTypedRuntimeBackendPlan(t *testing.T) {
	const typedSource = "+/til 8"
	if plan, ok := stdq.DescribeEvalPipelineBackendPlan(typedSource); !ok || !plan.Valid() || plan.Backend != stdq.EvalPipelineTypedRuntimeBackend {
		t.Fatalf("typed test source has no typed-runtime backend plan: plan=%#v ok=%v", plan, ok)
	}
	const untypedSource = "`a`b!10 20"
	if !stdq.EvalSourceCacheable(untypedSource) {
		t.Fatalf("untyped test source must be cacheable to isolate the backend-plan gate")
	}
	if plan, ok := stdq.DescribeEvalPipelineBackendPlan(untypedSource); ok && plan.Valid() && plan.Backend == stdq.EvalPipelineTypedRuntimeBackend {
		t.Fatalf("untyped test source unexpectedly has typed-runtime backend plan: %#v", plan)
	}

	for _, tc := range []struct {
		name        string
		receiver    string
		source      string
		wantLowered bool
	}{
		{name: "session typed source lowers", receiver: "q.session()", source: typedSource, wantLowered: true},
		{name: "workspace typed source lowers", receiver: "q.workspace()", source: typedSource, wantLowered: true},
		{name: "session non typed source stays call", receiver: "q.session()", source: untypedSource, wantLowered: false},
		{name: "workspace non typed source stays call", receiver: "q.workspace()", source: untypedSource, wantLowered: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			proto := compile(t, fmt.Sprintf(`
func run() {
	qs := %s
	return qs.eval(%q)
}
`, tc.receiver, tc.source))
			fn, _, err := RunTier2Pipeline(BuildGraph(proto), nil)
			if err != nil {
				t.Fatalf("RunTier2Pipeline: %v", err)
			}
			counts := countOps(fn)
			if tc.wantLowered {
				if got := counts[OpQEvalSessionEval]; got != 1 {
					t.Fatalf("OpQEvalSessionEval count = %d, want 1\n%s", got, Print(fn))
				}
				return
			}
			if got := counts[OpQEvalSessionEval]; got != 0 {
				t.Fatalf("OpQEvalSessionEval count = %d, want 0\n%s", got, Print(fn))
			}
			if got := counts[OpCall]; got == 0 {
				t.Fatalf("OpCall count = %d, want q.session/eval calls preserved\n%s", got, Print(fn))
			}
		})
	}
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
	site := cf.QEvalSessionEvalSites[7]
	if got := site.stats.plannedSuccess.Load(); got != 3 {
		t.Fatalf("site planned success counter = %d, want 3", got)
	}
	if got := site.stats.success.Load(); got != 0 {
		t.Fatalf("site shell success counter = %d, want 0 (all executions must take the planned route)", got)
	}
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
	if got := site.stats.plannedSuccess.Load(); got != 4 {
		t.Fatalf("site planned success counter after receiver switch = %d, want 4", got)
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

func TestExecuteQEvalSessionEvalStatsUseSiteBackendShape(t *testing.T) {
	receiver := qSessionEvalPlannedTestSession(t)
	const source = "x:til 64;y:x+1;idx:where (x mod 4)=1;+/y[idx]"
	cf := &CompiledFunction{
		Proto: &vm.FuncProto{Constants: []runtime.Value{runtime.StringValue(source)}},
		QEvalSessionEvalSites: []*qEvalSessionEvalSite{7: {
			kernel:        "QScriptPipelinePlan",
			shape:         "script-pipeline/where-index-reduce/sum-mod/assignments",
			pipelineShape: "script_pipeline",
		}},
	}

	out, err := cf.executeQEvalSessionEval(7, 0, receiver)
	if err != nil || !out.IsInt() || out.Int() != 512 {
		t.Fatalf("executeQEvalSessionEval = %s,%v; want 512,nil", out.String(), err)
	}
	for _, stat := range cf.QKernelExecutionStats() {
		if stat.Kernel == "QScriptPipelinePlan" &&
			stat.Shape == "script-pipeline/where-index-reduce/sum-mod/assignments" &&
			stat.PipelineShape == "script_pipeline" &&
			stat.Route == "session_planned_op_exit" &&
			stat.Outcome == "success" &&
			stat.Count == 1 {
			return
		}
	}
	t.Fatalf("QKernelExecutionStats missing site backend shape row: %#v", cf.QKernelExecutionStats())
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
	if got := cf.QEvalSessionEvalSites[7].stats.success.Load(); got != 1 {
		t.Fatalf("site shell success counter = %d, want 1", got)
	}
	if got := cf.QEvalSessionEvalSites[7].stats.plannedSuccess.Load(); got != 0 {
		t.Fatalf("site planned success counter = %d, want 0", got)
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

func TestExecuteQEvalSessionEvalRecordsPlannedErrorStats(t *testing.T) {
	const source = "x:til 64;+/x"
	const wantErr = "planned eval failed"
	tbl := runtime.NewTable()
	tbl.RawSetString(stdq.SessionPlannedEvalField, runtime.FunctionValue(&runtime.GoFunction{
		Name: "test.planned_eval",
		FastArg1: func(src runtime.Value) (runtime.Value, error) {
			if !src.IsString() || src.Str() != source {
				t.Fatalf("planned resolver received %s, want constant source %q", src.String(), source)
			}
			return runtime.FunctionValue(&runtime.GoFunction{
				Name: "test.planned_eval.exec",
				FastArg1: func(runtime.Value) (runtime.Value, error) {
					return runtime.NilValue(), fmt.Errorf(wantErr)
				},
			}), nil
		},
	}))
	receiver := runtime.TableValue(tbl)
	cf := &CompiledFunction{
		Proto:                 &vm.FuncProto{Constants: []runtime.Value{runtime.StringValue(source)}},
		QEvalSessionEvalSites: []*qEvalSessionEvalSite{7: {}},
	}

	out, err := cf.executeQEvalSessionEval(7, 0, receiver)
	if err == nil || err.Error() != wantErr || !out.IsNil() {
		t.Fatalf("executeQEvalSessionEval planned error = %s,%v; want nil,%q", out.String(), err, wantErr)
	}
	if got := cf.QEvalSessionEvalSites[7].stats.plannedErrors.Load(); got != 1 {
		t.Fatalf("site planned error counter = %d, want 1", got)
	}
	stats := cf.QKernelExecutionStats()
	assertQEvalSessionEvalStat(t, stats, "QEvalSessionEval", "q-eval/session-eval", "unknown", qEvalSessionEvalRoutePlanned, "error", qEvalSessionEvalReasonPlannedError, 1)
	if got := qKernelExecutionCount(stats, "methodjit_q_eval_runtime", "QEvalSessionEval", qEvalSessionEvalRouteShell, "error"); got != 0 {
		t.Fatalf("shell error route count = %d, want 0", got)
	}
}

func TestExecuteQEvalSessionEvalRecordsShellErrorStats(t *testing.T) {
	const source = "1+2"
	const wantErr = "shell eval failed"
	tbl := runtime.NewTable()
	tbl.RawSetString("eval", runtime.FunctionValue(&runtime.GoFunction{
		Name: "test.eval",
		FastArg1: func(src runtime.Value) (runtime.Value, error) {
			if !src.IsString() || src.Str() != source {
				t.Fatalf("shell eval received %s, want constant source %q", src.String(), source)
			}
			return runtime.NilValue(), fmt.Errorf(wantErr)
		},
	}))
	receiver := runtime.TableValue(tbl)
	cf := &CompiledFunction{
		Proto:                 &vm.FuncProto{Constants: []runtime.Value{runtime.StringValue(source)}},
		QEvalSessionEvalSites: []*qEvalSessionEvalSite{7: {}},
	}

	out, err := cf.executeQEvalSessionEval(7, 0, receiver)
	if err == nil || err.Error() != wantErr || !out.IsNil() {
		t.Fatalf("executeQEvalSessionEval shell error = %s,%v; want nil,%q", out.String(), err, wantErr)
	}
	if got := cf.QEvalSessionEvalSites[7].stats.errors.Load(); got != 1 {
		t.Fatalf("site shell error counter = %d, want 1", got)
	}
	if got := cf.QEvalSessionEvalSites[7].stats.plannedErrors.Load(); got != 0 {
		t.Fatalf("site planned error counter = %d, want 0", got)
	}
	stats := cf.QKernelExecutionStats()
	assertQEvalSessionEvalStat(t, stats, "QEvalSessionEval", "q-eval/session-eval", "unknown", qEvalSessionEvalRouteShell, "error", qEvalSessionEvalReasonShellError, 1)
	if got := qKernelExecutionCount(stats, "methodjit_q_eval_runtime", "QEvalSessionEval", qEvalSessionEvalRoutePlanned, "error"); got != 0 {
		t.Fatalf("planned error route count = %d, want 0", got)
	}
}

func TestQEvalSessionEvalReasonCodeBucketsRoutes(t *testing.T) {
	tests := []struct {
		name    string
		route   string
		outcome string
		want    string
	}{
		{name: "planned error", route: qEvalSessionEvalRoutePlanned, outcome: "error", want: qEvalSessionEvalReasonPlannedError},
		{name: "shell error", route: qEvalSessionEvalRouteShell, outcome: "error", want: qEvalSessionEvalReasonShellError},
		{name: "success", route: qEvalSessionEvalRoutePlanned, outcome: "success", want: "typed_kernel"},
		{name: "unknown error", route: "custom", outcome: "error", want: "runtime_error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := qEvalSessionEvalReasonCode(tt.route, tt.outcome); got != tt.want {
				t.Fatalf("qEvalSessionEvalReasonCode(%q, %q) = %q, want %q", tt.route, tt.outcome, got, tt.want)
			}
		})
	}
}

func assertQEvalSessionEvalStat(t *testing.T, stats []QKernelExecutionStat, kernel, shape, pipelineShape, route, outcome, reasonCode string, count uint64) {
	t.Helper()
	for _, stat := range stats {
		if stat.Source == "methodjit_q_eval_runtime" &&
			stat.Kernel == kernel &&
			stat.Shape == shape &&
			stat.PipelineShape == pipelineShape &&
			stat.Route == route &&
			stat.Outcome == outcome &&
			stat.ReasonCode == reasonCode {
			if stat.Count != count {
				t.Fatalf("QEvalSessionEval stat %s/%s/%s/%s count = %d, want %d; stats=%+v",
					shape, pipelineShape, route, outcome, stat.Count, count, stats)
			}
			return
		}
	}
	t.Fatalf("missing QEvalSessionEval stat kernel=%s shape=%s pipeline=%s route=%s outcome=%s reason=%s; stats=%+v",
		kernel, shape, pipelineShape, route, outcome, reasonCode, stats)
}
