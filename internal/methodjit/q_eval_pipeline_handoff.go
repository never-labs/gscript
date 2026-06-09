package methodjit

import (
	"strings"

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
	static qEvalPipelineStaticBackend
}

func newQRuntimeEvalPipelineBackend(refs []QEvalPipelinePlanRef) qRuntimeEvalPipelineBackend {
	return qRuntimeEvalPipelineBackend{
		static: newQEvalPipelineStaticBackend(qEvalPipelineTypedRuntimeBackend, refs),
	}
}

func (b qRuntimeEvalPipelineBackend) BackendName() string {
	return b.static.BackendName()
}

func (b qRuntimeEvalPipelineBackend) LookupQEvalPipelinePlan(ref QEvalPipelinePlanRef) (QEvalPipelinePlan, bool) {
	return b.static.LookupQEvalPipelinePlan(ref)
}

func (b qRuntimeEvalPipelineBackend) ExecuteQEvalPipelinePlan(ref QEvalPipelinePlanRef) (any, bool, error) {
	if ref.Backend != qEvalPipelineTypedRuntimeBackend {
		return nil, false, nil
	}
	if _, ok := b.LookupQEvalPipelinePlan(ref); !ok {
		return nil, false, nil
	}
	if descriptor, ok := qEvalPipelineDescriptorFromRef(ref); ok {
		return stdq.ExecuteEvalPipelineDescriptor(descriptor)
	}
	return stdq.ExecuteEvalPipeline(ref.Source)
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
