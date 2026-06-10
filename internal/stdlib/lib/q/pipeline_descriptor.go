package q

import (
	"strings"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

const EvalPipelineTypedRuntimeBackend = "q_pipeline_typed_runtime"

const (
	evalPipelineKindExpression = "expression"
	evalPipelineKindScript     = "script"
	evalPipelineKernelExpr     = "QPipelinePlan"
	evalPipelineKernelScript   = "QScriptPipelinePlan"
)

// EvalPipelineAssignment describes one assignment that participates in a q
// script-level pipeline descriptor. It is metadata-only: values remain owned by
// EvalState and the runtime evaluator.
type EvalPipelineAssignment struct {
	Name string
	RHS  string
}

type EvalPipelineIntegerDivModTerm struct {
	Op         string
	ScalarExpr string
}

type EvalPipelineCastTerm struct {
	DomainExpr string
	ValueExpr  string
	TargetKind string
	StringCast bool
	Count      bool
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
	RowValueExpr string
	RowIndexExpr string
	ColIndexExpr string
	CallableExpr string
	DyadicOp     string
	ScalarExpr   string
	ScalarLeft   bool
	IntegerTerms []EvalPipelineIntegerDivModTerm
	CastTerms    []EvalPipelineCastTerm
	IncludeCount bool

	SequenceValueExpr      string
	SequenceTransformChain string
	SequenceTransformNames string

	LeftExpr       string
	RightExpr      string
	UnaryOp        string
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

func evalPipelineBackendPlan(descriptor EvalPipelineDescriptor) EvalPipelineBackendPlan {
	if descriptor.Kind == "" {
		return EvalPipelineBackendPlan{}
	}
	return EvalPipelineBackendPlan{
		Backend:    EvalPipelineTypedRuntimeBackend,
		Detail:     "kind=" + descriptor.Kind,
		Descriptor: descriptor,
	}
}

func evalPipelineExpressionExecutable(plan qPipelinePlan) EvalPipelineExecutablePlan {
	return EvalPipelineExecutablePlan{
		backend:    EvalPipelineTypedRuntimeBackend,
		kind:       evalPipelineKindExpression,
		expression: cloneQPipelinePlan(plan),
	}
}

func evalPipelineScriptExecutable(plan *qScriptPipelineDescriptor) EvalPipelineExecutablePlan {
	return EvalPipelineExecutablePlan{
		backend: EvalPipelineTypedRuntimeBackend,
		kind:    evalPipelineKindScript,
		script:  cloneQScriptPipelineDescriptor(plan),
	}
}

// DescribeEvalPipeline returns the stable descriptor for a q source string
// when the runtime planner can recognize it as a typed pipeline candidate.
func DescribeEvalPipeline(source string) (EvalPipelineDescriptor, bool) {
	source = strings.TrimSpace(source)
	if source == "" {
		return EvalPipelineDescriptor{}, false
	}
	scriptPlan := buildQScriptPlan(source)
	if scriptPlan.scriptPipeline != nil {
		return evalScriptPipelineDescriptor(source, scriptPlan.scriptPipeline), true
	}
	if len(scriptPlan.statements) > 1 {
		return EvalPipelineDescriptor{}, false
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
	return evalPipelineBackendPlan(descriptor), true
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
	return state.executeEvalPipelineBackendPlan(plan)
}

// CompileEvalPipelineBackendPlan predecodes a stable backend plan into an
// executable q runtime plan. The returned value is intentionally opaque so
// callers can cache it without depending on q AST/runtime internals.
func CompileEvalPipelineBackendPlan(plan EvalPipelineBackendPlan) (EvalPipelineExecutablePlan, bool) {
	if !plan.Valid() || plan.Backend != EvalPipelineTypedRuntimeBackend {
		return EvalPipelineExecutablePlan{}, false
	}
	return CompileEvalPipelineDescriptor(plan.Descriptor)
}

// CompileEvalPipelineDescriptor predecodes descriptor metadata into the
// executable q runtime plan used by sessions and MethodJIT. Descriptor execution
// should funnel through this helper so expression/script shape restoration has
// one owner.
func CompileEvalPipelineDescriptor(descriptor EvalPipelineDescriptor) (EvalPipelineExecutablePlan, bool) {
	switch descriptor.Kind {
	case evalPipelineKindExpression:
		expression, ok := qPipelinePlanFromEvalDescriptor(descriptor)
		if !ok {
			return EvalPipelineExecutablePlan{}, false
		}
		return evalPipelineExpressionExecutable(expression), true
	case evalPipelineKindScript:
		script, ok := qScriptPipelineDescriptorFromEvalDescriptor(descriptor)
		if !ok {
			return EvalPipelineExecutablePlan{}, false
		}
		return evalPipelineScriptExecutable(&script), true
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
	return s.executeEvalPipelineBackendPlan(plan)
}

// ExecuteEvalPipelineExecutablePlan executes an opaque predecoded plan using
// this EvalState's caches and environment.
func (s *EvalState) ExecuteEvalPipelineExecutablePlan(plan EvalPipelineExecutablePlan) (any, bool, error) {
	return s.ExecuteEvalPipelineExecutablePlanRef(&plan)
}

// ExecuteEvalPipelineExecutablePlanRef executes an opaque predecoded plan
// without copying the plan payload. Hot session/JIT call paths should prefer
// this form when they already own a stable executable plan.
func (s *EvalState) ExecuteEvalPipelineExecutablePlanRef(plan *EvalPipelineExecutablePlan) (any, bool, error) {
	if s == nil || plan == nil || !plan.Valid() {
		return nil, false, nil
	}
	switch plan.kind {
	case evalPipelineKindExpression:
		return s.evalQPipelinePlan(&plan.expression)
	case evalPipelineKindScript:
		return s.tryEvalQScriptPipeline(plan.script)
	default:
		return nil, false, nil
	}
}

func (s *EvalState) compileEvalPipelineSource(source string) (EvalPipelineBackendPlan, EvalPipelineExecutablePlan, bool) {
	source = strings.TrimSpace(source)
	if s == nil || source == "" {
		return EvalPipelineBackendPlan{}, EvalPipelineExecutablePlan{}, false
	}
	scriptPlan := s.qScriptPlan(source)
	if scriptPlan.scriptPipeline != nil {
		descriptor := evalScriptPipelineDescriptor(source, scriptPlan.scriptPipeline)
		return evalPipelineBackendPlan(descriptor), evalPipelineScriptExecutable(scriptPlan.scriptPipeline), true
	}
	if len(scriptPlan.statements) > 1 {
		return EvalPipelineBackendPlan{}, EvalPipelineExecutablePlan{}, false
	}
	if plan := s.qPipelinePlan(source); plan.kind != qPipelineInvalid {
		descriptor := evalExpressionPipelineDescriptor(source, plan)
		return evalPipelineBackendPlan(descriptor), evalPipelineExpressionExecutable(plan), true
	}
	return EvalPipelineBackendPlan{}, EvalPipelineExecutablePlan{}, false
}

func (s *EvalState) executeEvalPipeline(source string) (any, bool, error) {
	_, executable, ok := s.compileEvalPipelineSource(source)
	if !ok {
		return nil, false, nil
	}
	return s.ExecuteEvalPipelineExecutablePlanRef(&executable)
}

func (s *EvalState) executeEvalPipelineBackendPlan(plan EvalPipelineBackendPlan) (any, bool, error) {
	executable, ok := CompileEvalPipelineBackendPlan(plan)
	if !ok {
		return nil, false, nil
	}
	return s.ExecuteEvalPipelineExecutablePlanRef(&executable)
}

func (s *EvalState) executeEvalPipelineDescriptor(descriptor EvalPipelineDescriptor) (any, bool, error) {
	plan, ok := CompileEvalPipelineDescriptor(descriptor)
	if !ok {
		return nil, false, nil
	}
	return s.ExecuteEvalPipelineExecutablePlanRef(&plan)
}

func evalScriptPipelineDescriptor(source string, d *qScriptPipelineDescriptor) EvalPipelineDescriptor {
	out := EvalPipelineDescriptor{
		Source:                 source,
		Kind:                   evalPipelineKindScript,
		Kernel:                 evalPipelineKernelScript,
		Shape:                  d.shape(),
		PipelineShape:          qRuntimeKernelPipelineShape(evalPipelineKernelScript, d.shape()),
		Terminal:               d.terminal,
		ValueExpr:              strings.TrimSpace(d.valueExpr),
		ValueBinding:           strings.TrimSpace(d.valueBinding),
		IndexExpr:              strings.TrimSpace(d.indexExpr),
		IndexBinding:           strings.TrimSpace(d.indexBinding),
		MaskExpr:               strings.TrimSpace(d.maskExpr),
		MaskBinding:            strings.TrimSpace(d.maskBinding),
		RowValueExpr:           strings.TrimSpace(d.rowValueExpr),
		RowIndexExpr:           strings.TrimSpace(d.rowIndexExpr),
		ColIndexExpr:           strings.TrimSpace(d.colIndexExpr),
		CallableExpr:           strings.TrimSpace(d.callableExpr),
		DyadicOp:               strings.TrimSpace(d.dyadicOp),
		ScalarExpr:             strings.TrimSpace(d.scalarExpr),
		ScalarLeft:             d.scalarLeft,
		SequenceValueExpr:      strings.TrimSpace(d.sequenceValueExpr),
		SequenceTransformChain: encodeQScriptPipelineSequenceTransformSteps(d.sequenceSteps),
		SequenceTransformNames: encodeQScriptPipelineNames(d.sequenceBindings),
		IncludeCount:           d.includeCount,
	}
	if len(d.integerTerms) > 0 {
		out.IntegerTerms = make([]EvalPipelineIntegerDivModTerm, 0, len(d.integerTerms))
		for _, term := range d.integerTerms {
			out.IntegerTerms = append(out.IntegerTerms, EvalPipelineIntegerDivModTerm{
				Op:         string(term.op),
				ScalarExpr: strings.TrimSpace(term.scalarExpr),
			})
		}
	}
	if out.ShapeFamily == "" && (d.kind == qScriptPipelineSequenceEdgeSum || d.kind == qScriptPipelineSequenceSumCount) {
		out.ShapeFamily = "sequence_edge"
		if d.kind == qScriptPipelineSequenceSumCount {
			out.ShapeReducer = "sum_count"
		} else {
			out.ShapeReducer = "sum_first_last"
		}
		if len(d.sequenceSteps) > 0 {
			out.ShapeTransform = qScriptPipelineSequenceTransformName(d.sequenceSteps)
		}
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
	out := EvalPipelineDescriptor{
		Source:         source,
		Kind:           evalPipelineKindExpression,
		Kernel:         evalPipelineKernelExpr,
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
		UnaryOp:        strings.TrimSpace(plan.unaryOp),
		CompareOp:      plan.compareOp,
		ComparePrefix:  plan.comparePrefix,
		ModExpr:        strings.TrimSpace(plan.modExpr),
		ModulusExpr:    strings.TrimSpace(plan.modulusExpr),
		ModTargetExpr:  strings.TrimSpace(plan.modTargetExpr),
		ReductionInput: strings.TrimSpace(plan.reductionInput),
	}
	if len(plan.castTerms) > 0 {
		out.CastTerms = make([]EvalPipelineCastTerm, 0, len(plan.castTerms))
		for _, term := range plan.castTerms {
			out.CastTerms = append(out.CastTerms, EvalPipelineCastTerm{
				DomainExpr: strings.TrimSpace(term.domainExpr),
				ValueExpr:  strings.TrimSpace(term.valueExpr),
				TargetKind: string(term.target.kind),
				StringCast: term.stringCast,
				Count:      term.count,
			})
		}
	}
	return out
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
		unaryOp:        strings.TrimSpace(descriptor.UnaryOp),
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
	case "apply-index/scalar-at":
		plan.kind = qPipelineApplyScalarIndex
		plan.compareOp = "at"
	case "apply-index/gather-at":
		plan.kind = qPipelineApplyGatherIndex
		plan.compareOp = "at"
	case "apply-index/scalar-dot":
		plan.kind = qPipelineApplyScalarIndex
		plan.compareOp = "dot"
	case "apply-index/path-dot":
		plan.kind = qPipelineApplyScalarIndex
		plan.compareOp = "dot"
	case "cast-envelope/sum":
		plan.kind = qPipelineCastEnvelopeSum
		plan.castTerms = make([]qPipelineCastTermPlan, 0, len(descriptor.CastTerms))
		for _, term := range descriptor.CastTerms {
			if strings.TrimSpace(term.ValueExpr) == "" || strings.TrimSpace(term.TargetKind) == "" {
				return qPipelinePlan{}, false
			}
			target := qCastTarget{kind: data.Kind(term.TargetKind), sourceText: strings.TrimSpace(term.DomainExpr)}
			if target.sourceText == "" {
				target.sourceText = string(target.kind)
			}
			plan.castTerms = append(plan.castTerms, qPipelineCastTermPlan{
				domainExpr: strings.TrimSpace(term.DomainExpr),
				valueExpr:  strings.TrimSpace(term.ValueExpr),
				target:     target,
				stringCast: term.StringCast,
				count:      term.Count,
			})
		}
	default:
		switch {
		case strings.HasPrefix(shape, "vector-reduce/sum-unary-"):
			plan.kind = qPipelineSumUnaryPrimitive
			if plan.unaryOp == "" {
				plan.unaryOp = strings.TrimPrefix(shape, "vector-reduce/sum-unary-")
			}
		case strings.HasPrefix(shape, "numeric-unary-compare-to-index/"):
			plan.kind = qPipelineWhereUnaryCompareIndexes
			if plan.unaryOp == "" {
				plan.unaryOp = strings.TrimPrefix(shape, "numeric-unary-compare-to-index/")
			}
			if plan.comparePrefix == "" {
				plan.comparePrefix = "numeric-unary-compare-to-index"
			}
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
		shapeText:         shape,
		terminal:          strings.TrimSpace(descriptor.Terminal),
		valueExpr:         strings.TrimSpace(descriptor.ValueExpr),
		valueBinding:      strings.TrimSpace(descriptor.ValueBinding),
		indexExpr:         strings.TrimSpace(descriptor.IndexExpr),
		indexBinding:      strings.TrimSpace(descriptor.IndexBinding),
		maskExpr:          strings.TrimSpace(descriptor.MaskExpr),
		maskBinding:       strings.TrimSpace(descriptor.MaskBinding),
		rowValueExpr:      strings.TrimSpace(descriptor.RowValueExpr),
		rowIndexExpr:      strings.TrimSpace(descriptor.RowIndexExpr),
		colIndexExpr:      strings.TrimSpace(descriptor.ColIndexExpr),
		callableExpr:      strings.TrimSpace(descriptor.CallableExpr),
		dyadicOp:          strings.TrimSpace(descriptor.DyadicOp),
		scalarExpr:        strings.TrimSpace(descriptor.ScalarExpr),
		scalarLeft:        descriptor.ScalarLeft,
		sequenceValueExpr: strings.TrimSpace(descriptor.SequenceValueExpr),
		sequenceBindings:  decodeQScriptPipelineNames(descriptor.SequenceTransformNames),
		includeCount:      descriptor.IncludeCount,
	}
	if steps, ok := decodeQScriptPipelineSequenceTransformSteps(descriptor.SequenceTransformChain); ok {
		out.sequenceSteps = steps
		out.sequenceShapeName = qScriptPipelineSequenceTransformName(steps)
	} else {
		return qScriptPipelineDescriptor{}, false
	}
	switch {
	case strings.Contains(shape, "sequence-edge-reduce/sum-first-last"):
		out.kind = qScriptPipelineSequenceEdgeSum
	case strings.Contains(shape, "sequence-reduce/sum-count"):
		out.kind = qScriptPipelineSequenceSumCount
	case strings.Contains(shape, "gather-reduce/sum-count"):
		out.kind = qScriptPipelineGatherReduceSumCount
		out.terminalPlan = qPipelinePlanWithBindingPlans(qPipelinePlan{
			kind:      qPipelineSumGatherIndexes,
			shape:     "gather-reduce/sum-count",
			valueExpr: out.valueExpr,
			indexExpr: out.indexExpr,
		})
	case strings.Contains(shape, "multi-reduce/sum-plus-dyadic-float-sum"):
		out.kind = qScriptPipelineSumPlusDyadicFloat
	case strings.Contains(shape, "multi-reduce/integer-divmod-sum-count"):
		out.kind = qScriptPipelineIntegerDivModReduce
	case strings.Contains(shape, "matrix-row-reduce/sum-count"):
		out.kind = qScriptPipelineMatrixRowSumCount
	case strings.Contains(shape, "matrix-rows-reduce/sum-plus-count"):
		out.kind = qScriptPipelineMatrixRowsSumCount
	case strings.Contains(shape, "matrix-cell-reduce/cell-plus-count"):
		out.kind = qScriptPipelineMatrixCellPlusCount
	case strings.Contains(shape, "matrix-nested-reduce/sum-cell-count"):
		out.kind = qScriptPipelineMatrixNestedCell
	case strings.Contains(shape, "callable-dot/sum-plus-count-right"):
		out.kind = qScriptPipelineCallableDotSumCount
		out.includeCount = true
	case strings.Contains(shape, "callable-dot/sum-plus-right"):
		out.kind = qScriptPipelineCallableDotSumRight
	case strings.Contains(shape, "callable-over/scan-sum-count"):
		out.kind = qScriptPipelineCallableOverScanSum
	case strings.Contains(shape, "string-join/counts"):
		out.kind = qScriptPipelineStringJoinCounts
	case strings.Contains(shape, "apply-index/scalar-at"):
		out.kind = qScriptPipelineApplyScalarAt
		out.terminalPlan = qPipelinePlanWithBindingPlans(qPipelinePlan{
			kind:      qPipelineApplyScalarIndex,
			shape:     "apply-index/scalar-at",
			compareOp: "at",
			valueExpr: out.valueExpr,
			indexExpr: out.indexExpr,
		})
	case strings.Contains(shape, "apply-index/gather-at"):
		out.kind = qScriptPipelineApplyGatherAt
		out.terminalPlan = qPipelinePlanWithBindingPlans(qPipelinePlan{
			kind:      qPipelineApplyGatherIndex,
			shape:     "apply-index/gather-at",
			compareOp: "at",
			valueExpr: out.valueExpr,
			indexExpr: out.indexExpr,
		})
	case strings.Contains(shape, "apply-index/scalar-dot"):
		out.kind = qScriptPipelineApplyScalarDot
		out.terminalPlan = qPipelinePlanWithBindingPlans(qPipelinePlan{
			kind:      qPipelineApplyScalarIndex,
			shape:     "apply-index/scalar-dot",
			compareOp: "dot",
			valueExpr: out.valueExpr,
			indexExpr: out.indexExpr,
		})
	case strings.Contains(shape, "apply-index/path-dot"):
		out.kind = qScriptPipelineApplyPathDot
		out.terminalPlan = qPipelinePlanWithBindingPlans(qPipelinePlan{
			kind:      qPipelineApplyScalarIndex,
			shape:     "apply-index/path-dot",
			compareOp: "dot",
			valueExpr: out.valueExpr,
			indexExpr: out.indexExpr,
		})
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
	if out.kind == qScriptPipelineCallableOverScanSum && strings.TrimSpace(out.valueBinding) != "" {
		out.valuePlan = buildQScriptWarmBindingPlan(out.valueBinding, parseCachedValueExpr(out.valueBinding))
	}
	if out.kind == qScriptPipelineStringJoinCounts {
		qScriptPipelineHoistStringJoinCounts(&out)
	}
	out.indexPlan = buildQScriptBindingPlanForRHS(out.indexExpr, nil)
	out.maskPlan = buildQScriptBindingPlanForRHS(out.maskExpr, nil)
	out.rowValuePlan = buildQScriptBindingPlanForRHS(out.rowValueExpr, nil)
	out.rowIndexPlan = buildQScriptBindingPlanForRHS(out.rowIndexExpr, nil)
	out.colIndexPlan = buildQScriptBindingPlanForRHS(out.colIndexExpr, nil)
	out.scalarPlan = buildQScriptBindingPlanForRHS(out.scalarExpr, nil)
	out.sequenceValuePlan = buildQScriptBindingPlanForRHS(out.sequenceValueExpr, nil)
	out.moduloMaskPlan = qScriptPipelineModuloMaskPlan(out.maskExpr)
	if len(descriptor.IntegerTerms) > 0 {
		out.integerTerms = make([]qScriptPipelineIntegerDivModTerm, 0, len(descriptor.IntegerTerms))
		for _, term := range descriptor.IntegerTerms {
			op := data.Op(strings.TrimSpace(term.Op))
			if op != data.OpDiv && op != data.OpMod {
				return qScriptPipelineDescriptor{}, false
			}
			scalarExpr := strings.TrimSpace(term.ScalarExpr)
			if scalarExpr == "" {
				return qScriptPipelineDescriptor{}, false
			}
			out.integerTerms = append(out.integerTerms, qScriptPipelineIntegerDivModTerm{
				op:         op,
				valueExpr:  out.valueExpr,
				scalarExpr: scalarExpr,
			})
		}
	}
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
