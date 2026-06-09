package methodjit

import "fmt"

// QEvalPipelineBackend is the stable handoff point between MethodJIT q.eval
// hot-path recognition and the q typed pipeline backend. The current slice only
// records a backend-facing plan reference; later lowering can route an
// OpQEvalVectorPlan or native op-exit through this interface without teaching
// MethodJIT q evaluator internals.
type QEvalPipelineBackend interface {
	BackendName() string
	LookupQEvalPipelinePlan(ref QEvalPipelinePlanRef) (QEvalPipelinePlan, bool)
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
	ID            int
	Kernel        string
	Shape         string
	PipelineShape string
	Source        string
	Backend       string
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
		ID:            len(fn.QEvalPipelinePlans),
		Kernel:        plan.Kernel,
		Shape:         plan.Shape,
		PipelineShape: plan.PipelineShape,
		Source:        source,
		Backend:       plan.Backend,
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
