//go:build darwin && arm64

package methodjit

import "github.com/gscript/gscript/internal/vm"

type tieringKernelDecision struct {
	reason string
	kernel string
	route  string
	callee *vm.FuncProto
}

func (tm *TieringManager) structuralKernelTieringDecision(proto *vm.FuncProto) (tieringKernelDecision, bool) {
	if info, ok := recognizedWholeCallKernelForTiering(proto); ok {
		return tieringKernelDecision{
			reason: "whole_call_structural_kernel",
			kernel: info.Name,
			route:  string(info.Route),
		}, true
	}
	if callee, info, ok := tm.wholeCallKernelCalleeForTiering(proto); ok {
		return tieringKernelDecision{
			reason: "whole_call_kernel_callee",
			kernel: info.Name,
			route:  string(info.Route),
			callee: callee,
		}, true
	}
	if info, ok := tm.driverLoopKernelForTiering(proto); ok {
		return tieringKernelDecision{
			reason: "driver_loop_structural_kernel",
			kernel: info.Name,
			route:  string(info.Route),
		}, true
	}
	return tieringKernelDecision{}, false
}

func (tm *TieringManager) disableForStructuralKernelTiering(proto *vm.FuncProto, d tieringKernelDecision) {
	tm.markJITDisabled(proto)
	fields := map[string]any{
		"reason":     d.reason,
		"call_count": proto.CallCount,
	}
	tierFields := map[string]any{"reason": d.reason}
	fallbackFields := map[string]any{
		"reason": d.reason,
		"target": "interpreter",
	}
	if d.kernel != "" {
		fields["kernel"] = d.kernel
		tierFields["kernel"] = d.kernel
		fallbackFields["kernel"] = d.kernel
	}
	if d.route != "" {
		fields["route"] = d.route
		tierFields["route"] = d.route
		fallbackFields["route"] = d.route
	}
	if d.callee != nil {
		calleeName := "<anonymous>"
		if d.callee.Name != "" {
			calleeName = d.callee.Name
		}
		fields["callee"] = calleeName
		tierFields["callee"] = calleeName
		fallbackFields["callee"] = calleeName
	}
	tm.traceEvent("runtime_disable", "jit", proto, fields)
	tm.traceEvent("tier1_skip", "tier1", proto, tierFields)
	tm.traceEvent("fallback", "tier0", proto, fallbackFields)
}

func recognizedWholeCallKernelForTiering(proto *vm.FuncProto) (vm.KernelInfo, bool) {
	if vm.IsRawIntNestedKernelProto(proto) {
		return vm.KernelInfo{
			Name:    "nested_int_recurrence",
			Route:   vm.KernelRouteWholeCallValue,
			Arity:   2,
			Results: 1,
		}, true
	}
	for _, info := range vm.RecognizedWholeCallKernels(proto) {
		if info.Route == vm.KernelRouteWholeCallValue &&
			(info.Name == "record_walk_fold" ||
				info.Name == "int_grid_aggregate" ||
				info.Name == "permutation_flip_checksum" ||
				info.Name == "bool_table_strike_count" ||
				info.Name == "matrix_multiply" ||
				info.Name == "lazy_recursive_table_builder" ||
				info.Name == "lazy_recursive_table_fold") {
			return info, true
		}
		if info.Route == vm.KernelRouteWholeCallNoResult && protoHasFloatConstant(proto) {
			return info, true
		}
	}
	return vm.KernelInfo{}, false
}

func protoHasFloatConstant(proto *vm.FuncProto) bool {
	if proto == nil {
		return false
	}
	for _, c := range proto.Constants {
		if c.IsFloat() {
			return true
		}
	}
	return false
}

func (tm *TieringManager) wholeCallKernelCalleeForTiering(proto *vm.FuncProto) (*vm.FuncProto, vm.KernelInfo, bool) {
	if tm == nil || tm.envTier2NoFilter || proto == nil {
		return nil, vm.KernelInfo{}, false
	}
	globals := tm.buildLoopCallGlobals(proto)
	if len(globals) == 0 {
		return nil, vm.KernelInfo{}, false
	}
	for pc, inst := range proto.Code {
		if vm.DecodeOp(inst) != vm.OP_CALL {
			continue
		}
		callee, ok := findGetGlobalCallee(proto, pc, vm.DecodeA(inst), globals)
		if !ok || callee == nil {
			continue
		}
		if info, ok := recognizedWholeCallKernelForTiering(callee); ok {
			return callee, info, true
		}
	}
	return nil, vm.KernelInfo{}, false
}

func (tm *TieringManager) driverLoopKernelForTiering(proto *vm.FuncProto) (vm.KernelInfo, bool) {
	if tm == nil || tm.envTier2NoFilter || proto == nil {
		return vm.KernelInfo{}, false
	}
	globals := tm.buildLoopCallGlobals(proto)
	if len(globals) == 0 {
		return vm.KernelInfo{}, false
	}
	for _, info := range vm.RecognizedDriverLoopKernels(proto, globals) {
		return info, true
	}
	return vm.KernelInfo{}, false
}
