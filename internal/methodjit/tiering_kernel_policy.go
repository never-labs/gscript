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
	if info, ok := recognizedWholeCallRuntimeSpecializationForTiering(proto); ok {
		return tieringKernelDecision{
			reason: "whole_call_structural_kernel",
			kernel: info.Name,
			route:  string(info.Route),
		}, true
	}
	if callee, info, ok := tm.wholeCallRuntimeSpecializationCalleeForTiering(proto); ok {
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

func recognizedWholeCallRuntimeSpecializationForTiering(proto *vm.FuncProto) (vm.KernelInfo, bool) {
	for _, info := range vm.RecognizedWholeCallRuntimeSpecializations(proto) {
		if info.AllowsStructuralTiering(proto) {
			return info, true
		}
	}
	return vm.KernelInfo{}, false
}

func (tm *TieringManager) wholeCallRuntimeSpecializationCalleeForTiering(proto *vm.FuncProto) (*vm.FuncProto, vm.KernelInfo, bool) {
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
		if info, ok := recognizedWholeCallRuntimeSpecializationForTiering(callee); ok {
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
		if info.AllowsStructuralTiering(proto) {
			return info, true
		}
	}
	return vm.KernelInfo{}, false
}
