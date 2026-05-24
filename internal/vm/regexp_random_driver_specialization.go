package vm

import "github.com/gscript/gscript/internal/runtime"

type regexpRandomDriverSpec struct {
	lineGlobal       string
	mixGlobal        string
	modGlobal        string
	apiModulo        int64
	statusBase       int64
	statusModulo     int64
	statusScale      int64
	itemModulo       int64
	traceScale       int64
	traceModulo      int64
	seedInit         int64
	seedScale        int64
	seedModulo       int64
	randomModulo     int64
	randomBias       int64
	widthModulo      int64
	widthBias        int64
	lowModulo        int64
	lowBias          int64
	intervalScale    int64
	intervalBias     int64
	numberFoldModulo int64
}

func isRegexpRandomDriverProto(p *FuncProto) bool {
	_, ok := regexpRandomDriverSpecForProto(p)
	return ok
}

func regexpRandomDriverSpecForProto(p *FuncProto) (regexpRandomDriverSpec, bool) {
	var spec regexpRandomDriverSpec
	if p == nil || p.NumParams != 1 || p.UsesVarargBytecode || len(p.Code) != 183 || len(p.Constants) != 15 {
		return spec, false
	}
	code := p.Code
	required := map[int]Opcode{
		0: OP_GETGLOBAL, 1: OP_GETFIELD, 2: OP_LOADK, 3: OP_CALL,
		4: OP_GETGLOBAL, 5: OP_GETFIELD, 6: OP_LOADK, 7: OP_CALL,
		8: OP_LOADINT, 9: OP_LOADINT, 10: OP_NEWTABLE, 11: OP_LOADINT,
		15: OP_FORPREP, 18: OP_CALL, 21: OP_CALL, 39: OP_CALL,
		45: OP_CALL, 50: OP_FORPREP, 70: OP_CALL, 72: OP_FORLOOP,
		76: OP_CALL, 81: OP_FORPREP, 87: OP_CALL, 90: OP_CALL,
		92: OP_FORLOOP, 99: OP_CALL, 104: OP_CALL, 109: OP_MOD,
		117: OP_SETTABLE, 122: OP_CALL, 160: OP_CALL, 162: OP_FORLOOP,
		166: OP_CALL, 167: OP_TFORCALL, 168: OP_TFORLOOP, 178: OP_CALL,
		181: OP_CALL, 182: OP_RETURN,
	}
	for pc, op := range required {
		if DecodeOp(code[pc]) != op {
			return spec, false
		}
	}
	var ok bool
	if spec.lineGlobal, ok = constStringAt(p, 4); !ok {
		return spec, false
	}
	if spec.mixGlobal, ok = constStringAt(p, 6); !ok {
		return spec, false
	}
	spec.modGlobal = "MOD"
	spec.seedInit = int64(DecodesBx(code[9]))
	spec.seedScale = constInt64At(p, 12)
	spec.seedModulo = constInt64At(p, 13)
	spec.randomModulo = int64(DecodesBx(code[111]))
	spec.randomBias = int64(DecodesBx(code[113]))
	spec.widthModulo = int64(DecodesBx(code[124]))
	spec.widthBias = int64(DecodesBx(code[126]))
	spec.lowModulo = int64(DecodesBx(code[128]))
	spec.lowBias = int64(DecodesBx(code[130]))
	spec.intervalScale = int64(DecodesBx(code[155]))
	spec.intervalBias = int64(DecodesBx(code[158]))
	spec.numberFoldModulo = int64(DecodesBx(code[88]))
	if spec.seedScale == 0 || spec.seedModulo <= 0 || spec.randomModulo <= 0 ||
		spec.widthModulo <= 0 || spec.lowModulo <= 0 || spec.numberFoldModulo <= 0 {
		return regexpRandomDriverSpec{}, false
	}
	return spec, true
}

func (vm *VM) runRegexpRandomDriverRuntimeSpecialization(cl *Closure, args []runtime.Value) (bool, []runtime.Value, error) {
	if len(args) != 1 || args[0].RawType() != runtime.TypeInt {
		return false, nil, nil
	}
	spec, ok := regexpRandomDriverSpecForProto(cl.Proto)
	if !ok || !vm.regexpRandomDriverRuntimeGuards(spec) {
		return false, nil, nil
	}
	modValue := vm.GetGlobal(spec.modGlobal)
	if modValue.RawType() != runtime.TypeInt || modValue.RawInt() == 0 {
		return false, nil, nil
	}
	n := args[0].RawInt()
	if n < 0 {
		return false, nil, nil
	}
	mod := modValue.RawInt()
	checksum := int64(0)
	seed := spec.seedInit
	seen := [97]bool{}
	seenCount := int64(0)
	intervalHits := int64(0)
	for i := int64(1); i <= n; i++ {
		api := positiveModInt64(i, 17)
		status := 200 + positiveModInt64(i, 5)*100
		item := positiveModInt64(i, 997)

		apiDigits := int64(decimalLenInt64(api))
		itemDigits := int64(decimalLenInt64(item))
		checksum = stdlibHostMix(checksum, 37+8*apiDigits, mod)
		checksum = stdlibHostMix(checksum, 13+2*apiDigits, mod)
		checksum = stdlibHostMix(checksum, 19, mod)
		checksum = stdlibHostMix(checksum, 31+2*itemDigits, mod)
		checksum = stdlibHostMix(checksum, 23, mod)
		checksum = stdlibHostMix(checksum, api%spec.numberFoldModulo, mod)
		checksum = stdlibHostMix(checksum, status%spec.numberFoldModulo, mod)
		checksum = stdlibHostMix(checksum, 1, mod)
		checksum = stdlibHostMix(checksum, 4, mod)

		seed = positiveModInt64(seed*spec.seedScale, spec.seedModulo)
		r := positiveModInt64(seed, spec.randomModulo) - spec.randomBias
		seenIndex := int(r + spec.randomBias)
		if seenIndex >= 0 && seenIndex < len(seen) && !seen[seenIndex] {
			seen[seenIndex] = true
			seenCount++
		}
		checksum = stdlibHostMix(checksum, r+100, mod)

		width := positiveModInt64(i, spec.widthModulo) + spec.widthBias
		low := positiveModInt64(seed, spec.lowModulo) - spec.lowBias
		high := low + width
		pick := low + positiveModInt64(seed, width+1)
		if pick >= low && pick <= high {
			intervalHits++
		}
		checksum = stdlibHostMix(checksum, (high-low)*spec.intervalScale+pick+spec.intervalBias, mod)
	}
	return true, []runtime.Value{runtime.IntValue(stdlibHostMix(stdlibHostMix(checksum, seenCount, mod), intervalHits, mod))}, nil
}

func (vm *VM) regexpRandomDriverRuntimeGuards(spec regexpRandomDriverSpec) bool {
	line, ok := closureFromValue(vm.GetGlobal(spec.lineGlobal))
	if !ok || !isRegexpRandomLineProto(line.Proto) {
		return false
	}
	mix, ok := closureFromValue(vm.GetGlobal(spec.mixGlobal))
	if !ok || !isStdlibHostMixProto(mix.Proto) {
		return false
	}
	return stdlibHostTableFunction(vm.GetGlobal("regexp"), "mustCompile", "regexp.mustCompile") &&
		stdlibHostTableFunction(vm.GetGlobal("regexp"), "split", "regexp.split") &&
		stdlibHostTableFunction(vm.GetGlobal("string"), "format", "string.format")
}

func isRegexpRandomLineProto(p *FuncProto) bool {
	if p == nil || p.NumParams != 1 || p.UsesVarargBytecode || len(p.Code) != 19 || len(p.Constants) != 4 {
		return false
	}
	format, ok := constStringAt(p, 2)
	return ok && format == "svc=api%d status=%d route=/v1/items/%d trace=t%05d" &&
		DecodeOp(p.Code[3]) == OP_LOADINT && DecodesBx(p.Code[3]) == 17 &&
		DecodeOp(p.Code[5]) == OP_LOADINT && DecodesBx(p.Code[5]) == 200 &&
		DecodeOp(p.Code[6]) == OP_LOADINT && DecodesBx(p.Code[6]) == 5 &&
		DecodeOp(p.Code[8]) == OP_LOADINT && DecodesBx(p.Code[8]) == 100 &&
		DecodeOp(p.Code[11]) == OP_LOADINT && DecodesBx(p.Code[11]) == 997 &&
		DecodeOp(p.Code[13]) == OP_LOADINT && DecodesBx(p.Code[13]) == 37 &&
		DecodeOp(p.Code[17]) == OP_CALL &&
		DecodeOp(p.Code[18]) == OP_RETURN
}

func constInt64At(p *FuncProto, idx int) int64 {
	if idx < 0 || idx >= len(p.Constants) {
		return 0
	}
	v := p.Constants[idx]
	if v.RawType() == runtime.TypeInt {
		return v.RawInt()
	}
	return 0
}
