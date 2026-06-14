//go:build darwin && arm64

package methodjit

import (
	"errors"
	"testing"
	"unsafe"

	"github.com/never-labs/leia/internal/jit"
	"github.com/never-labs/leia/internal/runtime"
)

type testQSQLKernelBackendExecutor struct {
	out     any
	handled bool
	err     error
	seen    []QSQLKernelBackendPlan
}

func (e *testQSQLKernelBackendExecutor) ExecuteQSQLKernelBackendPlan(plan QSQLKernelBackendPlan) (any, bool, error) {
	e.seen = append(e.seen, plan)
	return e.out, e.handled, e.err
}

func TestQSQLKernelRuntimeBackendPlanNormalizesStableHandoff(t *testing.T) {
	plan := QSQLKernelRuntimeBackendPlan(QSQLKernelPipelineRef{
		Shape:         "select/where/project",
		PipelineShape: "scan=frame|where=compare_mask:column_literal|filter=index|project=column:1",
		SchemaHash:    "schema-a",
	})
	if !plan.Valid() {
		t.Fatalf("QSQLKernelRuntimeBackendPlan invalid: %#v", plan)
	}
	if plan.Backend != QSQLKernelRuntimeRoute {
		t.Fatalf("Backend = %q, want %q", plan.Backend, QSQLKernelRuntimeRoute)
	}
	if plan.Ref.Kernel != QSQLKernelName {
		t.Fatalf("Ref.Kernel = %q, want %q", plan.Ref.Kernel, QSQLKernelName)
	}
	if plan.Ref.Route != QSQLKernelRuntimeRoute {
		t.Fatalf("Ref.Route = %q, want %q", plan.Ref.Route, QSQLKernelRuntimeRoute)
	}
	if plan.Detail == "" {
		t.Fatalf("Detail empty for normalized qSQL backend plan")
	}
}

func TestQSQLKernelRuntimeDescriptorFromBackendPlanPreservesPipelineShape(t *testing.T) {
	plan := QSQLKernelBackendPlan{
		Backend: "typed_runtime_qsql_columnar",
		Ref: QSQLKernelPipelineRef{
			Kernel:        "CustomQSQLKernel",
			Shape:         "select/by/aggregate",
			PipelineShape: "scan=frame|group=column:1|aggregate=sum:1",
			Route:         QSQLKernelRuntimeRoute,
			SchemaHash:    "schema-b",
		},
	}
	descriptor := QSQLKernelRuntimeDescriptorFromBackendPlan(plan)
	if descriptor.Source != QSQLKernelRuntimeSource ||
		descriptor.Kind != "runtime_kernel" ||
		descriptor.Kernel != "CustomQSQLKernel" ||
		descriptor.Shape != "select/by/aggregate" ||
		descriptor.PipelineShape != "scan=frame|group=column:1|aggregate=sum:1" ||
		descriptor.Route != "typed_runtime_qsql_columnar" ||
		descriptor.Outcome != "supported" {
		t.Fatalf("descriptor = %#v, want backend route and qSQL pipeline fields preserved", descriptor)
	}
}

func TestQSQLKernelPlanOpExitExecutesBackendPlan(t *testing.T) {
	plan := QSQLKernelRuntimeBackendPlan(QSQLKernelPipelineRef{
		Shape:         "select/where/project",
		PipelineShape: "scan=frame|where=compare_mask:column_literal|filter=index|project=column:1",
		SchemaHash:    "schema-a",
	})
	executor := &testQSQLKernelBackendExecutor{out: int64(42), handled: true}
	cf := &CompiledFunction{
		QSQLKernelPlans:   []QSQLKernelBackendPlan{plan},
		QSQLKernelBackend: executor,
	}
	regs := []runtime.Value{runtime.NilValue()}
	ctx := &ExecContext{
		OpExitOp:   int64(OpQSQLKernelPlan),
		OpExitSlot: 0,
		OpExitAux:  0,
	}

	if err := cf.executeOpExit(ctx, regs); err != nil {
		t.Fatalf("executeOpExit(QSQLKernelPlan): %v", err)
	}
	if !regs[0].IsInt() || regs[0].Int() != 42 {
		t.Fatalf("executeOpExit(QSQLKernelPlan) = %v, want int 42", regs[0])
	}
	if len(executor.seen) != 1 || executor.seen[0].Detail != plan.Detail {
		t.Fatalf("executor plans = %+v, want one qSQL backend plan", executor.seen)
	}
	assertQSQLKernelExecutionStat(t, cf.QKernelExecutionStats(), plan.Ref, "success", 1)
	assertQSQLKernelDescriptorCacheStat(t, cf.QKernelDescriptorCacheStats(), plan.Ref, 1, 0, 1, 0)

	if err := cf.executeOpExit(ctx, regs); err != nil {
		t.Fatalf("executeOpExit(QSQLKernelPlan) second run: %v", err)
	}
	assertQSQLKernelExecutionStat(t, cf.QKernelExecutionStats(), plan.Ref, "success", 2)
	assertQSQLKernelDescriptorCacheStat(t, cf.QKernelDescriptorCacheStats(), plan.Ref, 1, 1, 1, 0)
}

func TestQSQLKernelPlanOpExitRecordsBackendError(t *testing.T) {
	plan := QSQLKernelRuntimeBackendPlan(QSQLKernelPipelineRef{
		Shape:         "select/by/aggregate",
		PipelineShape: "scan=frame|group=column:1|aggregate=sum:1",
		SchemaHash:    "schema-b",
	})
	cf := &CompiledFunction{
		QSQLKernelPlans:   []QSQLKernelBackendPlan{plan},
		QSQLKernelBackend: &testQSQLKernelBackendExecutor{handled: true, err: errors.New("boom")},
	}
	regs := []runtime.Value{runtime.NilValue()}
	ctx := &ExecContext{
		OpExitOp:   int64(OpQSQLKernelPlan),
		OpExitSlot: 0,
		OpExitAux:  0,
	}

	if err := cf.executeOpExit(ctx, regs); err == nil || err.Error() != "boom" {
		t.Fatalf("executeOpExit(QSQLKernelPlan) error = %v, want boom", err)
	}
	assertQSQLKernelExecutionStat(t, cf.QKernelExecutionStats(), plan.Ref, "error", 1)
	assertQSQLKernelExecutionReason(t, cf.QKernelExecutionStats(), plan.Ref, "typed_runtime_op_exit", "error", QSQLKernelReasonExecutionError, 1)
}

func TestTier2DirectHelperBridgeQSQLKernelPlanRecordsDirectRoute(t *testing.T) {
	plan := QSQLKernelRuntimeBackendPlan(QSQLKernelPipelineRef{
		Shape:         "select/where/project",
		PipelineShape: "scan=frame|where=compare_mask:column_literal|filter=index|project=column:1",
		SchemaHash:    "schema-direct",
	})
	executor := &testQSQLKernelBackendExecutor{out: int64(64), handled: true}
	cf := &CompiledFunction{
		QSQLKernelPlans:   []QSQLKernelBackendPlan{plan},
		QSQLKernelBackend: executor,
	}
	regs := []runtime.Value{runtime.NilValue()}
	ctx := &ExecContext{
		HelperCF:   uintptr(unsafe.Pointer(cf)),
		RegsBase:   uintptr(unsafe.Pointer(&regs[0])),
		Regs:       uintptr(unsafe.Pointer(&regs[0])),
		RegsEnd:    uintptr(unsafe.Pointer(&regs[0])) + uintptr(len(regs))*uintptr(jit.ValueSize),
		OpExitOp:   int64(OpQSQLKernelPlan),
		OpExitSlot: 0,
		OpExitAux:  0,
	}

	beforeDirect := Tier2DirectHelperCallCount()
	tier2JITHelperBridge(uintptr(unsafe.Pointer(ctx)))
	if ctx.HelperErrFlag != 0 || ctx.HelperErr != nil {
		t.Fatalf("tier2JITHelperBridge QSQLKernelPlan error flag=%d err=%v", ctx.HelperErrFlag, ctx.HelperErr)
	}
	if got := Tier2DirectHelperCallCount() - beforeDirect; got != 1 {
		t.Fatalf("QSQLKernelPlan direct helper calls = %d, want 1", got)
	}
	if !regs[0].IsInt() || regs[0].Int() != 64 {
		t.Fatalf("QSQLKernelPlan direct helper result = %v, want int 64", regs[0])
	}
	assertQSQLKernelExecutionStatWithRoute(t, cf.QKernelExecutionStats(), plan.Ref, string(qTypedRuntimeExecutionRouteDirectHelper), "success", 1)
	assertQSQLKernelDescriptorCacheStatWithRoute(t, cf.QKernelDescriptorCacheStats(), plan.Ref, string(qTypedRuntimeExecutionRouteDirectHelper), 1, 0, 1, 0)
	tier2JITHelperBridge(uintptr(unsafe.Pointer(ctx)))
	if ctx.HelperErrFlag != 0 || ctx.HelperErr != nil {
		t.Fatalf("tier2JITHelperBridge QSQLKernelPlan second run error flag=%d err=%v", ctx.HelperErrFlag, ctx.HelperErr)
	}
	assertQSQLKernelExecutionStatWithRoute(t, cf.QKernelExecutionStats(), plan.Ref, string(qTypedRuntimeExecutionRouteDirectHelper), "success", 2)
	assertQSQLKernelDescriptorCacheStatWithRoute(t, cf.QKernelDescriptorCacheStats(), plan.Ref, string(qTypedRuntimeExecutionRouteDirectHelper), 1, 1, 1, 0)
	if got := qKernelExecutionCount(cf.QKernelExecutionStats(), QSQLKernelRuntimeSource, plan.Ref.Kernel, "typed_runtime_op_exit", "success"); got != 0 {
		t.Fatalf("QSQLKernelPlan op-exit route count = %d, want 0", got)
	}
	if got := qSQLKernelDescriptorCacheCount(cf.QKernelDescriptorCacheStats(), plan.Ref, "typed_runtime_op_exit"); got != 0 {
		t.Fatalf("QSQLKernelPlan op-exit cache rows = %d, want 0", got)
	}
}

func TestTier2DirectHelperBridgeQSQLKernelPlanReportsSlotRangeError(t *testing.T) {
	cf := &CompiledFunction{}
	regs := []runtime.Value{runtime.NilValue()}
	ctx := &ExecContext{
		HelperCF:   uintptr(unsafe.Pointer(cf)),
		RegsBase:   uintptr(unsafe.Pointer(&regs[0])),
		Regs:       uintptr(unsafe.Pointer(&regs[0])),
		RegsEnd:    uintptr(unsafe.Pointer(&regs[0])) + uintptr(len(regs))*uintptr(jit.ValueSize),
		OpExitOp:   int64(OpQSQLKernelPlan),
		OpExitSlot: 1,
		OpExitAux:  0,
	}

	beforeDirect := Tier2DirectHelperCallCount()
	tier2JITHelperBridge(uintptr(unsafe.Pointer(ctx)))
	if got := Tier2DirectHelperCallCount() - beforeDirect; got != 1 {
		t.Fatalf("QSQLKernelPlan direct helper calls = %d, want 1", got)
	}
	const want = "tier2: direct helper: QSQLKernelPlan register range out of bounds"
	if ctx.HelperErrFlag != 1 || ctx.HelperErr == nil || ctx.HelperErr.Error() != want {
		t.Fatalf("QSQLKernelPlan range error flag=%d err=%v, want %q", ctx.HelperErrFlag, ctx.HelperErr, want)
	}
	if len(cf.QKernelExecutionStats()) != 0 {
		t.Fatalf("QSQLKernelPlan range error stats = %+v, want none before backend execution", cf.QKernelExecutionStats())
	}
}

func TestTier2DirectHelperBridgeQSQLKernelPlanRecordsDirectRouteBackendUnhandled(t *testing.T) {
	plan := QSQLKernelRuntimeBackendPlan(QSQLKernelPipelineRef{
		Shape:         "select/where/project",
		PipelineShape: "scan=frame|where=compare_mask:column_literal|filter=index|project=column:1",
		SchemaHash:    "schema-direct-unhandled",
	})
	executor := &testQSQLKernelBackendExecutor{handled: false}
	cf := &CompiledFunction{
		QSQLKernelPlans:   []QSQLKernelBackendPlan{plan},
		QSQLKernelBackend: executor,
	}
	regs := []runtime.Value{runtime.NilValue()}
	ctx := &ExecContext{
		HelperCF:   uintptr(unsafe.Pointer(cf)),
		RegsBase:   uintptr(unsafe.Pointer(&regs[0])),
		Regs:       uintptr(unsafe.Pointer(&regs[0])),
		RegsEnd:    uintptr(unsafe.Pointer(&regs[0])) + uintptr(len(regs))*uintptr(jit.ValueSize),
		OpExitOp:   int64(OpQSQLKernelPlan),
		OpExitSlot: 0,
		OpExitAux:  0,
	}

	tier2JITHelperBridge(uintptr(unsafe.Pointer(ctx)))
	const want = "QSQLKernelPlan op-exit plan 0 was not handled"
	if ctx.HelperErrFlag != 1 || ctx.HelperErr == nil || ctx.HelperErr.Error() != want {
		t.Fatalf("QSQLKernelPlan unhandled error flag=%d err=%v, want %q", ctx.HelperErrFlag, ctx.HelperErr, want)
	}
	if len(executor.seen) != 1 {
		t.Fatalf("QSQLKernelPlan backend executions = %d, want 1", len(executor.seen))
	}
	assertQSQLKernelExecutionStatWithRoute(t, cf.QKernelExecutionStats(), plan.Ref, string(qTypedRuntimeExecutionRouteDirectHelper), "error", 1)
	assertQSQLKernelExecutionReason(t, cf.QKernelExecutionStats(), plan.Ref, string(qTypedRuntimeExecutionRouteDirectHelper), "error", QSQLKernelReasonPlanUnhandled, 1)
	if got := qKernelExecutionCount(cf.QKernelExecutionStats(), QSQLKernelRuntimeSource, plan.Ref.Kernel, "typed_runtime_op_exit", "error"); got != 0 {
		t.Fatalf("QSQLKernelPlan op-exit error route count = %d, want 0", got)
	}
}

func TestTier2DirectHelperBridgeQSQLKernelPlanRecordsDirectRouteConversionError(t *testing.T) {
	plan := QSQLKernelRuntimeBackendPlan(QSQLKernelPipelineRef{
		Shape:         "select/where/project",
		PipelineShape: "scan=frame|where=compare_mask:column_literal|filter=index|project=column:1",
		SchemaHash:    "schema-direct-conversion",
	})
	executor := &testQSQLKernelBackendExecutor{out: struct{}{}, handled: true}
	cf := &CompiledFunction{
		QSQLKernelPlans:   []QSQLKernelBackendPlan{plan},
		QSQLKernelBackend: executor,
	}
	regs := []runtime.Value{runtime.IntValue(7)}
	ctx := &ExecContext{
		HelperCF:   uintptr(unsafe.Pointer(cf)),
		RegsBase:   uintptr(unsafe.Pointer(&regs[0])),
		Regs:       uintptr(unsafe.Pointer(&regs[0])),
		RegsEnd:    uintptr(unsafe.Pointer(&regs[0])) + uintptr(len(regs))*uintptr(jit.ValueSize),
		OpExitOp:   int64(OpQSQLKernelPlan),
		OpExitSlot: 0,
		OpExitAux:  0,
	}

	tier2JITHelperBridge(uintptr(unsafe.Pointer(ctx)))
	const want = "methodjit: q eval pipeline result type struct {} is not runtime-value supported"
	if ctx.HelperErrFlag != 1 || ctx.HelperErr == nil || ctx.HelperErr.Error() != want {
		t.Fatalf("QSQLKernelPlan conversion error flag=%d err=%v, want %q", ctx.HelperErrFlag, ctx.HelperErr, want)
	}
	if !regs[0].IsInt() || regs[0].Int() != 7 {
		t.Fatalf("QSQLKernelPlan conversion error register = %v, want original int 7", regs[0])
	}
	assertQSQLKernelExecutionStatWithRoute(t, cf.QKernelExecutionStats(), plan.Ref, string(qTypedRuntimeExecutionRouteDirectHelper), "error", 1)
	assertQSQLKernelExecutionReason(t, cf.QKernelExecutionStats(), plan.Ref, string(qTypedRuntimeExecutionRouteDirectHelper), "error", QSQLKernelReasonExecutionError, 1)
	if got := qKernelExecutionCount(cf.QKernelExecutionStats(), QSQLKernelRuntimeSource, plan.Ref.Kernel, "typed_runtime_op_exit", "error"); got != 0 {
		t.Fatalf("QSQLKernelPlan op-exit error route count = %d, want 0", got)
	}
}

func BenchmarkQSQLKernelPlanRouteMetrics(b *testing.B) {
	for _, tc := range []struct {
		name  string
		route string
	}{
		{name: "op_exit", route: string(qTypedRuntimeExecutionRouteOpExit)},
		{name: "direct_helper", route: string(qTypedRuntimeExecutionRouteDirectHelper)},
	} {
		b.Run(tc.name, func(b *testing.B) {
			plan := QSQLKernelRuntimeBackendPlan(QSQLKernelPipelineRef{
				Shape:         "select/where/project",
				PipelineShape: "scan=frame|where=compare_mask:column_literal|filter=index|project=column:1",
				SchemaHash:    "bench-qsql-route",
			})
			cf := &CompiledFunction{
				QSQLKernelPlans:   []QSQLKernelBackendPlan{plan},
				QSQLKernelBackend: &testQSQLKernelBackendExecutor{out: int64(42), handled: true},
			}
			regs := []runtime.Value{runtime.NilValue()}
			if err := cf.executeQSQLKernelPlanSlotWithRoute(0, 0, regs, tc.route); err != nil {
				b.Fatalf("warm QSQLKernelPlan: %v", err)
			}
			before := cf.QKernelExecutionStats()
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := cf.executeQSQLKernelPlanSlotWithRoute(0, 0, regs, tc.route); err != nil {
					b.Fatalf("QSQLKernelPlan: %v", err)
				}
				if !regs[0].IsInt() || regs[0].Int() != 42 {
					b.Fatalf("QSQLKernelPlan result = %v, want int 42", regs[0])
				}
			}
			b.StopTimer()
			reportMethodJITQSQLRouteBenchmarkStats(b, b.N, qKernelExecutionStatsDelta(before, cf.QKernelExecutionStats()))
		})
	}
}

func assertQSQLKernelExecutionStat(t *testing.T, rows []QKernelExecutionStat, ref QSQLKernelPipelineRef, outcome string, count uint64) {
	t.Helper()
	assertQSQLKernelExecutionStatWithRoute(t, rows, ref, "typed_runtime_op_exit", outcome, count)
}

func assertQSQLKernelExecutionStatWithRoute(t *testing.T, rows []QKernelExecutionStat, ref QSQLKernelPipelineRef, route, outcome string, count uint64) {
	t.Helper()
	ref = ref.normalized(QSQLKernelRuntimeSource)
	for _, row := range rows {
		if row.Source == QSQLKernelRuntimeSource &&
			row.Kernel == ref.Kernel &&
			row.Shape == ref.Shape &&
			row.PipelineShape == ref.PipelineShape &&
			row.Route == route &&
			row.Outcome == outcome &&
			row.Count == count {
			return
		}
	}
	t.Fatalf("missing qSQL execution stat for outcome=%s count=%d ref=%+v rows=%+v", outcome, count, ref, rows)
}

func assertQSQLKernelExecutionReason(t *testing.T, rows []QKernelExecutionStat, ref QSQLKernelPipelineRef, route, outcome, reasonCode string, count uint64) {
	t.Helper()
	ref = ref.normalized(QSQLKernelRuntimeSource)
	for _, row := range rows {
		if row.Source == QSQLKernelRuntimeSource &&
			row.Kernel == ref.Kernel &&
			row.Shape == ref.Shape &&
			row.PipelineShape == ref.PipelineShape &&
			row.Route == route &&
			row.Outcome == outcome &&
			row.ReasonCode == reasonCode &&
			row.Count == count {
			return
		}
	}
	t.Fatalf("missing qSQL execution reason outcome=%s reason=%s count=%d ref=%+v rows=%+v", outcome, reasonCode, count, ref, rows)
}

func assertQSQLKernelDescriptorCacheStat(t *testing.T, rows []QKernelDescriptorCacheStat, ref QSQLKernelPipelineRef, entries, hits, misses, evictions uint64) {
	t.Helper()
	assertQSQLKernelDescriptorCacheStatWithRoute(t, rows, ref, "typed_runtime_op_exit", entries, hits, misses, evictions)
}

func assertQSQLKernelDescriptorCacheStatWithRoute(t *testing.T, rows []QKernelDescriptorCacheStat, ref QSQLKernelPipelineRef, route string, entries, hits, misses, evictions uint64) {
	t.Helper()
	ref = ref.normalized(QSQLKernelRuntimeSource)
	for _, row := range rows {
		if row.Source == QSQLKernelRuntimeSource &&
			row.Kernel == ref.Kernel &&
			row.Shape == ref.Shape &&
			row.PipelineShape == ref.PipelineShape &&
			row.Route == route &&
			row.SchemaHash == ref.SchemaHash &&
			row.Entries == entries &&
			row.Hits == hits &&
			row.Misses == misses &&
			row.Evictions == evictions {
			return
		}
	}
	t.Fatalf("missing qSQL descriptor cache stat ref=%+v rows=%+v", ref, rows)
}

func qSQLKernelDescriptorCacheCount(rows []QKernelDescriptorCacheStat, ref QSQLKernelPipelineRef, route string) uint64 {
	ref = ref.normalized(QSQLKernelRuntimeSource)
	var count uint64
	for _, row := range rows {
		if row.Source == QSQLKernelRuntimeSource &&
			row.Kernel == ref.Kernel &&
			row.Shape == ref.Shape &&
			row.PipelineShape == ref.PipelineShape &&
			row.Route == route &&
			row.SchemaHash == ref.SchemaHash {
			count++
		}
	}
	return count
}
