//go:build darwin && arm64

package methodjit

import (
	"fmt"

	"github.com/never-labs/leia/internal/runtime"
)

const (
	QSQLKernelRuntimeSource  = "methodjit_qsql_kernel_runtime"
	QSQLKernelLoweringSource = "methodjit_qsql_kernel_lowering"
	QSQLKernelName           = "QSQLQueryKernel"
	QSQLKernelRuntimeRoute   = "typed_runtime_qsql_kernel"
	QSQLKernelLoweringRoute  = "lowering"
)

// QSQLKernelPipelineRef is MethodJIT's stable metadata-only handoff for qSQL
// column pipelines. The q frontend/data layer owns parsing and
// QueryKernelPlanPipelineShape; MethodJIT only keeps enough schema-stable
// identity to diagnose, cache, and later lower the shape to typed runtime code.
type QSQLKernelPipelineRef struct {
	Kernel        string
	Shape         string
	PipelineShape string
	Route         string
	SchemaHash    string
}

// QSQLKernelBackendPlan is the executable-backend contract for a qSQL column
// pipeline. It intentionally carries only stable identity and execution route;
// the data runtime owns the frame-bound QueryKernel and may attach it behind an
// implementation-specific executor.
type QSQLKernelBackendPlan struct {
	Backend string
	Detail  string
	Ref     QSQLKernelPipelineRef
}

func (plan QSQLKernelBackendPlan) Valid() bool {
	ref := plan.Ref.normalized(QSQLKernelRuntimeSource)
	return plan.Backend != "" &&
		ref.Kernel != "" &&
		ref.Shape != "" &&
		ref.PipelineShape != "" &&
		ref.Route != ""
}

// QSQLKernelBackendExecutor is the narrow MethodJIT hook a qSQL typed runtime
// backend must satisfy. The executor receives a schema-stable backend plan and
// owns any frame/query-kernel lookup needed to run it.
type QSQLKernelBackendExecutor interface {
	ExecuteQSQLKernelBackendPlan(QSQLKernelBackendPlan) (any, bool, error)
}

func qSQLKernelBackendPlanByID(plans []QSQLKernelBackendPlan, id int) (QSQLKernelBackendPlan, bool) {
	if id < 0 || id >= len(plans) {
		return QSQLKernelBackendPlan{}, false
	}
	plan := plans[id]
	return plan, plan.Valid()
}

func (ref QSQLKernelPipelineRef) normalized(source string) QSQLKernelPipelineRef {
	if source == "" {
		source = QSQLKernelRuntimeSource
	}
	if ref.Kernel == "" {
		ref.Kernel = QSQLKernelName
	}
	if ref.Shape == "" {
		ref.Shape = "unknown"
	}
	if ref.PipelineShape == "" {
		ref.PipelineShape = "unknown"
	}
	if ref.Route == "" {
		ref.Route = QSQLKernelRuntimeRoute
	}
	if ref.SchemaHash == "" {
		ref.SchemaHash = "unknown"
	}
	return ref
}

// QSQLKernelRuntimeBackendPlan normalizes a qSQL pipeline handoff into the same
// plan-shaped boundary used by q.eval runtime pipelines. This is the stable
// MethodJIT/JIT-backend attachment point; it does not require MethodJIT to own
// qSQL frame execution.
func QSQLKernelRuntimeBackendPlan(ref QSQLKernelPipelineRef) QSQLKernelBackendPlan {
	ref = ref.normalized(QSQLKernelRuntimeSource)
	return QSQLKernelBackendPlan{
		Backend: QSQLKernelRuntimeRoute,
		Detail:  "kernel=" + ref.Kernel + "|shape=" + ref.Shape + "|pipeline=" + ref.PipelineShape,
		Ref:     ref,
	}
}

// QSQLKernelRuntimeDescriptor returns a normalized descriptor row for a qSQL
// pipeline that is ready to be called through a typed runtime/JIT backend.
func QSQLKernelRuntimeDescriptor(ref QSQLKernelPipelineRef) QKernelDescriptor {
	ref = ref.normalized(QSQLKernelRuntimeSource)
	return QKernelDescriptor{
		Source:        QSQLKernelRuntimeSource,
		Kind:          "runtime_kernel",
		Kernel:        ref.Kernel,
		Shape:         ref.Shape,
		PipelineShape: ref.PipelineShape,
		Route:         ref.Route,
		Outcome:       "supported",
	}
}

// QSQLKernelRuntimeDescriptorFromBackendPlan returns the diagnostic descriptor
// for a backend plan after enforcing the route selected by the executor.
func QSQLKernelRuntimeDescriptorFromBackendPlan(plan QSQLKernelBackendPlan) QKernelDescriptor {
	ref := plan.Ref.normalized(QSQLKernelRuntimeSource)
	if plan.Backend != "" {
		ref.Route = plan.Backend
	}
	return QSQLKernelRuntimeDescriptor(ref)
}

// QSQLKernelLoweringDescriptor returns a normalized lowering decision row. Use
// reason fields for unsupported qSQL shapes so cache_stats can quantify and
// retire fallback families over time.
func QSQLKernelLoweringDescriptor(ref QSQLKernelPipelineRef, outcome, reasonFamily, reasonCode string) QKernelDescriptor {
	ref = ref.normalized(QSQLKernelLoweringSource)
	if ref.Route == QSQLKernelRuntimeRoute {
		ref.Route = QSQLKernelLoweringRoute
	}
	if outcome == "" {
		outcome = "fallback"
	}
	if reasonFamily == "" {
		reasonFamily = "lowering"
	}
	return QKernelDescriptor{
		Source:        QSQLKernelLoweringSource,
		Kind:          "fallback",
		Kernel:        ref.Kernel,
		Shape:         ref.Shape,
		PipelineShape: ref.PipelineShape,
		Route:         ref.Route,
		Outcome:       outcome,
		ReasonFamily:  reasonFamily,
		ReasonCode:    reasonCode,
	}
}

// QSQLKernelDescriptorCacheStat packages schema-stable qSQL kernel cache
// counters with both semantic shape and column-pipeline shape. It mirrors the
// runtime descriptor cache stat row without requiring bind to import MethodJIT.
func QSQLKernelDescriptorCacheStat(ref QSQLKernelPipelineRef, entries, hits, misses, evictions uint64) QKernelDescriptorCacheStat {
	ref = ref.normalized(QSQLKernelRuntimeSource)
	return QKernelDescriptorCacheStat{
		Source:        QSQLKernelRuntimeSource,
		Kernel:        ref.Kernel,
		Shape:         ref.Shape,
		PipelineShape: ref.PipelineShape,
		Route:         ref.Route,
		SchemaHash:    ref.SchemaHash,
		Entries:       entries,
		Hits:          hits,
		Misses:        misses,
		Evictions:     evictions,
	}
}

// RecordQSQLKernelDescriptorCacheLookup records a qSQL typed-runtime handoff
// lookup on a compiled function. This is intentionally metadata-only: it does
// not execute qSQL, but it gives the future qSQL JIT backend the same
// observable descriptor-cache channel as existing q runtime kernels.
func (cf *CompiledFunction) RecordQSQLKernelDescriptorCacheLookup(ref QSQLKernelPipelineRef) {
	if cf == nil {
		return
	}
	ref = ref.normalized(QSQLKernelRuntimeSource)
	cf.recordQKernelDescriptorCacheLookup(qKernelDescriptorCacheKey{
		source:        QSQLKernelRuntimeSource,
		kernel:        ref.Kernel,
		shape:         ref.Shape,
		pipelineShape: ref.PipelineShape,
		route:         ref.Route,
		schemaHash:    ref.SchemaHash,
	})
}

func (cf *CompiledFunction) RecordQSQLKernelPlanExecution(ref QSQLKernelPipelineRef, outcome string) {
	cf.RecordQSQLKernelPlanExecutionWithRoute(ref, "typed_runtime_op_exit", outcome)
}

func (cf *CompiledFunction) RecordQSQLKernelPlanExecutionWithRoute(ref QSQLKernelPipelineRef, route, outcome string) {
	if cf == nil {
		return
	}
	ref = ref.normalized(QSQLKernelRuntimeSource)
	if route == "" {
		route = "typed_runtime_op_exit"
	}
	cf.recordQKernelExecutionWithPipelineShape(
		QSQLKernelRuntimeSource,
		ref.Kernel,
		ref.Shape,
		ref.PipelineShape,
		route,
		outcome,
		ref.SchemaHash,
	)
}

func (cf *CompiledFunction) ExecuteQSQLKernelPlanValue(id int) (runtime.Value, bool, error) {
	return cf.ExecuteQSQLKernelPlanValueWithRoute(id, "typed_runtime_op_exit")
}

func (cf *CompiledFunction) ExecuteQSQLKernelPlanValueWithRoute(id int, route string) (runtime.Value, bool, error) {
	if cf == nil || cf.QSQLKernelBackend == nil {
		return runtime.NilValue(), false, nil
	}
	plan, ok := qSQLKernelBackendPlanByID(cf.QSQLKernelPlans, id)
	if !ok {
		return runtime.NilValue(), false, nil
	}
	out, handled, err := cf.QSQLKernelBackend.ExecuteQSQLKernelBackendPlan(plan)
	if err != nil || !handled {
		cf.RecordQSQLKernelPlanExecutionWithRoute(plan.Ref, route, "error")
		return runtime.NilValue(), handled, err
	}
	value, err := qEvalPipelineRuntimeValue(out)
	if err != nil {
		cf.RecordQSQLKernelPlanExecutionWithRoute(plan.Ref, route, "error")
		return runtime.NilValue(), false, err
	}
	cf.RecordQSQLKernelPlanExecutionWithRoute(plan.Ref, route, "success")
	return value, true, nil
}

func (cf *CompiledFunction) executeQSQLKernelPlanSlot(planID, absSlot int, regs []runtime.Value) error {
	return cf.executeQSQLKernelPlanSlotWithRoute(planID, absSlot, regs, "typed_runtime_op_exit")
}

func (cf *CompiledFunction) executeQSQLKernelPlanSlotWithRoute(planID, absSlot int, regs []runtime.Value, route string) error {
	if absSlot < 0 || absSlot >= len(regs) {
		return fmt.Errorf("QSQLKernelPlan op-exit out of register range")
	}
	out, handled, err := cf.ExecuteQSQLKernelPlanValueWithRoute(planID, route)
	if err != nil {
		return err
	}
	if !handled {
		return fmt.Errorf("QSQLKernelPlan op-exit plan %d was not handled", planID)
	}
	regs[absSlot] = out
	return nil
}
