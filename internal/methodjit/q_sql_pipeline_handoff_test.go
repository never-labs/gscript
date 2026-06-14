//go:build darwin && arm64

package methodjit

import (
	"errors"
	"testing"

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
}

func assertQSQLKernelExecutionStat(t *testing.T, rows []QKernelExecutionStat, ref QSQLKernelPipelineRef, outcome string, count uint64) {
	t.Helper()
	ref = ref.normalized(QSQLKernelRuntimeSource)
	for _, row := range rows {
		if row.Source == QSQLKernelRuntimeSource &&
			row.Kernel == ref.Kernel &&
			row.Shape == ref.Shape &&
			row.PipelineShape == ref.PipelineShape &&
			row.Route == "typed_runtime_op_exit" &&
			row.Outcome == outcome &&
			row.Count == count {
			return
		}
	}
	t.Fatalf("missing qSQL execution stat for outcome=%s count=%d ref=%+v rows=%+v", outcome, count, ref, rows)
}

func assertQSQLKernelDescriptorCacheStat(t *testing.T, rows []QKernelDescriptorCacheStat, ref QSQLKernelPipelineRef, entries, hits, misses, evictions uint64) {
	t.Helper()
	ref = ref.normalized(QSQLKernelRuntimeSource)
	for _, row := range rows {
		if row.Source == QSQLKernelRuntimeSource &&
			row.Kernel == ref.Kernel &&
			row.Shape == ref.Shape &&
			row.PipelineShape == ref.PipelineShape &&
			row.Route == "typed_runtime_op_exit" &&
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
