package runtime

import (
	"fmt"
	stdlibstring "github.com/never-labs/gscript/internal/stringlib"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"unsafe"
)

// native_string_format.go holds runtime-owned native string.format helpers:
// fast-arity value entry points, the simple-format compiled-program cache, and
// the numeric coercion helpers used by VM/JIT paths.

func stringFormatValue(args []Value) (Value, error) {
	if len(args) < 1 || !args[0].IsString() {
		return NilValue(), fmt.Errorf("bad argument #1 to 'string.format' (string expected)")
	}
	formatStr := args[0].Str()
	if prog, ok, err := cachedSimpleFormat(formatStr); err != nil {
		return NilValue(), err
	} else if ok {
		RecordRuntimePathStringFormatFast()
		return prog.formatValue(args)
	}
	RecordRuntimePathStringFormatFallback()
	argIdx := 1

	var buf strings.Builder
	i := 0
	for i < len(formatStr) {
		if formatStr[i] != '%' {
			buf.WriteByte(formatStr[i])
			i++
			continue
		}
		i++ // skip %
		if i >= len(formatStr) {
			return NilValue(), fmt.Errorf("invalid format string (ends with %%)")
		}

		if formatStr[i] == '%' {
			buf.WriteByte('%')
			i++
			continue
		}

		token, ok := stdlibstring.ScanToken(formatStr, i-1)
		if !ok {
			return NilValue(), fmt.Errorf("invalid format string")
		}
		spec := token.Verb
		i = token.End
		fmtSpec := token.Spec

		if argIdx >= len(args) {
			return NilValue(), fmt.Errorf("bad argument #%d to 'string.format' (no value)", argIdx+1)
		}
		arg := args[argIdx]
		argIdx++

		switch spec {
		case 'd', 'i', 'u':
			n := toInt(arg)
			if !stdlibstring.WriteFastIntegerFormat(&buf, fmtSpec, spec, n) {
				goFmt := strings.Replace(fmtSpec, string(spec), "d", 1)
				buf.WriteString(fmt.Sprintf(goFmt, n))
			}
		case 'f', 'e', 'E', 'g', 'G':
			buf.WriteString(fmt.Sprintf(fmtSpec, toFloat(arg)))
		case 'x':
			n := toInt(arg)
			if !stdlibstring.WriteFastIntegerFormat(&buf, fmtSpec, spec, n) {
				goFmt := strings.Replace(fmtSpec, "x", "x", 1)
				buf.WriteString(fmt.Sprintf(goFmt, n))
			}
		case 'X':
			n := toInt(arg)
			if !stdlibstring.WriteFastIntegerFormat(&buf, fmtSpec, spec, n) {
				goFmt := strings.Replace(fmtSpec, "X", "X", 1)
				buf.WriteString(fmt.Sprintf(goFmt, n))
			}
		case 'o':
			n := toInt(arg)
			if !stdlibstring.WriteFastIntegerFormat(&buf, fmtSpec, spec, n) {
				goFmt := strings.Replace(fmtSpec, "o", "o", 1)
				buf.WriteString(fmt.Sprintf(goFmt, n))
			}
		case 'c':
			buf.WriteString(fmt.Sprintf(fmtSpec, rune(toInt(arg))))
		case 's':
			s := arg.String()
			if fmtSpec == "%s" {
				buf.WriteString(s)
			} else {
				goFmt := strings.Replace(fmtSpec, "s", "s", 1)
				buf.WriteString(fmt.Sprintf(goFmt, s))
			}
		case 'q':
			q, err := luaQuoteLiteral(arg)
			if err != nil {
				return NilValue(), err
			}
			buf.WriteString(q)
		case 'p':
			ptr := luaPointerString(arg)
			goFmt := strings.Replace(fmtSpec, "p", "s", 1)
			buf.WriteString(fmt.Sprintf(goFmt, ptr))
		default:
			return NilValue(), fmt.Errorf("invalid format specifier '%%%c'", spec)
		}
	}
	return StringValue(buf.String()), nil
}

func luaPointerString(v Value) string {
	switch v.Type() {
	case TypeNil, TypeBool, TypeInt, TypeFloat:
		return "(null)"
	default:
		return fmt.Sprintf("0x%x", v.Raw())
	}
}

func luaQuoteLiteral(v Value) (string, error) {
	switch v.Type() {
	case TypeNil:
		return stdlibstring.LuaQuoteNil(), nil
	case TypeBool:
		return stdlibstring.LuaQuoteBool(v.Bool()), nil
	case TypeInt:
		return stdlibstring.LuaQuoteInt(v.Int()), nil
	case TypeFloat:
		return stdlibstring.LuaQuoteFloat(v.Float()), nil
	case TypeString:
		return stdlibstring.LuaQuoteString(v.Str()), nil
	default:
		return "", fmt.Errorf("bad argument to 'string.format' (value has no literal form)")
	}
}

// StringFormatValue applies the runtime-native string.format implementation to
// a pre-built argument slice. It is used by JIT op-exit paths after guarding the
// callee identity.
func StringFormatValue(args []Value) (Value, error) {
	return stringFormatValue(args)
}

func stringFormat2Value(format, arg Value) (Value, error) {
	if !format.IsString() {
		return NilValue(), fmt.Errorf("bad argument #1 to 'string.format' (string expected)")
	}
	formatStr := format.Str()
	if prog, ok, err := cachedSimpleFormat(formatStr); err != nil {
		return NilValue(), err
	} else if ok && prog.singleInt {
		RecordRuntimePathStringFormatFast()
		n := toInt(arg)
		if v, ok := prog.cachedResult(n); ok {
			return v, nil
		}
		s := prog.formatSingleInt(n)
		v := StringValue(s)
		prog.storeCachedResult(n, v)
		return v, nil
	}
	args := [2]Value{format, arg}
	return stringFormatValue(args[:])
}

func stringFormat3Value(format, arg0, arg1 Value) (Value, error) {
	if !format.IsString() {
		return NilValue(), fmt.Errorf("bad argument #1 to 'string.format' (string expected)")
	}
	formatStr := format.Str()
	if prog, ok, err := cachedSimpleFormat(formatStr); err != nil {
		return NilValue(), err
	} else if ok && prog.minArgs == 3 {
		RecordRuntimePathStringFormatFast()
		s, err := prog.formatTwoArgs(arg0, arg1)
		if err != nil {
			return NilValue(), err
		}
		return StringValue(s), nil
	}
	RecordRuntimePathStringFormatFallback()
	args := [3]Value{format, arg0, arg1}
	return stringFormatValue(args[:])
}

func stringFormat4Value(format, arg0, arg1, arg2 Value) (Value, error) {
	if !format.IsString() {
		return NilValue(), fmt.Errorf("bad argument #1 to 'string.format' (string expected)")
	}
	formatStr := format.Str()
	if prog, ok, err := cachedSimpleFormat(formatStr); err != nil {
		return NilValue(), err
	} else if ok && prog.minArgs == 4 {
		RecordRuntimePathStringFormatFast()
		args := [3]Value{arg0, arg1, arg2}
		s, err := prog.formatFixedArgs(args[:])
		if err != nil {
			return NilValue(), err
		}
		return StringValue(s), nil
	}
	RecordRuntimePathStringFormatFallback()
	args := [4]Value{format, arg0, arg1, arg2}
	return stringFormatValue(args[:])
}

func stringFormat5Value(format, arg0, arg1, arg2, arg3 Value) (Value, error) {
	if !format.IsString() {
		return NilValue(), fmt.Errorf("bad argument #1 to 'string.format' (string expected)")
	}
	formatStr := format.Str()
	if prog, ok, err := cachedSimpleFormat(formatStr); err != nil {
		return NilValue(), err
	} else if ok && prog.minArgs == 5 {
		RecordRuntimePathStringFormatFast()
		args := [4]Value{arg0, arg1, arg2, arg3}
		s, err := prog.formatFixedArgs(args[:])
		if err != nil {
			return NilValue(), err
		}
		return StringValue(s), nil
	}
	RecordRuntimePathStringFormatFallback()
	args := [5]Value{format, arg0, arg1, arg2, arg3}
	return stringFormatValue(args[:])
}

// IsStdStringFormatFunction reports whether v is the runtime-native
// string.format GoFunction installed by BuildStringLibWithCaller. The legacy
// StdString name is retained for VM/JIT API compatibility.
func IsStdStringFormatFunction(v Value) bool {
	gf := v.GoFunction()
	return gf != nil &&
		gf.NativeKind == NativeKindStdStringFormat &&
		gf.NativeData == StdStringFormatIdentityPtr() &&
		gf.FastArg2 != nil
}

// StringFormatSingleInt formats a cached simple one-integer pattern. It is the
// semantic helper for JIT specializations of string.format(pattern, int).
func StringFormatSingleInt(pattern string, n int64) (Value, bool, error) {
	prog, ok, err := cachedSimpleFormat(pattern)
	if err != nil || !ok || !prog.singleInt {
		return NilValue(), false, err
	}
	if v, ok := prog.cachedResult(n); ok {
		return v, true, nil
	}
	v := StringValue(prog.formatSingleInt(n))
	prog.storeCachedResult(n, v)
	return v, true, nil
}

type simpleFormatProgram struct {
	formatStr string
	parts     []stdlibstring.Part
	minArgs   int
	litBytes  int
	singleInt bool

	resultMu       sync.Mutex
	resultCache    map[int64]Value
	resultOrder    []int64
	resultEvict    int
	fastResultTags [64]atomic.Uint64
	fastResultVals [64]atomic.Uint64
}

const simpleFormatCacheLimit = 64
const simpleFormatResultCacheLimit = 8192
const simpleFormatFastCacheSize = 32
const simpleFormatResultTagSalt = 0x9e3779b97f4a7c15

var simpleFormatCache = struct {
	sync.Mutex
	entries map[string]*simpleFormatProgram
	order   []string
}{
	entries: make(map[string]*simpleFormatProgram),
}

var simpleFormatFastCache [simpleFormatFastCacheSize]atomic.Pointer[simpleFormatProgram]
var simpleFormatPtrFastCache [simpleFormatFastCacheSize]atomic.Pointer[simpleFormatProgram]

func cachedSimpleFormat(formatStr string) (*simpleFormatProgram, bool, error) {
	if prog := lookupSimpleFormatFast(formatStr); prog != nil {
		return prog, true, nil
	}

	simpleFormatCache.Lock()
	if prog, ok := simpleFormatCache.entries[formatStr]; ok {
		simpleFormatCache.Unlock()
		storeSimpleFormatFast(formatStr, prog)
		return prog, true, nil
	}
	simpleFormatCache.Unlock()

	prog, ok, err := compileSimpleFormat(formatStr)
	if err != nil {
		return nil, false, err
	}
	if ok {
		simpleFormatCache.Lock()
		if cached, exists := simpleFormatCache.entries[formatStr]; exists {
			simpleFormatCache.Unlock()
			storeSimpleFormatFast(formatStr, cached)
			return cached, true, nil
		}
		if len(simpleFormatCache.entries) >= simpleFormatCacheLimit && len(simpleFormatCache.order) > 0 {
			delete(simpleFormatCache.entries, simpleFormatCache.order[0])
			copy(simpleFormatCache.order, simpleFormatCache.order[1:])
			simpleFormatCache.order = simpleFormatCache.order[:len(simpleFormatCache.order)-1]
		}
		simpleFormatCache.entries[formatStr] = prog
		simpleFormatCache.order = append(simpleFormatCache.order, formatStr)
		simpleFormatCache.Unlock()
		storeSimpleFormatFast(formatStr, prog)
		return prog, true, nil
	}
	return nil, false, nil
}

func lookupSimpleFormatFast(formatStr string) *simpleFormatProgram {
	ptrSlot := simpleFormatPtrFastSlot(formatStr)
	prog := simpleFormatPtrFastCache[ptrSlot].Load()
	if prog != nil && prog.formatStr == formatStr {
		return prog
	}
	slot := simpleFormatFastSlot(formatStr)
	prog = simpleFormatFastCache[slot].Load()
	if prog != nil && prog.formatStr == formatStr {
		simpleFormatPtrFastCache[ptrSlot].Store(prog)
		return prog
	}
	return nil
}

func storeSimpleFormatFast(formatStr string, prog *simpleFormatProgram) {
	if prog == nil {
		return
	}
	simpleFormatFastCache[simpleFormatFastSlot(formatStr)].Store(prog)
	simpleFormatPtrFastCache[simpleFormatPtrFastSlot(formatStr)].Store(prog)
}

func simpleFormatPtrFastSlot(s string) uint64 {
	return uint64((stringDataPtr(s)>>4)^uintptr(len(s))) & (simpleFormatFastCacheSize - 1)
}

func simpleFormatFastSlot(s string) uint64 {
	var h uint64 = 1469598103934665603
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= 1099511628211
	}
	return h & (simpleFormatFastCacheSize - 1)
}

func compileSimpleFormat(formatStr string) (*simpleFormatProgram, bool, error) {
	prog, status := stdlibstring.CompileSimple(formatStr)
	switch status {
	case stdlibstring.CompileOK:
		return &simpleFormatProgram{
			formatStr: formatStr,
			parts:     prog.Parts,
			minArgs:   prog.ArgCount + 1,
			litBytes:  prog.LitBytes,
			singleInt: prog.SingleInt,
		}, true, nil
	case stdlibstring.CompileErrEndsWithPercent:
		return nil, false, fmt.Errorf("invalid format string (ends with %%)")
	case stdlibstring.CompileErrInvalid:
		return nil, false, fmt.Errorf("invalid format string")
	default:
		return nil, false, nil
	}
}

func (p *simpleFormatProgram) formatValue(args []Value) (Value, error) {
	if p.singleInt {
		if len(args) < p.minArgs {
			return NilValue(), fmt.Errorf("bad argument #%d to 'string.format' (no value)", len(args)+1)
		}
		n := toInt(args[1])
		if v, ok := p.cachedResult(n); ok {
			return v, nil
		}
		s, err := p.format(args)
		if err != nil {
			return NilValue(), err
		}
		v := StringValue(s)
		p.storeCachedResult(n, v)
		return v, nil
	}
	s, err := p.format(args)
	if err != nil {
		return NilValue(), err
	}
	return StringValue(s), nil
}

func (p *simpleFormatProgram) cachedResult(n int64) (Value, bool) {
	if v, ok := p.cachedResultFast(n); ok {
		return v, true
	}
	p.resultMu.Lock()
	defer p.resultMu.Unlock()
	if p.resultCache == nil {
		return NilValue(), false
	}
	v, ok := p.resultCache[n]
	if ok {
		p.storeCachedResultFast(n, v)
	}
	return v, ok
}

func (p *simpleFormatProgram) storeCachedResult(n int64, v Value) {
	p.resultMu.Lock()
	defer p.resultMu.Unlock()
	if p.resultCache == nil {
		p.resultCache = make(map[int64]Value, 64)
	}
	if _, exists := p.resultCache[n]; exists {
		p.resultCache[n] = v
		return
	}
	if len(p.resultCache) >= simpleFormatResultCacheLimit && len(p.resultOrder) > 0 {
		delete(p.resultCache, p.resultOrder[p.resultEvict])
		p.resultOrder[p.resultEvict] = n
		p.resultEvict++
		if p.resultEvict == len(p.resultOrder) {
			p.resultEvict = 0
		}
	} else {
		p.resultOrder = append(p.resultOrder, n)
	}
	p.resultCache[n] = v
	p.storeCachedResultFast(n, v)
}

func (p *simpleFormatProgram) cachedResultFast(n int64) (Value, bool) {
	slot := simpleFormatResultSlot(n)
	want := simpleFormatResultTag(n)
	if p.fastResultTags[slot].Load() != want {
		return NilValue(), false
	}
	bits := p.fastResultVals[slot].Load()
	if bits == 0 || p.fastResultTags[slot].Load() != want {
		return NilValue(), false
	}
	return Value(bits), true
}

func (p *simpleFormatProgram) storeCachedResultFast(n int64, v Value) {
	slot := simpleFormatResultSlot(n)
	tag := simpleFormatResultTag(n)
	p.fastResultTags[slot].Store(0)
	p.fastResultVals[slot].Store(uint64(v))
	p.fastResultTags[slot].Store(tag)
}

func simpleFormatResultSlot(n int64) uint64 {
	x := uint64(n) ^ simpleFormatResultTagSalt
	x ^= x >> 33
	x *= 0xff51afd7ed558ccd
	x ^= x >> 33
	return x & 63
}

func simpleFormatResultTag(n int64) uint64 {
	tag := uint64(n) ^ simpleFormatResultTagSalt
	if tag == 0 {
		return 1
	}
	return tag
}

func (p *simpleFormatProgram) format(args []Value) (string, error) {
	if len(args) < p.minArgs {
		return "", fmt.Errorf("bad argument #%d to 'string.format' (no value)", len(args)+1)
	}
	var buf strings.Builder
	buf.Grow(p.litBytes + 16*(p.minArgs-1))
	argIdx := 1
	for _, part := range p.parts {
		if part.Verb == 0 {
			buf.WriteString(part.Lit)
			continue
		}
		arg := args[argIdx]
		argIdx++
		switch part.Verb {
		case 'd', 'i', 'u', 'x', 'X', 'o':
			writeCompiledIntegerFormat(&buf, part, toInt(arg))
		case 'f':
			writeCompiledFloatFormat(&buf, part, toFloat(arg))
		case 's':
			buf.WriteString(arg.String())
		}
	}
	return buf.String(), nil
}

func (p *simpleFormatProgram) formatTwoArgs(arg0, arg1 Value) (string, error) {
	if p.minArgs > 3 {
		return "", fmt.Errorf("bad argument #3 to 'string.format' (no value)")
	}
	var buf strings.Builder
	buf.Grow(p.litBytes + 32)
	argIdx := 0
	for _, part := range p.parts {
		if part.Verb == 0 {
			buf.WriteString(part.Lit)
			continue
		}
		var arg Value
		switch argIdx {
		case 0:
			arg = arg0
		case 1:
			arg = arg1
		default:
			return "", fmt.Errorf("bad argument #%d to 'string.format' (no value)", argIdx+2)
		}
		argIdx++
		switch part.Verb {
		case 'd', 'i', 'u', 'x', 'X', 'o':
			writeCompiledIntegerFormat(&buf, part, toInt(arg))
		case 'f':
			writeCompiledFloatFormat(&buf, part, toFloat(arg))
		case 's':
			buf.WriteString(arg.String())
		}
	}
	return buf.String(), nil
}

func (p *simpleFormatProgram) formatFixedArgs(args []Value) (string, error) {
	if len(args)+1 < p.minArgs {
		return "", fmt.Errorf("bad argument #%d to 'string.format' (no value)", len(args)+2)
	}
	var buf strings.Builder
	buf.Grow(p.litBytes + 16*len(args))
	argIdx := 0
	for _, part := range p.parts {
		if part.Verb == 0 {
			buf.WriteString(part.Lit)
			continue
		}
		if argIdx >= len(args) {
			return "", fmt.Errorf("bad argument #%d to 'string.format' (no value)", argIdx+2)
		}
		arg := args[argIdx]
		argIdx++
		switch part.Verb {
		case 'd', 'i', 'u', 'x', 'X', 'o':
			writeCompiledIntegerFormat(&buf, part, toInt(arg))
		case 'f':
			writeCompiledFloatFormat(&buf, part, toFloat(arg))
		case 's':
			buf.WriteString(arg.String())
		}
	}
	return buf.String(), nil
}

func (p *simpleFormatProgram) formatSingleInt(n int64) string {
	var buf strings.Builder
	buf.Grow(p.litBytes + 16)
	for _, part := range p.parts {
		if part.Verb == 0 {
			buf.WriteString(part.Lit)
			continue
		}
		writeCompiledIntegerFormat(&buf, part, n)
	}
	return buf.String()
}

func writeCompiledFloatFormat(buf *strings.Builder, part stdlibstring.Part, f float64) {
	var scratch [128]byte
	digits := strconv.AppendFloat(scratch[:0], f, 'f', part.Prec, 64)
	if part.Width <= len(digits) {
		buf.Write(digits)
		return
	}
	for i := 0; i < part.Width-len(digits); i++ {
		buf.WriteByte(' ')
	}
	buf.Write(digits)
}

func writeCompiledIntegerFormat(buf *strings.Builder, part stdlibstring.Part, n int64) {
	stdlibstring.WritePaddedInteger(buf, part.Verb, part.Pad, part.Width, n)
}

func scanSimpleFormatCacheRoots(visitor func(unsafe.Pointer), seen map[uintptr]struct{}) {
	simpleFormatCache.Lock()
	programs := make([]*simpleFormatProgram, 0, len(simpleFormatCache.entries))
	for _, prog := range simpleFormatCache.entries {
		programs = append(programs, prog)
	}
	simpleFormatCache.Unlock()

	for _, prog := range programs {
		prog.resultMu.Lock()
		for _, v := range prog.resultCache {
			ScanValueRoots(v, visitor, seen)
		}
		prog.resultMu.Unlock()
		for i := range prog.fastResultVals {
			if bits := prog.fastResultVals[i].Load(); bits != 0 {
				ScanValueRoots(Value(bits), visitor, seen)
			}
		}
	}
}

// toInt converts a Value to int64. Handles ints, floats, and string-to-number coercion.
func toInt(v Value) int64 {
	switch v.Type() {
	case TypeInt:
		return v.Int()
	case TypeFloat:
		return int64(v.Float())
	case TypeString:
		n, ok := v.ToNumber()
		if ok {
			return toInt(n)
		}
		return 0
	default:
		return 0
	}
}

// toFloat converts a Value to float64.
func toFloat(v Value) float64 {
	switch v.Type() {
	case TypeInt:
		return float64(v.Int())
	case TypeFloat:
		return v.Float()
	case TypeString:
		n, ok := v.ToNumber()
		if ok {
			return toFloat(n)
		}
		return 0
	default:
		return 0
	}
}
