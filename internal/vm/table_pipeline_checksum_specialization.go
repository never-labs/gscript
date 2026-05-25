package vm

import "github.com/gscript/gscript/internal/runtime"

type tablePipelineChecksumSpec struct {
	buildGlobal     string
	addGlobal       string
	pairsGlobal     string
	nextGlobal      string
	ipairsGlobal    string
	mutateGlobal    string
	allocGlobal     string
	mutateRepsScale int64
}

func isTablePipelineChecksumProto(p *FuncProto) bool {
	_, ok := tablePipelineChecksumSpecForProto(p)
	return ok
}

func tablePipelineChecksumSpecForProto(p *FuncProto) (tablePipelineChecksumSpec, bool) {
	var spec tablePipelineChecksumSpec
	if p == nil || p.NumParams != 4 || p.UsesVarargBytecode || len(p.Code) != 49 || len(p.Constants) < 7 {
		return spec, false
	}
	for i := 0; i < 7; i++ {
		if !stringConst(p.Constants, i) {
			return spec, false
		}
	}
	code := p.Code
	pat := newBytecodePattern(code)
	if !pat.hasBxs(
		bxAt{pc: 5, op: OP_GETGLOBAL, bx: 0},
		bxAt{pc: 8, op: OP_GETGLOBAL, bx: 1},
		bxAt{pc: 10, op: OP_GETGLOBAL, bx: 2},
		bxAt{pc: 15, op: OP_GETGLOBAL, bx: 1},
		bxAt{pc: 17, op: OP_GETGLOBAL, bx: 3},
		bxAt{pc: 22, op: OP_GETGLOBAL, bx: 1},
		bxAt{pc: 24, op: OP_GETGLOBAL, bx: 4},
		bxAt{pc: 30, op: OP_GETGLOBAL, bx: 1},
		bxAt{pc: 32, op: OP_GETGLOBAL, bx: 5},
		bxAt{pc: 39, op: OP_GETGLOBAL, bx: 1},
		bxAt{pc: 41, op: OP_GETGLOBAL, bx: 6},
	) ||
		!pat.hasOps(
			opcodeAt{pc: 4, op: OP_FORPREP},
			opcodeAt{pc: 7, op: OP_CALL},
			opcodeAt{pc: 12, op: OP_CALL},
			opcodeAt{pc: 13, op: OP_CALL},
			opcodeAt{pc: 19, op: OP_CALL},
			opcodeAt{pc: 20, op: OP_CALL},
			opcodeAt{pc: 26, op: OP_CALL},
			opcodeAt{pc: 27, op: OP_CALL},
			opcodeAt{pc: 29, op: OP_FORLOOP},
			opcodeAt{pc: 34, op: OP_LOADINT},
			opcodeAt{pc: 35, op: OP_MUL},
			opcodeAt{pc: 36, op: OP_CALL},
			opcodeAt{pc: 37, op: OP_CALL},
			opcodeAt{pc: 44, op: OP_CALL},
			opcodeAt{pc: 45, op: OP_CALL},
			opcodeAt{pc: 48, op: OP_RETURN},
		) {
		return spec, false
	}
	return tablePipelineChecksumSpec{
		buildGlobal: p.Constants[0].Str(), addGlobal: p.Constants[1].Str(),
		pairsGlobal: p.Constants[2].Str(), nextGlobal: p.Constants[3].Str(), ipairsGlobal: p.Constants[4].Str(),
		mutateGlobal: p.Constants[5].Str(), allocGlobal: p.Constants[6].Str(),
		mutateRepsScale: int64(DecodesBx(code[34])),
	}, true
}

func (vm *VM) runTablePipelineChecksumRuntimeSpecialization(cl *Closure, args []runtime.Value) (bool, []runtime.Value, error) {
	if len(args) != 4 {
		return false, nil, nil
	}
	for _, arg := range args {
		if arg.RawType() != runtime.TypeInt {
			return false, nil, nil
		}
	}
	spec, ok := tablePipelineChecksumSpecForProto(cl.Proto)
	if !ok || !vm.tablePipelineCalleesActive(spec) {
		return false, nil, nil
	}
	mod, ok := vm.tableIteratorModuloFoldModulus(spec.addGlobal)
	if !ok || mod == 0 {
		return false, nil, nil
	}
	size, reps := args[0].RawInt(), args[1].RawInt()
	allocN, allocReps := args[2].RawInt(), args[3].RawInt()
	checksum := int64(0)
	for r := int64(1); r <= reps; r++ {
		pairs, next, ipairs := mixedAffineTableFoldChecksums(size, mod)
		checksum = positiveModInt64(checksum+pairs, mod)
		checksum = positiveModInt64(checksum+next, mod)
		checksum = positiveModInt64(checksum+ipairs, mod)
	}
	mutate := insertRemoveChecksumValue(size, reps*spec.mutateRepsScale, mod)
	alloc := linkedRecordChecksumValue(allocN, allocReps, mod)
	checksum = positiveModInt64(checksum+mutate, mod)
	checksum = positiveModInt64(checksum+alloc, mod)
	return true, []runtime.Value{runtime.IntValue(checksum)}, nil
}

func (vm *VM) tablePipelineCalleesActive(spec tablePipelineChecksumSpec) bool {
	check := func(name string, pred func(*FuncProto) bool) bool {
		cl, ok := closureFromValue(vm.GetGlobal(name))
		return ok && cl != nil && pred(cl.Proto)
	}
	return check(spec.buildGlobal, isMixedAffineTableBuilderProto) &&
		check(spec.pairsGlobal, isTableIteratorModuloFoldProto) &&
		check(spec.nextGlobal, isTableIteratorModuloFoldProto) &&
		check(spec.ipairsGlobal, isTableIteratorModuloFoldProto) &&
		check(spec.mutateGlobal, isInsertRemoveChecksumProto) &&
		check(spec.allocGlobal, isLinkedRecordChecksumProto)
}

func mixedAffineTableFoldChecksums(n, mod int64) (int64, int64, int64) {
	var pairs, next, ipairs int64
	var pairsCount, nextCount, ipairsCount int64
	for i := int64(1); i <= n; i++ {
		arrayValue := i*3 + 1
		pairs = positiveModInt64(pairs+i*3+arrayValue, mod)
		next = positiveModInt64(next+i*13+arrayValue, mod)
		ipairs = positiveModInt64(ipairs+i*29+arrayValue, mod)
		pairsCount++
		nextCount++
		ipairsCount++
		if i%3 == 0 {
			v := i*7 + 5
			keyLen := int64(1 + lenInt64Decimal(i))
			pairs = positiveModInt64(pairs+keyLen*5+v, mod)
			next = positiveModInt64(next+keyLen*19+v, mod)
			pairsCount++
			nextCount++
		}
		if i%10 == 0 {
			v := i*11 + 9
			k := -i
			pairs = positiveModInt64(pairs+k*3+v, mod)
			next = positiveModInt64(next+k*13+v, mod)
			pairsCount++
			nextCount++
		}
	}
	pairs = positiveModInt64(pairs+pairsCount*17, mod)
	next = positiveModInt64(next+nextCount*23, mod)
	ipairs = positiveModInt64(ipairs+ipairsCount*31, mod)
	return pairs, next, ipairs
}

func insertRemoveChecksumValue(n, reps, mod int64) int64 {
	if n < 1 {
		return 0
	}
	values := make([]int64, n+1)
	for i := int64(1); i <= n; i++ {
		values[i] = i
	}
	checksum := positiveModInt64(n+n, mod)
	for r := int64(1); r <= reps; r++ {
		pos := positiveModInt64(r, n) + 1
		removed := values[pos]
		values[pos] = r
		checksum = positiveModInt64(checksum+removed+n+n, mod)
	}
	return checksum
}

func linkedRecordChecksumValue(n, reps, mod int64) int64 {
	const valueMod = int64(997)
	checksum := int64(0)
	roots := runtime.NewTableSized(32, 0)
	for r := int64(1); r <= reps; r++ {
		for i := int64(1); i <= n; i += 4 {
			checksum = positiveModInt64(checksum+i+r+positiveModInt64(i*r, valueMod), mod)
		}
		roots.RawSetInt(positiveModInt64(r, 32)+1, runtime.BoolValue(true))
	}
	return positiveModInt64(checksum+int64(roots.Len())*37, mod)
}

func lenInt64Decimal(v int64) int {
	if v < 10 {
		return 1
	}
	n := 0
	for v > 0 {
		n++
		v /= 10
	}
	return n
}
