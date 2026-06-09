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
	Source         string
	Kind           string
	Kernel         string
	Shape          string
	PipelineShape  string
	ShapeFamily    string
	ShapeReducer   string
	ShapeSelector  string
	ShapeTransform string

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
	ComparePrefix  string
	ModExpr        string
	ModulusExpr    string
	ModTargetExpr  string
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

// ExecuteEvalPipeline executes source only when the q runtime pipeline planner
// recognizes it. It is the execution-side counterpart to DescribeEvalPipeline:
// callers such as MethodJIT can try the typed pipeline backend without falling
// back through the full q evaluator. The returned handled flag is false when no
// pipeline shape is available.
func ExecuteEvalPipeline(source string) (any, bool, error) {
	return ExecuteEvalPipelineWithEnv(source, nil)
}

// ExecuteEvalPipelineWithEnv is the environment-bearing variant used by future
// q.session and JIT entry points that need existing q bindings.
func ExecuteEvalPipelineWithEnv(source string, env map[string]any) (any, bool, error) {
	state := NewEvalState(env)
	return state.executeEvalPipeline(strings.TrimSpace(source))
}

// ExecuteEvalPipelineDescriptor executes a descriptor produced by
// DescribeEvalPipeline without repeating source-level pipeline recognition.
func ExecuteEvalPipelineDescriptor(descriptor EvalPipelineDescriptor) (any, bool, error) {
	return ExecuteEvalPipelineDescriptorWithEnv(descriptor, nil)
}

// ExecuteEvalPipelineDescriptorWithEnv is the environment-bearing descriptor
// execution path used by JIT backends that already hold a stable plan ref.
func ExecuteEvalPipelineDescriptorWithEnv(descriptor EvalPipelineDescriptor, env map[string]any) (any, bool, error) {
	state := NewEvalState(env)
	return state.executeEvalPipelineDescriptor(descriptor)
}

func (s *EvalState) executeEvalPipeline(source string) (any, bool, error) {
	if source == "" {
		return nil, false, nil
	}
	if plan := s.qScriptPlan(source); plan.scriptPipeline != nil {
		return s.tryEvalQScriptPipeline(plan.scriptPipeline)
	}
	if plan := s.qPipelinePlan(source); plan.kind != qPipelineInvalid {
		return s.evalQPipelinePlan(plan)
	}
	return nil, false, nil
}

func (s *EvalState) executeEvalPipelineDescriptor(descriptor EvalPipelineDescriptor) (any, bool, error) {
	switch descriptor.Kind {
	case "expression":
		plan, ok := qPipelinePlanFromEvalDescriptor(descriptor)
		if !ok {
			return nil, false, nil
		}
		return s.evalQPipelinePlan(plan)
	case "script":
		plan, ok := qScriptPipelineDescriptorFromEvalDescriptor(descriptor)
		if !ok {
			return nil, false, nil
		}
		return s.tryEvalQScriptPipeline(&plan)
	default:
		return nil, false, nil
	}
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
	shapeSpec := qPipelinePlanShapeSpec(plan)
	return EvalPipelineDescriptor{
		Source:         source,
		Kind:           "expression",
		Kernel:         "QPipelinePlan",
		Shape:          plan.stableShape(),
		PipelineShape:  plan.stablePipelineShape(),
		ShapeFamily:    string(shapeSpec.Family),
		ShapeReducer:   shapeSpec.Reducer,
		ShapeSelector:  shapeSpec.Selector,
		ShapeTransform: shapeSpec.Transform,
		ValueExpr:      strings.TrimSpace(plan.valueExpr),
		IndexExpr:      strings.TrimSpace(plan.indexExpr),
		MaskExpr:       strings.TrimSpace(plan.maskExpr),
		LeftExpr:       strings.TrimSpace(plan.leftExpr),
		RightExpr:      strings.TrimSpace(plan.rightExpr),
		CompareOp:      plan.compareOp,
		ComparePrefix:  plan.comparePrefix,
		ModExpr:        strings.TrimSpace(plan.modExpr),
		ModulusExpr:    strings.TrimSpace(plan.modulusExpr),
		ModTargetExpr:  strings.TrimSpace(plan.modTargetExpr),
		ReductionInput: strings.TrimSpace(plan.reductionInput),
	}
}

func qPipelinePlanFromEvalDescriptor(descriptor EvalPipelineDescriptor) (qPipelinePlan, bool) {
	shape := strings.TrimSpace(descriptor.Shape)
	if shape == "" {
		return qPipelinePlan{}, false
	}
	plan := qPipelinePlan{
		shape:          shape,
		valueExpr:      strings.TrimSpace(descriptor.ValueExpr),
		indexExpr:      strings.TrimSpace(descriptor.IndexExpr),
		maskExpr:       strings.TrimSpace(descriptor.MaskExpr),
		leftExpr:       strings.TrimSpace(descriptor.LeftExpr),
		rightExpr:      strings.TrimSpace(descriptor.RightExpr),
		compareOp:      descriptor.CompareOp,
		comparePrefix:  descriptor.ComparePrefix,
		modExpr:        strings.TrimSpace(descriptor.ModExpr),
		modulusExpr:    strings.TrimSpace(descriptor.ModulusExpr),
		modTargetExpr:  strings.TrimSpace(descriptor.ModTargetExpr),
		reductionInput: strings.TrimSpace(descriptor.ReductionInput),
	}
	switch shape {
	case "where-reduce/sum":
		plan.kind = qPipelineSumWhereMask
	case "where-index-reduce/sum":
		plan.kind = qPipelineSumWhereIndex
	case "gather-reduce/sum":
		plan.kind = qPipelineSumGatherIndexes
	case "compare-to-index-sum":
		plan.kind = qPipelineSumWhereCompare
	case "compare-to-index-count":
		plan.kind = qPipelineCountWhereCompare
	case "compare-to-index":
		plan.kind = qPipelineWhereCompareIndexes
	case "compare-to-index-sum-mod":
		plan.kind = qPipelineSumWhereModuloCompare
	case "compare-to-index-count-mod":
		plan.kind = qPipelineCountWhereModuloCompare
	case "compare-to-index-mod":
		plan.kind = qPipelineWhereModuloCompareIndexes
	case "vector-reduce/sum-deltas":
		plan.kind = qPipelineSumDeltas
	case "bin-reduce/sum":
		plan.kind = qPipelineSumBin
	case "vector-reduce/sum-expr":
		plan.kind = qPipelineSumVectorExpr
	case "vector-count/expr":
		plan.kind = qPipelineCountVectorExpr
	case "vector-reduce/sum-msum", "vector-reduce/sum-mavg", "vector-reduce/sum-mcount", "vector-reduce/sum-mmin", "vector-reduce/sum-mmax":
		plan.kind = qPipelineSumMovingWindow
	case "vector-count/sums", "vector-count/prds", "vector-count/mins", "vector-count/maxs", "vector-count/avgs":
		plan.kind = qPipelineCountRunningScan
	default:
		return qPipelinePlan{}, false
	}
	if plan.comparePrefix == "" {
		plan.comparePrefix = shape
	}
	return qPipelinePlanWithBindingPlans(plan), true
}

func qScriptPipelineDescriptorFromEvalDescriptor(descriptor EvalPipelineDescriptor) (qScriptPipelineDescriptor, bool) {
	shape := strings.TrimSpace(descriptor.Shape)
	if !strings.HasPrefix(shape, "script-pipeline/") {
		return qScriptPipelineDescriptor{}, false
	}
	out := qScriptPipelineDescriptor{
		shapeText:    shape,
		terminal:     strings.TrimSpace(descriptor.Terminal),
		valueExpr:    strings.TrimSpace(descriptor.ValueExpr),
		valueBinding: strings.TrimSpace(descriptor.ValueBinding),
		indexExpr:    strings.TrimSpace(descriptor.IndexExpr),
		indexBinding: strings.TrimSpace(descriptor.IndexBinding),
		maskExpr:     strings.TrimSpace(descriptor.MaskExpr),
		maskBinding:  strings.TrimSpace(descriptor.MaskBinding),
	}
	switch {
	case strings.Contains(shape, "where-index-reduce/sum"):
		out.kind = qScriptPipelineWhereIndexReduceSum
		out.terminalPlan = qPipelinePlanWithBindingPlans(qPipelinePlan{
			kind:      qPipelineSumWhereIndex,
			shape:     "where-index-reduce/sum",
			valueExpr: out.valueExpr,
			indexExpr: out.indexExpr,
			maskExpr:  out.maskExpr,
		})
	case strings.Contains(shape, "where-reduce/sum"):
		out.kind = qScriptPipelineWhereReduceSum
		out.terminalUsesWhere = true
		out.terminalPlan = qPipelinePlanWithBindingPlans(qPipelinePlan{
			kind:      qPipelineSumWhereMask,
			shape:     "where-reduce/sum",
			valueExpr: out.valueExpr,
			maskExpr:  out.maskExpr,
		})
	case strings.Contains(shape, "gather-reduce/sum"):
		out.kind = qScriptPipelineGatherReduceSum
		out.terminalPlan = qPipelinePlanWithBindingPlans(qPipelinePlan{
			kind:      qPipelineSumGatherIndexes,
			shape:     "gather-reduce/sum",
			valueExpr: out.valueExpr,
			indexExpr: out.indexExpr,
		})
	default:
		return qScriptPipelineDescriptor{}, false
	}
	out.valuePlan = buildQScriptBindingPlanForRHS(out.valueExpr, nil)
	out.indexPlan = buildQScriptBindingPlanForRHS(out.indexExpr, nil)
	out.maskPlan = buildQScriptBindingPlanForRHS(out.maskExpr, nil)
	out.moduloMaskPlan = qScriptPipelineModuloMaskPlan(out.maskExpr)
	if len(descriptor.Assignments) > 0 {
		out.assignments = make([]qScriptPipelineAssignment, 0, len(descriptor.Assignments))
		for _, assignment := range descriptor.Assignments {
			name := strings.TrimSpace(assignment.Name)
			rhs := strings.TrimSpace(assignment.RHS)
			if name == "" || rhs == "" {
				return qScriptPipelineDescriptor{}, false
			}
			out.assignments = append(out.assignments, qScriptPipelineAssignment{
				name:    name,
				rhs:     rhs,
				binding: buildQScriptBindingPlanForRHS(rhs, nil),
			})
		}
	}
	return out, true
}
