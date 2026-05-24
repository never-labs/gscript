package vm

import "github.com/gscript/gscript/internal/runtime"

type callsVarargCoroutineDriverSpec struct {
	makeWorkerGlobal string
	adjustedGlobal   string
	varargGlobal     string
	pipelineGlobal   string
	modGlobal        string
	seed             int64
	adjusted         callsAffineModuloLeafSpec
	vararg           callsVarargFoldSpec
	worker           callsCapturedWorkerSpec
	pipeline         callsCoroutinePipelineSpec
}

type callsAffineModuloLeafSpec struct {
	offsets [4]int64
	scales  [5]int64
}

type callsVarargFoldSpec struct {
	countScale int64
	scales     [5]int64
}

type callsCapturedWorkerSpec struct {
	argOffsets       [2]int64
	valueScales      [4]int64
	captureBias      int64
	returnModuloName string
}

type callsCoroutinePipelineSpec struct {
	start        int64
	stepModulo   int64
	argOffsets   [3]int64
	totalScale   int64
	returnModulo string
}

func isCallsVarargCoroutineDriverProto(p *FuncProto) bool {
	_, ok := callsVarargCoroutineDriverSpecForProto(p)
	return ok
}

func (vm *VM) runCallsVarargCoroutineDriverRuntimeSpecialization(cl *Closure, args []runtime.Value) (bool, []runtime.Value, error) {
	if len(args) != 2 || args[0].RawType() != runtime.TypeInt || args[1].RawType() != runtime.TypeInt {
		return false, nil, nil
	}
	spec, ok := callsVarargCoroutineDriverSpecForProto(cl.Proto)
	if !ok {
		return false, nil, nil
	}
	spec, ok = callsVarargCoroutineRuntimeGuards(vm, spec)
	if !ok {
		return false, nil, nil
	}
	nCalls := args[0].RawInt()
	nCoro := args[1].RawInt()
	if nCalls < 0 || nCoro < 0 {
		return false, nil, nil
	}
	modVal := vm.GetGlobal(spec.modGlobal)
	if modVal.RawType() != runtime.TypeInt || modVal.RawInt() == 0 {
		return false, nil, nil
	}
	mod := modVal.RawInt()
	checksum := int64(0)
	captured := modInt64(spec.seed, mod)
	adjusted := spec.adjusted
	vararg := spec.vararg
	worker := spec.worker
	for i := int64(1); i <= nCalls; i++ {
		adjustedValue := (i*adjusted.scales[0] +
			(i+adjusted.offsets[0])*adjusted.scales[1] +
			(i+adjusted.offsets[1])*adjusted.scales[2] +
			(i+adjusted.offsets[2])*adjusted.scales[3] +
			i*adjusted.scales[4]) % mod
		checksum = (checksum + adjustedValue) % mod

		varargValue := (i +
			4*vararg.countScale +
			(i+1)*vararg.scales[0] +
			(i+2)*vararg.scales[1] +
			(i+3)*vararg.scales[2] +
			(i+4)*vararg.scales[3] +
			(i+4)*vararg.scales[4]) % mod
		checksum = (checksum + varargValue) % mod

		a := i + captured
		b := a + worker.argOffsets[0]
		c := a + worker.argOffsets[1]
		v := (a +
			(i+1)*worker.valueScales[0] +
			(i+2)*worker.valueScales[1] +
			(i+3)*worker.valueScales[2] +
			(i+4)*worker.valueScales[3]) % mod
		nextCaptured := (captured + b + c + worker.captureBias) % mod
		captured = nextCaptured
		checksum = (checksum + (v+nextCaptured)%mod) % mod
	}
	total := moduloArithmeticSeries(spec.pipeline.totalScale, nCoro-1, mod)
	checksum = (checksum + total) % mod
	return true, []runtime.Value{runtime.IntValue(checksum)}, nil
}

func (s callsAffineModuloLeafSpec) eval(i, mod int64) int64 {
	values := [5]int64{i, i + s.offsets[0], i + s.offsets[1], i + s.offsets[2], i}
	sum := int64(0)
	for idx, value := range values {
		sum += value * s.scales[idx]
	}
	return modInt64(sum, mod)
}

func (s callsVarargFoldSpec) eval(base, mod int64) int64 {
	args := [4]int64{base + 1, base + 2, base + 3, base + 4}
	sum := base + 4*s.countScale
	for i, arg := range args {
		sum += arg * s.scales[i]
	}
	sum += args[3] * s.scales[4]
	return modInt64(sum, mod)
}

func (s callsCapturedWorkerSpec) eval(captured, mod int64, i, v1, v2, v3, v4 int64) (int64, int64) {
	a := i + captured
	b := a + s.argOffsets[0]
	c := a + s.argOffsets[1]
	v := modInt64(a+v1*s.valueScales[0]+v2*s.valueScales[1]+v3*s.valueScales[2]+v4*s.valueScales[3], mod)
	next := modInt64(captured+b+c+s.captureBias, mod)
	return modInt64(v+next, mod), next
}

func callsVarargCoroutineDriverSpecForProto(p *FuncProto) (callsVarargCoroutineDriverSpec, bool) {
	var spec callsVarargCoroutineDriverSpec
	if p == nil || p.NumParams != 2 || p.IsVarArg || len(p.Code) != 56 || len(p.Constants) < 5 {
		return spec, false
	}
	code := p.Code
	required := map[int]Opcode{
		0: OP_GETGLOBAL, 1: OP_LOADINT, 2: OP_CALL, 3: OP_LOADINT,
		4: OP_LOADINT, 5: OP_MOVE, 6: OP_LOADINT, 7: OP_FORPREP,
		8: OP_GETGLOBAL, 10: OP_CALL, 12: OP_GETGLOBAL,
		15: OP_GETGLOBAL, 25: OP_CALL, 27: OP_GETGLOBAL,
		30: OP_MOVE, 40: OP_CALL, 42: OP_GETGLOBAL,
		45: OP_FORLOOP, 46: OP_GETGLOBAL, 49: OP_CALL, 51: OP_GETGLOBAL,
		55: OP_RETURN,
	}
	for pc, op := range required {
		if DecodeOp(code[pc]) != op {
			return spec, false
		}
	}
	if DecodesBx(code[3]) != 0 || DecodesBx(code[4]) != 1 || DecodesBx(code[6]) != 1 {
		return spec, false
	}
	var ok bool
	if spec.makeWorkerGlobal, ok = constStringAt(p, DecodeBx(code[0])); !ok {
		return spec, false
	}
	if spec.adjustedGlobal, ok = constStringAt(p, DecodeBx(code[8])); !ok {
		return spec, false
	}
	if spec.modGlobal, ok = constStringAt(p, DecodeBx(code[12])); !ok {
		return spec, false
	}
	if spec.varargGlobal, ok = constStringAt(p, DecodeBx(code[15])); !ok {
		return spec, false
	}
	if spec.pipelineGlobal, ok = constStringAt(p, DecodeBx(code[46])); !ok {
		return spec, false
	}
	spec.seed = int64(DecodesBx(code[1]))
	return spec, true
}

func callsVarargCoroutineRuntimeGuards(vm *VM, spec callsVarargCoroutineDriverSpec) (callsVarargCoroutineDriverSpec, bool) {
	adjusted, ok := closureFromValue(vm.GetGlobal(spec.adjustedGlobal))
	if !ok {
		return spec, false
	}
	if spec.adjusted, ok = callsAffineModuloLeafSpecForProto(adjusted.Proto, spec.modGlobal); !ok {
		return spec, false
	}
	varargFold, ok := closureFromValue(vm.GetGlobal(spec.varargGlobal))
	if !ok {
		return spec, false
	}
	if spec.vararg, ok = callsVarargFoldSpecForProto(varargFold.Proto, spec.modGlobal); !ok {
		return spec, false
	}
	makeWorker, ok := closureFromValue(vm.GetGlobal(spec.makeWorkerGlobal))
	if !ok || makeWorker.Proto == nil || len(makeWorker.Proto.Protos) != 1 {
		return spec, false
	}
	if spec.worker, ok = callsCapturedWorkerSpecForProto(makeWorker.Proto.Protos[0], spec.modGlobal); !ok {
		return spec, false
	}
	pipeline, ok := closureFromValue(vm.GetGlobal(spec.pipelineGlobal))
	if !ok {
		return spec, false
	}
	if spec.pipeline, ok = callsCoroutinePipelineSpecForProto(pipeline.Proto, spec.modGlobal); !ok {
		return spec, false
	}
	return spec, true
}

func callsAffineModuloLeafSpecForProto(p *FuncProto, modGlobal string) (callsAffineModuloLeafSpec, bool) {
	var spec callsAffineModuloLeafSpec
	if p == nil || p.NumParams != 1 || p.IsVarArg || len(p.Code) != 22 || len(p.Protos) != 0 {
		return spec, false
	}
	code := p.Code
	if DecodeOp(code[1]) != OP_LOADINT || DecodeOp(code[3]) != OP_LOADINT || DecodeOp(code[5]) != OP_LOADINT ||
		DecodeOp(code[7]) != OP_LOADINT || DecodeOp(code[10]) != OP_LOADINT || DecodeOp(code[13]) != OP_LOADINT ||
		DecodeOp(code[16]) != OP_LOADINT || DecodeOp(code[19]) != OP_GETGLOBAL || DecodeOp(code[20]) != OP_MOD {
		return spec, false
	}
	name, ok := constStringAt(p, DecodeBx(code[19]))
	if !ok || name != modGlobal {
		return spec, false
	}
	spec.offsets = [4]int64{int64(DecodesBx(code[1])), int64(DecodesBx(code[3])), int64(DecodesBx(code[5])), 0}
	spec.scales = [5]int64{1, int64(DecodesBx(code[7])), int64(DecodesBx(code[10])), int64(DecodesBx(code[13])), int64(DecodesBx(code[16]))}
	return spec, true
}

func callsVarargFoldSpecForProto(p *FuncProto, modGlobal string) (callsVarargFoldSpec, bool) {
	var spec callsVarargFoldSpec
	if p == nil || p.NumParams != 1 || !p.IsVarArg || len(p.Code) != 45 || len(p.Protos) != 0 {
		return spec, false
	}
	code := p.Code
	if DecodeOp(code[0]) != OP_GETGLOBAL || DecodeOp(code[1]) != OP_LOADK || DecodeOp(code[2]) != OP_VARARG ||
		DecodeOp(code[3]) != OP_CALL || DecodeOp(code[24]) != OP_LOADINT || DecodeOp(code[42]) != OP_GETGLOBAL ||
		DecodeOp(code[43]) != OP_MOD {
		return spec, false
	}
	selectName, ok := constStringAt(p, DecodeBx(code[0]))
	if !ok || selectName != "select" {
		return spec, false
	}
	hash, ok := constStringAt(p, DecodeBx(code[1]))
	if !ok || hash != "#" {
		return spec, false
	}
	name, ok := constStringAt(p, DecodeBx(code[42]))
	if !ok || name != modGlobal {
		return spec, false
	}
	for k, pc := range []int{5, 9, 13, 17, 21} {
		if DecodeOp(code[pc-1]) != OP_GETGLOBAL || DecodeOp(code[pc]) != OP_LOADINT ||
			DecodeOp(code[pc+1]) != OP_VARARG || DecodeOp(code[pc+2]) != OP_CALL {
			return spec, false
		}
		name, ok := constStringAt(p, DecodeBx(code[pc-1]))
		if !ok || name != "select" || DecodesBx(code[pc]) != k+1 && !(k == 4 && DecodesBx(code[pc]) == 4) {
			return spec, false
		}
	}
	spec.countScale = int64(DecodesBx(code[24]))
	spec.scales = [5]int64{
		int64(DecodesBx(code[27])),
		int64(DecodesBx(code[30])),
		int64(DecodesBx(code[33])),
		int64(DecodesBx(code[36])),
		int64(DecodesBx(code[39])),
	}
	return spec, true
}

func callsCapturedWorkerSpecForProto(p *FuncProto, modGlobal string) (callsCapturedWorkerSpec, bool) {
	var spec callsCapturedWorkerSpec
	if p == nil || p.NumParams != 5 || p.IsVarArg || len(p.Code) != 33 || len(p.Upvalues) != 1 {
		return spec, false
	}
	code := p.Code
	required := map[int]Opcode{
		0: OP_GETUPVAL, 1: OP_ADD, 2: OP_LOADINT, 3: OP_ADD, 4: OP_LOADINT, 5: OP_ADD,
		6: OP_LOADINT, 9: OP_LOADINT, 12: OP_LOADINT, 15: OP_LOADINT,
		18: OP_GETGLOBAL, 19: OP_MOD, 20: OP_GETUPVAL, 23: OP_LOADINT,
		25: OP_GETGLOBAL, 26: OP_MOD, 27: OP_SETUPVAL, 28: OP_GETUPVAL,
		30: OP_GETGLOBAL, 31: OP_MOD, 32: OP_RETURN,
	}
	for pc, op := range required {
		if DecodeOp(code[pc]) != op {
			return spec, false
		}
	}
	for _, pc := range []int{18, 25, 30} {
		name, ok := constStringAt(p, DecodeBx(code[pc]))
		if !ok || name != modGlobal {
			return spec, false
		}
	}
	spec.argOffsets = [2]int64{int64(DecodesBx(code[2])), int64(DecodesBx(code[4]))}
	spec.valueScales = [4]int64{int64(DecodesBx(code[6])), int64(DecodesBx(code[9])), int64(DecodesBx(code[12])), int64(DecodesBx(code[15]))}
	spec.captureBias = int64(DecodesBx(code[23]))
	return spec, true
}

func callsCoroutinePipelineSpecForProto(p *FuncProto, modGlobal string) (callsCoroutinePipelineSpec, bool) {
	var spec callsCoroutinePipelineSpec
	if p == nil || p.NumParams != 2 || p.IsVarArg || len(p.Code) != 40 || len(p.Protos) != 1 {
		return spec, false
	}
	code := p.Code
	if DecodeOp(code[0]) != OP_GETGLOBAL || DecodeOp(code[1]) != OP_GETFIELD || DecodeOp(code[2]) != OP_CLOSURE ||
		DecodeOp(code[5]) != OP_LOADINT || DecodeOp(code[6]) != OP_RESUME || DecodeOp(code[14]) != OP_LOADINT ||
		DecodeOp(code[15]) != OP_LOADINT || DecodeOp(code[16]) != OP_SUB || DecodeOp(code[18]) != OP_FORPREP ||
		DecodeOp(code[20]) != OP_RESUME || DecodeOp(code[29]) != OP_LOADINT || DecodeOp(code[32]) != OP_GETGLOBAL ||
		DecodeOp(code[33]) != OP_MOD || DecodeOp(code[37]) != OP_RETURN {
		return spec, false
	}
	name, ok := constStringAt(p, DecodeBx(code[32]))
	if !ok || name != modGlobal {
		return spec, false
	}
	spec.start = int64(DecodesBx(code[5]))
	spec.totalScale = int64(DecodesBx(code[29]))
	if spec.stepModulo, spec.argOffsets, ok = callsCoroutineBodySpecForProto(p.Protos[0], modGlobal); !ok {
		return spec, false
	}
	return spec, true
}

func callsCoroutineBodySpecForProto(p *FuncProto, modGlobal string) (int64, [3]int64, bool) {
	var offsets [3]int64
	if p == nil || p.NumParams != 1 || p.IsVarArg || len(p.Code) != 31 || len(p.Upvalues) != 2 {
		return 0, offsets, false
	}
	code := p.Code
	required := map[int]Opcode{
		1: OP_LOADINT, 2: OP_GETUPVAL, 4: OP_FORPREP, 6: OP_YIELD, 7: OP_LOADINT,
		8: OP_MOD, 9: OP_LOADINT, 10: OP_ADD, 11: OP_LOADINT, 12: OP_ADD,
		13: OP_LOADINT, 14: OP_ADD, 15: OP_GETUPVAL, 16: OP_ADD, 21: OP_CALL,
		24: OP_GETGLOBAL, 25: OP_MOD, 27: OP_FORLOOP, 30: OP_RETURN,
	}
	for pc, op := range required {
		if DecodeOp(code[pc]) != op {
			return 0, offsets, false
		}
	}
	name, ok := constStringAt(p, DecodeBx(code[24]))
	if !ok || name != modGlobal {
		return 0, offsets, false
	}
	return int64(DecodesBx(code[7])), [3]int64{int64(DecodesBx(code[9])), int64(DecodesBx(code[11])), int64(DecodesBx(code[13]))}, true
}

func modInt64(v, m int64) int64 {
	r := v % m
	if r < 0 {
		r += m
	}
	return r
}

func moduloArithmeticSeries(scale, n, mod int64) int64 {
	if n <= 0 {
		return 0
	}
	total := int64(0)
	for i := int64(1); i <= n; i++ {
		total = (total + i*scale) % mod
	}
	return total
}
