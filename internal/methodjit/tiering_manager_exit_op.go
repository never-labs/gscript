//go:build darwin && arm64

// tiering_manager_exit_op.go holds the Tier 2 generic op-exit dispatcher and
// the per-op handlers it delegates to (modulo, closure creation, upvalue
// get/set, vararg copy).
//
// Pure code movement from tiering_manager_exit.go; no behavior change.

package methodjit

import (
	"fmt"
	"math"
	"unsafe"

	"github.com/never-labs/leia/internal/runtime"
	"github.com/never-labs/leia/internal/vm"
)

// executeOpExit handles generic op-exits in the TieringManager's Tier 2 path.
func (tm *TieringManager) executeOpExit(ctx *ExecContext, regs []runtime.Value, base int, proto *vm.FuncProto) error {
	op := Op(ctx.OpExitOp)
	absSlot := base + int(ctx.OpExitSlot)
	absArg1 := base + int(ctx.OpExitArg1)
	absArg2 := base + int(ctx.OpExitArg2)
	aux := int(ctx.OpExitAux)

	switch op {
	case OpCall:
		nArgs := int(ctx.OpExitArg1)
		nRets := int(ctx.OpExitArg2)
		if nRets != 0 {
			return fmt.Errorf("call op-exit only supports no-result call-site runtime specializations")
		}
		if tm.callVM == nil {
			return fmt.Errorf("no callVM set for call-site runtime specialization op-exit")
		}
		if absSlot < 0 || nArgs < 0 || absSlot+nArgs >= len(regs) {
			return fmt.Errorf("call-site runtime specialization op-exit out of register range")
		}
		fnVal := regs[absSlot]
		args := regs[absSlot+1 : absSlot+1+nArgs]
		handled, err := tm.callVM.TryRunNoResultCallSiteRuntimeSpecializationForJIT(fnVal, args)
		if err != nil {
			return err
		}
		if handled {
			return tm.executeCallSiteNoResultRuntimeSpecializationBatch(ctx, regs, base, proto)
		}
		if _, err = tm.callVM.CallValue(fnVal, args); err != nil {
			return err
		}
		return tm.executeCallSiteNoResultRuntimeSpecializationBatch(ctx, regs, base, proto)

	case OpConstString:
		if aux >= 0 && aux < len(proto.Constants) {
			if absSlot < len(regs) {
				regs[absSlot] = proto.Constants[aux]
			}
		}

	case OpMatrixDense:
		if absSlot >= len(regs) || absArg1 < 0 || absArg2 < 0 || absArg1 >= len(regs) || absArg2 >= len(regs) {
			return fmt.Errorf("matrix.dense op-exit out of register range")
		}
		rowsv := regs[absArg1]
		colsv := regs[absArg2]
		if !rowsv.IsInt() || !colsv.IsInt() {
			return fmt.Errorf("matrix.dense: rows and cols must be integers")
		}
		regs[absSlot] = runtime.TableValue(runtime.NewDenseMatrix(int(rowsv.Int()), int(colsv.Int())))

	case OpConcat:
		tempBase := absArg1
		nArgs := int(ctx.OpExitArg2)
		if absSlot < len(regs) && tempBase >= 0 && nArgs >= 0 && tempBase+nArgs <= len(regs) {
			if tm.callVM != nil {
				v, err := tm.callVM.ConcatValues(regs[tempBase : tempBase+nArgs])
				if err != nil {
					return err
				}
				regs[absSlot] = v
			} else {
				regs[absSlot] = runtime.ConcatValues(regs[tempBase : tempBase+nArgs])
			}
		}

	case OpStringFormatInt:
		tempBase := absArg1
		if absSlot >= len(regs) || tempBase < 0 || tempBase+3 > len(regs) {
			return fmt.Errorf("string.format int op-exit out of register range")
		}
		callee := regs[tempBase]
		patternVal := regs[tempBase+1]
		intVal := regs[tempBase+2]
		if runtime.IsStdStringFormatFunction(callee) && patternVal.IsString() && intVal.IsInt() {
			patternIdx := aux
			if cf, _ := tm.tier2CompiledFor(proto); cf != nil && patternIdx >= 0 && patternIdx < len(cf.StringFormatPatterns) {
				pattern := cf.StringFormatPatterns[patternIdx]
				if patternVal.Str() == pattern {
					v, ok, err := runtime.StringFormatSingleInt(pattern, intVal.Int())
					if err != nil {
						return err
					}
					if ok {
						regs[absSlot] = v
						return nil
					}
				}
			}
		}
		if tm.callVM == nil {
			return fmt.Errorf("no callVM set for string.format int fallback")
		}
		results, err := tm.callVM.CallValue(callee, []runtime.Value{patternVal, intVal})
		if err != nil {
			return err
		}
		if len(results) > 0 {
			regs[absSlot] = results[0]
		} else {
			regs[absSlot] = runtime.NilValue()
		}

	case OpStringSplitPart:
		tempBase := absArg1
		if absSlot >= len(regs) || tempBase < 0 || tempBase+3 > len(regs) {
			return fmt.Errorf("string.split projection op-exit out of register range")
		}
		callee := regs[tempBase]
		sv := regs[tempBase+1]
		sepv := regs[tempBase+2]
		if runtime.IsStdStringSplitFunction(callee) {
			v, err := runtime.StringSplitProject(sv, sepv, int64(aux))
			if err != nil {
				return err
			}
			regs[absSlot] = v
			return nil
		}
		if tm.callVM == nil {
			return fmt.Errorf("no callVM set for string.split projection fallback")
		}
		results, err := tm.callVM.CallValue(callee, []runtime.Value{sv, sepv})
		if err != nil {
			return err
		}
		if len(results) == 0 || !results[0].IsTable() {
			regs[absSlot] = runtime.NilValue()
			return nil
		}
		regs[absSlot] = results[0].Table().RawGetInt(int64(aux))

	case OpStringSplitSubstr:
		tempBase := absArg1
		nArgs := int(ctx.OpExitArg2)
		if absSlot >= len(regs) || tempBase < 0 || nArgs < 4 || tempBase+nArgs > len(regs) {
			return fmt.Errorf("string.split substring op-exit out of register range")
		}
		splitCallee := regs[tempBase]
		subCallees := regs[tempBase+1 : tempBase+nArgs-2]
		sv := regs[tempBase+nArgs-2]
		sepv := regs[tempBase+nArgs-1]
		cf, _ := tm.tier2CompiledFor(proto)
		if cf != nil && aux >= 0 && aux < len(cf.StringSplitSubSpecs) &&
			runtime.IsStdStringSplitFunction(splitCallee) &&
			allStdStringSubFunctions(subCallees) {
			spec := cf.StringSplitSubSpecs[aux]
			v, err := runtime.StringSplitProjectSub(sv, sepv, spec.TokenIndex, spec.Start, spec.End, spec.HasEnd)
			if err != nil {
				return err
			}
			regs[absSlot] = v
			return nil
		}
		if tm.callVM == nil || cf == nil {
			return fmt.Errorf("no callVM set for string.split substring fallback")
		}
		v, err := executeStringSplitSubstrFallback(tm.callVM, splitCallee, subCallees, sv, sepv, cf.StringSplitSubSpecs, aux)
		if err != nil {
			return err
		}
		regs[absSlot] = v

	case OpStringSplitSubstrNumber:
		tempBase := absArg1
		nArgs := int(ctx.OpExitArg2)
		if absSlot >= len(regs) || tempBase < 0 || nArgs < 5 || tempBase+nArgs > len(regs) {
			return fmt.Errorf("string.split substring number op-exit out of register range")
		}
		splitCallee := regs[tempBase]
		subCallees := regs[tempBase+1 : tempBase+nArgs-3]
		tonumberCallee := regs[tempBase+nArgs-3]
		sv := regs[tempBase+nArgs-2]
		sepv := regs[tempBase+nArgs-1]
		cf, _ := tm.tier2CompiledFor(proto)
		if cf != nil && aux >= 0 && aux < len(cf.StringSplitSubSpecs) &&
			runtime.IsStdStringSplitFunction(splitCallee) &&
			allStdStringSubFunctions(subCallees) &&
			runtime.IsStdToNumberFunction(tonumberCallee) {
			spec := cf.StringSplitSubSpecs[aux]
			v, err := runtime.StringSplitProjectSubToNumber(sv, sepv, spec.TokenIndex, spec.Start, spec.End, spec.HasEnd)
			if err != nil {
				return err
			}
			regs[absSlot] = v
			return nil
		}
		if tm.callVM == nil || cf == nil {
			return fmt.Errorf("no callVM set for string.split substring number fallback")
		}
		v, err := executeStringSplitSubstrNumberFallback(tm.callVM, splitCallee, subCallees, tonumberCallee, sv, sepv, cf.StringSplitSubSpecs, aux)
		if err != nil {
			return err
		}
		regs[absSlot] = v

	case OpGetTableStringFormatInt:
		tempBase := absArg1
		if absSlot >= len(regs) || tempBase < 0 || tempBase+4 > len(regs) {
			return fmt.Errorf("string.format table-get op-exit out of register range")
		}
		tblVal := regs[tempBase]
		callee := regs[tempBase+1]
		patternVal := regs[tempBase+2]
		intVal := regs[tempBase+3]
		var keyVal runtime.Value
		if runtime.IsStdStringFormatFunction(callee) && patternVal.IsString() && intVal.IsInt() {
			patternIdx := aux
			if cf, _ := tm.tier2CompiledFor(proto); cf != nil && patternIdx >= 0 && patternIdx < len(cf.StringFormatPatterns) {
				pattern := cf.StringFormatPatterns[patternIdx]
				if patternVal.Str() == pattern {
					if tblVal.IsTable() && !tblVal.Table().HasMetatable() {
						v, ok, err := runtime.RawGetStringFormatIntCached(tblVal.Table(), pattern, intVal.Int(), nil)
						if err != nil {
							return err
						}
						if ok {
							regs[absSlot] = v
							return nil
						}
					}
					v, ok, err := runtime.StringFormatSingleInt(pattern, intVal.Int())
					if err != nil {
						return err
					}
					if ok {
						keyVal = v
					}
				}
			}
		}
		if keyVal.IsNil() {
			if tm.callVM == nil {
				return fmt.Errorf("no callVM set for string.format table-get fallback")
			}
			results, err := tm.callVM.CallValue(callee, []runtime.Value{patternVal, intVal})
			if err != nil {
				return err
			}
			if len(results) > 0 {
				keyVal = results[0]
			} else {
				keyVal = runtime.NilValue()
			}
		}
		if tblVal.IsTable() && !tblVal.Table().HasMetatable() {
			if keyVal.IsString() {
				regs[absSlot] = tblVal.Table().RawGetStringDynamicCached(keyVal.Str(), nil)
			} else {
				regs[absSlot] = tblVal.Table().RawGet(keyVal)
			}
			return nil
		}
		if tm.callVM == nil {
			return fmt.Errorf("no callVM set for string.format table-get fallback")
		}
		result, err := tm.callVM.TableGetForJIT(tblVal, keyVal)
		if err != nil {
			return err
		}
		regs[absSlot] = result

	case OpStringFormatConst:
		tempBase := absArg1
		nArgs := int(ctx.OpExitArg2)
		if absSlot >= len(regs) || tempBase < 0 || nArgs < 3 || tempBase+nArgs > len(regs) {
			return fmt.Errorf("string.format const op-exit out of register range")
		}
		callee := regs[tempBase]
		patternVal := regs[tempBase+1]
		if runtime.IsStdStringFormatFunction(callee) && patternVal.IsString() {
			patternIdx := aux
			if cf, _ := tm.tier2CompiledFor(proto); cf != nil && patternIdx >= 0 && patternIdx < len(cf.StringFormatPatterns) &&
				patternVal.Str() == cf.StringFormatPatterns[patternIdx] {
				v, err := runtime.StringFormatValue(regs[tempBase+1 : tempBase+nArgs])
				if err != nil {
					return err
				}
				regs[absSlot] = v
				return nil
			}
		}
		if tm.callVM == nil {
			return fmt.Errorf("no callVM set for string.format const fallback")
		}
		results, err := tm.callVM.CallValue(callee, regs[tempBase+1:tempBase+nArgs])
		if err != nil {
			return err
		}
		if len(results) > 0 {
			regs[absSlot] = results[0]
		} else {
			regs[absSlot] = runtime.NilValue()
		}

	case OpLen:
		if absArg1 < len(regs) && absSlot < len(regs) {
			v := regs[absArg1]
			if v.IsTable() {
				regs[absSlot] = runtime.IntValue(int64(v.Table().Len()))
			} else if v.IsString() {
				regs[absSlot] = runtime.IntValue(int64(runtime.StringLen(v)))
			} else {
				regs[absSlot] = runtime.IntValue(0)
			}
		}

	case OpFrameLen:
		if absArg1 >= len(regs) || absSlot >= len(regs) {
			return fmt.Errorf("FrameLen op-exit out of register range")
		}
		out, err := executeFrameLenValue(regs[absArg1])
		if err != nil {
			tm.recordQRuntimePrimitiveExecution(proto, OpFrameLen, "error")
			return err
		}
		tm.recordQRuntimePrimitiveExecution(proto, OpFrameLen, "success")
		regs[absSlot] = out

	case OpFrameColumn:
		if absArg1 >= len(regs) || absSlot >= len(regs) {
			return fmt.Errorf("FrameColumn op-exit out of register range")
		}
		if aux < 0 || proto == nil || aux >= len(proto.Constants) || !proto.Constants[aux].IsString() {
			return fmt.Errorf("FrameColumn column name must be a string constant")
		}
		out, err := executeFrameColumnValue(regs[absArg1], proto.Constants[aux].Str())
		if err != nil {
			tm.recordQRuntimePrimitiveExecution(proto, OpFrameColumn, "error")
			return err
		}
		tm.recordQRuntimePrimitiveExecution(proto, OpFrameColumn, "success")
		regs[absSlot] = out

	case OpFrameMask:
		if absArg1 >= len(regs) || absSlot >= len(regs) {
			return fmt.Errorf("FrameMask op-exit out of register range")
		}
		if aux < 0 || proto == nil || aux >= len(proto.Constants) {
			return fmt.Errorf("FrameMask spec constant is out of range")
		}
		out, err := executeFrameMaskValue(regs[absArg1], proto.Constants[aux])
		if err != nil {
			tm.recordQRuntimePrimitiveExecution(proto, OpFrameMask, "error")
			return err
		}
		tm.recordQRuntimePrimitiveExecution(proto, OpFrameMask, "success")
		regs[absSlot] = out

	case OpFrameProject:
		if absArg1 >= len(regs) || absSlot >= len(regs) {
			return fmt.Errorf("FrameProject op-exit out of register range")
		}
		if aux < 0 || proto == nil || aux >= len(proto.Constants) {
			return fmt.Errorf("FrameProject column list constant is out of range")
		}
		names, err := frameProjectColumnNames(proto.Constants[aux])
		if err != nil {
			return err
		}
		out, err := executeFrameProjectValue(regs[absArg1], names)
		if err != nil {
			tm.recordQRuntimePrimitiveExecution(proto, OpFrameProject, "error")
			return err
		}
		tm.recordQRuntimePrimitiveExecution(proto, OpFrameProject, "success")
		regs[absSlot] = out

	case OpFrameFilter:
		if absArg1 >= len(regs) || absArg2 >= len(regs) || absSlot >= len(regs) {
			return fmt.Errorf("FrameFilter op-exit out of register range")
		}
		out, err := executeFrameFilterValue(regs[absArg1], regs[absArg2])
		if err != nil {
			tm.recordQRuntimePrimitiveExecution(proto, OpFrameFilter, "error")
			return err
		}
		tm.recordQRuntimePrimitiveExecution(proto, OpFrameFilter, "success")
		regs[absSlot] = out

	case OpFrameFilterProject:
		if absArg1 >= len(regs) || absArg2 >= len(regs) || absSlot >= len(regs) {
			return fmt.Errorf("FrameFilterProject op-exit out of register range")
		}
		if aux < 0 || proto == nil || aux >= len(proto.Constants) {
			return fmt.Errorf("FrameFilterProject column list constant is out of range")
		}
		names, err := frameProjectColumnNames(proto.Constants[aux])
		if err != nil {
			return err
		}
		out, err := executeFrameFilterProjectValue(regs[absArg1], regs[absArg2], names)
		if err != nil {
			tm.recordQRuntimePrimitiveExecution(proto, OpFrameFilterProject, "error")
			return err
		}
		tm.recordQRuntimePrimitiveExecution(proto, OpFrameFilterProject, "success")
		regs[absSlot] = out

	case OpFrameGather:
		if absArg1 >= len(regs) || absArg2 >= len(regs) || absSlot >= len(regs) {
			return fmt.Errorf("FrameGather op-exit out of register range")
		}
		out, err := executeFrameGatherValue(regs[absArg1], regs[absArg2])
		if err != nil {
			tm.recordQRuntimePrimitiveExecution(proto, OpFrameGather, "error")
			return err
		}
		tm.recordQRuntimePrimitiveExecution(proto, OpFrameGather, "success")
		regs[absSlot] = out

	case OpFrameSlice:
		if absArg1 >= len(regs) || absArg2 >= len(regs) || absSlot >= len(regs) {
			return fmt.Errorf("FrameSlice op-exit out of register range")
		}
		out, err := executeFrameSliceValue(regs[absArg1], regs[absArg2])
		if err != nil {
			tm.recordQRuntimePrimitiveExecution(proto, OpFrameSlice, "error")
			return err
		}
		tm.recordQRuntimePrimitiveExecution(proto, OpFrameSlice, "success")
		regs[absSlot] = out

	case OpFrameOrder:
		if absArg1 >= len(regs) || absSlot >= len(regs) {
			return fmt.Errorf("FrameOrder op-exit out of register range")
		}
		if aux < 0 || proto == nil || aux >= len(proto.Constants) {
			return fmt.Errorf("FrameOrder spec constant is out of range")
		}
		out, err := executeFrameOrderValue(regs[absArg1], proto.Constants[aux])
		if err != nil {
			tm.recordQRuntimePrimitiveExecution(proto, OpFrameOrder, "error")
			return err
		}
		tm.recordQRuntimePrimitiveExecution(proto, OpFrameOrder, "success")
		regs[absSlot] = out

	case OpFrameOrderGather:
		if absArg1 >= len(regs) || absSlot >= len(regs) {
			return fmt.Errorf("FrameOrderGather op-exit out of register range")
		}
		if aux < 0 || proto == nil || aux >= len(proto.Constants) {
			return fmt.Errorf("FrameOrderGather spec constant is out of range")
		}
		out, err := executeFrameOrderGatherValue(regs[absArg1], proto.Constants[aux])
		if err != nil {
			tm.recordQRuntimePrimitiveExecution(proto, OpFrameOrderGather, "error")
			return err
		}
		tm.recordQRuntimePrimitiveExecution(proto, OpFrameOrderGather, "success")
		regs[absSlot] = out

	case OpFrameProjectColumn:
		if absArg1 >= len(regs) || absSlot >= len(regs) {
			return fmt.Errorf("FrameProjectColumn op-exit out of register range")
		}
		if aux < 0 || proto == nil || aux >= len(proto.Constants) {
			return fmt.Errorf("FrameProjectColumn spec constant is out of range")
		}
		cf, _ := tm.tier2CompiledFor(proto)
		if cf == nil {
			return fmt.Errorf("FrameProjectColumn op-exit missing compiled function")
		}
		out, err := cf.qFrameVectorRuntimeExecutionAdapter().executeFrameProjectColumn(
			regs[absArg1], proto.Constants[aux], qTypedRuntimeExecutionRouteOpExit)
		if err != nil {
			return err
		}
		regs[absSlot] = out

	case OpFrameFilterProjectColumn:
		if absArg1 >= len(regs) || absArg2 >= len(regs) || absSlot >= len(regs) {
			return fmt.Errorf("FrameFilterProjectColumn op-exit out of register range")
		}
		if aux < 0 || proto == nil || aux >= len(proto.Constants) {
			return fmt.Errorf("FrameFilterProjectColumn spec constant is out of range")
		}
		cf, _ := tm.tier2CompiledFor(proto)
		if cf == nil {
			return fmt.Errorf("FrameFilterProjectColumn op-exit missing compiled function")
		}
		out, err := cf.qFrameVectorRuntimeExecutionAdapter().executeFrameFilterProjectColumn(
			regs[absArg1], regs[absArg2], proto.Constants[aux], qTypedRuntimeExecutionRouteOpExit)
		if err != nil {
			return err
		}
		regs[absSlot] = out

	case OpFrameGroupAggregate:
		if absArg1 >= len(regs) || absArg2 >= len(regs) || absSlot >= len(regs) {
			return fmt.Errorf("FrameGroupAggregate op-exit out of register range")
		}
		if aux < 0 || proto == nil || aux >= len(proto.Constants) {
			return fmt.Errorf("FrameGroupAggregate spec constant is out of range")
		}
		cf, _ := tm.tier2CompiledFor(proto)
		out, err := cf.qFrameVectorRuntimeExecutionAdapter().executeFrameGroupAggregate(
			regs[absArg1], regs[absArg2], proto.Constants[aux], qTypedRuntimeExecutionRouteOpExit)
		if err != nil {
			return err
		}
		regs[absSlot] = out

	case OpQFrameSelectColumn:
		if absArg1 >= len(regs) || absSlot >= len(regs) {
			return fmt.Errorf("QFrameSelectColumn op-exit out of register range")
		}
		if proto == nil {
			return fmt.Errorf("QFrameSelectColumn op-exit missing proto")
		}
		cf, _ := tm.tier2CompiledFor(proto)
		if cf == nil {
			return fmt.Errorf("QFrameSelectColumn op-exit missing compiled function")
		}
		rhs := runtime.NilValue()
		hasRHS := false
		if absArg2 >= 0 && absArg2 < len(regs) {
			rhs = regs[absArg2]
			hasRHS = true
		}
		out, err := cf.qFrameVectorRuntimeExecutionAdapter().executeQFrameSelectColumn(
			proto.Constants, int(aux), regs[absArg1], rhs, hasRHS, qTypedRuntimeExecutionRouteOpExit)
		if err != nil {
			return err
		}
		regs[absSlot] = out

	case OpVectorGather:
		if absArg1 >= len(regs) || absArg2 >= len(regs) || absSlot >= len(regs) {
			return fmt.Errorf("VectorGather op-exit out of register range")
		}
		out, err := executeVectorGatherValue(regs[absArg1], regs[absArg2])
		if err != nil {
			tm.recordQRuntimePrimitiveExecution(proto, OpVectorGather, "error")
			return err
		}
		tm.recordQRuntimePrimitiveExecution(proto, OpVectorGather, "success")
		regs[absSlot] = out

	case OpVectorCompare:
		if absArg1 >= len(regs) || absArg2 >= len(regs) || absSlot >= len(regs) {
			return fmt.Errorf("VectorCompare op-exit out of register range")
		}
		out, err := executeVectorCompareValue(aux, regs[absArg1], regs[absArg2])
		if err != nil {
			tm.recordQRuntimePrimitiveExecution(proto, OpVectorCompare, "error")
			return err
		}
		tm.recordQRuntimePrimitiveExecution(proto, OpVectorCompare, "success")
		regs[absSlot] = out

	case OpVectorMask:
		if absArg1 >= len(regs) || absArg2 >= len(regs) || absSlot >= len(regs) {
			return fmt.Errorf("VectorMask op-exit out of register range")
		}
		out, err := executeVectorMaskValue(aux, regs[absArg1], regs[absArg2])
		if err != nil {
			tm.recordQRuntimePrimitiveExecution(proto, OpVectorMask, "error")
			return err
		}
		tm.recordQRuntimePrimitiveExecution(proto, OpVectorMask, "success")
		regs[absSlot] = out

	case OpVectorWhere:
		tempBase := absArg1
		nArgs := int(ctx.OpExitArg2)
		if absSlot >= len(regs) || tempBase < 0 || nArgs != 3 || tempBase+nArgs > len(regs) {
			return fmt.Errorf("VectorWhere op-exit out of register range")
		}
		out, err := executeVectorWhereValue(regs[tempBase], regs[tempBase+1], regs[tempBase+2])
		if err != nil {
			tm.recordQRuntimePrimitiveExecution(proto, OpVectorWhere, "error")
			return err
		}
		tm.recordQRuntimePrimitiveExecution(proto, OpVectorWhere, "success")
		regs[absSlot] = out

	case OpVectorReduce:
		if absArg1 >= len(regs) || absSlot >= len(regs) {
			return fmt.Errorf("VectorReduce op-exit out of register range")
		}
		out, err := executeVectorReduceValue(aux, regs[absArg1])
		if err != nil {
			tm.recordQRuntimePrimitiveExecution(proto, OpVectorReduce, "error")
			return err
		}
		tm.recordQRuntimePrimitiveExecution(proto, OpVectorReduce, "success")
		regs[absSlot] = out

	case OpQVectorWhereReduce:
		tempBase := absArg1
		nArgs := int(ctx.OpExitArg2)
		if absSlot >= len(regs) || tempBase < 0 || nArgs != 3 || tempBase+nArgs > len(regs) {
			return fmt.Errorf("QVectorWhereReduce op-exit out of register range")
		}
		cf, _ := tm.tier2CompiledFor(proto)
		out, err := cf.qFrameVectorRuntimeExecutionAdapter().executeQVectorWhereReduce(
			int(ctx.OpExitID), aux, regs[tempBase], regs[tempBase+1], regs[tempBase+2], qTypedRuntimeExecutionRouteOpExit)
		if err != nil {
			return err
		}
		regs[absSlot] = out

	case OpQVectorGatherReduce:
		if absArg1 >= len(regs) || absArg2 >= len(regs) || absSlot >= len(regs) {
			return fmt.Errorf("QVectorGatherReduce op-exit out of register range")
		}
		cf, _ := tm.tier2CompiledFor(proto)
		out, err := cf.qFrameVectorRuntimeExecutionAdapter().executeQVectorGatherReduce(
			int(ctx.OpExitID), aux, regs[absArg1], regs[absArg2], qTypedRuntimeExecutionRouteOpExit)
		if err != nil {
			return err
		}
		regs[absSlot] = out

	case OpQEvalPipelinePlan:
		if absSlot >= len(regs) {
			return fmt.Errorf("QEvalPipelinePlan op-exit out of register range")
		}
		cf, _ := tm.tier2CompiledFor(proto)
		if cf == nil {
			return fmt.Errorf("QEvalPipelinePlan op-exit missing compiled function")
		}
		ctx.OpExitSlot = int64(absSlot - base)
		ctx.OpExitAux = int64(aux)
		return cf.executeQEvalPipelinePlanExit(ctx, regs, base, qEvalPipelineExecutionRouteOpExit)

	case OpQSQLKernelPlan:
		cf, _ := tm.tier2CompiledFor(proto)
		if cf == nil {
			return fmt.Errorf("QSQLKernelPlan op-exit missing compiled function")
		}
		return cf.executeQSQLKernelPlanSlot(aux, absSlot, regs)

	case OpQEvalSessionEval:
		if absArg1 >= len(regs) || absSlot >= len(regs) {
			return fmt.Errorf("QEvalSessionEval op-exit out of register range")
		}
		cf, _ := tm.tier2CompiledFor(proto)
		var out runtime.Value
		var err error
		if cf != nil {
			out, err = cf.executeQEvalSessionEval(int(ctx.OpExitID), aux, regs[absArg1])
		} else {
			var constants []runtime.Value
			if proto != nil {
				constants = proto.Constants
			}
			out, err = executeQEvalSessionEvalValue(constants, aux, regs[absArg1])
		}
		if err != nil {
			return err
		}
		regs[absSlot] = out

	case OpVectorScan:
		if absArg1 >= len(regs) || absSlot >= len(regs) {
			return fmt.Errorf("VectorScan op-exit out of register range")
		}
		out, err := executeVectorScanValue(regs[absArg1])
		if err != nil {
			tm.recordQRuntimePrimitiveExecution(proto, OpVectorScan, "error")
			return err
		}
		tm.recordQRuntimePrimitiveExecution(proto, OpVectorScan, "success")
		regs[absSlot] = out

	case OpEq:
		if absArg1 < len(regs) && absArg2 < len(regs) && absSlot < len(regs) {
			regs[absSlot] = runtime.BoolValue(regs[absArg1].Equal(regs[absArg2]))
		}

	case OpLt:
		if absArg1 < len(regs) && absArg2 < len(regs) && absSlot < len(regs) {
			lt, ok := regs[absArg1].LessThan(regs[absArg2])
			if !ok {
				return fmt.Errorf("attempt to compare %s with %s", regs[absArg1].TypeName(), regs[absArg2].TypeName())
			}
			regs[absSlot] = runtime.BoolValue(lt)
		}

	case OpLe:
		if absArg1 < len(regs) && absArg2 < len(regs) && absSlot < len(regs) {
			lt, ok := regs[absArg2].LessThan(regs[absArg1])
			if !ok {
				return fmt.Errorf("attempt to compare %s with %s", regs[absArg1].TypeName(), regs[absArg2].TypeName())
			}
			regs[absSlot] = runtime.BoolValue(!lt)
		}

	case OpMod:
		if absArg1 < len(regs) && absArg2 < len(regs) && absSlot < len(regs) {
			result, err := tier2OpExitMod(regs[absArg1], regs[absArg2])
			if err != nil {
				return err
			}
			regs[absSlot] = result
		}

	case OpPow:
		if absArg1 < len(regs) && absArg2 < len(regs) && absSlot < len(regs) {
			var base2, exp float64
			v1 := regs[absArg1]
			v2 := regs[absArg2]
			if v1.IsInt() {
				base2 = float64(v1.Int())
			} else {
				base2 = v1.Float()
			}
			if v2.IsInt() {
				exp = float64(v2.Int())
			} else {
				exp = v2.Float()
			}
			regs[absSlot] = runtime.FloatValue(math.Pow(base2, exp))
		}

	case OpSetGlobal:
		if tm.callVM == nil {
			return fmt.Errorf("no callVM set for SetGlobal op-exit")
		}
		if aux >= 0 && aux < len(proto.Constants) {
			name := proto.Constants[aux].Str()
			if absArg1 < len(regs) {
				tm.callVM.SetGlobal(name, regs[absArg1])
			}
			tm.invalidateGlobalValueCaches(name)
		}

	case OpAppend:
		if absArg1 < len(regs) && absArg2 < len(regs) {
			tblVal := regs[absArg1]
			val := regs[absArg2]
			if tblVal.IsTable() {
				tblVal.Table().Append(val)
			}
		}

	case OpSelf:
		if absArg1 < len(regs) && absSlot < len(regs) && absSlot+1 < len(regs) {
			tblVal := regs[absArg1]
			regs[absSlot+1] = tblVal
			if tblVal.IsTable() && aux >= 0 && aux < len(proto.Constants) {
				methodName := proto.Constants[aux].Str()
				regs[absSlot] = tblVal.Table().RawGetString(methodName)
			} else {
				regs[absSlot] = runtime.NilValue()
			}
		}

	case OpClose:
		// No-op.

	case OpSetList:
		// SetList: slot=nValues, arg1=table slot, arg2=tempBase slot, aux=arrayStart
		nValues := int(ctx.OpExitSlot)
		absTable := base + int(ctx.OpExitArg1)
		absTempBase := base + int(ctx.OpExitArg2)
		arrayStart := aux // 1-based array start index
		if absTable < len(regs) && regs[absTable].IsTable() {
			tbl := regs[absTable].Table()
			for i := 0; i < nValues; i++ {
				absVal := absTempBase + i
				if absVal < len(regs) {
					tbl.RawSetInt(int64(arrayStart+i), regs[absVal])
				}
			}
		}

	case OpClosure:
		return tm.executeClosureOpExit(ctx, regs, base, proto)

	case OpGetUpval:
		return tm.executeGetUpvalOpExit(ctx, regs, base)

	case OpSetUpval:
		return tm.executeSetUpvalOpExit(ctx, regs, base)

	case OpVararg:
		return tm.executeVarargOpExit(ctx, regs, base)

	case OpResume:
		if tm.callVM == nil {
			return fmt.Errorf("no callVM set for coroutine resume op-exit")
		}
		packed := uint64(ctx.OpExitArg1)
		nArgs := int(uint32(packed>>32)) - 1
		c := int(uint32(packed))
		if nArgs < 0 {
			return fmt.Errorf("coroutine resume op-exit has invalid B")
		}
		payloadFieldOnly := tm.callVM.ResumePayloadIsFieldOnly(proto, int(ctx.BaselinePC), int(ctx.OpExitSlot), c)
		return tm.callVM.ResumeCoroutineFromSlots(absSlot, nArgs, c, payloadFieldOnly)

	case OpTestSet:
		return fmt.Errorf("op-exit not yet implemented: %s", op)

	case OpForPrep, OpForLoop:
		return fmt.Errorf("op-exit unexpected: %s (should be decomposed by graph builder)", op)

	case OpTForCall, OpTForLoop:
		return fmt.Errorf("op-exit not yet implemented: %s", op)

	case OpGuardType, OpGuardIntRange, OpGuardConstString, OpGuardTableKind, OpGuardCalleeProto, OpGuardShapeFieldType, OpGuardShapeFieldTypeMask, OpGuardShapeFieldVMClosure, OpGuardNonNil, OpGuardTruthy:
		return fmt.Errorf("op-exit guard failure: %s", op)

	case OpGo, OpMakeChan, OpSend, OpRecv:
		return fmt.Errorf("op-exit not yet implemented: %s", op)

	default:
		return fmt.Errorf("unsupported op-exit: %s (%d)", op, int(op))
	}

	return nil
}

func tier2OpExitMod(a, b runtime.Value) (runtime.Value, error) {
	if a.IsInt() && b.IsInt() {
		bi := b.Int()
		if bi == 0 {
			return runtime.NilValue(), fmt.Errorf("attempt to perform 'n%%0'")
		}
		r := a.Int() % bi
		if r != 0 && (r^bi) < 0 {
			r += bi
		}
		return runtime.IntValue(r), nil
	}
	if a.IsNumber() && b.IsNumber() {
		bf := b.Number()
		if bf == 0 {
			return runtime.NilValue(), fmt.Errorf("attempt to perform 'n%%0'")
		}
		r := math.Mod(a.Number(), bf)
		if r != 0 && (r < 0) != (bf < 0) {
			r += bf
		}
		return runtime.FloatValue(r), nil
	}
	return runtime.NilValue(), fmt.Errorf("attempt to perform arithmetic on %s and %s", a.TypeName(), b.TypeName())
}

// executeClosureOpExit handles OpClosure via op-exit. Creates a new closure
// with the child proto and captures upvalues from the parent closure and the
// register file, mirroring Tier 1's handleClosure in tier1_handlers_misc.go.
//
// Op-exit descriptor:
//
//	OpExitSlot = result slot (where to store the new closure)
//	OpExitAux  = child proto index (bx from OP_CLOSURE)
func (tm *TieringManager) executeClosureOpExit(ctx *ExecContext, regs []runtime.Value, base int, proto *vm.FuncProto) error {
	absSlot := base + int(ctx.OpExitSlot)
	bx := int(ctx.OpExitAux)

	if bx < 0 || bx >= len(proto.Protos) {
		return fmt.Errorf("closure proto index %d out of range (len %d)", bx, len(proto.Protos))
	}
	subProto := proto.Protos[bx]

	cl := vm.NewClosure(subProto)

	switch len(subProto.Upvalues) {
	case 0:
	case 1:
		desc := subProto.Upvalues[0]
		if desc.InStack {
			absIdx := base + desc.Index
			if absIdx < len(regs) {
				uv := vm.NewOpenUpvalue(&regs[absIdx], absIdx)
				if tm.callVM != nil {
					uv = tm.callVM.FindOrCreateUpvalue(absIdx)
				}
				cl.Upvalues[0] = uv
			}
		} else {
			var parentCl *vm.Closure
			if tm.callVM != nil {
				parentCl = tm.callVM.CurrentClosure()
			}
			if parentCl != nil && desc.Index < len(parentCl.Upvalues) && parentCl.Upvalues[desc.Index] != nil {
				cl.Upvalues[0] = parentCl.Upvalues[desc.Index]
			} else {
				cl.Upvalues[0] = vm.NewOpenUpvalue(new(runtime.Value), 0)
			}
		}
	default:
		// Get the parent closure for non-InStack upvalues.
		var parentCl *vm.Closure
		if tm.callVM != nil {
			parentCl = tm.callVM.CurrentClosure()
		}

		for i, desc := range subProto.Upvalues {
			if desc.InStack {
				// Upvalue refers to a local in the current frame's register file.
				absIdx := base + desc.Index
				if absIdx < len(regs) {
					uv := vm.NewOpenUpvalue(&regs[absIdx], absIdx)
					if tm.callVM != nil {
						uv = tm.callVM.FindOrCreateUpvalue(absIdx)
					}
					cl.Upvalues[i] = uv
				}
			} else {
				// Upvalue refers to a parent closure's upvalue.
				if parentCl != nil && desc.Index < len(parentCl.Upvalues) && parentCl.Upvalues[desc.Index] != nil {
					cl.Upvalues[i] = parentCl.Upvalues[desc.Index]
				} else {
					cl.Upvalues[i] = vm.NewOpenUpvalue(new(runtime.Value), 0)
				}
			}
		}
	}

	if absSlot < len(regs) {
		regs[absSlot] = runtime.VMClosureFastValue(unsafe.Pointer(cl))
	}
	return nil
}

// executeGetUpvalOpExit handles OpGetUpval via op-exit. Reads a captured
// upvalue from the current closure.
//
// Op-exit descriptor:
//
//	OpExitSlot = result slot
//	OpExitAux  = upvalue index
func (tm *TieringManager) executeGetUpvalOpExit(ctx *ExecContext, regs []runtime.Value, base int) error {
	if tm.callVM == nil {
		return fmt.Errorf("no callVM for GetUpval op-exit")
	}
	cl := tm.callVM.CurrentClosure()
	if cl == nil {
		return fmt.Errorf("GetUpval: no current closure")
	}

	absSlot := base + int(ctx.OpExitSlot)
	uvIdx := int(ctx.OpExitAux)

	if uvIdx < 0 || uvIdx >= len(cl.Upvalues) || cl.Upvalues[uvIdx] == nil {
		return fmt.Errorf("GetUpval: upvalue %d out of range (len %d)", uvIdx, len(cl.Upvalues))
	}

	if absSlot < len(regs) {
		regs[absSlot] = cl.Upvalues[uvIdx].Get()
	}
	return nil
}

// executeSetUpvalOpExit handles OpSetUpval via op-exit. Writes a value to a
// captured upvalue in the current closure.
//
// Op-exit descriptor:
//
//	OpExitArg1 = source slot (the value to set)
//	OpExitAux  = upvalue index
func (tm *TieringManager) executeSetUpvalOpExit(ctx *ExecContext, regs []runtime.Value, base int) error {
	if tm.callVM == nil {
		return fmt.Errorf("no callVM for SetUpval op-exit")
	}
	cl := tm.callVM.CurrentClosure()
	if cl == nil {
		return fmt.Errorf("SetUpval: no current closure")
	}

	absArg1 := base + int(ctx.OpExitArg1)
	uvIdx := int(ctx.OpExitAux)

	if uvIdx < 0 || uvIdx >= len(cl.Upvalues) || cl.Upvalues[uvIdx] == nil {
		return fmt.Errorf("SetUpval: upvalue %d out of range (len %d)", uvIdx, len(cl.Upvalues))
	}

	if absArg1 < len(regs) {
		cl.Upvalues[uvIdx].Set(regs[absArg1])
	}
	return nil
}

// executeVarargOpExit handles OpVararg via op-exit. Copies variable arguments
// from the VM frame into the register file.
//
// Op-exit descriptor:
//
//	OpExitAux  = dest register (a from OP_VARARG)
//	OpExitSlot = result slot (used for storing first vararg result to SSA home)
//
// The actual varargs come from the VM frame. Aux2 encoding: Aux = a (dest base),
// the count is derived from the graph builder's Aux2 (stored in OpExitArg1 as
// a secondary channel since op-exit only has Aux for one aux field).
func (tm *TieringManager) executeVarargOpExit(ctx *ExecContext, regs []runtime.Value, base int) error {
	if tm.callVM == nil {
		return fmt.Errorf("no callVM for Vararg op-exit")
	}

	destReg := int(ctx.OpExitAux)     // destination register (a)
	resultSlot := int(ctx.OpExitSlot) // SSA result slot
	bCount := int(ctx.OpExitArg1)     // B field (0 = all, >=2 means B-1 results)

	va := tm.callVM.CurrentVarargs()

	if bCount == 0 {
		// B=0: copy all varargs.
		for i, v := range va {
			absIdx := base + destReg + i
			if absIdx < len(regs) {
				regs[absIdx] = v
			}
		}
	} else {
		// B>=2: copy exactly B-1 varargs.
		n := bCount - 1
		for i := 0; i < n; i++ {
			absIdx := base + destReg + i
			if absIdx < len(regs) {
				if i < len(va) {
					regs[absIdx] = va[i]
				} else {
					regs[absIdx] = runtime.NilValue()
				}
			}
		}
	}

	// Also write the first vararg to the SSA result slot so the JIT can
	// pick it up after resuming.
	absResult := base + resultSlot
	if absResult < len(regs) {
		if len(va) > 0 {
			regs[absResult] = va[0]
		} else {
			regs[absResult] = runtime.NilValue()
		}
	}

	return nil
}
