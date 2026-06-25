//go:build darwin && arm64

package methodjit

import (
	"fmt"

	"github.com/never-labs/leia/internal/runtime"
)

// executeQTypedRuntimeOpExit is the shared Go-side dispatcher for q typed
// runtime ops reached through op-exit style protocols. Slot and argument
// offsets in ctx are relative to base; regs uses absolute indexes.
func (cf *CompiledFunction) executeQTypedRuntimeOpExit(ctx *ExecContext, regs []runtime.Value, base int, route qTypedRuntimeExecutionRoute) (bool, error) {
	if cf == nil || ctx == nil {
		return false, nil
	}
	op := Op(ctx.OpExitOp)
	slot := base + int(ctx.OpExitSlot)
	arg1 := base + int(ctx.OpExitArg1)
	arg2 := base + int(ctx.OpExitArg2)
	aux := int(ctx.OpExitAux)
	adapter := cf.qFrameVectorRuntimeExecutionAdapter()
	constants := cf.protoConstants()

	constant := func(name, kind string) (runtime.Value, error) {
		if aux < 0 || aux >= len(constants) {
			return runtime.NilValue(), fmt.Errorf("%s %s constant is out of range", name, kind)
		}
		return constants[aux], nil
	}
	store := func(out runtime.Value, err error) (bool, error) {
		if err != nil {
			return true, err
		}
		regs[slot] = out
		return true, nil
	}
	unary := func(name string) (bool, error) {
		if arg1 < 0 || arg1 >= len(regs) || slot < 0 || slot >= len(regs) {
			return false, fmt.Errorf("%s op-exit out of register range", name)
		}
		return true, nil
	}
	binary := func(name string) (bool, error) {
		if arg1 < 0 || arg1 >= len(regs) || arg2 < 0 || arg2 >= len(regs) || slot < 0 || slot >= len(regs) {
			return false, fmt.Errorf("%s op-exit out of register range", name)
		}
		return true, nil
	}

	switch op {
	case OpFrameLen:
		if _, err := unary("FrameLen"); err != nil {
			return true, err
		}
		return store(adapter.executeFrameLen(regs[arg1], route))
	case OpFrameColumn:
		if _, err := unary("FrameColumn"); err != nil {
			return true, err
		}
		name, err := constant("FrameColumn", "column name")
		if err != nil {
			return true, err
		}
		return store(adapter.executeFrameColumn(regs[arg1], name, route))
	case OpFrameMask:
		if _, err := unary("FrameMask"); err != nil {
			return true, err
		}
		spec, err := constant("FrameMask", "spec")
		if err != nil {
			return true, err
		}
		return store(adapter.executeFrameMask(regs[arg1], spec, route))
	case OpFrameProject:
		if _, err := unary("FrameProject"); err != nil {
			return true, err
		}
		columns, err := constant("FrameProject", "column list")
		if err != nil {
			return true, err
		}
		return store(adapter.executeFrameProject(regs[arg1], columns, route))
	case OpFrameFilter:
		if _, err := binary("FrameFilter"); err != nil {
			return true, err
		}
		return store(adapter.executeFrameFilter(regs[arg1], regs[arg2], route))
	case OpFrameFilterProject:
		if _, err := binary("FrameFilterProject"); err != nil {
			return true, err
		}
		columns, err := constant("FrameFilterProject", "column list")
		if err != nil {
			return true, err
		}
		return store(adapter.executeFrameFilterProject(regs[arg1], regs[arg2], columns, route))
	case OpFrameGather:
		if _, err := binary("FrameGather"); err != nil {
			return true, err
		}
		return store(adapter.executeFrameGather(regs[arg1], regs[arg2], route))
	case OpFrameSlice:
		if _, err := binary("FrameSlice"); err != nil {
			return true, err
		}
		return store(adapter.executeFrameSlice(regs[arg1], regs[arg2], route))
	case OpFrameOrder:
		if _, err := unary("FrameOrder"); err != nil {
			return true, err
		}
		spec, err := constant("FrameOrder", "spec")
		if err != nil {
			return true, err
		}
		return store(adapter.executeFrameOrder(regs[arg1], spec, route))
	case OpFrameOrderGather:
		if _, err := unary("FrameOrderGather"); err != nil {
			return true, err
		}
		spec, err := constant("FrameOrderGather", "spec")
		if err != nil {
			return true, err
		}
		return store(adapter.executeFrameOrderGather(regs[arg1], spec, route))
	case OpFrameProjectColumn:
		if _, err := unary("FrameProjectColumn"); err != nil {
			return true, err
		}
		spec, err := constant("FrameProjectColumn", "spec")
		if err != nil {
			return true, err
		}
		return store(adapter.executeFrameProjectColumn(regs[arg1], spec, route))
	case OpFrameFilterProjectColumn:
		if _, err := binary("FrameFilterProjectColumn"); err != nil {
			return true, err
		}
		spec, err := constant("FrameFilterProjectColumn", "spec")
		if err != nil {
			return true, err
		}
		return store(adapter.executeFrameFilterProjectColumn(regs[arg1], regs[arg2], spec, route))
	case OpFrameGroupAggregate:
		if _, err := binary("FrameGroupAggregate"); err != nil {
			return true, err
		}
		spec, err := constant("FrameGroupAggregate", "spec")
		if err != nil {
			return true, err
		}
		return store(adapter.executeFrameGroupAggregate(regs[arg1], regs[arg2], spec, route))
	case OpQFrameSelectColumn:
		if _, err := unary("QFrameSelectColumn"); err != nil {
			return true, err
		}
		rhs := runtime.NilValue()
		hasRHS := false
		if ctx.OpExitArg2 >= 0 {
			if arg2 >= len(regs) {
				return true, fmt.Errorf("QFrameSelectColumn rhs out of register range")
			}
			rhs = regs[arg2]
			hasRHS = true
		}
		return store(adapter.executeQFrameSelectColumn(constants, aux, regs[arg1], rhs, hasRHS, route))
	case OpVectorGather:
		if _, err := binary("VectorGather"); err != nil {
			return true, err
		}
		return store(adapter.executeVectorGather(int(ctx.OpExitID), regs[arg1], regs[arg2], route))
	case OpVectorCompare:
		if _, err := binary("VectorCompare"); err != nil {
			return true, err
		}
		return store(adapter.executeVectorCompare(int(ctx.OpExitID), aux, regs[arg1], regs[arg2], route))
	case OpVectorMask:
		if _, err := binary("VectorMask"); err != nil {
			return true, err
		}
		return store(adapter.executeVectorMask(int(ctx.OpExitID), aux, regs[arg1], regs[arg2], route))
	case OpVectorWhere:
		tempBase := arg1
		nArgs := int(ctx.OpExitArg2)
		if slot >= len(regs) || tempBase < 0 || nArgs != 3 || tempBase+nArgs > len(regs) {
			return true, fmt.Errorf("VectorWhere op-exit out of register range")
		}
		return store(adapter.executeVectorWhere(int(ctx.OpExitID), regs[tempBase], regs[tempBase+1], regs[tempBase+2], route))
	case OpVectorReduce:
		if _, err := unary("VectorReduce"); err != nil {
			return true, err
		}
		return store(adapter.executeVectorReduce(int(ctx.OpExitID), aux, regs[arg1], route))
	case OpQVectorWhereReduce:
		tempBase := arg1
		nArgs := int(ctx.OpExitArg2)
		if slot >= len(regs) || tempBase < 0 || nArgs != 3 || tempBase+nArgs > len(regs) {
			return true, fmt.Errorf("QVectorWhereReduce op-exit out of register range")
		}
		return store(adapter.executeQVectorWhereReduce(int(ctx.OpExitID), aux, regs[tempBase], regs[tempBase+1], regs[tempBase+2], route))
	case OpQVectorGatherReduce:
		if _, err := binary("QVectorGatherReduce"); err != nil {
			return true, err
		}
		return store(adapter.executeQVectorGatherReduce(int(ctx.OpExitID), aux, regs[arg1], regs[arg2], route))
	case OpQEvalPipelinePlan:
		if slot < 0 || slot >= len(regs) {
			return true, fmt.Errorf("QEvalPipelinePlan op-exit out of register range")
		}
		pipelineRoute := qEvalPipelineExecutionRouteOpExit
		if route == qTypedRuntimeExecutionRouteNativeExit || route == qTypedRuntimeExecutionRouteDirectHelper {
			pipelineRoute = qEvalPipelineExecutionRouteNativeExit
		}
		return true, cf.executeQEvalPipelinePlanExit(ctx, regs, base, pipelineRoute)
	case OpQSQLKernelPlan:
		if err := cf.executeQSQLKernelPlanSlotWithRoute(aux, slot, regs, string(route)); err != nil {
			return true, err
		}
		return true, nil
	case OpQEvalSessionEval:
		if _, err := unary("QEvalSessionEval"); err != nil {
			return true, err
		}
		return store(cf.executeQEvalSessionEval(int(ctx.OpExitID), aux, regs[arg1]))
	case OpVectorScan:
		if _, err := unary("VectorScan"); err != nil {
			return true, err
		}
		return store(adapter.executeVectorScan(int(ctx.OpExitID), regs[arg1], route))
	default:
		return false, nil
	}
}
