package methodjit

import (
	"reflect"
	"strings"
	"sync"

	"github.com/never-labs/leia/internal/runtime"
	stdq "github.com/never-labs/leia/internal/stdlib/lib/q"
)

const qEvalPipelineTypedRuntimeBackend = stdq.EvalPipelineTypedRuntimeBackend

// QEvalPipelineDescriptor is MethodJIT's metadata-only view of a q runtime
// pipeline. It deliberately mirrors only stable handoff fields, not q AST nodes
// or EvalState-owned runtime values.
type QEvalPipelineDescriptor struct {
	Kernel                 string
	Shape                  string
	PipelineShape          string
	ShapeFamily            string
	ShapeReducer           string
	ShapeSelector          string
	ShapeTransform         string
	Source                 string
	Backend                string
	Detail                 string
	Kind                   string
	Terminal               string
	AssignmentText         string
	ValueExpr              string
	ValueBinding           string
	IndexExpr              string
	IndexBinding           string
	MaskExpr               string
	MaskBinding            string
	RowValueExpr           string
	RowIndexExpr           string
	ColIndexExpr           string
	CallableExpr           string
	DyadicOp               string
	ScalarExpr             string
	ScalarLeft             bool
	IncludeCount           bool
	SequenceValueExpr      string
	SequenceTransformChain string
	SequenceTransformNames string
	LeftExpr               string
	RightExpr              string
	CompareOp              string
	ComparePrefix          string
	ModExpr                string
	ModulusExpr            string
	ModTargetExpr          string
	ReductionInput         string
}

// QEvalPipelinePlanner describes q.eval source strings that can be handed to
// the q runtime pipeline backend. Implementations own q syntax normalization,
// environment binding, and future runtime execution.
type QEvalPipelinePlanner interface {
	DescribeQEvalPipeline(source string) (QEvalPipelineDescriptor, bool)
}

type qRuntimeEvalPipelinePlanner struct{}

type qEvalPipelineBackendPlan struct {
	plan           stdq.EvalPipelineBackendPlan
	assignmentText string
}

func (p qEvalPipelineBackendPlan) valid() bool {
	return p.plan.Valid()
}

func (p qEvalPipelineBackendPlan) methodDescriptor() QEvalPipelineDescriptor {
	return qEvalMethodDescriptorFromBackendPlan(p.plan, p.assignmentText)
}

func qEvalMethodDescriptorFromBackendPlan(plan stdq.EvalPipelineBackendPlan, assignmentText string) QEvalPipelineDescriptor {
	descriptor := plan.Descriptor
	return QEvalPipelineDescriptor{
		Kernel:                 descriptor.Kernel,
		Shape:                  descriptor.Shape,
		PipelineShape:          descriptor.PipelineShape,
		ShapeFamily:            descriptor.ShapeFamily,
		ShapeReducer:           descriptor.ShapeReducer,
		ShapeSelector:          descriptor.ShapeSelector,
		ShapeTransform:         descriptor.ShapeTransform,
		Source:                 descriptor.Source,
		Backend:                plan.Backend,
		Detail:                 plan.Detail,
		Kind:                   descriptor.Kind,
		Terminal:               descriptor.Terminal,
		AssignmentText:         assignmentText,
		ValueExpr:              descriptor.ValueExpr,
		ValueBinding:           descriptor.ValueBinding,
		IndexExpr:              descriptor.IndexExpr,
		IndexBinding:           descriptor.IndexBinding,
		MaskExpr:               descriptor.MaskExpr,
		MaskBinding:            descriptor.MaskBinding,
		RowValueExpr:           descriptor.RowValueExpr,
		RowIndexExpr:           descriptor.RowIndexExpr,
		ColIndexExpr:           descriptor.ColIndexExpr,
		CallableExpr:           descriptor.CallableExpr,
		DyadicOp:               descriptor.DyadicOp,
		ScalarExpr:             descriptor.ScalarExpr,
		ScalarLeft:             descriptor.ScalarLeft,
		IncludeCount:           descriptor.IncludeCount,
		SequenceValueExpr:      descriptor.SequenceValueExpr,
		SequenceTransformChain: descriptor.SequenceTransformChain,
		SequenceTransformNames: descriptor.SequenceTransformNames,
		LeftExpr:               descriptor.LeftExpr,
		RightExpr:              descriptor.RightExpr,
		CompareOp:              descriptor.CompareOp,
		ComparePrefix:          descriptor.ComparePrefix,
		ModExpr:                descriptor.ModExpr,
		ModulusExpr:            descriptor.ModulusExpr,
		ModTargetExpr:          descriptor.ModTargetExpr,
		ReductionInput:         descriptor.ReductionInput,
	}
}

func qEvalHotPlanFromDescriptor(plan stdq.EvalPipelineBackendPlan, assignmentText string) qEvalHotPlan {
	descriptor := plan.Descriptor
	planValue := plan
	return qEvalHotPlan{
		BackendPlan:            &planValue,
		Kernel:                 descriptor.Kernel,
		Shape:                  descriptor.Shape,
		PipelineShape:          descriptor.PipelineShape,
		ShapeFamily:            descriptor.ShapeFamily,
		ShapeReducer:           descriptor.ShapeReducer,
		ShapeSelector:          descriptor.ShapeSelector,
		ShapeTransform:         descriptor.ShapeTransform,
		Backend:                plan.Backend,
		Detail:                 plan.Detail,
		Kind:                   descriptor.Kind,
		Terminal:               descriptor.Terminal,
		AssignmentText:         assignmentText,
		ValueExpr:              descriptor.ValueExpr,
		ValueBinding:           descriptor.ValueBinding,
		IndexExpr:              descriptor.IndexExpr,
		IndexBinding:           descriptor.IndexBinding,
		MaskExpr:               descriptor.MaskExpr,
		MaskBinding:            descriptor.MaskBinding,
		RowValueExpr:           descriptor.RowValueExpr,
		RowIndexExpr:           descriptor.RowIndexExpr,
		ColIndexExpr:           descriptor.ColIndexExpr,
		CallableExpr:           descriptor.CallableExpr,
		DyadicOp:               descriptor.DyadicOp,
		ScalarExpr:             descriptor.ScalarExpr,
		ScalarLeft:             descriptor.ScalarLeft,
		IncludeCount:           descriptor.IncludeCount,
		SequenceValueExpr:      descriptor.SequenceValueExpr,
		SequenceTransformChain: descriptor.SequenceTransformChain,
		SequenceTransformNames: descriptor.SequenceTransformNames,
		LeftExpr:               descriptor.LeftExpr,
		RightExpr:              descriptor.RightExpr,
		CompareOp:              descriptor.CompareOp,
		ComparePrefix:          descriptor.ComparePrefix,
		ModExpr:                descriptor.ModExpr,
		ModulusExpr:            descriptor.ModulusExpr,
		ModTargetExpr:          descriptor.ModTargetExpr,
		ReductionInput:         descriptor.ReductionInput,
	}
}

func (qRuntimeEvalPipelinePlanner) DescribeQEvalPipelineBackendPlan(source string) (qEvalPipelineBackendPlan, bool) {
	plan, ok := stdq.DescribeEvalPipelineBackendPlan(source)
	if !ok {
		return qEvalPipelineBackendPlan{}, false
	}
	return qEvalPipelineBackendPlan{
		plan:           plan,
		assignmentText: encodeQEvalPipelineAssignments(plan.Descriptor.Assignments),
	}, true
}

func (qRuntimeEvalPipelinePlanner) DescribeQEvalPipeline(source string) (QEvalPipelineDescriptor, bool) {
	plan, ok := qRuntimeEvalPipelinePlanner{}.DescribeQEvalPipelineBackendPlan(source)
	if !ok {
		return QEvalPipelineDescriptor{}, false
	}
	return plan.methodDescriptor(), true
}

func qEvalTypedRuntimeBackendPlanForCacheableSource(source string) (stdq.EvalPipelineBackendPlan, bool) {
	if !stdq.EvalSourceCacheable(source) {
		return stdq.EvalPipelineBackendPlan{}, false
	}
	plan, ok := qRuntimeEvalPipelinePlanner{}.DescribeQEvalPipelineBackendPlan(source)
	if !ok || !plan.valid() || plan.plan.Backend != qEvalPipelineTypedRuntimeBackend {
		return stdq.EvalPipelineBackendPlan{}, false
	}
	return plan.plan, true
}

func qEvalRuntimePipelinePlan(source string) (qEvalHotPlan, bool) {
	backendPlan, ok := qRuntimeEvalPipelinePlanner{}.DescribeQEvalPipelineBackendPlan(source)
	if !ok {
		return qEvalHotPlan{}, false
	}
	return qEvalHotPlanFromBackendPlan(backendPlan), true
}

func qEvalHotPlanFromBackendPlan(backendPlan qEvalPipelineBackendPlan) qEvalHotPlan {
	return qEvalHotPlanFromDescriptor(backendPlan.plan, backendPlan.assignmentText)
}

func qEvalPipelinePlanRefFromHotPlan(id int, source string, plan qEvalHotPlan) QEvalPipelinePlanRef {
	backendPlan := qEvalPipelineBackendPlanFromHotPlan(source, plan)
	ref := QEvalPipelinePlanRef{ID: id}
	qEvalPipelineMirrorRefFromDescriptor(&ref, qEvalPipelineDescriptorFromHotPlan(source, plan), plan.Backend)
	if backendPlan.Valid() {
		ref.BackendPlan = &backendPlan
		qEvalPipelineMirrorRefFromBackendPlan(&ref, backendPlan)
	}
	return ref
}

func qEvalPipelineBackendPlanFromHotPlan(source string, plan qEvalHotPlan) stdq.EvalPipelineBackendPlan {
	if plan.BackendPlan != nil && plan.BackendPlan.Valid() {
		return *plan.BackendPlan
	}
	if plan.Backend == "" || plan.Kind == "" || plan.Kernel == "" || plan.Shape == "" {
		return stdq.EvalPipelineBackendPlan{}
	}
	return stdq.EvalPipelineBackendPlan{
		Backend:    plan.Backend,
		Detail:     plan.Detail,
		Descriptor: qEvalPipelineDescriptorFromHotPlan(source, plan),
	}
}

func qEvalPipelineDescriptorFromHotPlan(source string, plan qEvalHotPlan) stdq.EvalPipelineDescriptor {
	return stdq.EvalPipelineDescriptor{
		Source:                 source,
		Kind:                   plan.Kind,
		Kernel:                 plan.Kernel,
		Shape:                  plan.Shape,
		PipelineShape:          plan.PipelineShape,
		ShapeFamily:            plan.ShapeFamily,
		ShapeReducer:           plan.ShapeReducer,
		ShapeSelector:          plan.ShapeSelector,
		ShapeTransform:         plan.ShapeTransform,
		Terminal:               plan.Terminal,
		Assignments:            decodeQEvalPipelineAssignments(plan.AssignmentText),
		ValueExpr:              plan.ValueExpr,
		ValueBinding:           plan.ValueBinding,
		IndexExpr:              plan.IndexExpr,
		IndexBinding:           plan.IndexBinding,
		MaskExpr:               plan.MaskExpr,
		MaskBinding:            plan.MaskBinding,
		RowValueExpr:           plan.RowValueExpr,
		RowIndexExpr:           plan.RowIndexExpr,
		ColIndexExpr:           plan.ColIndexExpr,
		CallableExpr:           plan.CallableExpr,
		DyadicOp:               plan.DyadicOp,
		ScalarExpr:             plan.ScalarExpr,
		ScalarLeft:             plan.ScalarLeft,
		IncludeCount:           plan.IncludeCount,
		SequenceValueExpr:      plan.SequenceValueExpr,
		SequenceTransformChain: plan.SequenceTransformChain,
		SequenceTransformNames: plan.SequenceTransformNames,
		LeftExpr:               plan.LeftExpr,
		RightExpr:              plan.RightExpr,
		CompareOp:              plan.CompareOp,
		ComparePrefix:          plan.ComparePrefix,
		ModExpr:                plan.ModExpr,
		ModulusExpr:            plan.ModulusExpr,
		ModTargetExpr:          plan.ModTargetExpr,
		ReductionInput:         plan.ReductionInput,
	}
}

type qRuntimeEvalPipelineBackend struct {
	static             qEvalPipelineStaticBackend
	descriptorByID     map[int]stdq.EvalPipelineDescriptor
	backendPlanByID    map[int]stdq.EvalPipelineBackendPlan
	executablePlanByID map[int]stdq.EvalPipelineExecutablePlan
	evalStateByID      map[int]*qEvalPipelineBackendEvalState
	executeExecutable  func(stdq.EvalPipelineExecutablePlan) (any, bool, error)
	executeBackendPlan func(stdq.EvalPipelineBackendPlan) (any, bool, error)
	executeDescriptor  func(stdq.EvalPipelineDescriptor) (any, bool, error)
	executeSource      func(string) (any, bool, error)
}

type qEvalPipelineBackendEvalState struct {
	mu    sync.Mutex
	state *stdq.EvalState
}

type qEvalPipelinePlanHelper struct {
	ref                QEvalPipelinePlanRef
	descriptor         stdq.EvalPipelineDescriptor
	backendPlan        stdq.EvalPipelineBackendPlan
	executablePlan     stdq.EvalPipelineExecutablePlan
	hasDescriptor      bool
	hasBackendPlan     bool
	hasExecutablePlan  bool
	evalState          *stdq.EvalState
	evalStateMu        sync.Mutex
	executeExecutable  func(stdq.EvalPipelineExecutablePlan) (any, bool, error)
	executeBackendPlan func(stdq.EvalPipelineBackendPlan) (any, bool, error)
	executeDescriptor  func(stdq.EvalPipelineDescriptor) (any, bool, error)
	executeSource      func(string) (any, bool, error)
}

func newQRuntimeEvalPipelineBackend(refs []QEvalPipelinePlanRef) qRuntimeEvalPipelineBackend {
	descriptorByID := make(map[int]stdq.EvalPipelineDescriptor, len(refs))
	backendPlanByID := make(map[int]stdq.EvalPipelineBackendPlan, len(refs))
	executablePlanByID := make(map[int]stdq.EvalPipelineExecutablePlan, len(refs))
	evalStateByID := make(map[int]*qEvalPipelineBackendEvalState, len(refs))
	for _, ref := range refs {
		if plan, ok := qEvalPipelineExecutableTypedRuntimeBackendPlanFromRef(ref); ok {
			descriptorByID[ref.ID] = plan.Descriptor
			backendPlanByID[ref.ID] = plan
			if executable, ok := stdq.CompileEvalPipelineBackendPlan(plan); ok {
				executablePlanByID[ref.ID] = executable
				evalStateByID[ref.ID] = &qEvalPipelineBackendEvalState{state: stdq.NewEvalState(nil)}
			}
		}
	}
	return qRuntimeEvalPipelineBackend{
		static:             newQEvalPipelineStaticBackend(qEvalPipelineTypedRuntimeBackend, refs),
		descriptorByID:     descriptorByID,
		backendPlanByID:    backendPlanByID,
		executablePlanByID: executablePlanByID,
		evalStateByID:      evalStateByID,
		executeBackendPlan: stdq.ExecuteEvalPipelineBackendPlan,
		executeDescriptor:  stdq.ExecuteEvalPipelineDescriptor,
		executeSource:      stdq.ExecuteEvalPipeline,
	}
}

func newQEvalPipelinePlanHelpers(refs []QEvalPipelinePlanRef, backend qRuntimeEvalPipelineBackend) []qEvalPipelinePlanHelper {
	if len(refs) == 0 {
		return nil
	}
	helpers := make([]qEvalPipelinePlanHelper, len(refs))
	for _, ref := range refs {
		if !ref.Valid() || ref.ID < 0 || ref.ID >= len(helpers) || qEvalPipelineBackendNameFromRef(ref) != qEvalPipelineTypedRuntimeBackend {
			continue
		}
		if _, ok := backend.LookupQEvalPipelinePlan(ref); !ok {
			continue
		}
		backendPlan, hasBackendPlan := backend.lookupBackendPlan(ref)
		if !hasBackendPlan {
			continue
		}
		descriptor, hasDescriptor := backend.lookupDescriptor(ref)
		executablePlan, hasExecutablePlan := backend.lookupExecutablePlan(ref)
		helpers[ref.ID] = qEvalPipelinePlanHelper{
			ref:                ref,
			descriptor:         descriptor,
			backendPlan:        backendPlan,
			executablePlan:     executablePlan,
			hasDescriptor:      hasDescriptor,
			hasBackendPlan:     hasBackendPlan,
			hasExecutablePlan:  hasExecutablePlan,
			evalState:          qEvalPipelineReusableEvalState(ref, backend, hasExecutablePlan),
			executeExecutable:  backend.executeExecutable,
			executeBackendPlan: backend.executeBackendPlan,
			executeDescriptor:  backend.executeDescriptor,
			executeSource:      backend.executeSource,
		}
	}
	return helpers
}

func qEvalPipelineReusableEvalState(ref QEvalPipelinePlanRef, backend qRuntimeEvalPipelineBackend, hasExecutablePlan bool) *stdq.EvalState {
	if !ref.Valid() {
		return nil
	}
	if hasExecutablePlan {
		if backend.executeExecutable != nil {
			return nil
		}
		// The q package has already validated and compiled the descriptor into
		// an opaque executable plan, so a source round-trip is unnecessary.
	} else if qEvalPipelinePlanRefSource(ref) == "" || !stdq.EvalSourceCacheable(qEvalPipelinePlanRefSource(ref)) {
		return nil
	} else {
		plan, ok := qRuntimeEvalPipelinePlanner{}.DescribeQEvalPipelineBackendPlan(qEvalPipelinePlanRefSource(ref))
		if !ok || !plan.valid() ||
			plan.plan.Kind() != qEvalPipelinePlanRefKind(ref) ||
			plan.plan.Shape() != qEvalPipelinePlanRefShape(ref) ||
			plan.plan.PipelineShape() != qEvalPipelinePlanRefPipelineShape(ref) {
			return nil
		}
	}
	if !hasExecutablePlan && qEvalPipelinePlanRefSource(ref) == "" {
		return nil
	}
	if !hasExecutablePlan {
		if backend.executeBackendPlan != nil && !sameQEvalPipelineBackendPlanExecutor(backend.executeBackendPlan, stdq.ExecuteEvalPipelineBackendPlan) {
			return nil
		}
		if backend.executeDescriptor != nil && !sameQEvalPipelineDescriptorExecutor(backend.executeDescriptor, stdq.ExecuteEvalPipelineDescriptor) {
			return nil
		}
		if backend.executeSource != nil && !sameQEvalPipelineSourceExecutor(backend.executeSource, stdq.ExecuteEvalPipeline) {
			return nil
		}
	}
	return stdq.NewEvalState(nil)
}

func sameQEvalPipelineBackendPlanExecutor(a, b func(stdq.EvalPipelineBackendPlan) (any, bool, error)) bool {
	if a == nil || b == nil {
		return false
	}
	return reflect.ValueOf(a).Pointer() == reflect.ValueOf(b).Pointer()
}

func sameQEvalPipelineDescriptorExecutor(a, b func(stdq.EvalPipelineDescriptor) (any, bool, error)) bool {
	if a == nil || b == nil {
		return false
	}
	return reflect.ValueOf(a).Pointer() == reflect.ValueOf(b).Pointer()
}

func sameQEvalPipelineSourceExecutor(a, b func(string) (any, bool, error)) bool {
	if a == nil || b == nil {
		return false
	}
	return reflect.ValueOf(a).Pointer() == reflect.ValueOf(b).Pointer()
}

func (h *qEvalPipelinePlanHelper) validForID(id int) bool {
	return h != nil && h.ref.ID == id && (h.hasExecutablePlan || h.hasBackendPlan) && qEvalPipelineBackendNameFromRef(h.ref) == qEvalPipelineTypedRuntimeBackend
}

func (h *qEvalPipelinePlanHelper) execute() (runtime.Value, bool, error) {
	if h == nil {
		return runtime.NilValue(), false, nil
	}
	if !h.validForID(h.ref.ID) {
		return runtime.NilValue(), false, nil
	}
	if h.hasExecutablePlan && h.executeExecutable != nil {
		out, handled, err := h.executeExecutable(h.executablePlan)
		if err != nil || !handled {
			return runtime.NilValue(), handled, err
		}
		value, err := qEvalPipelineRuntimeValue(out)
		if err != nil {
			return runtime.NilValue(), false, err
		}
		return value, true, nil
	}
	if h.evalState != nil {
		h.evalStateMu.Lock()
		var (
			out     any
			handled bool
			err     error
		)
		if h.hasExecutablePlan {
			out, handled, err = h.evalState.ExecuteEvalPipelineExecutablePlanRefHot(&h.executablePlan)
		} else if h.hasBackendPlan {
			out, handled, err = h.evalState.ExecuteEvalPipelineBackendPlan(h.backendPlan)
		} else if h.hasDescriptor {
			out, handled, err = h.evalState.ExecuteEvalPipelineDescriptor(h.descriptor)
		} else {
			out, err = h.evalState.Eval(qEvalPipelinePlanRefSource(h.ref))
			handled = err == nil
		}
		h.evalStateMu.Unlock()
		if err != nil || !handled {
			return runtime.NilValue(), handled, err
		}
		value, err := qEvalPipelineRuntimeValue(out)
		if err != nil {
			return runtime.NilValue(), false, err
		}
		return value, true, nil
	}
	var (
		out     any
		handled bool
		err     error
	)
	if h.hasExecutablePlan {
		out, handled, err = stdq.NewEvalState(nil).ExecuteEvalPipelineExecutablePlanRefHot(&h.executablePlan)
	} else if h.hasBackendPlan {
		execute := h.executeBackendPlan
		if execute == nil {
			execute = stdq.ExecuteEvalPipelineBackendPlan
		}
		out, handled, err = execute(h.backendPlan)
	} else if h.hasDescriptor {
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
		out, handled, err = execute(qEvalPipelinePlanRefSource(h.ref))
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
	if qEvalPipelineBackendNameFromRef(ref) != qEvalPipelineTypedRuntimeBackend {
		return nil, false, nil
	}
	if _, ok := b.LookupQEvalPipelinePlan(ref); !ok {
		return nil, false, nil
	}
	if executable, ok := b.lookupExecutablePlan(ref); ok {
		return b.executeEvalPipelineExecutablePlan(executable)
	}
	if backendPlan, ok := b.lookupBackendPlan(ref); ok {
		return b.executeEvalPipelineBackendPlan(backendPlan)
	}
	if descriptor, ok := b.lookupDescriptor(ref); ok {
		return b.executeEvalPipelineDescriptor(descriptor)
	}
	return b.executeEvalPipelineSource(qEvalPipelinePlanRefSource(ref))
}

func (b qRuntimeEvalPipelineBackend) ExecuteQEvalPipelinePlanValue(ref QEvalPipelinePlanRef) (runtime.Value, bool, error) {
	if qEvalPipelineBackendNameFromRef(ref) != qEvalPipelineTypedRuntimeBackend {
		return runtime.NilValue(), false, nil
	}
	if executable, ok := b.lookupExecutablePlan(ref); ok && b.executeExecutable == nil {
		if state := b.lookupEvalState(ref.ID); state != nil {
			state.mu.Lock()
			out, handled, err := state.state.ExecuteEvalPipelineExecutablePlanRefHot(&executable)
			state.mu.Unlock()
			if err != nil || !handled {
				return runtime.NilValue(), handled, err
			}
			value, err := qEvalPipelineRuntimeValue(out)
			if err != nil {
				return runtime.NilValue(), false, err
			}
			return value, true, nil
		}
	}
	if _, ok := b.lookupBackendPlan(ref); !ok {
		return runtime.NilValue(), false, nil
	}
	out, handled, err := b.ExecuteQEvalPipelinePlan(ref)
	if err != nil || !handled {
		return runtime.NilValue(), handled, err
	}
	value, err := qEvalPipelineRuntimeValue(out)
	if err != nil {
		return runtime.NilValue(), false, err
	}
	return value, true, nil
}

func (b qRuntimeEvalPipelineBackend) lookupExecutablePlan(ref QEvalPipelinePlanRef) (stdq.EvalPipelineExecutablePlan, bool) {
	if b.executablePlanByID == nil {
		return stdq.EvalPipelineExecutablePlan{}, false
	}
	plan, ok := b.executablePlanByID[ref.ID]
	return plan, ok && plan.Valid()
}

func (b qRuntimeEvalPipelineBackend) lookupEvalState(id int) *qEvalPipelineBackendEvalState {
	if b.evalStateByID == nil {
		return nil
	}
	return b.evalStateByID[id]
}

func (b qRuntimeEvalPipelineBackend) lookupBackendPlan(ref QEvalPipelinePlanRef) (stdq.EvalPipelineBackendPlan, bool) {
	if b.backendPlanByID != nil {
		if plan, ok := b.backendPlanByID[ref.ID]; ok && plan.Valid() {
			return plan, true
		}
	}
	return qEvalPipelineExecutableTypedRuntimeBackendPlanFromRef(ref)
}

func (b qRuntimeEvalPipelineBackend) lookupDescriptor(ref QEvalPipelinePlanRef) (stdq.EvalPipelineDescriptor, bool) {
	if b.descriptorByID != nil {
		if descriptor, ok := b.descriptorByID[ref.ID]; ok {
			return descriptor, true
		}
	}
	return stdq.EvalPipelineDescriptor{}, false
}

func (b qRuntimeEvalPipelineBackend) executeEvalPipelineBackendPlan(plan stdq.EvalPipelineBackendPlan) (any, bool, error) {
	if b.executeBackendPlan != nil {
		return b.executeBackendPlan(plan)
	}
	return stdq.ExecuteEvalPipelineBackendPlan(plan)
}

func (b qRuntimeEvalPipelineBackend) executeEvalPipelineExecutablePlan(plan stdq.EvalPipelineExecutablePlan) (any, bool, error) {
	if b.executeExecutable != nil {
		return b.executeExecutable(plan)
	}
	return stdq.NewEvalState(nil).ExecuteEvalPipelineExecutablePlanRefHot(&plan)
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
	return qEvalPipelineDescriptorViewFromRef(ref)
}

func qEvalPipelineBackendPlanFromRef(ref QEvalPipelinePlanRef) (stdq.EvalPipelineBackendPlan, bool) {
	if ref.BackendPlan != nil && ref.BackendPlan.Valid() {
		return *ref.BackendPlan, true
	}
	source := qEvalPipelinePlanRefMirroredSource(ref)
	if source != "" {
		plan, ok := qRuntimeEvalPipelinePlanner{}.DescribeQEvalPipelineBackendPlan(source)
		if ok && plan.valid() {
			return qEvalPipelineBackendPlanFromRefSourcePlan(ref, plan.plan)
		}
	}
	return qEvalPipelineLegacyBackendPlanFromMirroredDescriptor(ref)
}

func qEvalPipelineTypedRuntimeBackendPlanFromRef(ref QEvalPipelinePlanRef) (stdq.EvalPipelineBackendPlan, bool) {
	plan, ok := qEvalPipelineBackendPlanFromRef(ref)
	if !ok || !plan.Valid() || plan.Backend != qEvalPipelineTypedRuntimeBackend {
		return stdq.EvalPipelineBackendPlan{}, false
	}
	return plan, true
}

func qEvalPipelineExecutableTypedRuntimeBackendPlanFromRef(ref QEvalPipelinePlanRef) (stdq.EvalPipelineBackendPlan, bool) {
	if ref.BackendPlan != nil && ref.BackendPlan.Valid() && ref.BackendPlan.Backend == qEvalPipelineTypedRuntimeBackend {
		return *ref.BackendPlan, true
	}
	source := qEvalPipelinePlanRefMirroredSource(ref)
	if source == "" {
		return stdq.EvalPipelineBackendPlan{}, false
	}
	plan, ok := qRuntimeEvalPipelinePlanner{}.DescribeQEvalPipelineBackendPlan(source)
	if !ok || !plan.valid() || plan.plan.Backend != qEvalPipelineTypedRuntimeBackend {
		return stdq.EvalPipelineBackendPlan{}, false
	}
	return qEvalPipelineBackendPlanFromRefSourcePlan(ref, plan.plan)
}

func qEvalPipelineBackendPlanFromRefSourcePlan(ref QEvalPipelinePlanRef, plan stdq.EvalPipelineBackendPlan) (stdq.EvalPipelineBackendPlan, bool) {
	if !plan.Valid() {
		return stdq.EvalPipelineBackendPlan{}, false
	}
	if backend := qEvalPipelineBackendNameFromRef(ref); backend != "" && backend != plan.Backend {
		return stdq.EvalPipelineBackendPlan{}, false
	}
	if kind := qEvalPipelinePlanRefKind(ref); kind != "" && kind != plan.Kind() {
		return stdq.EvalPipelineBackendPlan{}, false
	}
	if shape := qEvalPipelinePlanRefShape(ref); shape != "" && shape != plan.Shape() {
		return stdq.EvalPipelineBackendPlan{}, false
	}
	if pipelineShape := qEvalPipelinePlanRefPipelineShape(ref); pipelineShape != "" && pipelineShape != plan.PipelineShape() {
		return stdq.EvalPipelineBackendPlan{}, false
	}
	return plan, true
}

func qEvalPipelineLegacyBackendPlanFromMirroredDescriptor(ref QEvalPipelinePlanRef) (stdq.EvalPipelineBackendPlan, bool) {
	descriptor, ok := qEvalPipelineMirroredDescriptorFromRef(ref)
	if !ok || descriptor.Kind == "" {
		return stdq.EvalPipelineBackendPlan{}, false
	}
	plan := stdq.EvalPipelineBackendPlan{
		Backend:    ref.Backend,
		Detail:     "kind=" + descriptor.Kind,
		Descriptor: descriptor,
	}
	if !plan.Valid() {
		return stdq.EvalPipelineBackendPlan{}, false
	}
	return plan, true
}

func qEvalPipelineDescriptorViewFromRef(ref QEvalPipelinePlanRef) (stdq.EvalPipelineDescriptor, bool) {
	if ref.BackendPlan != nil && ref.BackendPlan.Valid() {
		return ref.BackendPlan.Descriptor, true
	}
	return qEvalPipelineMirroredDescriptorFromRef(ref)
}

func qEvalPipelineMirroredDescriptorFromRef(ref QEvalPipelinePlanRef) (stdq.EvalPipelineDescriptor, bool) {
	if ref.Kernel == "" || ref.Shape == "" {
		return stdq.EvalPipelineDescriptor{}, false
	}
	return stdq.EvalPipelineDescriptor{
		Source:                 ref.Source,
		Kind:                   ref.Kind,
		Kernel:                 ref.Kernel,
		Shape:                  ref.Shape,
		PipelineShape:          ref.PipelineShape,
		ShapeFamily:            ref.ShapeFamily,
		ShapeReducer:           ref.ShapeReducer,
		ShapeSelector:          ref.ShapeSelector,
		ShapeTransform:         ref.ShapeTransform,
		Terminal:               ref.Terminal,
		Assignments:            decodeQEvalPipelineAssignments(ref.AssignmentText),
		ValueExpr:              ref.ValueExpr,
		ValueBinding:           ref.ValueBinding,
		IndexExpr:              ref.IndexExpr,
		IndexBinding:           ref.IndexBinding,
		MaskExpr:               ref.MaskExpr,
		MaskBinding:            ref.MaskBinding,
		RowValueExpr:           ref.RowValueExpr,
		RowIndexExpr:           ref.RowIndexExpr,
		ColIndexExpr:           ref.ColIndexExpr,
		CallableExpr:           ref.CallableExpr,
		DyadicOp:               ref.DyadicOp,
		ScalarExpr:             ref.ScalarExpr,
		ScalarLeft:             ref.ScalarLeft,
		IncludeCount:           ref.IncludeCount,
		SequenceValueExpr:      ref.SequenceValueExpr,
		SequenceTransformChain: ref.SequenceTransformChain,
		SequenceTransformNames: ref.SequenceTransformNames,
		LeftExpr:               ref.LeftExpr,
		RightExpr:              ref.RightExpr,
		CompareOp:              ref.CompareOp,
		ComparePrefix:          ref.ComparePrefix,
		ModExpr:                ref.ModExpr,
		ModulusExpr:            ref.ModulusExpr,
		ModTargetExpr:          ref.ModTargetExpr,
		ReductionInput:         ref.ReductionInput,
	}, true
}

func qEvalPipelineBackendNameFromRef(ref QEvalPipelinePlanRef) string {
	if ref.BackendPlan != nil && ref.BackendPlan.Valid() && ref.BackendPlan.Backend != "" {
		return ref.BackendPlan.Backend
	}
	return ref.Backend
}

func qEvalPipelinePlanRefSource(ref QEvalPipelinePlanRef) string {
	if descriptor, ok := qEvalPipelineDescriptorViewFromRef(ref); ok {
		return descriptor.Source
	}
	return ref.Source
}

func qEvalPipelinePlanRefMirroredSource(ref QEvalPipelinePlanRef) string {
	if ref.Source != "" {
		return ref.Source
	}
	return qEvalPipelinePlanRefSource(ref)
}

func qEvalPipelinePlanRefKind(ref QEvalPipelinePlanRef) string {
	if descriptor, ok := qEvalPipelineDescriptorViewFromRef(ref); ok {
		return descriptor.Kind
	}
	return ref.Kind
}

func qEvalPipelinePlanRefKernel(ref QEvalPipelinePlanRef) string {
	if descriptor, ok := qEvalPipelineDescriptorViewFromRef(ref); ok {
		return descriptor.Kernel
	}
	return ref.Kernel
}

func qEvalPipelinePlanRefShape(ref QEvalPipelinePlanRef) string {
	if descriptor, ok := qEvalPipelineDescriptorViewFromRef(ref); ok {
		return descriptor.Shape
	}
	return ref.Shape
}

func qEvalPipelinePlanRefPipelineShape(ref QEvalPipelinePlanRef) string {
	if descriptor, ok := qEvalPipelineDescriptorViewFromRef(ref); ok {
		return descriptor.PipelineShape
	}
	return ref.PipelineShape
}

func qEvalPipelineMirrorRefFromBackendPlan(ref *QEvalPipelinePlanRef, plan stdq.EvalPipelineBackendPlan) {
	if ref == nil || !plan.Valid() {
		return
	}
	qEvalPipelineMirrorRefFromDescriptor(ref, plan.Descriptor, plan.Backend)
}

func qEvalPipelineMirrorRefFromDescriptor(ref *QEvalPipelinePlanRef, descriptor stdq.EvalPipelineDescriptor, backend string) {
	if ref == nil {
		return
	}
	ref.Kernel = descriptor.Kernel
	ref.Shape = descriptor.Shape
	ref.PipelineShape = descriptor.PipelineShape
	ref.ShapeFamily = descriptor.ShapeFamily
	ref.ShapeReducer = descriptor.ShapeReducer
	ref.ShapeSelector = descriptor.ShapeSelector
	ref.ShapeTransform = descriptor.ShapeTransform
	ref.Source = descriptor.Source
	ref.Backend = backend
	ref.Kind = descriptor.Kind
	ref.Terminal = descriptor.Terminal
	ref.AssignmentText = encodeQEvalPipelineAssignments(descriptor.Assignments)
	ref.ValueExpr = descriptor.ValueExpr
	ref.ValueBinding = descriptor.ValueBinding
	ref.IndexExpr = descriptor.IndexExpr
	ref.IndexBinding = descriptor.IndexBinding
	ref.MaskExpr = descriptor.MaskExpr
	ref.MaskBinding = descriptor.MaskBinding
	ref.RowValueExpr = descriptor.RowValueExpr
	ref.RowIndexExpr = descriptor.RowIndexExpr
	ref.ColIndexExpr = descriptor.ColIndexExpr
	ref.CallableExpr = descriptor.CallableExpr
	ref.DyadicOp = descriptor.DyadicOp
	ref.ScalarExpr = descriptor.ScalarExpr
	ref.ScalarLeft = descriptor.ScalarLeft
	ref.IncludeCount = descriptor.IncludeCount
	ref.SequenceValueExpr = descriptor.SequenceValueExpr
	ref.SequenceTransformChain = descriptor.SequenceTransformChain
	ref.SequenceTransformNames = descriptor.SequenceTransformNames
	ref.LeftExpr = descriptor.LeftExpr
	ref.RightExpr = descriptor.RightExpr
	ref.CompareOp = descriptor.CompareOp
	ref.ComparePrefix = descriptor.ComparePrefix
	ref.ModExpr = descriptor.ModExpr
	ref.ModulusExpr = descriptor.ModulusExpr
	ref.ModTargetExpr = descriptor.ModTargetExpr
	ref.ReductionInput = descriptor.ReductionInput
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
