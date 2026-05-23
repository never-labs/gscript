//go:build darwin && arm64

package methodjit

import "github.com/gscript/gscript/internal/vm"

type tieringRuntimeSpecializationDecision struct {
	reason         string
	specialization string
	route          string
	callee         *vm.FuncProto
}

func (tm *TieringManager) runtimeSpecializationTieringDecision(proto *vm.FuncProto) (tieringRuntimeSpecializationDecision, bool) {
	if info, ok := recognizedCallSiteRuntimeSpecializationForTiering(proto); ok {
		return tieringRuntimeSpecializationDecision{
			reason:         "whole_call_runtime_specialization",
			specialization: info.Name,
			route:          string(info.Route),
		}, true
	}
	if callee, info, ok := tm.callSiteRuntimeSpecializationCalleeForTiering(proto); ok {
		return tieringRuntimeSpecializationDecision{
			reason:         "whole_call_runtime_specialization_callee",
			specialization: info.Name,
			route:          string(info.Route),
			callee:         callee,
		}, true
	}
	if info, ok := tm.driverLoopRuntimeSpecializationForTiering(proto); ok {
		return tieringRuntimeSpecializationDecision{
			reason:         "driver_loop_runtime_specialization",
			specialization: info.Name,
			route:          string(info.Route),
		}, true
	}
	return tieringRuntimeSpecializationDecision{}, false
}

func (tm *TieringManager) disableForRuntimeSpecializationTiering(proto *vm.FuncProto, d tieringRuntimeSpecializationDecision) {
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
	if d.specialization != "" {
		fields["specialization"] = d.specialization
		tierFields["specialization"] = d.specialization
		fallbackFields["specialization"] = d.specialization
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

func recognizedCallSiteRuntimeSpecializationForTiering(proto *vm.FuncProto) (vm.RuntimeSpecializationInfo, bool) {
	for _, info := range vm.RecognizedCallSiteRuntimeSpecializations(proto) {
		if info.AllowsStructuralTiering(proto) {
			return info, true
		}
	}
	return vm.RuntimeSpecializationInfo{}, false
}

func (tm *TieringManager) callSiteRuntimeSpecializationCalleeForTiering(proto *vm.FuncProto) (*vm.FuncProto, vm.RuntimeSpecializationInfo, bool) {
	if tm == nil || tm.envTier2NoFilter || proto == nil {
		return nil, vm.RuntimeSpecializationInfo{}, false
	}
	globals := tm.buildLoopCallGlobals(proto)
	if len(globals) == 0 {
		return nil, vm.RuntimeSpecializationInfo{}, false
	}
	for pc, inst := range proto.Code {
		if vm.DecodeOp(inst) != vm.OP_CALL {
			continue
		}
		callee, ok := findGetGlobalCallee(proto, pc, vm.DecodeA(inst), globals)
		if !ok || callee == nil {
			continue
		}
		if info, ok := recognizedCallSiteRuntimeSpecializationForTiering(callee); ok {
			return callee, info, true
		}
	}
	return nil, vm.RuntimeSpecializationInfo{}, false
}

func (tm *TieringManager) driverLoopRuntimeSpecializationForTiering(proto *vm.FuncProto) (vm.RuntimeSpecializationInfo, bool) {
	if tm == nil || tm.envTier2NoFilter || proto == nil {
		return vm.RuntimeSpecializationInfo{}, false
	}
	globals := tm.buildLoopCallGlobals(proto)
	if len(globals) == 0 {
		return vm.RuntimeSpecializationInfo{}, false
	}
	for _, info := range vm.RecognizedDriverLoopRuntimeSpecializations(proto, globals) {
		if info.AllowsStructuralTiering(proto) {
			return info, true
		}
	}
	return vm.RuntimeSpecializationInfo{}, false
}
