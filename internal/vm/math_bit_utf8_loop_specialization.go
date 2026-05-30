package vm

import (
	"math/bits"
	"strings"
	"unicode/utf8"

	"github.com/Never-Labs/gscript/internal/runtime"
)

type mathBitUTF8LoopSpec struct {
	inputsGlobal      string
	basesGlobal       string
	toIntGlobal       string
	textGlobal        string
	textOffsetsGlobal string
	textLenGlobal     string
	maskGlobal        string
	modGlobal         string
}

func isMathBitUTF8HotLoopProto(p *FuncProto) bool {
	_, ok := mathBitUTF8LoopSpecForProto(p)
	return ok
}

func mathBitUTF8LoopSpecForProto(p *FuncProto) (mathBitUTF8LoopSpec, bool) {
	var spec mathBitUTF8LoopSpec
	if p == nil || p.NumParams != 1 || p.IsVarArg || len(p.Code) != 207 {
		return spec, false
	}
	code := p.Code
	pat := newBytecodePattern(code)
	if !pat.hasASBxs(asbxAt{pc: 0, op: OP_LOADINT, a: 1, sbx: 0}) ||
		!pat.hasAs(
			aAt{pc: 1, op: OP_LOADK, a: 2},
			aAt{pc: 2, op: OP_GETGLOBAL, a: 4},
		) ||
		!pat.hasABs(abAt{pc: 3, op: OP_LEN, a: 3, b: 4}) ||
		!pat.hasOps(
			opcodeAt{pc: 7, op: OP_FORPREP},
			opcodeAt{pc: 204, op: OP_FORLOOP},
			opcodeAt{pc: 206, op: OP_RETURN},
		) ||
		!pat.hasABCs(
			abcAt{pc: 18, op: OP_CALL, a: 9, b: 3, c: 2},
			abcAt{pc: 45, op: OP_CALL, a: 12, b: 3, c: 2},
			abcAt{pc: 47, op: OP_CALL, a: 10, b: 2, c: 2},
			abcAt{pc: 54, op: OP_CALL, a: 11, b: 2, c: 2},
			abcAt{pc: 61, op: OP_CALL, a: 14, b: 3, c: 2},
			abcAt{pc: 63, op: OP_CALL, a: 12, b: 2, c: 2},
			abcAt{pc: 77, op: OP_CALL, a: 15, b: 3, c: 2},
			abcAt{pc: 83, op: OP_CALL, a: 16, b: 3, c: 2},
			abcAt{pc: 88, op: OP_CALL, a: 17, b: 3, c: 2},
			abcAt{pc: 102, op: OP_CALL, a: 18, b: 3, c: 2},
			abcAt{pc: 114, op: OP_CALL, a: 21, b: 4, c: 2},
			abcAt{pc: 122, op: OP_CALL, a: 22, b: 5, c: 2},
			abcAt{pc: 129, op: OP_CALL, a: 23, b: 2, c: 2},
			abcAt{pc: 138, op: OP_CALL, a: 24, b: 2, c: 2},
			abcAt{pc: 152, op: OP_CALL, a: 25, b: 3, c: 2},
			abcAt{pc: 157, op: OP_CALL, a: 27, b: 2, c: 4},
			abcAt{pc: 175, op: OP_CALL, a: 30, b: 3, c: 2},
			abcAt{pc: 188, op: OP_CALL, a: 33, b: 9, c: 2},
			abcAt{pc: 191, op: OP_CALL, a: 31, b: 3, c: 2},
		) ||
		!pat.hasACs(acAt{pc: 158, op: OP_TFORCALL, a: 27, c: 2}) {
		return spec, false
	}
	var ok bool
	if spec.inputsGlobal, ok = mathBitUTF8ConstStringAt(p, DecodeBx(code[2])); !ok {
		return spec, false
	}
	if spec.basesGlobal, ok = mathBitUTF8ConstStringAt(p, DecodeBx(code[15])); !ok {
		return spec, false
	}
	if spec.toIntGlobal, ok = mathBitUTF8ConstStringAt(p, DecodeBx(code[123])); !ok {
		return spec, false
	}
	if spec.textGlobal, ok = mathBitUTF8ConstStringAt(p, DecodeBx(code[156])); !ok {
		return spec, false
	}
	if spec.textOffsetsGlobal, ok = mathBitUTF8ConstStringAt(p, DecodeBx(code[168])); !ok {
		return spec, false
	}
	if spec.textLenGlobal, ok = mathBitUTF8ConstStringAt(p, DecodeBx(code[170])); !ok {
		return spec, false
	}
	if spec.maskGlobal, ok = mathBitUTF8ConstStringAt(p, DecodeBx(code[190])); !ok {
		return spec, false
	}
	if spec.modGlobal, ok = mathBitUTF8ConstStringAt(p, DecodeBx(code[193])); !ok {
		return spec, false
	}
	return spec, true
}

func (vm *VM) runMathBitUTF8HotLoopRuntimeSpecialization(cl *Closure, args []runtime.Value) (bool, []runtime.Value, error) {
	if len(args) != 1 || args[0].RawType() != runtime.TypeInt {
		return false, nil, nil
	}
	spec, ok := mathBitUTF8LoopSpecForProto(cl.Proto)
	if !ok || !vm.mathBitUTF8LoopRuntimeGuards(spec) {
		return false, nil, nil
	}
	n := args[0].RawInt()
	if n < 0 {
		return false, nil, nil
	}
	inputs, bases, ok := vm.mathBitUTF8Inputs(spec)
	if !ok || len(inputs) == 0 || len(inputs) != len(bases) {
		return false, nil, nil
	}
	textValue := vm.GetGlobal(spec.textGlobal)
	textLenValue := vm.GetGlobal(spec.textLenGlobal)
	maskValue := vm.GetGlobal(spec.maskGlobal)
	modValue := vm.GetGlobal(spec.modGlobal)
	offsetsValue := vm.GetGlobal(spec.textOffsetsGlobal)
	if !textValue.IsString() || textLenValue.RawType() != runtime.TypeInt ||
		maskValue.RawType() != runtime.TypeInt || modValue.RawType() != runtime.TypeInt ||
		!offsetsValue.IsTable() || modValue.RawInt() == 0 {
		return false, nil, nil
	}
	text := textValue.Str()
	textLen := textLenValue.RawInt()
	if textLen <= 0 {
		return false, nil, nil
	}
	offsets, ok := mathBitUTF8IntTable(offsetsValue.Table())
	if !ok || len(offsets) == 0 {
		return false, nil, nil
	}
	cpSum, ok := mathBitUTF8CodepointPosSum(text)
	if !ok {
		return false, nil, nil
	}

	mod := modValue.RawInt()
	mask := uint32(maskValue.RawInt())
	checksum := int64(0)
	rolling := uint32(305419896)
	inputCount := int64(len(inputs))

	for i := int64(1); i <= n; i++ {
		idx := int((i % inputCount) + 1)
		parsed, ok := mathBitUTF8ParseBase(inputs[idx-1], bases[idx-1])
		if !ok {
			parsed = 0
		}
		folded := (parsed*13 + i*17) % 1048573
		floored := (i * 97) / 11
		fmodded := (floored + folded) % 251
		modulo := (i*31 + fmodded) % 65521

		shift := int((i % 63) - 31)
		left := mathBitUTF8Lshift(uint32(folded), shift)
		right := mathBitUTF8Rshift(rolling, -shift)
		arith := mathBitUTF8Arshift(rolling, shift)
		rot := bits.RotateLeft32(rolling, shift&31) ^ bits.RotateLeft32(uint32(folded), -(shift&31))
		field := uint(i % 24)
		width := uint((i % 8) + 1)
		extracted := (rot >> field) & mathBitUTF8Mask(width)
		replaceField := uint(i % 16)
		replaced := mathBitUTF8Replace(rolling, extracted, replaceField, 4)

		xi := uint32(i % 256)
		yi := uint32((i * 3) % 1024)
		coerced := (xi | yi) ^ (replaced & 65535)

		offsetIndex := int(((i + cpSum) % textLen) + 1)
		if offsetIndex < 1 || offsetIndex > len(offsets) {
			return false, nil, nil
		}
		cpAt, ok := mathBitUTF8CodepointAt(text, offsets[offsetIndex-1])
		if !ok {
			return false, nil, nil
		}

		rolling = (replaced ^ coerced ^ uint32(cpSum) ^ uint32(cpAt) ^ left ^ right ^ arith ^ uint32(modulo)) & mask
		checksum = (checksum + (int64(rolling) % mod) + int64(extracted) + textLen + fmodded + modulo) % mod
	}
	return true, []runtime.Value{runtime.IntValue(checksum)}, nil
}

func (vm *VM) mathBitUTF8LoopRuntimeGuards(spec mathBitUTF8LoopSpec) bool {
	if !mathBitUTF8IsGoFunction(vm.GetGlobal("tonumber"), "tonumber") ||
		!mathBitUTF8IsGoFunction(vm.GetGlobal("tostring"), "tostring") ||
		!vm.standardUTF8CodesActive() {
		return false
	}
	toint, ok := closureFromValue(vm.GetGlobal(spec.toIntGlobal))
	if !ok || !mathBitUTF8ToIntProto(toint.Proto) {
		return false
	}
	return mathBitUTF8TableFunction(vm.GetGlobal("math"), "floor", "math.floor") &&
		mathBitUTF8TableFunction(vm.GetGlobal("math"), "fmod", "math.fmod") &&
		mathBitUTF8TableFunction(vm.GetGlobal("math"), "tointeger", "math.tointeger") &&
		mathBitUTF8TableFunction(vm.GetGlobal("bit32"), "lshift", "bit32.lshift") &&
		mathBitUTF8TableFunction(vm.GetGlobal("bit32"), "rshift", "bit32.rshift") &&
		mathBitUTF8TableFunction(vm.GetGlobal("bit32"), "arshift", "bit32.arshift") &&
		mathBitUTF8TableFunction(vm.GetGlobal("bit32"), "bxor", "bit32.bxor") &&
		mathBitUTF8TableFunction(vm.GetGlobal("bit32"), "lrotate", "bit32.lrotate") &&
		mathBitUTF8TableFunction(vm.GetGlobal("bit32"), "rrotate", "bit32.rrotate") &&
		mathBitUTF8TableFunction(vm.GetGlobal("bit32"), "extract", "bit32.extract") &&
		mathBitUTF8TableFunction(vm.GetGlobal("bit32"), "replace", "bit32.replace") &&
		mathBitUTF8TableFunction(vm.GetGlobal("bit32"), "bor", "bit32.bor") &&
		mathBitUTF8TableFunction(vm.GetGlobal("bit32"), "band", "bit32.band") &&
		mathBitUTF8TableFunction(vm.GetGlobal("utf8"), "codepoint", "utf8.codepoint")
}

func (vm *VM) mathBitUTF8Inputs(spec mathBitUTF8LoopSpec) ([]string, []int64, bool) {
	inputsValue := vm.GetGlobal(spec.inputsGlobal)
	basesValue := vm.GetGlobal(spec.basesGlobal)
	if !inputsValue.IsTable() || !basesValue.IsTable() {
		return nil, nil, false
	}
	inputsTable := inputsValue.Table()
	basesTable := basesValue.Table()
	n := inputsTable.Length()
	if n <= 0 || basesTable.Length() != n {
		return nil, nil, false
	}
	inputs := make([]string, n)
	bases := make([]int64, n)
	for i := 1; i <= n; i++ {
		s := inputsTable.RawGetInt(int64(i))
		b := basesTable.RawGetInt(int64(i))
		if !s.IsString() || b.RawType() != runtime.TypeInt {
			return nil, nil, false
		}
		inputs[i-1] = s.Str()
		bases[i-1] = b.RawInt()
	}
	return inputs, bases, true
}

func mathBitUTF8ToIntProto(p *FuncProto) bool {
	if p == nil || p.NumParams != 1 || p.IsVarArg || len(p.Code) != 30 {
		return false
	}
	pat := newBytecodePattern(p.Code)
	return pat.hasOps(
		opcodeAt{pc: 0, op: OP_GETGLOBAL},
		opcodeAt{pc: 2, op: OP_CALL},
		opcodeAt{pc: 19, op: OP_GETGLOBAL},
		opcodeAt{pc: 20, op: OP_GETFIELD},
		opcodeAt{pc: 22, op: OP_CALL},
		opcodeAt{pc: 29, op: OP_RETURN},
	)
}

func mathBitUTF8TableFunction(tableValue runtime.Value, field string, name string) bool {
	if !tableValue.IsTable() {
		return false
	}
	return mathBitUTF8IsGoFunction(tableValue.Table().RawGetString(field), name)
}

func mathBitUTF8IsGoFunction(v runtime.Value, name string) bool {
	gf := v.GoFunction()
	return gf != nil && gf.Name == name
}

func mathBitUTF8ConstStringAt(p *FuncProto, idx int) (string, bool) {
	if p == nil || idx < 0 || idx >= len(p.Constants) || !p.Constants[idx].IsString() {
		return "", false
	}
	return p.Constants[idx].Str(), true
}

func mathBitUTF8IntTable(tbl *runtime.Table) ([]int64, bool) {
	n := tbl.Length()
	out := make([]int64, n)
	for i := 1; i <= n; i++ {
		v := tbl.RawGetInt(int64(i))
		if v.RawType() != runtime.TypeInt {
			return nil, false
		}
		out[i-1] = v.RawInt()
	}
	return out, true
}

func mathBitUTF8ParseBase(s string, base int64) (int64, bool) {
	if base < 2 || base > 36 {
		return 0, false
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	sign := int64(1)
	switch s[0] {
	case '+':
		s = s[1:]
	case '-':
		sign = -1
		s = s[1:]
	}
	if s == "" {
		return 0, false
	}
	var acc int64
	for i := 0; i < len(s); i++ {
		d := mathBitUTF8DigitValue(s[i])
		if d < 0 || int64(d) >= base {
			return 0, false
		}
		acc = acc*base + int64(d)
	}
	return sign * acc, true
}

func mathBitUTF8DigitValue(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'z':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'Z':
		return int(c-'A') + 10
	default:
		return -1
	}
}

func mathBitUTF8CodepointPosSum(s string) (int64, bool) {
	var sum int64
	for pos := 0; pos < len(s); {
		r, size := utf8.DecodeRuneInString(s[pos:])
		if r == utf8.RuneError && size == 1 {
			return 0, false
		}
		sum += int64(r) + int64(pos+1)
		pos += size
	}
	return sum, true
}

func mathBitUTF8CodepointAt(s string, oneBasedByteOffset int64) (int64, bool) {
	i := oneBasedByteOffset
	if i < 0 {
		i = int64(len(s)) + i + 1
	}
	if i < 1 || i > int64(len(s)) {
		return 0, false
	}
	r, size := utf8.DecodeRuneInString(s[int(i)-1:])
	if r == utf8.RuneError && size == 1 {
		return 0, false
	}
	return int64(r), true
}

func mathBitUTF8Mask(width uint) uint32 {
	if width >= 32 {
		return ^uint32(0)
	}
	return uint32((uint64(1) << width) - 1)
}

func mathBitUTF8Lshift(n uint32, disp int) uint32 {
	if disp < 0 {
		return n >> uint(-disp)
	}
	if disp >= 32 {
		return 0
	}
	return n << uint(disp)
}

func mathBitUTF8Rshift(n uint32, disp int) uint32 {
	if disp < 0 {
		return n << uint(-disp)
	}
	if disp >= 32 {
		return 0
	}
	return n >> uint(disp)
}

func mathBitUTF8Arshift(n uint32, disp int) uint32 {
	signed := int32(n)
	if disp < 0 {
		return uint32(signed) << uint(-disp)
	}
	if disp >= 32 {
		if signed < 0 {
			return ^uint32(0)
		}
		return 0
	}
	return uint32(signed >> uint(disp))
}

func mathBitUTF8Replace(n uint32, v uint32, field uint, width uint) uint32 {
	mask := mathBitUTF8Mask(width)
	return (n &^ (mask << field)) | ((v & mask) << field)
}
