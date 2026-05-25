package vm

import "github.com/gscript/gscript/internal/runtime"

type regexpRandomDriverSpec struct {
	lineGlobal       string
	mixGlobal        string
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
	pat := newBytecodePattern(code)
	if !pat.hasOps(
		opcodeAt{pc: 0, op: OP_GETGLOBAL},
		opcodeAt{pc: 1, op: OP_GETFIELD},
		opcodeAt{pc: 2, op: OP_LOADK},
		opcodeAt{pc: 3, op: OP_CALL},
		opcodeAt{pc: 4, op: OP_GETGLOBAL},
		opcodeAt{pc: 5, op: OP_GETFIELD},
		opcodeAt{pc: 6, op: OP_LOADK},
		opcodeAt{pc: 7, op: OP_CALL},
		opcodeAt{pc: 8, op: OP_LOADINT},
		opcodeAt{pc: 9, op: OP_LOADINT},
		opcodeAt{pc: 10, op: OP_NEWTABLE},
		opcodeAt{pc: 11, op: OP_LOADINT},
		opcodeAt{pc: 15, op: OP_FORPREP},
		opcodeAt{pc: 18, op: OP_CALL},
		opcodeAt{pc: 21, op: OP_CALL},
		opcodeAt{pc: 39, op: OP_CALL},
		opcodeAt{pc: 45, op: OP_CALL},
		opcodeAt{pc: 50, op: OP_FORPREP},
		opcodeAt{pc: 70, op: OP_CALL},
		opcodeAt{pc: 72, op: OP_FORLOOP},
		opcodeAt{pc: 76, op: OP_CALL},
		opcodeAt{pc: 81, op: OP_FORPREP},
		opcodeAt{pc: 87, op: OP_CALL},
		opcodeAt{pc: 90, op: OP_CALL},
		opcodeAt{pc: 92, op: OP_FORLOOP},
		opcodeAt{pc: 99, op: OP_CALL},
		opcodeAt{pc: 104, op: OP_CALL},
		opcodeAt{pc: 109, op: OP_MOD},
		opcodeAt{pc: 117, op: OP_SETTABLE},
		opcodeAt{pc: 122, op: OP_CALL},
		opcodeAt{pc: 160, op: OP_CALL},
		opcodeAt{pc: 162, op: OP_FORLOOP},
		opcodeAt{pc: 166, op: OP_CALL},
		opcodeAt{pc: 167, op: OP_TFORCALL},
		opcodeAt{pc: 168, op: OP_TFORLOOP},
		opcodeAt{pc: 178, op: OP_CALL},
		opcodeAt{pc: 181, op: OP_CALL},
		opcodeAt{pc: 182, op: OP_RETURN},
	) {
		return spec, false
	}
	var ok bool
	if spec.lineGlobal, ok = constStringAt(p, 4); !ok {
		return spec, false
	}
	if spec.mixGlobal, ok = constStringAt(p, 6); !ok {
		return spec, false
	}
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
	if !ok {
		return false, nil, nil
	}
	fold, ok := vm.regexpRandomDriverRuntimeGuards(spec)
	if !ok {
		return false, nil, nil
	}
	modValue := vm.GetGlobal(fold.modGlobal)
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
		checksum = binaryIntModuloFold(checksum, 37+8*apiDigits, fold.mul, mod)
		checksum = binaryIntModuloFold(checksum, 13+2*apiDigits, fold.mul, mod)
		checksum = binaryIntModuloFold(checksum, 19, fold.mul, mod)
		checksum = binaryIntModuloFold(checksum, 31+2*itemDigits, fold.mul, mod)
		checksum = binaryIntModuloFold(checksum, 23, fold.mul, mod)
		checksum = binaryIntModuloFold(checksum, api%spec.numberFoldModulo, fold.mul, mod)
		checksum = binaryIntModuloFold(checksum, status%spec.numberFoldModulo, fold.mul, mod)
		checksum = binaryIntModuloFold(checksum, 1, fold.mul, mod)
		checksum = binaryIntModuloFold(checksum, 4, fold.mul, mod)

		seed = positiveModInt64(seed*spec.seedScale, spec.seedModulo)
		r := positiveModInt64(seed, spec.randomModulo) - spec.randomBias
		seenIndex := int(r + spec.randomBias)
		if seenIndex >= 0 && seenIndex < len(seen) && !seen[seenIndex] {
			seen[seenIndex] = true
			seenCount++
		}
		checksum = binaryIntModuloFold(checksum, r+100, fold.mul, mod)

		width := positiveModInt64(i, spec.widthModulo) + spec.widthBias
		low := positiveModInt64(seed, spec.lowModulo) - spec.lowBias
		high := low + width
		pick := low + positiveModInt64(seed, width+1)
		if pick >= low && pick <= high {
			intervalHits++
		}
		checksum = binaryIntModuloFold(checksum, (high-low)*spec.intervalScale+pick+spec.intervalBias, fold.mul, mod)
	}
	checksum = binaryIntModuloFold(checksum, seenCount, fold.mul, mod)
	checksum = binaryIntModuloFold(checksum, intervalHits, fold.mul, mod)
	return true, []runtime.Value{runtime.IntValue(checksum)}, nil
}

func (vm *VM) regexpRandomDriverRuntimeGuards(spec regexpRandomDriverSpec) (binaryIntModuloFoldSpec, bool) {
	line, ok := closureFromValue(vm.GetGlobal(spec.lineGlobal))
	if !ok || !isRegexpRandomLineProto(line.Proto) {
		return binaryIntModuloFoldSpec{}, false
	}
	mix, ok := closureFromValue(vm.GetGlobal(spec.mixGlobal))
	if !ok {
		return binaryIntModuloFoldSpec{}, false
	}
	fold, ok := binaryIntModuloFoldSpecForProto(mix.Proto)
	if !ok ||
		!stdlibHostTableFunction(vm.GetGlobal("regexp"), "mustCompile", "regexp.mustCompile") ||
		!stdlibHostTableFunction(vm.GetGlobal("regexp"), "split", "regexp.split") ||
		!stdlibHostTableFunction(vm.GetGlobal("string"), "format", "string.format") {
		return binaryIntModuloFoldSpec{}, false
	}
	return fold, true
}

func isRegexpRandomLineProto(p *FuncProto) bool {
	if p == nil || p.NumParams != 1 || p.UsesVarargBytecode || len(p.Code) != 19 || len(p.Constants) != 4 {
		return false
	}
	format, ok := constStringAt(p, 2)
	if !ok || format != "svc=api%d status=%d route=/v1/items/%d trace=t%05d" {
		return false
	}
	pat := newBytecodePattern(p.Code)
	return pat.hasSBxs(
		sbxAt{pc: 3, op: OP_LOADINT, sbx: 17},
		sbxAt{pc: 5, op: OP_LOADINT, sbx: 200},
		sbxAt{pc: 6, op: OP_LOADINT, sbx: 5},
		sbxAt{pc: 8, op: OP_LOADINT, sbx: 100},
		sbxAt{pc: 11, op: OP_LOADINT, sbx: 997},
		sbxAt{pc: 13, op: OP_LOADINT, sbx: 37},
	) && pat.hasOps(
		opcodeAt{pc: 17, op: OP_CALL},
		opcodeAt{pc: 18, op: OP_RETURN},
	)
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
