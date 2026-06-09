package methodjit

import (
	"strings"

	"github.com/never-labs/leia/internal/runtime"
	stdq "github.com/never-labs/leia/internal/stdlib/lib/q"
)

const qEvalPipelineTypedRuntimeBackend = "q_pipeline_typed_runtime"

// QEvalPipelineDescriptor is MethodJIT's metadata-only view of a q runtime
// pipeline. It deliberately mirrors only stable handoff fields, not q AST nodes
// or EvalState-owned runtime values.
type QEvalPipelineDescriptor struct {
	Kernel         string
	Shape          string
	PipelineShape  string
	Source         string
	Backend        string
	Detail         string
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

// QEvalPipelinePlanner describes q.eval source strings that can be handed to
// the q runtime pipeline backend. Implementations own q syntax normalization,
// environment binding, and future runtime execution.
type QEvalPipelinePlanner interface {
	DescribeQEvalPipeline(source string) (QEvalPipelineDescriptor, bool)
}

type qRuntimeEvalPipelinePlanner struct{}

func (qRuntimeEvalPipelinePlanner) DescribeQEvalPipeline(source string) (QEvalPipelineDescriptor, bool) {
	descriptor, ok := stdq.DescribeEvalPipeline(source)
	if !ok {
		return QEvalPipelineDescriptor{}, false
	}
	return QEvalPipelineDescriptor{
		Kernel:         descriptor.Kernel,
		Shape:          descriptor.Shape,
		PipelineShape:  descriptor.PipelineShape,
		Source:         descriptor.Source,
		Backend:        qEvalPipelineTypedRuntimeBackend,
		Detail:         "kind=" + descriptor.Kind,
		Kind:           descriptor.Kind,
		Terminal:       descriptor.Terminal,
		AssignmentText: encodeQEvalPipelineAssignments(descriptor.Assignments),
		ValueExpr:      descriptor.ValueExpr,
		ValueBinding:   descriptor.ValueBinding,
		IndexExpr:      descriptor.IndexExpr,
		IndexBinding:   descriptor.IndexBinding,
		MaskExpr:       descriptor.MaskExpr,
		MaskBinding:    descriptor.MaskBinding,
		LeftExpr:       descriptor.LeftExpr,
		RightExpr:      descriptor.RightExpr,
		CompareOp:      descriptor.CompareOp,
		ComparePrefix:  descriptor.ComparePrefix,
		ModExpr:        descriptor.ModExpr,
		ModulusExpr:    descriptor.ModulusExpr,
		ModTargetExpr:  descriptor.ModTargetExpr,
		ReductionInput: descriptor.ReductionInput,
	}, true
}

func qEvalRuntimePipelinePlan(source string) (qEvalHotPlan, bool) {
	descriptor, ok := qRuntimeEvalPipelinePlanner{}.DescribeQEvalPipeline(source)
	if !ok {
		return qEvalHotPlan{}, false
	}
	return qEvalHotPlan{
		Kernel:         descriptor.Kernel,
		Shape:          descriptor.Shape,
		PipelineShape:  descriptor.PipelineShape,
		Backend:        descriptor.Backend,
		Detail:         descriptor.Detail,
		Kind:           descriptor.Kind,
		Terminal:       descriptor.Terminal,
		AssignmentText: descriptor.AssignmentText,
		ValueExpr:      descriptor.ValueExpr,
		ValueBinding:   descriptor.ValueBinding,
		IndexExpr:      descriptor.IndexExpr,
		IndexBinding:   descriptor.IndexBinding,
		MaskExpr:       descriptor.MaskExpr,
		MaskBinding:    descriptor.MaskBinding,
		LeftExpr:       descriptor.LeftExpr,
		RightExpr:      descriptor.RightExpr,
		CompareOp:      descriptor.CompareOp,
		ComparePrefix:  descriptor.ComparePrefix,
		ModExpr:        descriptor.ModExpr,
		ModulusExpr:    descriptor.ModulusExpr,
		ModTargetExpr:  descriptor.ModTargetExpr,
		ReductionInput: descriptor.ReductionInput,
	}, true
}

type qRuntimeEvalPipelineBackend struct {
	static            qEvalPipelineStaticBackend
	descriptorByID    map[int]stdq.EvalPipelineDescriptor
	executeDescriptor func(stdq.EvalPipelineDescriptor) (any, bool, error)
	executeSource     func(string) (any, bool, error)
}

type qEvalPipelinePlanHelper struct {
	ref               QEvalPipelinePlanRef
	descriptor        stdq.EvalPipelineDescriptor
	hasDescriptor     bool
	executeDescriptor func(stdq.EvalPipelineDescriptor) (any, bool, error)
	executeSource     func(string) (any, bool, error)
}

func newQRuntimeEvalPipelineBackend(refs []QEvalPipelinePlanRef) qRuntimeEvalPipelineBackend {
	descriptorByID := make(map[int]stdq.EvalPipelineDescriptor, len(refs))
	for _, ref := range refs {
		if descriptor, ok := qEvalPipelineDescriptorFromRef(ref); ok {
			descriptorByID[ref.ID] = descriptor
		}
	}
	return qRuntimeEvalPipelineBackend{
		static:            newQEvalPipelineStaticBackend(qEvalPipelineTypedRuntimeBackend, refs),
		descriptorByID:    descriptorByID,
		executeDescriptor: stdq.ExecuteEvalPipelineDescriptor,
		executeSource:     stdq.ExecuteEvalPipeline,
	}
}

func newQEvalPipelinePlanHelpers(refs []QEvalPipelinePlanRef, backend qRuntimeEvalPipelineBackend) []qEvalPipelinePlanHelper {
	if len(refs) == 0 {
		return nil
	}
	helpers := make([]qEvalPipelinePlanHelper, len(refs))
	for _, ref := range refs {
		if !ref.Valid() || ref.ID < 0 || ref.ID >= len(helpers) || ref.Backend != qEvalPipelineTypedRuntimeBackend {
			continue
		}
		if _, ok := backend.LookupQEvalPipelinePlan(ref); !ok {
			continue
		}
		descriptor, hasDescriptor := backend.lookupDescriptor(ref)
		helpers[ref.ID] = qEvalPipelinePlanHelper{
			ref:               ref,
			descriptor:        descriptor,
			hasDescriptor:     hasDescriptor,
			executeDescriptor: backend.executeDescriptor,
			executeSource:     backend.executeSource,
		}
	}
	return helpers
}

func (h qEvalPipelinePlanHelper) validForID(id int) bool {
	return h.ref.ID == id && h.ref.Valid() && h.ref.Backend == qEvalPipelineTypedRuntimeBackend
}

func (h qEvalPipelinePlanHelper) execute() (runtime.Value, bool, error) {
	if !h.ref.Valid() || h.ref.Backend != qEvalPipelineTypedRuntimeBackend {
		return runtime.NilValue(), false, nil
	}
	var (
		out     any
		handled bool
		err     error
	)
	if h.hasDescriptor {
		execute := h.executeDescriptor
		if execute == nil {
			execute = stdq.ExecuteEvalPipelineDescriptor
		}
		out, handled, err = execute(h.descriptor)
	} else {
		execute := h.executeSource
		if execute == nil {
			execute = stdq.ExecuteEvalPipeline
		}
		out, handled, err = execute(h.ref.Source)
	}
	if err != nil || !handled {
		return runtime.NilValue(), handled, err
	}
	value, err := qEvalPipelineRuntimeValue(out)
	if err != nil {
		return runtime.NilValue(), false, err
	}
	return value, true, nil
}

func (b qRuntimeEvalPipelineBackend) BackendName() string {
	return b.static.BackendName()
}

func (b qRuntimeEvalPipelineBackend) LookupQEvalPipelinePlan(ref QEvalPipelinePlanRef) (QEvalPipelinePlan, bool) {
	return b.static.LookupQEvalPipelinePlan(ref)
}

func (b qRuntimeEvalPipelineBackend) hasPlans() bool {
	return len(b.static.plans) > 0
}

func (b qRuntimeEvalPipelineBackend) ExecuteQEvalPipelinePlan(ref QEvalPipelinePlanRef) (any, bool, error) {
	if ref.Backend != qEvalPipelineTypedRuntimeBackend {
		return nil, false, nil
	}
	if _, ok := b.LookupQEvalPipelinePlan(ref); !ok {
		return nil, false, nil
	}
	if descriptor, ok := b.lookupDescriptor(ref); ok {
		return b.executeEvalPipelineDescriptor(descriptor)
	}
	return b.executeEvalPipelineSource(ref.Source)
}

func (b qRuntimeEvalPipelineBackend) lookupDescriptor(ref QEvalPipelinePlanRef) (stdq.EvalPipelineDescriptor, bool) {
	if b.descriptorByID != nil {
		if descriptor, ok := b.descriptorByID[ref.ID]; ok {
			return descriptor, true
		}
	}
	return qEvalPipelineDescriptorFromRef(ref)
}

func (b qRuntimeEvalPipelineBackend) executeEvalPipelineDescriptor(descriptor stdq.EvalPipelineDescriptor) (any, bool, error) {
	if b.executeDescriptor != nil {
		return b.executeDescriptor(descriptor)
	}
	return stdq.ExecuteEvalPipelineDescriptor(descriptor)
}

func (b qRuntimeEvalPipelineBackend) executeEvalPipelineSource(source string) (any, bool, error) {
	if b.executeSource != nil {
		return b.executeSource(source)
	}
	return stdq.ExecuteEvalPipeline(source)
}

func encodeQEvalPipelineAssignments(in []stdq.EvalPipelineAssignment) string {
	if len(in) == 0 {
		return ""
	}
	var b strings.Builder
	for _, assignment := range in {
		if b.Len() > 0 {
			b.WriteByte('\x1e')
		}
		b.WriteString(assignment.Name)
		b.WriteByte('\x1f')
		b.WriteString(assignment.RHS)
	}
	return b.String()
}

func qEvalPipelineDescriptorFromRef(ref QEvalPipelinePlanRef) (stdq.EvalPipelineDescriptor, bool) {
	if ref.Kind == "" || ref.Shape == "" {
		return stdq.EvalPipelineDescriptor{}, false
	}
	return stdq.EvalPipelineDescriptor{
		Source:         ref.Source,
		Kind:           ref.Kind,
		Kernel:         ref.Kernel,
		Shape:          ref.Shape,
		PipelineShape:  ref.PipelineShape,
		Terminal:       ref.Terminal,
		Assignments:    decodeQEvalPipelineAssignments(ref.AssignmentText),
		ValueExpr:      ref.ValueExpr,
		ValueBinding:   ref.ValueBinding,
		IndexExpr:      ref.IndexExpr,
		IndexBinding:   ref.IndexBinding,
		MaskExpr:       ref.MaskExpr,
		MaskBinding:    ref.MaskBinding,
		LeftExpr:       ref.LeftExpr,
		RightExpr:      ref.RightExpr,
		CompareOp:      ref.CompareOp,
		ComparePrefix:  ref.ComparePrefix,
		ModExpr:        ref.ModExpr,
		ModulusExpr:    ref.ModulusExpr,
		ModTargetExpr:  ref.ModTargetExpr,
		ReductionInput: ref.ReductionInput,
	}, true
}

func decodeQEvalPipelineAssignments(text string) []stdq.EvalPipelineAssignment {
	if text == "" {
		return nil
	}
	parts := strings.Split(text, "\x1e")
	out := make([]stdq.EvalPipelineAssignment, 0, len(parts))
	for _, part := range parts {
		name, rhs, ok := strings.Cut(part, "\x1f")
		if !ok {
			return nil
		}
		out = append(out, stdq.EvalPipelineAssignment{Name: name, RHS: rhs})
	}
	return out
}
