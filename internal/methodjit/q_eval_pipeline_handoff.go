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
	descriptor := p.plan.Descriptor
	return QEvalPipelineDescriptor{
		Kernel:                 descriptor.Kernel,
		Shape:                  descriptor.Shape,
		PipelineShape:          descriptor.PipelineShape,
		ShapeFamily:            descriptor.ShapeFamily,
		ShapeReducer:           descriptor.ShapeReducer,
		ShapeSelector:          descriptor.ShapeSelector,
		ShapeTransform:         descriptor.ShapeTransform,
		Source:                 descriptor.Source,
		Backend:                p.plan.Backend,
		Detail:                 p.plan.Detail,
		Kind:                   descriptor.Kind,
		Terminal:               descriptor.Terminal,
		AssignmentText:         p.assignmentText,
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

func qEvalRuntimePipelinePlan(source string) (qEvalHotPlan, bool) {
	descriptor, ok := qRuntimeEvalPipelinePlanner{}.DescribeQEvalPipeline(source)
	if !ok {
		return qEvalHotPlan{}, false
	}
	return qEvalHotPlan{
		Kernel:                 descriptor.Kernel,
		Shape:                  descriptor.Shape,
		PipelineShape:          descriptor.PipelineShape,
		ShapeFamily:            descriptor.ShapeFamily,
		ShapeReducer:           descriptor.ShapeReducer,
		ShapeSelector:          descriptor.ShapeSelector,
		ShapeTransform:         descriptor.ShapeTransform,
		Backend:                descriptor.Backend,
		Detail:                 descriptor.Detail,
		Kind:                   descriptor.Kind,
		Terminal:               descriptor.Terminal,
		AssignmentText:         descriptor.AssignmentText,
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
	}, true
}

type qRuntimeEvalPipelineBackend struct {
	static             qEvalPipelineStaticBackend
	descriptorByID     map[int]stdq.EvalPipelineDescriptor
	backendPlanByID    map[int]stdq.EvalPipelineBackendPlan
	executablePlanByID map[int]stdq.EvalPipelineExecutablePlan
	executeExecutable  func(stdq.EvalPipelineExecutablePlan) (any, bool, error)
	executeBackendPlan func(stdq.EvalPipelineBackendPlan) (any, bool, error)
	executeDescriptor  func(stdq.EvalPipelineDescriptor) (any, bool, error)
	executeSource      func(string) (any, bool, error)
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
	executeBackendPlan func(stdq.EvalPipelineBackendPlan) (any, bool, error)
	executeDescriptor  func(stdq.EvalPipelineDescriptor) (any, bool, error)
	executeSource      func(string) (any, bool, error)
}

func newQRuntimeEvalPipelineBackend(refs []QEvalPipelinePlanRef) qRuntimeEvalPipelineBackend {
	descriptorByID := make(map[int]stdq.EvalPipelineDescriptor, len(refs))
	backendPlanByID := make(map[int]stdq.EvalPipelineBackendPlan, len(refs))
	executablePlanByID := make(map[int]stdq.EvalPipelineExecutablePlan, len(refs))
	for _, ref := range refs {
		if descriptor, ok := qEvalPipelineDescriptorFromRef(ref); ok {
			descriptorByID[ref.ID] = descriptor
			plan := stdq.EvalPipelineBackendPlan{
				Backend:    qEvalPipelineTypedRuntimeBackend,
				Detail:     "kind=" + descriptor.Kind,
				Descriptor: descriptor,
			}
			backendPlanByID[ref.ID] = plan
			if executable, ok := stdq.CompileEvalPipelineBackendPlan(plan); ok {
				executablePlanByID[ref.ID] = executable
			}
		}
	}
	return qRuntimeEvalPipelineBackend{
		static:             newQEvalPipelineStaticBackend(qEvalPipelineTypedRuntimeBackend, refs),
		descriptorByID:     descriptorByID,
		backendPlanByID:    backendPlanByID,
		executablePlanByID: executablePlanByID,
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
		if !ref.Valid() || ref.ID < 0 || ref.ID >= len(helpers) || ref.Backend != qEvalPipelineTypedRuntimeBackend {
			continue
		}
		if _, ok := backend.LookupQEvalPipelinePlan(ref); !ok {
			continue
		}
		backendPlan, hasBackendPlan := backend.lookupBackendPlan(ref)
		descriptor, hasDescriptor := backend.lookupDescriptor(ref)
		executablePlan, hasExecutablePlan := stdq.CompileEvalPipelineBackendPlan(backendPlan)
		helpers[ref.ID] = qEvalPipelinePlanHelper{
			ref:                ref,
			descriptor:         descriptor,
			backendPlan:        backendPlan,
			executablePlan:     executablePlan,
			hasDescriptor:      hasDescriptor,
			hasBackendPlan:     hasBackendPlan,
			hasExecutablePlan:  hasExecutablePlan,
			evalState:          qEvalPipelineReusableEvalState(ref, backend, hasExecutablePlan),
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
		// The q package has already validated and compiled the descriptor into
		// an opaque executable plan, so a source round-trip is unnecessary.
	} else if ref.Source == "" || !stdq.EvalSourceCacheable(ref.Source) {
		return nil
	} else {
		plan, ok := qRuntimeEvalPipelinePlanner{}.DescribeQEvalPipelineBackendPlan(ref.Source)
		if !ok || !plan.valid() || plan.plan.Kind() != ref.Kind || plan.plan.Shape() != ref.Shape || plan.plan.PipelineShape() != ref.PipelineShape {
			return nil
		}
	}
	if !hasExecutablePlan && ref.Source == "" {
		return nil
	}
	if backend.executeBackendPlan != nil && !sameQEvalPipelineBackendPlanExecutor(backend.executeBackendPlan, stdq.ExecuteEvalPipelineBackendPlan) {
		return nil
	}
	if backend.executeDescriptor != nil && !sameQEvalPipelineDescriptorExecutor(backend.executeDescriptor, stdq.ExecuteEvalPipelineDescriptor) {
		return nil
	}
	if backend.executeSource != nil && !sameQEvalPipelineSourceExecutor(backend.executeSource, stdq.ExecuteEvalPipeline) {
		return nil
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
	return h != nil && h.ref.ID == id && h.ref.Valid() && h.ref.Backend == qEvalPipelineTypedRuntimeBackend
}

func (h *qEvalPipelinePlanHelper) execute() (runtime.Value, bool, error) {
	if h == nil {
		return runtime.NilValue(), false, nil
	}
	if !h.ref.Valid() || h.ref.Backend != qEvalPipelineTypedRuntimeBackend {
		return runtime.NilValue(), false, nil
	}
	if h.evalState != nil {
		h.evalStateMu.Lock()
		var (
			out     any
			handled bool
			err     error
		)
		if h.hasExecutablePlan {
			out, handled, err = h.evalState.ExecuteEvalPipelineExecutablePlan(h.executablePlan)
		} else if h.hasBackendPlan {
			out, handled, err = h.evalState.ExecuteEvalPipelineBackendPlan(h.backendPlan)
		} else if h.hasDescriptor {
			out, handled, err = h.evalState.ExecuteEvalPipelineDescriptor(h.descriptor)
		} else {
			out, err = h.evalState.Eval(h.ref.Source)
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
	if h.hasBackendPlan {
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
	if executable, ok := b.lookupExecutablePlan(ref); ok && b.canUseExecutablePlan() {
		return b.executeEvalPipelineExecutablePlan(executable)
	}
	if backendPlan, ok := b.lookupBackendPlan(ref); ok {
		return b.executeEvalPipelineBackendPlan(backendPlan)
	}
	if descriptor, ok := b.lookupDescriptor(ref); ok {
		return b.executeEvalPipelineDescriptor(descriptor)
	}
	return b.executeEvalPipelineSource(ref.Source)
}

func (b qRuntimeEvalPipelineBackend) lookupExecutablePlan(ref QEvalPipelinePlanRef) (stdq.EvalPipelineExecutablePlan, bool) {
	if b.executablePlanByID == nil {
		return stdq.EvalPipelineExecutablePlan{}, false
	}
	plan, ok := b.executablePlanByID[ref.ID]
	return plan, ok && plan.Valid()
}

func (b qRuntimeEvalPipelineBackend) canUseExecutablePlan() bool {
	if b.executeExecutable != nil {
		return true
	}
	if b.executeBackendPlan != nil && !sameQEvalPipelineBackendPlanExecutor(b.executeBackendPlan, stdq.ExecuteEvalPipelineBackendPlan) {
		return false
	}
	if b.executeDescriptor != nil && !sameQEvalPipelineDescriptorExecutor(b.executeDescriptor, stdq.ExecuteEvalPipelineDescriptor) {
		return false
	}
	if b.executeSource != nil && !sameQEvalPipelineSourceExecutor(b.executeSource, stdq.ExecuteEvalPipeline) {
		return false
	}
	return true
}

func (b qRuntimeEvalPipelineBackend) lookupBackendPlan(ref QEvalPipelinePlanRef) (stdq.EvalPipelineBackendPlan, bool) {
	if b.backendPlanByID != nil {
		if plan, ok := b.backendPlanByID[ref.ID]; ok && plan.Valid() {
			return plan, true
		}
	}
	descriptor, ok := qEvalPipelineDescriptorFromRef(ref)
	if !ok {
		return stdq.EvalPipelineBackendPlan{}, false
	}
	plan := stdq.EvalPipelineBackendPlan{
		Backend:    qEvalPipelineTypedRuntimeBackend,
		Detail:     "kind=" + descriptor.Kind,
		Descriptor: descriptor,
	}
	return plan, plan.Valid()
}

func (b qRuntimeEvalPipelineBackend) lookupDescriptor(ref QEvalPipelinePlanRef) (stdq.EvalPipelineDescriptor, bool) {
	if b.descriptorByID != nil {
		if descriptor, ok := b.descriptorByID[ref.ID]; ok {
			return descriptor, true
		}
	}
	return qEvalPipelineDescriptorFromRef(ref)
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
	return stdq.NewEvalState(nil).ExecuteEvalPipelineExecutablePlan(plan)
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
