package methodjit

import (
	"reflect"

	"github.com/gscript/gscript/internal/runtime"
)

type GlobalConstSpecializationConfig struct {
	Values             map[int]runtime.Value
	Globals            CompilationGlobalLookup
	DependencyRegistry *CompilationDependencyRegistry
}

func GlobalConstSpecializationPass(values map[int]runtime.Value) PassFunc {
	return func(fn *Function) (*Function, error) {
		return globalConstSpecializationPass(fn, values)
	}
}

func globalConstSpecializationPass(fn *Function, values map[int]runtime.Value) (*Function, error) {
	return GlobalConstSpecializationPassWith(GlobalConstSpecializationConfig{Values: values})(fn)
}

func GlobalConstSpecializationPassWith(config GlobalConstSpecializationConfig) PassFunc {
	return func(fn *Function) (*Function, error) {
		return globalConstSpecializationPassWith(fn, config)
	}
}

func globalConstSpecializationPassWith(fn *Function, config GlobalConstSpecializationConfig) (*Function, error) {
	values := config.Values
	if fn == nil || len(values) == 0 {
		return fn, nil
	}
	for _, block := range fn.Blocks {
		changed := false
		for _, instr := range block.Instrs {
			if instr == nil || instr.Op != OpGetGlobal {
				continue
			}
			v, ok := values[int(instr.Aux)]
			if !ok || (!v.IsInt() && !v.IsFloat()) {
				continue
			}
			changed = true
			break
		}
		if !changed {
			continue
		}
		newInstrs := make([]*Instr, 0, len(block.Instrs)*2)
		for _, instr := range block.Instrs {
			if instr == nil || instr.Op != OpGetGlobal {
				newInstrs = append(newInstrs, instr)
				continue
			}
			v, ok := values[int(instr.Aux)]
			if !ok || (!v.IsInt() && !v.IsFloat()) {
				newInstrs = append(newInstrs, instr)
				continue
			}
			guard := emitIRInstr(fn, block, OpGuardGlobalConst, TypeUnknown, nil, instr.Aux, int64(uint64(v)))
			guard.copySourceFrom(instr)
			newInstrs = append(newInstrs, guard)
			recordGlobalConstDependency(fn, int(instr.Aux), config)
			if v.IsInt() {
				instr.Op = OpConstInt
				instr.Type = TypeInt
				instr.Aux = v.Int()
			} else {
				instr.Op = OpConstFloat
				instr.Type = TypeFloat
				instr.Aux = int64(uint64(v))
			}
			instr.Args = nil
			instr.Aux2 = 0
			functionRemarks(fn).Add("GlobalConstSpecialization", "changed", block.ID, instr.ID, OpGetGlobal,
				"guarded numeric global as constant")
			newInstrs = append(newInstrs, instr)
		}
		block.Instrs = newInstrs
	}
	return fn, nil
}

func recordGlobalConstDependency(fn *Function, constIdx int, config GlobalConstSpecializationConfig) {
	if config.DependencyRegistry == nil || compilationGlobalLookupNil(config.Globals) || fn == nil || fn.Proto == nil ||
		constIdx < 0 || constIdx >= len(fn.Proto.Constants) {
		return
	}
	c := fn.Proto.Constants[constIdx]
	if !c.IsString() || c.Str() == "" {
		return
	}
	config.DependencyRegistry.RecordGlobalValue(config.Globals, c.Str())
}

func compilationGlobalLookupNil(globals CompilationGlobalLookup) bool {
	if globals == nil {
		return true
	}
	v := reflect.ValueOf(globals)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}

func globalConstFunctionSafe(fn *Function) bool {
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr == nil {
				continue
			}
			spec, ok := instr.Op.Spec()
			if ok && spec.GlobalConstUnsafe {
				return false
			}
		}
	}
	return true
}
