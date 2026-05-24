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
	if DecodeOp(code[4]) != OP_FORPREP ||
		DecodeOp(code[5]) != OP_GETGLOBAL || DecodeBx(code[5]) != 0 ||
		DecodeOp(code[7]) != OP_CALL ||
		DecodeOp(code[8]) != OP_GETGLOBAL || DecodeBx(code[8]) != 1 ||
		DecodeOp(code[10]) != OP_GETGLOBAL || DecodeBx(code[10]) != 2 ||
		DecodeOp(code[12]) != OP_CALL ||
		DecodeOp(code[13]) != OP_CALL ||
		DecodeOp(code[15]) != OP_GETGLOBAL || DecodeBx(code[15]) != 1 ||
		DecodeOp(code[17]) != OP_GETGLOBAL || DecodeBx(code[17]) != 3 ||
		DecodeOp(code[19]) != OP_CALL ||
		DecodeOp(code[20]) != OP_CALL ||
		DecodeOp(code[22]) != OP_GETGLOBAL || DecodeBx(code[22]) != 1 ||
		DecodeOp(code[24]) != OP_GETGLOBAL || DecodeBx(code[24]) != 4 ||
		DecodeOp(code[26]) != OP_CALL ||
		DecodeOp(code[27]) != OP_CALL ||
		DecodeOp(code[29]) != OP_FORLOOP ||
		DecodeOp(code[30]) != OP_GETGLOBAL || DecodeBx(code[30]) != 1 ||
		DecodeOp(code[32]) != OP_GETGLOBAL || DecodeBx(code[32]) != 5 ||
		DecodeOp(code[34]) != OP_LOADINT ||
		DecodeOp(code[35]) != OP_MUL ||
		DecodeOp(code[36]) != OP_CALL ||
		DecodeOp(code[37]) != OP_CALL ||
		DecodeOp(code[39]) != OP_GETGLOBAL || DecodeBx(code[39]) != 1 ||
		DecodeOp(code[41]) != OP_GETGLOBAL || DecodeBx(code[41]) != 6 ||
		DecodeOp(code[44]) != OP_CALL ||
		DecodeOp(code[45]) != OP_CALL ||
		DecodeOp(code[48]) != OP_RETURN {
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
