package methodjit

import (
	"fmt"

	"github.com/never-labs/leia/internal/runtime"
)

// QEvalPipelineBackend is the stable handoff point between MethodJIT q.eval
// hot-path recognition and the q typed pipeline backend. The current slice only
// records a backend-facing plan reference; later lowering can route an
// OpQEvalVectorPlan or native op-exit through this interface without teaching
// MethodJIT q evaluator internals.
type QEvalPipelineBackend interface {
	BackendName() string
	LookupQEvalPipelinePlan(ref QEvalPipelinePlanRef) (QEvalPipelinePlan, bool)
}

// QEvalPipelineExecutor is the execution-side extension for backends that can
// run a plan ref through the q typed runtime without re-entering generic q.eval.
type QEvalPipelineExecutor interface {
	ExecuteQEvalPipelinePlan(ref QEvalPipelinePlanRef) (any, bool, error)
}

// QEvalPipelinePlan is intentionally metadata-only in MethodJIT. Ownership of
// q AST normalization, schema binding, and kernel dispatch stays behind the
// backend boundary.
type QEvalPipelinePlan interface {
	Ref() QEvalPipelinePlanRef
}

// QEvalPipelinePlanRef is the compact, compile-time plan handle stored on the
// IR Function and copied into CompiledFunction for future execution lowering.
type QEvalPipelinePlanRef struct {
	ID             int
	Kernel         string
	Shape          string
	PipelineShape  string
	Source         string
	Backend        string
	Kind           string
	Terminal       string
	AssignmentText string
	ValueExpr      string
	ValueBinding   string
	IndexExpr      string
	IndexBinding   string
	MaskExpr       string
	MaskBinding    string
	LeftExpr       string
	RightExpr      string
	CompareOp      string
	ComparePrefix  string
	ModExpr        string
	ModulusExpr    string
	ModTargetExpr  string
	ReductionInput string
}

func (r QEvalPipelinePlanRef) Valid() bool {
	return r.ID >= 0 && r.Kernel != "" && r.Shape != "" && r.Backend != ""
}

type qEvalPipelineStaticPlan struct {
	ref QEvalPipelinePlanRef
}

func (p qEvalPipelineStaticPlan) Ref() QEvalPipelinePlanRef {
	return p.ref
}

type qEvalPipelineStaticBackend struct {
	name  string
	plans []qEvalPipelineStaticPlan
}

func newQEvalPipelineStaticBackend(name string, refs []QEvalPipelinePlanRef) qEvalPipelineStaticBackend {
	plans := make([]qEvalPipelineStaticPlan, 0, len(refs))
	for _, ref := range refs {
		if ref.Valid() {
			plans = append(plans, qEvalPipelineStaticPlan{ref: ref})
		}
	}
	return qEvalPipelineStaticBackend{name: name, plans: plans}
}

func (b qEvalPipelineStaticBackend) BackendName() string {
	if b.name == "" {
		return "methodjit_static_q_eval_pipeline"
	}
	return b.name
}

func (b qEvalPipelineStaticBackend) LookupQEvalPipelinePlan(ref QEvalPipelinePlanRef) (QEvalPipelinePlan, bool) {
	for _, plan := range b.plans {
		if plan.ref.ID == ref.ID && plan.ref.Kernel == ref.Kernel && plan.ref.Shape == ref.Shape {
			return plan, true
		}
	}
	return nil, false
}

func (fn *Function) addQEvalPipelinePlan(source string, plan qEvalHotPlan) QEvalPipelinePlanRef {
	if fn == nil {
		return QEvalPipelinePlanRef{ID: -1}
	}
	for _, ref := range fn.QEvalPipelinePlans {
		if ref.Source == source && ref.Kernel == plan.Kernel && ref.Shape == plan.Shape && ref.Backend == plan.Backend {
			return ref
		}
	}
	ref := QEvalPipelinePlanRef{
		ID:             len(fn.QEvalPipelinePlans),
		Kernel:         plan.Kernel,
		Shape:          plan.Shape,
		PipelineShape:  plan.PipelineShape,
		Source:         source,
		Backend:        plan.Backend,
		Kind:           plan.Kind,
		Terminal:       plan.Terminal,
		AssignmentText: plan.AssignmentText,
		ValueExpr:      plan.ValueExpr,
		ValueBinding:   plan.ValueBinding,
		IndexExpr:      plan.IndexExpr,
		IndexBinding:   plan.IndexBinding,
		MaskExpr:       plan.MaskExpr,
		MaskBinding:    plan.MaskBinding,
		LeftExpr:       plan.LeftExpr,
		RightExpr:      plan.RightExpr,
		CompareOp:      plan.CompareOp,
		ComparePrefix:  plan.ComparePrefix,
		ModExpr:        plan.ModExpr,
		ModulusExpr:    plan.ModulusExpr,
		ModTargetExpr:  plan.ModTargetExpr,
		ReductionInput: plan.ReductionInput,
	}
	fn.QEvalPipelinePlans = append(fn.QEvalPipelinePlans, ref)
	return ref
}

func qEvalPipelinePlanRefByID(refs []QEvalPipelinePlanRef, id int) (QEvalPipelinePlanRef, bool) {
	if id < 0 || id >= len(refs) {
		return QEvalPipelinePlanRef{}, false
	}
	ref := refs[id]
	return ref, ref.Valid()
}

func formatQEvalPipelinePlanRefs(refs []QEvalPipelinePlanRef) string {
	if len(refs) == 0 {
		return "0 q.eval pipeline plan(s)\n"
	}
	out := fmt.Sprintf("%d q.eval pipeline plan(s)\n", len(refs))
	for i, ref := range refs {
		out += fmt.Sprintf("  [%d] id=%d kernel=%s shape=%s pipeline_shape=%s backend=%s\n", i, ref.ID, ref.Kernel, ref.Shape, ref.PipelineShape, ref.Backend)
	}
	return out
}

func (cf *CompiledFunction) ExecuteQEvalPipelinePlanValue(id int) (runtime.Value, bool, error) {
	if cf == nil {
		return runtime.NilValue(), false, nil
	}
	ref, ok := qEvalPipelinePlanRefByID(cf.QEvalPipelinePlans, id)
	if !ok {
		return runtime.NilValue(), false, nil
	}
	backend := newQRuntimeEvalPipelineBackend(cf.QEvalPipelinePlans)
	return executeQEvalPipelinePlanValue(backend, ref)
}

func executeQEvalPipelinePlanValue(backend QEvalPipelineBackend, ref QEvalPipelinePlanRef) (runtime.Value, bool, error) {
	if backend == nil || !ref.Valid() {
		return runtime.NilValue(), false, nil
	}
	if _, ok := backend.LookupQEvalPipelinePlan(ref); !ok {
		return runtime.NilValue(), false, nil
	}
	executor, ok := backend.(QEvalPipelineExecutor)
	if !ok {
		return runtime.NilValue(), false, nil
	}
	out, handled, err := executor.ExecuteQEvalPipelinePlan(ref)
	if err != nil || !handled {
		return runtime.NilValue(), handled, err
	}
	value, err := qEvalPipelineRuntimeValue(out)
	if err != nil {
		return runtime.NilValue(), false, err
	}
	return value, true, nil
}

func qEvalPipelinePlanExecutionShape(refs []QEvalPipelinePlanRef, id int) string {
	if ref, ok := qEvalPipelinePlanRefByID(refs, id); ok && ref.Shape != "" {
		return ref.Shape
	}
	return "q-eval/pipeline-plan"
}

func qEvalPipelineRuntimeValue(v any) (runtime.Value, error) {
	switch x := v.(type) {
	case nil:
		return runtime.NilValue(), nil
	case bool:
		return runtime.BoolValue(x), nil
	case int:
		return runtime.IntValue(int64(x)), nil
	case int8:
		return runtime.IntValue(int64(x)), nil
	case int16:
		return runtime.IntValue(int64(x)), nil
	case int32:
		return runtime.IntValue(int64(x)), nil
	case int64:
		return runtime.IntValue(x), nil
	case uint:
		return runtime.IntValue(int64(x)), nil
	case uint8:
		return runtime.IntValue(int64(x)), nil
	case uint16:
		return runtime.IntValue(int64(x)), nil
	case uint32:
		return runtime.IntValue(int64(x)), nil
	case uint64:
		return runtime.IntValue(int64(x)), nil
	case float32:
		return runtime.FloatValue(float64(x)), nil
	case float64:
		return runtime.FloatValue(x), nil
	case string:
		return runtime.StringValue(x), nil
	default:
		return runtime.NilValue(), fmt.Errorf("methodjit: q eval pipeline result type %T is not runtime-value supported", v)
	}
}
