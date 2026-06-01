//go:build darwin && arm64

package methodjit

import (
	"github.com/never-labs/leia/internal/runtime"
	"github.com/never-labs/leia/internal/vm"
)

func GuardedConstCallFoldPass(globals map[string]*vm.FuncProto) PassFunc {
	return func(fn *Function) (*Function, error) {
		allowed := allowedDomainsForModule(
			analysisFacts(AnalysisFactCallABIs),
			analysisFacts(AnalysisFactGuardedConstCallFolds),
			nil,
		)
		return GuardedConstCallFoldPassCtx(globals)(newPassContext(fn, nil, allowed, false))
	}
}

func GuardedConstCallFoldPassCtx(globals map[string]*vm.FuncProto) CtxPassFunc {
	return func(ctx *PassContext) (*Function, error) {
		return annotateGuardedConstCallFolds(ctx.Func(), globals, ctx.Call()), nil
	}
}

func AnnotateGuardedConstCallFolds(fn *Function, globals map[string]*vm.FuncProto) *Function {
	if fn != nil {
		fn.ensureAnalysis()
	}
	allowed := allowedDomainsForModule(
		analysisFacts(AnalysisFactCallABIs),
		analysisFacts(AnalysisFactGuardedConstCallFolds),
		nil,
	)
	out, _ := GuardedConstCallFoldPassCtx(globals)(newPassContext(fn, nil, allowed, false))
	return out
}

func annotateGuardedConstCallFolds(fn *Function, globals map[string]*vm.FuncProto, callFacts *CallFacts) *Function {
	if fn == nil || fn.Proto == nil || len(globals) == 0 {
		return fn
	}
	if callFacts == nil {
		return fn
	}
	stableInts := collectProtocolStableIntGlobals(fn)
	folds := make(map[int]GuardedConstCallFoldFact)
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr == nil || instr.Op != OpCall || callResultCountFromAux2(instr.Aux2) != 1 {
				continue
			}
			name, callee := resolveCallee(instr, fn, InlineConfig{Globals: globals})
			if name == "" || callee == nil || len(instr.Args) < 2 || len(instr.Args)-1 > 4 {
				continue
			}
			args, intGuardConsts, intGuardValues, ok := guardedConstCallArgs(instr, stableInts)
			if !ok {
				continue
			}
			result, guardNames, guardProtos, ok := foldGuardedConstCall(callee, globals, args)
			if !ok {
				continue
			}
			if len(guardNames) == 0 {
				guardNames = []string{name}
				guardProtos = []*vm.FuncProto{callee}
			}
			guardConsts, ok := specializationEnsureGuardConstIndexes(fn.Proto, guardNames)
			if !ok {
				continue
			}
			folds[instr.ID] = GuardedConstCallFoldFact{
				CalleeProto:    callee,
				Result:         result,
				GuardConsts:    guardConsts,
				GuardProtos:    guardProtos,
				IntGuardConsts: intGuardConsts,
				IntGuardValues: intGuardValues,
			}
		}
	}
	if len(folds) == 0 {
		callFacts.SetGuardedConstCallFolds(nil)
		return fn
	}
	callFacts.SetGuardedConstCallFolds(folds)
	return fn
}

func guardedConstCallArgs(instr *Instr, stableInts map[int]int64) ([]runtime.Value, []int, []int64, bool) {
	n := len(instr.Args) - 1
	args := make([]runtime.Value, n)
	var guardConsts []int
	var guardValues []int64
	for i := 0; i < n; i++ {
		arg := instr.Args[1+i]
		if arg == nil || arg.Def == nil {
			return nil, nil, nil, false
		}
		switch arg.Def.Op {
		case OpConstInt:
			if arg.Def.Aux < 0 {
				return nil, nil, nil, false
			}
			args[i] = runtime.IntValue(arg.Def.Aux)
		case OpGetGlobal:
			constIdx := int(arg.Def.Aux)
			v, ok := stableInts[constIdx]
			if !ok || v < 0 {
				return nil, nil, nil, false
			}
			args[i] = runtime.IntValue(v)
			guardConsts = append(guardConsts, constIdx)
			guardValues = append(guardValues, v)
		default:
			return nil, nil, nil, false
		}
	}
	return args, guardConsts, guardValues, true
}

func foldGuardedConstCall(callee *vm.FuncProto, globals map[string]*vm.FuncProto, args []runtime.Value) (int64, []string, []*vm.FuncProto, bool) {
	if callee == nil || len(args) != callee.NumParams {
		return 0, nil, nil, false
	}
	if cf, ok := newMutualRecursiveIntSCCCompiled(callee, globals); ok {
		var intArgs [4]int64
		for i, arg := range args {
			if !arg.IsInt() || arg.Int() < 0 {
				return 0, nil, nil, false
			}
			intArgs[i] = arg.Int()
		}
		specialization := cf.MutualRecursiveIntSCC
		e := &mutualRecursiveIntEvaluator{
			specialization: specialization,
			memo:           make(map[mutualRecursiveIntKey]int64),
			active:         make(map[mutualRecursiveIntKey]bool),
		}
		out, ok := e.eval(specialization.entryIndex, intArgs)
		if !ok {
			return 0, nil, nil, false
		}
		return out, append([]string(nil), specialization.names...), append([]*vm.FuncProto(nil), specialization.protos...), true
	}
	return 0, nil, nil, false
}

func specializationEnsureGuardConstIndexes(proto *vm.FuncProto, names []string) ([]int, bool) {
	if proto == nil || len(names) == 0 {
		return nil, false
	}
	out := make([]int, 0, len(names))
	seen := make(map[int]bool, len(names))
	for _, name := range names {
		idx := -1
		for i, c := range proto.Constants {
			if c.IsString() && c.Str() == name {
				idx = i
				break
			}
		}
		if idx < 0 {
			idx = len(proto.Constants)
			proto.Constants = append(proto.Constants, runtime.StringValue(name))
		}
		if !seen[idx] {
			seen[idx] = true
			out = append(out, idx)
		}
	}
	return out, true
}

func collectProtocolStableIntGlobals(fn *Function) map[int]int64 {
	if fn == nil || fn.Proto == nil {
		return nil
	}
	values := make(map[int]int64)
	invalid := make(map[int]bool)
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr == nil || !opIsGlobalWrite(instr.Op) || len(instr.Args) == 0 {
				continue
			}
			constIdx := int(instr.Aux)
			arg := instr.Args[0]
			if arg == nil || arg.Def == nil || arg.Def.Op != OpConstInt {
				invalid[constIdx] = true
				delete(values, constIdx)
				continue
			}
			next := arg.Def.Aux
			if cur, ok := values[constIdx]; ok && cur != next {
				invalid[constIdx] = true
				delete(values, constIdx)
				continue
			}
			if !invalid[constIdx] {
				values[constIdx] = next
			}
		}
	}
	return values
}
