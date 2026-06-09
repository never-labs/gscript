package q

import (
	"strings"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

const EvalPipelineTypedRuntimeBackend = "q_pipeline_typed_runtime"

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

// EvalPipelineBackendPlan is the stable q-side handoff consumed by MethodJIT.
// It keeps backend identity, diagnostics, and the executable descriptor behind
// one API so JIT lowering does not need to rediscover q source strings or
// duplicate planner shape logic.
type EvalPipelineBackendPlan struct {
	Backend    string
	Detail     string
	Descriptor EvalPipelineDescriptor
}

// EvalPipelineExecutablePlan is an opaque, predecoded q runtime pipeline plan.
// It lets MethodJIT keep q planning behind the q package boundary while still
// avoiding descriptor-to-plan rebuilding on every hot-path execution.
type EvalPipelineExecutablePlan struct {
	backend    string
	kind       string
	expression qPipelinePlan
	script     *qScriptPipelineDescriptor
}

func (p EvalPipelineBackendPlan) Valid() bool {
	return p.Backend != "" && p.Descriptor.Kind != "" && p.Descriptor.Kernel != "" && p.Descriptor.Shape != ""
}

func (p EvalPipelineBackendPlan) Kind() string {
	return p.Descriptor.Kind
}

func (p EvalPipelineBackendPlan) Kernel() string {
	return p.Descriptor.Kernel
}

func (p EvalPipelineBackendPlan) Shape() string {
	return p.Descriptor.Shape
}

func (p EvalPipelineBackendPlan) PipelineShape() string {
	return p.Descriptor.PipelineShape
}

func (p EvalPipelineBackendPlan) Source() string {
	return p.Descriptor.Source
}

func (p EvalPipelineExecutablePlan) Valid() bool {
	return p.backend == EvalPipelineTypedRuntimeBackend && p.kind != ""
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

// DescribeEvalPipelineBackendPlan returns the q typed-runtime backend plan for
// source. MethodJIT should prefer this API over interpreting descriptor shape
// strings itself.
func DescribeEvalPipelineBackendPlan(source string) (EvalPipelineBackendPlan, bool) {
	descriptor, ok := DescribeEvalPipeline(source)
	if !ok {
		return EvalPipelineBackendPlan{}, false
	}
	return EvalPipelineBackendPlan{
		Backend:    EvalPipelineTypedRuntimeBackend,
		Detail:     "kind=" + descriptor.Kind,
		Descriptor: descriptor,
	}, true
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

// ExecuteEvalPipelineBackendPlan executes a plan returned by
// DescribeEvalPipelineBackendPlan without asking MethodJIT to understand q
// shape strings.
func ExecuteEvalPipelineBackendPlan(plan EvalPipelineBackendPlan) (any, bool, error) {
	return ExecuteEvalPipelineBackendPlanWithEnv(plan, nil)
}

func ExecuteEvalPipelineBackendPlanWithEnv(plan EvalPipelineBackendPlan, env map[string]any) (any, bool, error) {
	if !plan.Valid() || plan.Backend != EvalPipelineTypedRuntimeBackend {
		return nil, false, nil
	}
	state := NewEvalState(env)
	return state.executeEvalPipelineDescriptor(plan.Descriptor)
}

// CompileEvalPipelineBackendPlan predecodes a stable backend plan into an
// executable q runtime plan. The returned value is intentionally opaque so
// callers can cache it without depending on q AST/runtime internals.
func CompileEvalPipelineBackendPlan(plan EvalPipelineBackendPlan) (EvalPipelineExecutablePlan, bool) {
	if !plan.Valid() || plan.Backend != EvalPipelineTypedRuntimeBackend {
		return EvalPipelineExecutablePlan{}, false
	}
	switch plan.Descriptor.Kind {
	case "expression":
		expression, ok := qPipelinePlanFromEvalDescriptor(plan.Descriptor)
		if !ok {
			return EvalPipelineExecutablePlan{}, false
		}
		return EvalPipelineExecutablePlan{
			backend:    plan.Backend,
			kind:       plan.Descriptor.Kind,
			expression: expression,
		}, true
	case "script":
		script, ok := qScriptPipelineDescriptorFromEvalDescriptor(plan.Descriptor)
		if !ok {
			return EvalPipelineExecutablePlan{}, false
		}
		return EvalPipelineExecutablePlan{
			backend: plan.Backend,
			kind:    plan.Descriptor.Kind,
			script:  &script,
		}, true
	default:
		return EvalPipelineExecutablePlan{}, false
	}
}

// ExecuteEvalPipelineDescriptor executes a predecoded pipeline descriptor using
// this EvalState's caches and environment. It is intended for callers such as
// MethodJIT that already hold a stable plan and want to avoid re-entering
// source parsing on every execution.
func (s *EvalState) ExecuteEvalPipelineDescriptor(descriptor EvalPipelineDescriptor) (any, bool, error) {
	if s == nil {
		return nil, false, nil
	}
	return s.executeEvalPipelineDescriptor(descriptor)
}

// ExecuteEvalPipelineBackendPlan executes a typed-runtime backend plan using
// this EvalState's caches and environment.
func (s *EvalState) ExecuteEvalPipelineBackendPlan(plan EvalPipelineBackendPlan) (any, bool, error) {
	if s == nil || !plan.Valid() || plan.Backend != EvalPipelineTypedRuntimeBackend {
		return nil, false, nil
	}
	return s.executeEvalPipelineDescriptor(plan.Descriptor)
}

// ExecuteEvalPipelineExecutablePlan executes an opaque predecoded plan using
// this EvalState's caches and environment.
func (s *EvalState) ExecuteEvalPipelineExecutablePlan(plan EvalPipelineExecutablePlan) (any, bool, error) {
	if s == nil || !plan.Valid() {
		return nil, false, nil
	}
	switch plan.kind {
	case "expression":
		return s.evalQPipelinePlan(plan.expression)
	case "script":
		return s.tryEvalQScriptPipeline(plan.script)
	default:
		return nil, false, nil
	}
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
	case "vector-reduce/sum-dyadic-min":
		plan.kind = qPipelineSumDyadicMinMax
		plan.compareOp = "min"
	case "vector-reduce/sum-dyadic-max":
		plan.kind = qPipelineSumDyadicMinMax
		plan.compareOp = "max"
	case "vector-reduce/sum-dyadic-float-xexp":
		plan.kind = qPipelineSumDyadicFloatMath
		plan.compareOp = data.NumericDyadicXExp
	case "vector-reduce/sum-dyadic-float-xlog":
		plan.kind = qPipelineSumDyadicFloatMath
		plan.compareOp = data.NumericDyadicXLog
	case "vector-reduce/sum-reverse":
		plan.kind = qPipelineSumSequenceTransform
		plan.compareOp = data.SequenceTransformReverse
	case "vector-reduce/sum-rotate":
		plan.kind = qPipelineSumSequenceTransform
		plan.compareOp = data.SequenceTransformRotate
	case "vector-reduce/sum-sublist":
		plan.kind = qPipelineSumSequenceTransform
		plan.compareOp = data.SequenceTransformSublist
	case "vector-reduce/sum-ratios":
		plan.kind = qPipelineSumSequenceTransform
		plan.compareOp = data.SequenceTransformRatios
	case "vector-reduce/sum-raze":
		plan.kind = qPipelineSumRaze
	case "vector-count/expr":
		plan.kind = qPipelineCountVectorExpr
	case "vector-reduce/sum-msum", "vector-reduce/sum-mavg", "vector-reduce/sum-mcount", "vector-reduce/sum-mmin", "vector-reduce/sum-mmax":
		plan.kind = qPipelineSumMovingWindow
	case "vector-count/sums", "vector-count/prds", "vector-count/mins", "vector-count/maxs", "vector-count/avgs":
		plan.kind = qPipelineCountRunningScan
	case "vector-last/sums", "vector-last/prds", "vector-last/mins", "vector-last/maxs", "vector-last/avgs":
		plan.kind = qPipelineLastRunningScan
	case "sequence-count/trim", "sequence-count/ltrim", "sequence-count/rtrim",
		"sequence-count/cross", "sequence-count/cut", "sequence-count/sublist",
		"sequence-count/raze", "sequence-count/value":
		plan.kind = qPipelineCountSequencePrimitive
		plan.compareOp = strings.TrimPrefix(shape, "sequence-count/")
	default:
		switch {
		case strings.HasPrefix(shape, "runtime-unary/"):
			plan.kind = qPipelineUnaryPrimitive
			if plan.compareOp == "" {
				plan.compareOp = strings.TrimPrefix(shape, "runtime-unary/")
			}
		case strings.HasPrefix(shape, "runtime-dyadic/"):
			plan.kind = qPipelineDyadicPrimitive
			if plan.compareOp == "" {
				plan.compareOp = strings.TrimPrefix(shape, "runtime-dyadic/")
			}
		default:
			return qPipelinePlan{}, false
		}
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
