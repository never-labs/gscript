package methodjit

import (
	"fmt"

	"github.com/never-labs/leia/internal/runtime"
	"github.com/never-labs/leia/internal/vm"
)

func allStdStringSubFunctions(vals []runtime.Value) bool {
	if len(vals) == 0 {
		return false
	}
	for _, v := range vals {
		if !runtime.IsStdStringSubFunction(v) {
			return false
		}
	}
	return true
}

func executeStringSplitSubstrOpExit(regs []runtime.Value, slot, tempBase, nArgs, specIdx int, specs []StringSplitSubSpec, callVM *vm.VM, missingVMError string) error {
	if slot < 0 || slot >= len(regs) || tempBase < 0 || nArgs < 4 || tempBase+nArgs > len(regs) {
		return fmt.Errorf("string.split substring op-exit out of register range")
	}
	splitCallee := regs[tempBase]
	subCallees := regs[tempBase+1 : tempBase+nArgs-2]
	sv := regs[tempBase+nArgs-2]
	sepv := regs[tempBase+nArgs-1]
	if specIdx >= 0 && specIdx < len(specs) && runtime.IsStdStringSplitFunction(splitCallee) && allStdStringSubFunctions(subCallees) {
		spec := specs[specIdx]
		v, err := runtime.StringSplitProjectSub(sv, sepv, spec.TokenIndex, spec.Start, spec.End, spec.HasEnd)
		if err != nil {
			return err
		}
		regs[slot] = v
		return nil
	}
	if callVM == nil {
		return fmt.Errorf("%s", missingVMError)
	}
	v, err := executeStringSplitSubstrFallback(callVM, splitCallee, subCallees, sv, sepv, specs, specIdx)
	if err != nil {
		return err
	}
	regs[slot] = v
	return nil
}

func executeStringSplitSubstrNumberOpExit(regs []runtime.Value, slot, tempBase, nArgs, specIdx int, specs []StringSplitSubSpec, callVM *vm.VM, missingVMError string) error {
	if slot < 0 || slot >= len(regs) || tempBase < 0 || nArgs < 5 || tempBase+nArgs > len(regs) {
		return fmt.Errorf("string.split substring number op-exit out of register range")
	}
	splitCallee := regs[tempBase]
	subCallees := regs[tempBase+1 : tempBase+nArgs-3]
	tonumberCallee := regs[tempBase+nArgs-3]
	sv := regs[tempBase+nArgs-2]
	sepv := regs[tempBase+nArgs-1]
	if specIdx >= 0 && specIdx < len(specs) &&
		runtime.IsStdStringSplitFunction(splitCallee) &&
		allStdStringSubFunctions(subCallees) &&
		runtime.IsStdToNumberFunction(tonumberCallee) {
		spec := specs[specIdx]
		v, err := runtime.StringSplitProjectSubToNumber(sv, sepv, spec.TokenIndex, spec.Start, spec.End, spec.HasEnd)
		if err != nil {
			return err
		}
		regs[slot] = v
		return nil
	}
	if callVM == nil {
		return fmt.Errorf("%s", missingVMError)
	}
	v, err := executeStringSplitSubstrNumberFallback(callVM, splitCallee, subCallees, tonumberCallee, sv, sepv, specs, specIdx)
	if err != nil {
		return err
	}
	regs[slot] = v
	return nil
}

func executeStringSplitSubstrFallback(callVM *vm.VM, splitCallee runtime.Value, subCallees []runtime.Value, sv, sepv runtime.Value, specs []StringSplitSubSpec, specIdx int) (runtime.Value, error) {
	if specIdx < 0 || specIdx >= len(specs) {
		return runtime.NilValue(), fmt.Errorf("string.split substring spec out of range")
	}
	spec := specs[specIdx]
	if spec.SubCallCount < 1 || spec.SubCallCount > 2 || len(subCallees) < spec.SubCallCount {
		return runtime.NilValue(), fmt.Errorf("string.split substring fallback has invalid sub call count")
	}
	splitResults, err := callVM.CallValue(splitCallee, []runtime.Value{sv, sepv})
	if err != nil {
		return runtime.NilValue(), err
	}
	current := runtime.NilValue()
	if len(splitResults) > 0 && splitResults[0].IsTable() {
		current = splitResults[0].Table().RawGetInt(spec.TokenIndex)
	}
	ranges := []struct {
		start  int64
		end    int64
		hasEnd bool
	}{
		{start: spec.FirstStart, end: spec.FirstEnd, hasEnd: spec.FirstHasEnd},
		{start: spec.SecondStart, end: spec.SecondEnd, hasEnd: spec.SecondHasEnd},
	}
	for i := 0; i < spec.SubCallCount; i++ {
		args := []runtime.Value{current, runtime.IntValue(ranges[i].start)}
		if ranges[i].hasEnd {
			args = append(args, runtime.IntValue(ranges[i].end))
		}
		subResults, err := callVM.CallValue(subCallees[i], args)
		if err != nil {
			return runtime.NilValue(), err
		}
		if len(subResults) == 0 {
			current = runtime.NilValue()
		} else {
			current = subResults[0]
		}
	}
	return current, nil
}

func executeStringSplitSubstrNumberFallback(callVM *vm.VM, splitCallee runtime.Value, subCallees []runtime.Value, tonumberCallee, sv, sepv runtime.Value, specs []StringSplitSubSpec, specIdx int) (runtime.Value, error) {
	subValue, err := executeStringSplitSubstrFallback(callVM, splitCallee, subCallees, sv, sepv, specs, specIdx)
	if err != nil {
		return runtime.NilValue(), err
	}
	numberResults, err := callVM.CallValue(tonumberCallee, []runtime.Value{subValue})
	if err != nil {
		return runtime.NilValue(), err
	}
	if len(numberResults) == 0 {
		return runtime.NilValue(), nil
	}
	return numberResults[0], nil
}
