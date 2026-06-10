package methodjit

import (
	"fmt"
	"sync/atomic"

	"github.com/never-labs/leia/internal/runtime"
	"github.com/never-labs/leia/internal/stdlib/lib/data"
	stdq "github.com/never-labs/leia/internal/stdlib/lib/q"
)

// QEvalPipelineBackend is the stable handoff point between MethodJIT q.eval
// hot-path recognition and the q typed pipeline backend. The current slice only
// records a backend-facing plan reference; later lowering can route an
// OpQEvalVectorPlan or native op-exit through this interface without teaching
// MethodJIT q evaluator internals.
type QEvalPipelineBackend interface {
	BackendName() string
	LookupQEvalPipelinePlan(ref QEvalPipelinePlanRef) (QEvalPipelinePlan, bool)
}

// QEvalPipelineExecutor is the execution-side extension for backends that can
// run a plan ref through the q typed runtime without re-entering generic q.eval.
type QEvalPipelineExecutor interface {
	ExecuteQEvalPipelinePlan(ref QEvalPipelinePlanRef) (any, bool, error)
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
	ID                     int
	Kernel                 string
	Shape                  string
	PipelineShape          string
	ShapeFamily            string
	ShapeReducer           string
	ShapeSelector          string
	ShapeTransform         string
	Source                 string
	Backend                string
	BackendPlan            *stdq.EvalPipelineBackendPlan
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

func (r QEvalPipelinePlanRef) Valid() bool {
	descriptor, ok := qEvalPipelineDescriptorViewFromRef(r)
	return r.ID >= 0 && ok && descriptor.Kernel != "" && descriptor.Shape != "" && qEvalPipelineBackendNameFromRef(r) != ""
}

type qEvalPipelineStaticPlan struct {
	ref QEvalPipelinePlanRef
}

type qEvalPipelinePlanExecutionCounters struct {
	nativeSuccess atomic.Uint64
	nativeError   atomic.Uint64
	opSuccess     atomic.Uint64
	opError       atomic.Uint64
	directSuccess atomic.Uint64
	directError   atomic.Uint64
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
	kernel := qEvalPipelinePlanRefKernel(ref)
	shape := qEvalPipelinePlanRefShape(ref)
	for _, plan := range b.plans {
		if plan.ref.ID == ref.ID && qEvalPipelinePlanRefKernel(plan.ref) == kernel && qEvalPipelinePlanRefShape(plan.ref) == shape {
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
		if qEvalPipelinePlanRefSource(ref) == source &&
			qEvalPipelinePlanRefKernel(ref) == plan.Kernel &&
			qEvalPipelinePlanRefShape(ref) == plan.Shape &&
			qEvalPipelineBackendNameFromRef(ref) == plan.Backend {
			return ref
		}
	}
	ref := qEvalPipelinePlanRefFromHotPlan(len(fn.QEvalPipelinePlans), source, plan)
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
		out += fmt.Sprintf("  [%d] id=%d kernel=%s shape=%s pipeline_shape=%s backend=%s\n",
			i, ref.ID, qEvalPipelinePlanRefKernel(ref), qEvalPipelinePlanRefShape(ref), qEvalPipelinePlanRefPipelineShape(ref), qEvalPipelineBackendNameFromRef(ref))
	}
	return out
}

func (cf *CompiledFunction) ExecuteQEvalPipelinePlanValue(id int) (runtime.Value, bool, error) {
	if cf == nil {
		return runtime.NilValue(), false, nil
	}
	if helper := cf.qEvalPipelinePlanHelper(id); helper != nil {
		return helper.execute()
	}
	ref, ok := qEvalPipelinePlanRefByID(cf.QEvalPipelinePlans, id)
	if !ok {
		return runtime.NilValue(), false, nil
	}
	backend := cf.QEvalPipelineBackend
	if !backend.hasPlans() {
		backend = newQRuntimeEvalPipelineBackend(cf.QEvalPipelinePlans)
	}
	return executeQEvalPipelinePlanValue(backend, ref)
}

func (cf *CompiledFunction) tryExecuteQEvalPipelineDirectReturn(args []runtime.Value) ([]runtime.Value, bool, error) {
	if cf == nil || len(args) != 0 || !cf.QEvalPipelineDirectReturn || cf.QEvalPipelineDirectReturnID < 0 || exitResumeCheckEnabled() {
		return nil, false, nil
	}
	out, handled, err := cf.ExecuteQEvalPipelinePlanValue(cf.QEvalPipelineDirectReturnID)
	if err != nil || !handled {
		cf.recordQEvalPipelinePlanExecutionWithRoute(cf.QEvalPipelineDirectReturnID, "typed_runtime_direct_entry", "error")
		return nil, true, err
	}
	cf.recordQEvalPipelinePlanExecutionWithRoute(cf.QEvalPipelineDirectReturnID, "typed_runtime_direct_entry", "success")
	return []runtime.Value{out}, true, nil
}

func (cf *CompiledFunction) executeQEvalPipelinePlanExit(ctx *ExecContext, regs []runtime.Value, base int, route string) error {
	if cf == nil || ctx == nil {
		return fmt.Errorf("QEvalPipelinePlan exit missing compiled function")
	}
	absSlot := base + int(ctx.OpExitSlot)
	return cf.executeQEvalPipelinePlanSlot(int(ctx.OpExitAux), absSlot, regs, route)
}

func executeQEvalPipelinePlanValue(backend QEvalPipelineBackend, ref QEvalPipelinePlanRef) (runtime.Value, bool, error) {
	if backend == nil || !ref.Valid() {
		return runtime.NilValue(), false, nil
	}
	if _, ok := backend.LookupQEvalPipelinePlan(ref); !ok {
		return runtime.NilValue(), false, nil
	}
	executor, ok := backend.(QEvalPipelineExecutor)
	if !ok {
		return runtime.NilValue(), false, nil
	}
	out, handled, err := executor.ExecuteQEvalPipelinePlan(ref)
	if err != nil || !handled {
		return runtime.NilValue(), handled, err
	}
	value, err := qEvalPipelineRuntimeValue(out)
	if err != nil {
		return runtime.NilValue(), false, err
	}
	return value, true, nil
}

func (cf *CompiledFunction) qEvalPipelinePlanHelper(id int) *qEvalPipelinePlanHelper {
	if cf == nil || id < 0 || id >= len(cf.QEvalPipelinePlanHelpers) {
		return nil
	}
	helper := &cf.QEvalPipelinePlanHelpers[id]
	if !helper.validForID(id) {
		return nil
	}
	return helper
}

func qEvalPipelinePlanExecutionShape(refs []QEvalPipelinePlanRef, id int) string {
	if ref, ok := qEvalPipelinePlanRefByID(refs, id); ok {
		if shape := qEvalPipelinePlanRefShape(ref); shape != "" {
			return shape
		}
	}
	return "q-eval/pipeline-plan"
}

func qEvalPipelineResumeOffsetTable(fn *Function, offsets map[int]int) []int {
	if fn == nil || len(fn.QEvalPipelinePlans) == 0 || len(offsets) == 0 {
		return nil
	}
	maxID := -1
	for _, block := range fn.Blocks {
		if block == nil {
			continue
		}
		for _, instr := range block.Instrs {
			if instr != nil && instr.Op == OpQEvalPipelinePlan && instr.ID > maxID {
				maxID = instr.ID
			}
		}
	}
	if maxID < 0 {
		return nil
	}
	table := make([]int, maxID+1)
	for i := range table {
		table[i] = -1
	}
	for _, block := range fn.Blocks {
		if block == nil {
			continue
		}
		for _, instr := range block.Instrs {
			if instr == nil || instr.Op != OpQEvalPipelinePlan {
				continue
			}
			if off, ok := offsets[instr.ID]; ok {
				table[instr.ID] = off
			}
		}
	}
	return table
}

func qEvalPipelineTerminalReturnTable(fn *Function) []bool {
	if fn == nil || len(fn.QEvalPipelinePlans) == 0 {
		return nil
	}
	maxID := -1
	for _, block := range fn.Blocks {
		if block == nil {
			continue
		}
		for _, instr := range block.Instrs {
			if instr != nil && instr.Op == OpQEvalPipelinePlan && instr.ID > maxID {
				maxID = instr.ID
			}
		}
	}
	if maxID < 0 {
		return nil
	}
	table := make([]bool, maxID+1)
	for _, block := range fn.Blocks {
		if block == nil {
			continue
		}
		for i, instr := range block.Instrs {
			if instr == nil || instr.Op != OpQEvalPipelinePlan || instr.ID < 0 || instr.ID >= len(table) {
				continue
			}
			if i+1 != len(block.Instrs)-1 {
				continue
			}
			ret := block.Instrs[i+1]
			if ret == nil || ret.Op != OpReturn || len(ret.Args) != 1 || ret.Args[0] == nil {
				continue
			}
			table[instr.ID] = ret.Args[0].ID == instr.ID
		}
	}
	return table
}

func qEvalPipelineDirectReturnPlanID(fn *Function) int {
	if fn == nil || len(fn.QEvalPipelinePlans) == 0 || fn.Entry == nil || len(fn.Entry.Instrs) != 2 {
		return -1
	}
	plan := fn.Entry.Instrs[0]
	ret := fn.Entry.Instrs[1]
	if plan == nil || ret == nil || plan.Op != OpQEvalPipelinePlan || ret.Op != OpReturn || len(ret.Args) != 1 || ret.Args[0] == nil {
		return -1
	}
	if ret.Args[0].ID != plan.ID {
		return -1
	}
	id := int(plan.Aux)
	if _, ok := qEvalPipelinePlanRefByID(fn.QEvalPipelinePlans, id); !ok {
		return -1
	}
	return id
}

func newQEvalPipelinePlanExecutionCounters(refs []QEvalPipelinePlanRef) []qEvalPipelinePlanExecutionCounters {
	if len(refs) == 0 {
		return nil
	}
	return make([]qEvalPipelinePlanExecutionCounters, len(refs))
}

func (cf *CompiledFunction) qEvalPipelineResumeOffset(instrID int, numericPass bool) (int, bool) {
	if cf == nil || instrID < 0 {
		return 0, false
	}
	var table []int
	if numericPass {
		table = cf.QEvalPipelineNumericResumeOffsets
	} else {
		table = cf.QEvalPipelineResumeOffsets
	}
	if instrID < len(table) && table[instrID] >= 0 {
		return table[instrID], true
	}
	return cf.resumeOffset(instrID, numericPass)
}

func (cf *CompiledFunction) qEvalPipelineTerminalReturn(instrID int) bool {
	if cf == nil || instrID < 0 || instrID >= len(cf.QEvalPipelineTerminalReturns) {
		return false
	}
	return cf.QEvalPipelineTerminalReturns[instrID]
}

func (cf *CompiledFunction) executeQEvalPipelinePlanSlot(planID, absSlot int, regs []runtime.Value, route string) error {
	if absSlot < 0 || absSlot >= len(regs) {
		return fmt.Errorf("QEvalPipelinePlan exit out of register range")
	}
	if helper := cf.qEvalPipelinePlanHelper(planID); helper != nil {
		out, handled, err := helper.execute()
		if err != nil || !handled {
			cf.recordQEvalPipelinePlanExecutionWithRoute(planID, route, "error")
			if err != nil {
				return err
			}
			return fmt.Errorf("QEvalPipelinePlan exit plan %d was not handled", planID)
		}
		cf.recordQEvalPipelinePlanExecutionWithRoute(planID, route, "success")
		regs[absSlot] = out
		return nil
	}
	out, handled, err := cf.ExecuteQEvalPipelinePlanValue(planID)
	if err != nil || !handled {
		cf.recordQEvalPipelinePlanExecutionWithRoute(planID, route, "error")
		if err != nil {
			return err
		}
		return fmt.Errorf("QEvalPipelinePlan exit plan %d was not handled", planID)
	}
	cf.recordQEvalPipelinePlanExecutionWithRoute(planID, route, "success")
	regs[absSlot] = out
	return nil
}

func qEvalPipelineRuntimeValue(v any) (runtime.Value, error) {
	switch x := v.(type) {
	case nil:
		return runtime.NilValue(), nil
	case bool:
		return runtime.BoolValue(x), nil
	case int:
		return runtime.IntValue(int64(x)), nil
	case int8:
		return runtime.IntValue(int64(x)), nil
	case int16:
		return runtime.IntValue(int64(x)), nil
	case int32:
		return runtime.IntValue(int64(x)), nil
	case int64:
		return runtime.IntValue(x), nil
	case uint:
		return runtime.IntValue(int64(x)), nil
	case uint8:
		return runtime.IntValue(int64(x)), nil
	case uint16:
		return runtime.IntValue(int64(x)), nil
	case uint32:
		return runtime.IntValue(int64(x)), nil
	case uint64:
		return runtime.IntValue(int64(x)), nil
	case float32:
		return runtime.FloatValue(float64(x)), nil
	case float64:
		return runtime.FloatValue(x), nil
	case string:
		return runtime.StringValue(x), nil
	case data.Array:
		return qEvalPipelineArrayRuntimeValue(x)
	default:
		return runtime.NilValue(), fmt.Errorf("methodjit: q eval pipeline result type %T is not runtime-value supported", v)
	}
}

func qEvalPipelineArrayRuntimeValue(array data.Array) (runtime.Value, error) {
	switch array.Kind() {
	case data.KindBool:
		out := make([]bool, array.Len())
		for i := range out {
			v, ok := array.At(i)
			if !ok {
				return runtime.NilValue(), fmt.Errorf("methodjit: q eval bool array row %d out of range", i)
			}
			b, ok := v.(bool)
			if !ok {
				return runtime.NilValue(), fmt.Errorf("methodjit: q eval bool array row %d has %T", i, v)
			}
			out[i] = b
		}
		return runtime.DenseArrayValue(runtime.NewDenseArrayBool(out)), nil
	case data.KindI64:
		out := make([]int64, array.Len())
		for i := range out {
			v, ok := array.At(i)
			if !ok {
				return runtime.NilValue(), fmt.Errorf("methodjit: q eval i64 array row %d out of range", i)
			}
			n, ok := v.(int64)
			if !ok {
				return runtime.NilValue(), fmt.Errorf("methodjit: q eval i64 array row %d has %T", i, v)
			}
			out[i] = n
		}
		return runtime.DenseArrayValue(runtime.NewDenseArrayI64(out)), nil
	case data.KindF64:
		out := make([]float64, array.Len())
		for i := range out {
			v, ok := array.At(i)
			if !ok {
				return runtime.NilValue(), fmt.Errorf("methodjit: q eval f64 array row %d out of range", i)
			}
			switch n := v.(type) {
			case float64:
				out[i] = n
			case int64:
				out[i] = float64(n)
			default:
				return runtime.NilValue(), fmt.Errorf("methodjit: q eval f64 array row %d has %T", i, v)
			}
		}
		return runtime.DenseArrayValue(runtime.NewDenseArrayF64(out)), nil
	case data.KindString, data.KindSymbol:
		out := make([]string, array.Len())
		for i := range out {
			v, ok := array.At(i)
			if !ok {
				return runtime.NilValue(), fmt.Errorf("methodjit: q eval string array row %d out of range", i)
			}
			switch s := v.(type) {
			case string:
				out[i] = s
			case data.Symbol:
				out[i] = string(s)
			default:
				return runtime.NilValue(), fmt.Errorf("methodjit: q eval string array row %d has %T", i, v)
			}
		}
		return runtime.DenseArrayValue(runtime.NewDenseArrayString(out)), nil
	default:
		out := runtime.NewAppendArrayTable(array.Len())
		for i := 0; i < array.Len(); i++ {
			v, ok := array.At(i)
			if !ok {
				return runtime.NilValue(), fmt.Errorf("methodjit: q eval generic array row %d out of range", i)
			}
			item, err := qEvalPipelineRuntimeValue(v)
			if err != nil {
				return runtime.NilValue(), err
			}
			out.RawSetInt(int64(i+1), item)
		}
		return runtime.TableValue(out), nil
	}
}
