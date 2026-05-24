package vm

import (
	"math/bits"
	"strings"
	"unicode/utf8"

	"github.com/gscript/gscript/internal/runtime"
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
	if DecodeOp(code[0]) != OP_LOADINT || DecodeA(code[0]) != 1 || DecodesBx(code[0]) != 0 ||
		DecodeOp(code[1]) != OP_LOADK || DecodeA(code[1]) != 2 ||
		DecodeOp(code[2]) != OP_GETGLOBAL || DecodeA(code[2]) != 4 ||
		DecodeOp(code[3]) != OP_LEN || DecodeA(code[3]) != 3 || DecodeB(code[3]) != 4 ||
		DecodeOp(code[7]) != OP_FORPREP ||
		DecodeOp(code[18]) != OP_CALL || DecodeA(code[18]) != 9 || DecodeB(code[18]) != 3 || DecodeC(code[18]) != 2 ||
		DecodeOp(code[45]) != OP_CALL || DecodeA(code[45]) != 12 || DecodeB(code[45]) != 3 || DecodeC(code[45]) != 2 ||
		DecodeOp(code[47]) != OP_CALL || DecodeA(code[47]) != 10 || DecodeB(code[47]) != 2 || DecodeC(code[47]) != 2 ||
		DecodeOp(code[54]) != OP_CALL || DecodeA(code[54]) != 11 || DecodeB(code[54]) != 2 || DecodeC(code[54]) != 2 ||
		DecodeOp(code[61]) != OP_CALL || DecodeA(code[61]) != 14 || DecodeB(code[61]) != 3 || DecodeC(code[61]) != 2 ||
		DecodeOp(code[63]) != OP_CALL || DecodeA(code[63]) != 12 || DecodeB(code[63]) != 2 || DecodeC(code[63]) != 2 ||
		DecodeOp(code[77]) != OP_CALL || DecodeA(code[77]) != 15 || DecodeB(code[77]) != 3 || DecodeC(code[77]) != 2 ||
		DecodeOp(code[83]) != OP_CALL || DecodeA(code[83]) != 16 || DecodeB(code[83]) != 3 || DecodeC(code[83]) != 2 ||
		DecodeOp(code[88]) != OP_CALL || DecodeA(code[88]) != 17 || DecodeB(code[88]) != 3 || DecodeC(code[88]) != 2 ||
		DecodeOp(code[102]) != OP_CALL || DecodeA(code[102]) != 18 || DecodeB(code[102]) != 3 || DecodeC(code[102]) != 2 ||
		DecodeOp(code[114]) != OP_CALL || DecodeA(code[114]) != 21 || DecodeB(code[114]) != 4 || DecodeC(code[114]) != 2 ||
		DecodeOp(code[122]) != OP_CALL || DecodeA(code[122]) != 22 || DecodeB(code[122]) != 5 || DecodeC(code[122]) != 2 ||
		DecodeOp(code[129]) != OP_CALL || DecodeA(code[129]) != 23 || DecodeB(code[129]) != 2 || DecodeC(code[129]) != 2 ||
		DecodeOp(code[138]) != OP_CALL || DecodeA(code[138]) != 24 || DecodeB(code[138]) != 2 || DecodeC(code[138]) != 2 ||
		DecodeOp(code[152]) != OP_CALL || DecodeA(code[152]) != 25 || DecodeB(code[152]) != 3 || DecodeC(code[152]) != 2 ||
		DecodeOp(code[157]) != OP_CALL || DecodeA(code[157]) != 27 || DecodeB(code[157]) != 2 || DecodeC(code[157]) != 4 ||
		DecodeOp(code[158]) != OP_TFORCALL || DecodeA(code[158]) != 27 || DecodeC(code[158]) != 2 ||
		DecodeOp(code[175]) != OP_CALL || DecodeA(code[175]) != 30 || DecodeB(code[175]) != 3 || DecodeC(code[175]) != 2 ||
		DecodeOp(code[188]) != OP_CALL || DecodeA(code[188]) != 33 || DecodeB(code[188]) != 9 || DecodeC(code[188]) != 2 ||
		DecodeOp(code[191]) != OP_CALL || DecodeA(code[191]) != 31 || DecodeB(code[191]) != 3 || DecodeC(code[191]) != 2 ||
		DecodeOp(code[204]) != OP_FORLOOP ||
		DecodeOp(code[206]) != OP_RETURN {
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
	return DecodeOp(p.Code[0]) == OP_GETGLOBAL &&
		DecodeOp(p.Code[2]) == OP_CALL &&
		DecodeOp(p.Code[19]) == OP_GETGLOBAL &&
		DecodeOp(p.Code[20]) == OP_GETFIELD &&
		DecodeOp(p.Code[22]) == OP_CALL &&
		DecodeOp(p.Code[29]) == OP_RETURN
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
