package methodjit

import stdq "github.com/never-labs/leia/internal/stdlib/lib/q"

const qEvalPipelineTypedRuntimeBackend = "q_pipeline_typed_runtime"

// QEvalPipelineDescriptor is MethodJIT's metadata-only view of a q runtime
// pipeline. It deliberately mirrors only stable handoff fields, not q AST nodes
// or EvalState-owned runtime values.
type QEvalPipelineDescriptor struct {
	Kernel        string
	Shape         string
	PipelineShape string
	Source        string
	Backend       string
	Detail        string
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
		Kernel:        descriptor.Kernel,
		Shape:         descriptor.Shape,
		PipelineShape: descriptor.PipelineShape,
		Source:        descriptor.Source,
		Backend:       qEvalPipelineTypedRuntimeBackend,
		Detail:        "kind=" + descriptor.Kind,
	}, true
}

func qEvalRuntimePipelinePlan(source string) (qEvalHotPlan, bool) {
	descriptor, ok := qRuntimeEvalPipelinePlanner{}.DescribeQEvalPipeline(source)
	if !ok {
		return qEvalHotPlan{}, false
	}
	return qEvalHotPlan{
		Kernel:        descriptor.Kernel,
		Shape:         descriptor.Shape,
		PipelineShape: descriptor.PipelineShape,
		Backend:       descriptor.Backend,
		Detail:        descriptor.Detail,
	}, true
}
