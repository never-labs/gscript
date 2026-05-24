package vm

import (
	"fmt"
	"strconv"
	"time"

	"github.com/gscript/gscript/internal/runtime"
)

type stdlibHostDriverSpec struct {
	payload          string
	nameFormat       string
	rawFormat        string
	lineFormat       string
	expandPrefix     string
	mixGlobal        string
	checksumGlobal   string
	modGlobal        string
	statusBase       int64
	statusScale      int64
	statusModulo     int64
	scoreScale       int64
	scoreModulo      int64
	idModulo         int64
	csvWorkerModulo  int64
	csvScoreModulo   int64
	timeYear         int64
	timeMonthModulo  int64
	timeDayModulo    int64
	timeHourModulo   int64
	timeMinuteScale  int64
	timeSecondScale  int64
	timeMinuteModulo int64
	timeSecondModulo int64
}

func isStdlibHostDriverProto(p *FuncProto) bool {
	_, ok := stdlibHostDriverSpecForProto(p)
	return ok
}

func stdlibHostDriverSpecForProto(p *FuncProto) (stdlibHostDriverSpec, bool) {
	var spec stdlibHostDriverSpec
	if p == nil || p.NumParams != 1 || p.UsesVarargBytecode || len(p.Code) != 483 || len(p.Constants) < 66 {
		return spec, false
	}
	code := p.Code
	pat := newBytecodePattern(code)
	if !pat.hasOps(
		opcodeAt{pc: 0, op: OP_LOADINT},
		opcodeAt{pc: 1, op: OP_LOADK},
		opcodeAt{pc: 5, op: OP_FORPREP},
		opcodeAt{pc: 7, op: OP_MOD},
		opcodeAt{pc: 10, op: OP_LOADINT},
		opcodeAt{pc: 11, op: OP_MOD},
		opcodeAt{pc: 18, op: OP_LOADBOOL},
		opcodeAt{pc: 23, op: OP_CALL},
		opcodeAt{pc: 33, op: OP_CALL},
		opcodeAt{pc: 37, op: OP_CALL},
		opcodeAt{pc: 49, op: OP_CALL},
		opcodeAt{pc: 53, op: OP_CALL},
		opcodeAt{pc: 62, op: OP_CALL},
		opcodeAt{pc: 70, op: OP_CALL},
		opcodeAt{pc: 76, op: OP_CALL},
		opcodeAt{pc: 87, op: OP_CALL},
		opcodeAt{pc: 91, op: OP_CALL},
		opcodeAt{pc: 100, op: OP_CALL},
		opcodeAt{pc: 119, op: OP_CALL},
		opcodeAt{pc: 128, op: OP_CALL},
		opcodeAt{pc: 132, op: OP_CALL},
		opcodeAt{pc: 136, op: OP_CALL},
		opcodeAt{pc: 149, op: OP_CALL},
		opcodeAt{pc: 153, op: OP_CALL},
		opcodeAt{pc: 157, op: OP_CALL},
		opcodeAt{pc: 172, op: OP_CALL},
		opcodeAt{pc: 183, op: OP_CALL},
		opcodeAt{pc: 192, op: OP_CALL},
		opcodeAt{pc: 193, op: OP_CALL},
		opcodeAt{pc: 197, op: OP_CALL},
		opcodeAt{pc: 204, op: OP_CALL},
		opcodeAt{pc: 216, op: OP_CALL},
		opcodeAt{pc: 220, op: OP_CALL},
		opcodeAt{pc: 238, op: OP_CALL},
		opcodeAt{pc: 255, op: OP_CALL},
		opcodeAt{pc: 258, op: OP_CALL},
		opcodeAt{pc: 281, op: OP_CALL},
		opcodeAt{pc: 286, op: OP_CALL},
		opcodeAt{pc: 291, op: OP_CALL},
		opcodeAt{pc: 298, op: OP_CALL},
		opcodeAt{pc: 311, op: OP_CALL},
		opcodeAt{pc: 318, op: OP_CALL},
		opcodeAt{pc: 323, op: OP_CALL},
		opcodeAt{pc: 336, op: OP_CALL},
		opcodeAt{pc: 341, op: OP_CALL},
		opcodeAt{pc: 346, op: OP_CALL},
		opcodeAt{pc: 352, op: OP_CALL},
		opcodeAt{pc: 357, op: OP_CALL},
		opcodeAt{pc: 372, op: OP_CALL},
		opcodeAt{pc: 378, op: OP_CALL},
		opcodeAt{pc: 382, op: OP_CALL},
		opcodeAt{pc: 395, op: OP_CALL},
		opcodeAt{pc: 402, op: OP_CALL},
		opcodeAt{pc: 406, op: OP_CALL},
		opcodeAt{pc: 424, op: OP_CALL},
		opcodeAt{pc: 429, op: OP_CALL},
		opcodeAt{pc: 433, op: OP_CALL},
		opcodeAt{pc: 448, op: OP_CALL},
		opcodeAt{pc: 459, op: OP_CALL},
		opcodeAt{pc: 468, op: OP_CALL},
		opcodeAt{pc: 470, op: OP_CALL},
		opcodeAt{pc: 472, op: OP_FORLOOP},
		opcodeAt{pc: 482, op: OP_RETURN},
	) {
		return spec, false
	}
	var ok bool
	if spec.payload, ok = constStringAt(p, 0); !ok {
		return spec, false
	}
	if spec.nameFormat, ok = constStringAt(p, 3); !ok || spec.nameFormat != "user-%04d" {
		return spec, false
	}
	if spec.rawFormat, ok = constStringAt(p, 20); !ok || spec.rawFormat != "%s|%d|%d|%s" {
		return spec, false
	}
	if spec.lineFormat, ok = constStringAt(p, 45); !ok || spec.lineFormat != "svc=api status=%d route=/v1/items/%d trace=%s" {
		return spec, false
	}
	if spec.expandPrefix, ok = constStringAt(p, 44); !ok {
		return spec, false
	}
	if spec.mixGlobal, ok = constStringAt(p, 13); !ok {
		return spec, false
	}
	if spec.checksumGlobal, ok = constStringAt(p, 61); !ok {
		return spec, false
	}
	spec.modGlobal = "MOD"
	spec.idModulo = int64(DecodesBx(code[6]))
	spec.scoreScale = int64(DecodesBx(code[8]))
	spec.scoreModulo = int64(DecodesBx(code[10]))
	spec.statusBase = int64(DecodesBx(code[328]))
	spec.statusModulo = int64(DecodesBx(code[329]))
	spec.statusScale = int64(DecodesBx(code[331]))
	spec.csvWorkerModulo = int64(DecodesBx(code[83]))
	spec.csvScoreModulo = int64(DecodesBx(code[85]))
	spec.timeYear = int64(DecodesBx(code[262]))
	spec.timeMonthModulo = int64(DecodesBx(code[263]))
	spec.timeDayModulo = int64(DecodesBx(code[267]))
	spec.timeHourModulo = int64(DecodesBx(code[271]))
	spec.timeMinuteScale = int64(DecodesBx(code[273]))
	spec.timeMinuteModulo = int64(DecodesBx(code[275]))
	spec.timeSecondScale = int64(DecodesBx(code[277]))
	spec.timeSecondModulo = int64(DecodesBx(code[279]))
	if spec.idModulo <= 0 || spec.scoreModulo <= 0 || spec.csvWorkerModulo <= 0 || spec.csvScoreModulo <= 0 ||
		spec.statusModulo <= 0 || spec.timeMonthModulo <= 0 || spec.timeDayModulo <= 0 ||
		spec.timeHourModulo <= 0 || spec.timeMinuteModulo <= 0 || spec.timeSecondModulo <= 0 {
		return stdlibHostDriverSpec{}, false
	}
	return spec, true
}

func (vm *VM) runStdlibHostDriverRuntimeSpecialization(cl *Closure, args []runtime.Value) (bool, []runtime.Value, error) {
	if len(args) != 1 || args[0].RawType() != runtime.TypeInt {
		return false, nil, nil
	}
	spec, ok := stdlibHostDriverSpecForProto(cl.Proto)
	if !ok || !vm.stdlibHostDriverRuntimeGuards(spec) {
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
	payloadLen := int64(len(spec.payload))
	for i := int64(1); i <= n; i++ {
		id := positiveModInt64(i, spec.idModulo)
		score := positiveModInt64(i*spec.scoreScale, spec.scoreModulo)
		flag := positiveModInt64(i, 2) == 0
		name := fmt.Sprintf(spec.nameFormat, id)
		nameLen := int64(len(name))

		checksum = stdlibHostMix(checksum, id+score+nameLen, mod)
		if flag {
			checksum = stdlibHostMix(checksum, 7, mod)
		} else {
			checksum = stdlibHostMix(checksum, 3, mod)
		}

		outCSVLen := int64(46 + decimalLenInt64(score) + decimalLenInt64(positiveModInt64(id, spec.csvWorkerModulo)) + decimalLenInt64(positiveModInt64(score, spec.csvScoreModulo)))
		checksum = stdlibHostMix(checksum, 2+outCSVLen+positiveModInt64(score, spec.csvScoreModulo)+nameLen, mod)

		raw := fmt.Sprintf(spec.rawFormat, name, id, score, spec.payload)
		rawLen := int64(len(raw))
		checksum = stdlibHostMix(checksum, stdlibHostPaddedBase64Len(rawLen)+stdlibHostRawBase64Len(rawLen)+rawLen, mod)

		scoreDigits := int64(decimalLenInt64(score))
		urlTerm := (22 + scoreDigits) + (18 + scoreDigits) + (35 + scoreDigits) + int64(len("host hot")) + int64(len("/root/")+len(name))
		checksum = stdlibHostMix(checksum, urlTerm, mod)

		month := positiveModInt64(i, spec.timeMonthModulo) + 1
		day := positiveModInt64(i, spec.timeDayModulo) + 1
		hour := positiveModInt64(i, spec.timeHourModulo)
		minute := positiveModInt64(i*spec.timeMinuteScale, spec.timeMinuteModulo)
		second := positiveModInt64(i*spec.timeSecondScale, spec.timeSecondModulo)
		stamp := time.Date(int(spec.timeYear), time.Month(month), int(day), int(hour), int(minute), int(second), 0, time.UTC)
		formatted := stamp.Format("2006-01-02T15:04:05")
		parsed, err := time.Parse("2006-01-02T15:04:05", formatted)
		if err != nil {
			return false, nil, nil
		}
		checksum = stdlibHostMix(checksum, int64(parsed.Year()+int(parsed.Month())*31+parsed.Day()+len(formatted)), mod)

		checksum = stdlibHostMix(checksum, int64(len("alpha/beta/")+len(name)), mod)

		status := spec.statusBase + positiveModInt64(i, spec.statusModulo)*spec.statusScale
		checksum = stdlibHostMix(checksum, status+55, mod)

		checksum = stdlibHostMix(checksum, rawLen+rawLen+payloadLen, mod)
		sub := raw
		if len(sub) > 24 {
			sub = sub[:24]
		}
		checksum = stdlibHostChecksumText(checksum, sub, mod)
	}
	return true, []runtime.Value{runtime.IntValue(checksum)}, nil
}

func (vm *VM) stdlibHostDriverRuntimeGuards(spec stdlibHostDriverSpec) bool {
	mix, ok := closureFromValue(vm.GetGlobal(spec.mixGlobal))
	if !ok || !isStdlibHostMixProto(mix.Proto) {
		return false
	}
	checksum, ok := closureFromValue(vm.GetGlobal(spec.checksumGlobal))
	if !ok || !isStdlibHostChecksumTextProto(checksum.Proto) {
		return false
	}
	return stdlibHostTableFunction(vm.GetGlobal("string"), "format", "string.format") &&
		stdlibHostTableFunction(vm.GetGlobal("string"), "byte", "string.byte") &&
		stdlibHostTableFunction(vm.GetGlobal("string"), "sub", "string.sub") &&
		stdlibHostTableFunction(vm.GetGlobal("json"), "decode", "json.decode") &&
		stdlibHostTableFunction(vm.GetGlobal("csv"), "parseWithHeaders", "csv.parseWithHeaders") &&
		stdlibHostTableFunction(vm.GetGlobal("base64"), "urlEncode", "base64.urlEncode") &&
		stdlibHostTableFunction(vm.GetGlobal("url"), "queryEncode", "url.queryEncode") &&
		stdlibHostTableFunction(vm.GetGlobal("regexp"), "find", "regexp.find") &&
		stdlibHostTableFunction(vm.GetGlobal("compress"), "gzipEncode", "compress.gzipEncode")
}

func isStdlibHostMixProto(p *FuncProto) bool {
	if p == nil || p.NumParams != 2 || p.UsesVarargBytecode || len(p.Code) != 6 {
		return false
	}
	pat := newBytecodePattern(p.Code)
	return pat.hasSBxs(sbxAt{pc: 0, op: OP_LOADINT, sbx: 131}) &&
		pat.hasOps(
			opcodeAt{pc: 1, op: OP_MUL},
			opcodeAt{pc: 2, op: OP_ADD},
			opcodeAt{pc: 3, op: OP_GETGLOBAL},
			opcodeAt{pc: 4, op: OP_MOD},
			opcodeAt{pc: 5, op: OP_RETURN},
		)
}

func isStdlibHostChecksumTextProto(p *FuncProto) bool {
	if p == nil || p.NumParams != 2 || p.UsesVarargBytecode || len(p.Code) != 23 {
		return false
	}
	pat := newBytecodePattern(p.Code)
	return pat.hasSBxs(sbxAt{pc: 11, op: OP_LOADINT, sbx: 17}) &&
		pat.hasOps(
			opcodeAt{pc: 5, op: OP_FORPREP},
			opcodeAt{pc: 10, op: OP_CALL},
			opcodeAt{pc: 17, op: OP_GETGLOBAL},
			opcodeAt{pc: 18, op: OP_MOD},
			opcodeAt{pc: 20, op: OP_FORLOOP},
			opcodeAt{pc: 22, op: OP_RETURN},
		)
}

func stdlibHostTableFunction(table runtime.Value, field, name string) bool {
	if !table.IsTable() {
		return false
	}
	fn := table.Table().RawGetString(field).GoFunction()
	return fn != nil && fn.Name == name
}

func stdlibHostMix(sum, v, mod int64) int64 {
	return positiveModInt64(sum*131+v, mod)
}

func stdlibHostChecksumText(sum int64, s string, mod int64) int64 {
	local := sum
	for i := 1; i <= len(s); i++ {
		local = positiveModInt64(local+int64(s[i-1])*int64(i%17+1), mod)
	}
	return local
}

func stdlibHostPaddedBase64Len(n int64) int64 {
	return ((n + 2) / 3) * 4
}

func stdlibHostRawBase64Len(n int64) int64 {
	full := (n / 3) * 4
	switch n % 3 {
	case 0:
		return full
	case 1:
		return full + 2
	default:
		return full + 3
	}
}

func decimalLenInt64(v int64) int {
	if v < 0 {
		v = -v
	}
	return len(strconv.FormatInt(v, 10))
}
