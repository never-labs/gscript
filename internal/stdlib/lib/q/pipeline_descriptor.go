package q

import "strings"

// EvalPipelineAssignment describes one assignment that participates in a q
// script-level pipeline descriptor. It is metadata-only: values remain owned by
// EvalState and the runtime evaluator.
type EvalPipelineAssignment struct {
	Name string
	RHS  string
}

// EvalPipelineDescriptor is the read-only handoff shape exposed to MethodJIT
// and diagnostics. It describes the q runtime pipeline plan that would be used
// for a source string without binding environment values or caching results.
type EvalPipelineDescriptor struct {
	Source        string
	Kind          string
	Kernel        string
	Shape         string
	PipelineShape string

	Terminal    string
	Assignments []EvalPipelineAssignment

	ValueExpr    string
	ValueBinding string
	IndexExpr    string
	IndexBinding string
	MaskExpr     string
	MaskBinding  string

	LeftExpr       string
	RightExpr      string
	CompareOp      string
	ReductionInput string
}

// DescribeEvalPipeline returns the stable descriptor for a q source string
// when the runtime planner can recognize it as a typed pipeline candidate.
func DescribeEvalPipeline(source string) (EvalPipelineDescriptor, bool) {
	source = strings.TrimSpace(source)
	if source == "" {
		return EvalPipelineDescriptor{}, false
	}
	if plan := buildQScriptPlan(source); plan.scriptPipeline != nil {
		return evalScriptPipelineDescriptor(source, plan.scriptPipeline), true
	}
	if plan := buildQPipelinePlan(source); plan.kind != qPipelineInvalid {
		return evalExpressionPipelineDescriptor(source, plan), true
	}
	return EvalPipelineDescriptor{}, false
}

func evalScriptPipelineDescriptor(source string, d *qScriptPipelineDescriptor) EvalPipelineDescriptor {
	out := EvalPipelineDescriptor{
		Source:        source,
		Kind:          "script",
		Kernel:        "QScriptPipelinePlan",
		Shape:         d.shape(),
		PipelineShape: qRuntimeKernelPipelineShape("QScriptPipelinePlan", d.shape()),
		Terminal:      d.terminal,
		ValueExpr:     strings.TrimSpace(d.valueExpr),
		ValueBinding:  strings.TrimSpace(d.valueBinding),
		IndexExpr:     strings.TrimSpace(d.indexExpr),
		IndexBinding:  strings.TrimSpace(d.indexBinding),
		MaskExpr:      strings.TrimSpace(d.maskExpr),
		MaskBinding:   strings.TrimSpace(d.maskBinding),
	}
	if len(d.assignments) > 0 {
		out.Assignments = make([]EvalPipelineAssignment, 0, len(d.assignments))
		for _, assignment := range d.assignments {
			out.Assignments = append(out.Assignments, EvalPipelineAssignment{
				Name: assignment.name,
				RHS:  assignment.rhs,
			})
		}
	}
	return out
}

func evalExpressionPipelineDescriptor(source string, plan qPipelinePlan) EvalPipelineDescriptor {
	return EvalPipelineDescriptor{
		Source:         source,
		Kind:           "expression",
		Kernel:         "QPipelinePlan",
		Shape:          plan.shape,
		PipelineShape:  qRuntimeKernelPipelineShape("QPipelinePlan", plan.shape),
		ValueExpr:      strings.TrimSpace(plan.valueExpr),
		IndexExpr:      strings.TrimSpace(plan.indexExpr),
		MaskExpr:       strings.TrimSpace(plan.maskExpr),
		LeftExpr:       strings.TrimSpace(plan.leftExpr),
		RightExpr:      strings.TrimSpace(plan.rightExpr),
		CompareOp:      plan.compareOp,
		ReductionInput: strings.TrimSpace(plan.reductionInput),
	}
}
