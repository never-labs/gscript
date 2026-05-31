package runtime

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"unicode"
	"unsafe"
)

const (
	NativeKindStdStringFormat  uint8 = 2
	NativeKindStdStringSplit   uint8 = 100
	NativeKindStdStringSub     uint8 = 101
	NativeKindStdToNumber      uint8 = 102
	NativeKindStdSelect        uint8 = 103
	NativeKindStdPairs         uint8 = 104
	NativeKindStdIPairs        uint8 = 105
	NativeKindStdStringFind    uint8 = 106
	NativeKindStdStringMatch   uint8 = 107
	NativeKindStdRawGet        uint8 = 108
	NativeKindStdRawSet        uint8 = 109
	NativeKindStdRawLen        uint8 = 110
	NativeKindStdType          uint8 = 111
	NativeKindStdNext          uint8 = 112
	NativeKindStdGetMetatable  uint8 = 113
	NativeKindStdStringGSub    uint8 = 114
	NativeKindStdSoAAffineMany uint8 = 115
)

var stdStringFormatIdentity byte
var stdStringSplitIdentity byte
var stdStringSubIdentity byte
var stdToNumberIdentity byte
var stdSelectIdentity byte
var stdPairsIdentity byte
var stdIPairsIdentity byte
var stdStringFindIdentity byte
var stdStringMatchIdentity byte
var stdStringGSubIdentity byte
var stdRawGetIdentity byte
var stdRawSetIdentity byte
var stdRawLenIdentity byte
var stdTypeIdentity byte
var stdNextIdentity byte
var stdGetMetatableIdentity byte
var stdSoAAffineManyIdentity byte

type compiledLuaPatternCacheEntry struct {
	prog luaPatternProgram
	re   *regexp.Regexp
}

var compiledLuaPatternCache sync.Map
var simpleLuaPatternCache sync.Map

// NativeStringFormatIntCacheSize is the direct-mapped entry count used by the
// Tier 2 native string.format(pattern, int) path.
const NativeStringFormatIntCacheSize = 8192

// NativeStringFormatIntCacheEntry stores one immutable formatted string result
// for the Tier 2 native string.format(pattern, int) fast path.
type NativeStringFormatIntCacheEntry struct {
	PatternData uintptr
	PatternLen  uintptr
	N           int64
	Value       Value
	_           [16]byte
}

var nativeStringFormatIntCache [NativeStringFormatIntCacheSize]NativeStringFormatIntCacheEntry

// StdStringFormatIdentityPtr returns the process-wide identity token attached
// to stdlib string.format GoFunctions. JIT guards compare this token instead
// of trusting mutable function names or the presence of FastArg2.
func StdStringFormatIdentityPtr() unsafe.Pointer {
	return unsafe.Pointer(&stdStringFormatIdentity)
}

// NativeStringFormatIntCachePtr returns the base address of the process-wide
// direct-mapped native string.format(pattern, int) result cache.
func NativeStringFormatIntCachePtr() unsafe.Pointer {
	return unsafe.Pointer(&nativeStringFormatIntCache[0])
}

// BuildStringLibWithCaller creates the "string" standard library table using
// caller for function-valued replacements. A nil caller still supports native
// GoFunction callbacks.
func BuildStringLibWithCaller(caller ScriptFunctionCaller, maxHostResults ...func() int64) *Table {
	t := NewTable()
	maxHostResult := func() int64 {
		if len(maxHostResults) == 0 || maxHostResults[0] == nil {
			return 0
		}
		return maxHostResults[0]()
	}

	set := func(name string, fn func([]Value) ([]Value, error)) {
		t.RawSet(StringValue(name), FunctionValue(&GoFunction{
			Name: "string." + name,
			Fn:   fn,
		}))
	}
	setFastArg2 := func(name string, fn func([]Value) ([]Value, error), fast func([]Value) (Value, error), fast2 func(Value, Value) (Value, error)) {
		t.RawSet(StringValue(name), FunctionValue(&GoFunction{
			Name:     "string." + name,
			Fn:       fn,
			Fast1:    fast,
			FastArg2: fast2,
		}))
	}
	setFastArg23 := func(name string, fn func([]Value) ([]Value, error), fast func([]Value) (Value, error), fast2 func(Value, Value) (Value, error), fast3 func(Value, Value, Value) (Value, error)) {
		t.RawSet(StringValue(name), FunctionValue(&GoFunction{
			Name:     "string." + name,
			Fn:       fn,
			Fast1:    fast,
			FastArg2: fast2,
			FastArg3: fast3,
		}))
	}
	setFastArg2345 := func(name string, fn func([]Value) ([]Value, error), fast func([]Value) (Value, error), fast2 func(Value, Value) (Value, error), fast3 func(Value, Value, Value) (Value, error), fast4 func(Value, Value, Value, Value) (Value, error), fast5 func(Value, Value, Value, Value, Value) (Value, error)) {
		t.RawSet(StringValue(name), FunctionValue(&GoFunction{
			Name:     "string." + name,
			Fn:       fn,
			Fast1:    fast,
			FastArg2: fast2,
			FastArg3: fast3,
			FastArg4: fast4,
			FastArg5: fast5,
		}))
	}

	// string.len(s) -> int
	set("len", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'string.len' (string expected)")
		}
		return []Value{IntValue(int64(StringLen(args[0])))}, nil
	})

	// string.sub(s, i [, j]) -> string
	// 1-based indexing, negative indices count from end
	setFastArg23("sub", func(args []Value) ([]Value, error) {
		v, err := stringSubValue(args)
		if err != nil {
			return nil, err
		}
		return []Value{v}, nil
	}, stringSubValue, stringSub2Value, stringSub3Value)
	if v := t.RawGetString("sub"); v.IsFunction() {
		gf := v.GoFunction()
		gf.NativeKind = NativeKindStdStringSub
		gf.NativeData = StdStringSubIdentityPtr()
	}

	// string.pack/unpack/packsize expose the project's Go-style binary format
	// strings from the conventional string namespace. The canonical API is the
	// binary library; these are compatibility entry points, not Lua format
	// string clones.
	set("pack", func(args []Value) ([]Value, error) { return binaryPackValues("string.pack", args, maxHostResult()) })
	set("unpack", func(args []Value) ([]Value, error) { return binaryUnpackValues("string.unpack", args) })
	set("packsize", func(args []Value) ([]Value, error) { return binarySizeValues("string.packsize", args) })

	// string.upper(s) -> string
	set("upper", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'string.upper' (string expected)")
		}
		return []Value{StringValue(strings.ToUpper(args[0].Str()))}, nil
	})

	// string.lower(s) -> string
	set("lower", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'string.lower' (string expected)")
		}
		return []Value{StringValue(strings.ToLower(args[0].Str()))}, nil
	})

	// string.rep(s, n [, sep]) -> string
	set("rep", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'string.rep'")
		}
		if !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'string.rep' (string expected)")
		}
		s := args[0].Str()
		n := int(toInt(args[1]))
		if n <= 0 {
			return []Value{StringValue("")}, nil
		}
		sep := ""
		if len(args) >= 3 && args[2].IsString() {
			sep = args[2].Str()
		}
		if sep == "" {
			if err := CheckProjectedRepeatedStringBytes(maxHostResult(), len(s), n, 0); err != nil {
				return nil, err
			}
			return []Value{StringValue(strings.Repeat(s, n))}, nil
		}
		if err := CheckProjectedRepeatedStringBytes(maxHostResult(), len(s), n, len(sep)); err != nil {
			return nil, err
		}
		parts := make([]string, n)
		for i := range parts {
			parts[i] = s
		}
		return []Value{StringValue(strings.Join(parts, sep))}, nil
	})

	// string.reverse(s) -> string
	set("reverse", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'string.reverse' (string expected)")
		}
		s := args[0].Str()
		runes := []rune(s)
		for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
			runes[i], runes[j] = runes[j], runes[i]
		}
		return []Value{StringValue(string(runes))}, nil
	})

	// string.byte(s [, i [, j]]) -> int...
	set("byte", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'string.byte' (string expected)")
		}
		s := args[0].Str()
		i := 1
		j := i
		if len(args) >= 2 {
			i = int(toInt(args[1]))
			j = i
		}
		if len(args) >= 3 {
			j = int(toInt(args[2]))
		}
		if i < 0 {
			i = len(s) + i + 1
		}
		if j < 0 {
			j = len(s) + j + 1
		}
		if i < 1 {
			i = 1
		}
		if j > len(s) {
			j = len(s)
		}
		var result []Value
		for k := i; k <= j; k++ {
			result = append(result, IntValue(int64(s[k-1])))
		}
		if len(result) == 0 {
			return []Value{NilValue()}, nil
		}
		return result, nil
	})
	if v := t.RawGetString("byte"); v.IsFunction() {
		gf := v.GoFunction()
		gf.FastArg1 = stringByte1Value
		gf.FastArg2 = stringByte2Value
	}

	// string.char(i...) -> string
	set("char", func(args []Value) ([]Value, error) {
		if err := CheckProjectedHostStringBytes(maxHostResult(), len(args)); err != nil {
			return nil, err
		}
		buf := make([]byte, 0, len(args))
		for _, a := range args {
			n := int(toInt(a))
			if n < 0 || n > 255 {
				return nil, fmt.Errorf("bad argument to 'string.char' (value out of range)")
			}
			buf = append(buf, byte(n))
		}
		return []Value{StringValue(string(buf))}, nil
	})

	// string.find(s, pattern [, init [, plain]]) -> start, end [, captures...]
	set("find", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'string.find'")
		}
		if !args[0].IsString() || !args[1].IsString() {
			return nil, fmt.Errorf("bad argument to 'string.find' (string expected)")
		}
		s := args[0].Str()
		pattern := args[1].Str()
		init := 1
		plain := false
		if len(args) >= 3 {
			init = int(toInt(args[2]))
		}
		if len(args) >= 4 {
			plain = args[3].Truthy()
		}

		if init < 0 {
			init = len(s) + init + 1
		}
		if init < 1 {
			init = 1
		}
		if init > len(s)+1 {
			return []Value{NilValue()}, nil
		}
		searchStr := s[init-1:]

		if plain {
			idx := strings.Index(searchStr, pattern)
			if idx < 0 {
				return []Value{NilValue()}, nil
			}
			start := idx + init
			end := start + len(pattern) - 1
			return []Value{IntValue(int64(start)), IntValue(int64(end))}, nil
		}

		// Pattern matching
		if balanced, open, close := parseStandaloneBalancedPattern(pattern); balanced {
			loc := findBalancedRange(searchStr, open, close, 0)
			if loc == nil {
				return []Value{NilValue()}, nil
			}
			start := loc[0] + init
			end := loc[1] + init - 1
			return []Value{IntValue(int64(start)), IntValue(int64(end))}, nil
		}
		if simple, ok := cachedSimpleLuaPattern(pattern); ok {
			m, ok := simple.findNext(searchStr, 0)
			if !ok {
				return []Value{NilValue()}, nil
			}
			start := m.start + init
			end := m.end + init - 1
			result := []Value{IntValue(int64(start)), IntValue(int64(end))}
			if simple.captureCount > 0 {
				result = append(result, simpleMatchValues(searchStr, m)...)
			}
			return result, nil
		}
		prog, re, err := cachedLuaPatternRegexp(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid pattern: %s", err)
		}
		loc := prog.findSubmatchIndex(re, searchStr)
		if loc == nil {
			return []Value{NilValue()}, nil
		}
		start := loc[0] + init
		end := loc[1] + init - 1
		result := []Value{IntValue(int64(start)), IntValue(int64(end))}
		// Add captures if any
		for _, slot := range prog.captureSlots {
			pos := slot * 2
			if pos+1 < len(loc) && loc[pos] >= 0 {
				result = append(result, StringValue(searchStr[loc[pos]:loc[pos+1]]))
			} else {
				result = append(result, NilValue())
			}
		}
		return result, nil
	})
	if v := t.RawGetString("find"); v.IsFunction() {
		gf := v.GoFunction()
		gf.NativeKind = NativeKindStdStringFind
		gf.NativeData = StdStringFindIdentityPtr()
	}
	// string.match(s, pattern [, init]) -> captures...
	set("match", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'string.match'")
		}
		if !args[0].IsString() || !args[1].IsString() {
			return nil, fmt.Errorf("bad argument to 'string.match' (string expected)")
		}
		s := args[0].Str()
		pattern := args[1].Str()
		init := 1
		if len(args) >= 3 {
			init = int(toInt(args[2]))
		}
		if init < 0 {
			init = len(s) + init + 1
		}
		if init < 1 {
			init = 1
		}
		if init > len(s)+1 {
			return []Value{NilValue()}, nil
		}
		searchStr := s[init-1:]

		if balanced, open, close := parseStandaloneBalancedPattern(pattern); balanced {
			loc := findBalancedRange(searchStr, open, close, 0)
			if loc == nil {
				return []Value{NilValue()}, nil
			}
			return []Value{StringValue(searchStr[loc[0]:loc[1]])}, nil
		}
		if simple, ok := cachedSimpleLuaPattern(pattern); ok {
			m, ok := simple.findNext(searchStr, 0)
			if !ok {
				return []Value{NilValue()}, nil
			}
			return simpleMatchValues(searchStr, m), nil
		}
		prog, re, err := cachedLuaPatternRegexp(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid pattern: %s", err)
		}
		loc := prog.findSubmatchIndex(re, searchStr)
		if loc == nil {
			return []Value{NilValue()}, nil
		}
		if len(prog.captureSlots) == 0 {
			// No captures: return whole match
			return []Value{StringValue(searchStr[loc[0]:loc[1]])}, nil
		}
		// Return captures
		result := make([]Value, 0, len(prog.captureSlots))
		for _, slot := range prog.captureSlots {
			pos := slot * 2
			if pos+1 < len(loc) && loc[pos] >= 0 {
				result = append(result, StringValue(searchStr[loc[pos]:loc[pos+1]]))
			} else {
				result = append(result, NilValue())
			}
		}
		return result, nil
	})
	if v := t.RawGetString("match"); v.IsFunction() {
		gf := v.GoFunction()
		gf.NativeKind = NativeKindStdStringMatch
		gf.NativeData = StdStringMatchIdentityPtr()
	}
	// string.gmatch(s, pattern [, init]) -> iterator
	set("gmatch", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'string.gmatch'")
		}
		if !args[0].IsString() || !args[1].IsString() {
			return nil, fmt.Errorf("bad argument to 'string.gmatch' (string expected)")
		}
		s := args[0].Str()
		pattern := args[1].Str()
		init := 1
		if len(args) >= 3 {
			init = int(toInt(args[2]))
		}
		if init < 0 {
			init = len(s) + init + 1
		}
		if init < 1 {
			init = 1
		}
		if init > len(s)+1 {
			iter := &GoFunction{
				Name: "gmatch_iterator",
				Fn: func(_ []Value) ([]Value, error) {
					return []Value{NilValue()}, nil
				},
			}
			return []Value{FunctionValue(iter)}, nil
		}
		searchStr := s[init-1:]

		if balanced, open, close := parseStandaloneBalancedPattern(pattern); balanced {
			allMatches := findAllBalancedRanges(searchStr, open, close)
			idx := 0
			next := func() (Value, Value, int, error) {
				if idx >= len(allMatches) {
					return NilValue(), NilValue(), 1, nil
				}
				loc := allMatches[idx]
				idx++
				return StringValue(searchStr[loc[0]:loc[1]]), NilValue(), 1, nil
			}
			iter := &GoFunction{
				Name: "gmatch_iterator",
				Fn: func(_ []Value) ([]Value, error) {
					r0, _, _, err := next()
					if err != nil {
						return nil, err
					}
					return []Value{r0}, nil
				},
				FastArg2Ret2: func(_, _ Value) (Value, Value, int, error) {
					return next()
				},
			}
			return []Value{FunctionValue(iter)}, nil
		}
		if simple, ok := cachedSimpleLuaPattern(pattern); ok {
			nextStart := 0
			next := func() (Value, Value, int, error) {
				m, ok := simple.findNext(searchStr, nextStart)
				if !ok {
					return NilValue(), NilValue(), 1, nil
				}
				nextStart = simpleNextPatternSearchStart(searchStr, m.start, m.end)
				if m.ncap == 0 {
					return StringValue(searchStr[m.start:m.end]), NilValue(), 1, nil
				}
				r0 := StringValue(searchStr[m.caps[0][0]:m.caps[0][1]])
				if m.ncap == 1 {
					return r0, NilValue(), 1, nil
				}
				return r0, StringValue(searchStr[m.caps[1][0]:m.caps[1][1]]), 2, nil
			}
			iter := &GoFunction{
				Name: "gmatch_iterator",
				Fn: func(_ []Value) ([]Value, error) {
					m, ok := simple.findNext(searchStr, nextStart)
					if !ok {
						return []Value{NilValue()}, nil
					}
					nextStart = simpleNextPatternSearchStart(searchStr, m.start, m.end)
					return simpleMatchValues(searchStr, m), nil
				},
			}
			if simple.captureCount <= 2 {
				iter.FastArg2Ret2 = func(_, _ Value) (Value, Value, int, error) {
					return next()
				}
			}
			return []Value{FunctionValue(iter)}, nil
		}
		prog, re, err := cachedLuaPatternRegexp(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid pattern: %s", err)
		}
		allMatches := prog.findAllSubmatchIndex(re, searchStr)
		idx := 0
		next := func() (Value, Value, int, error) {
			if idx >= len(allMatches) {
				return NilValue(), NilValue(), 1, nil
			}
			loc := allMatches[idx]
			idx++
			if len(prog.captureSlots) == 0 {
				return StringValue(searchStr[loc[0]:loc[1]]), NilValue(), 1, nil
			}
			var r0, r1 Value
			for i, slot := range prog.captureSlots {
				pos := slot * 2
				v := NilValue()
				if pos+1 < len(loc) && loc[pos] >= 0 {
					if prog.captureKinds[i] == luaPatternCapturePosition {
						v = IntValue(int64(loc[pos] + init))
					} else {
						v = StringValue(searchStr[loc[pos]:loc[pos+1]])
					}
				}
				if i == 0 {
					r0 = v
				} else if i == 1 {
					r1 = v
				}
			}
			switch len(prog.captureSlots) {
			case 1:
				return r0, NilValue(), 1, nil
			default:
				return r0, r1, 2, nil
			}
		}
		iter := &GoFunction{
			Name: "gmatch_iterator",
			Fn: func(_ []Value) ([]Value, error) {
				if len(prog.captureSlots) == 0 {
					r0, _, _, err := next()
					if err != nil {
						return nil, err
					}
					return []Value{r0}, nil
				}
				result := make([]Value, 0, len(prog.captureSlots))
				if idx >= len(allMatches) {
					return []Value{NilValue()}, nil
				}
				loc := allMatches[idx]
				idx++
				for i, slot := range prog.captureSlots {
					pos := slot * 2
					if pos+1 < len(loc) && loc[pos] >= 0 {
						if prog.captureKinds[i] == luaPatternCapturePosition {
							result = append(result, IntValue(int64(loc[pos]+init)))
						} else {
							result = append(result, StringValue(searchStr[loc[pos]:loc[pos+1]]))
						}
					} else {
						result = append(result, NilValue())
					}
				}
				return result, nil
			},
		}
		if len(prog.captureSlots) <= 2 {
			iter.FastArg2Ret2 = func(_, _ Value) (Value, Value, int, error) {
				return next()
			}
		}
		return []Value{FunctionValue(iter)}, nil
	})

	// string.gsub(s, pattern, repl [, n]) -> string, count
	set("gsub", func(args []Value) ([]Value, error) {
		if len(args) < 3 {
			return nil, fmt.Errorf("bad argument to 'string.gsub'")
		}
		if !args[0].IsString() || !args[1].IsString() {
			return nil, fmt.Errorf("bad argument to 'string.gsub' (string expected)")
		}
		s := args[0].Str()
		pattern := args[1].Str()
		repl := args[2]
		maxRepl := -1
		if len(args) >= 4 {
			maxRepl = int(toInt(args[3]))
		}

		if balanced, open, close := parseStandaloneBalancedPattern(pattern); balanced {
			count := 0
			var result string
			if repl.IsString() {
				replStr := repl.Str()
				if err := validateLuaReplacementString(replStr, 0); err != nil {
					return nil, err
				}
				result = replaceBalancedPatternString(s, open, close, replStr, maxRepl, &count)
			} else if repl.IsTable() {
				var err error
				result, err = replaceBalancedPatternTable(s, open, close, repl.Table(), maxRepl, &count)
				if err != nil {
					return nil, err
				}
			} else if repl.IsFunction() {
				var err error
				result, err = replaceBalancedPatternFunction(s, open, close, repl, caller, maxRepl, &count)
				if err != nil {
					return nil, err
				}
			} else {
				result = replaceBalancedPatternRaw(s, open, close, repl.String(), maxRepl, &count)
			}
			return []Value{StringValue(result), IntValue(int64(count))}, nil
		}
		if simple, ok := cachedSimpleLuaPattern(pattern); ok {
			count := 0
			var result string
			if repl.IsString() {
				replStr := repl.Str()
				if err := validateLuaReplacementString(replStr, simple.captureCount); err != nil {
					return nil, err
				}
				result = replaceSimpleLuaPatternString(s, simple, replStr, maxRepl, &count)
			} else if repl.IsTable() {
				var err error
				result, err = replaceSimpleLuaPatternTable(s, simple, repl.Table(), maxRepl, &count)
				if err != nil {
					return nil, err
				}
			} else if repl.IsFunction() {
				var err error
				result, err = replaceSimpleLuaPatternFunction(s, simple, repl, caller, maxRepl, &count)
				if err != nil {
					return nil, err
				}
			} else {
				result = replaceSimpleLuaPatternRaw(s, simple, repl.String(), maxRepl, &count)
			}
			return []Value{StringValue(result), IntValue(int64(count))}, nil
		}

		prog, re, err := cachedLuaPatternRegexp(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid pattern: %s", err)
		}

		count := 0
		var result string
		if repl.IsString() {
			replStr := repl.Str()
			if err := validateLuaReplacementString(replStr, len(prog.captureKinds)); err != nil {
				return nil, err
			}
			result = replaceLuaPatternString(s, re, prog, replStr, maxRepl, &count)
		} else if repl.IsTable() {
			var err error
			result, err = replaceLuaPatternTable(s, re, prog, repl.Table(), maxRepl, &count)
			if err != nil {
				return nil, err
			}
		} else if repl.IsFunction() {
			var err error
			result, err = replaceLuaPatternFunction(s, re, prog, repl, caller, maxRepl, &count)
			if err != nil {
				return nil, err
			}
		} else {
			// For non-string replacement, just do string replacement
			replStr := repl.String()
			result = re.ReplaceAllStringFunc(s, func(match string) string {
				if maxRepl >= 0 && count >= maxRepl {
					return match
				}
				count++
				return replStr
			})
		}
		return []Value{StringValue(result), IntValue(int64(count))}, nil
	})
	if v := t.RawGetString("gsub"); v.IsFunction() {
		gf := v.GoFunction()
		gf.NativeKind = NativeKindStdStringGSub
		gf.NativeData = StdStringGSubIdentityPtr()
	}

	// string.format(fmt, ...) -> string
	setFastArg2345("format", func(args []Value) ([]Value, error) {
		v, err := stringFormatValue(args)
		if err != nil {
			return nil, err
		}
		return []Value{v}, nil
	}, stringFormatValue, stringFormat2Value, stringFormat3Value, stringFormat4Value, stringFormat5Value)
	if v := t.RawGetString("format"); v.IsFunction() {
		gf := v.GoFunction()
		gf.NativeKind = NativeKindStdStringFormat
		gf.NativeData = StdStringFormatIdentityPtr()
	}

	// string.split(s, sep) -> table. sep="" splits by byte
	setFastArg2("split", func(args []Value) ([]Value, error) {
		v, err := stringSplitValue(args)
		if err != nil {
			return nil, err
		}
		return []Value{v}, nil
	}, stringSplitValue, stringSplit2Value)
	if v := t.RawGetString("split"); v.IsFunction() {
		gf := v.GoFunction()
		gf.NativeKind = NativeKindStdStringSplit
		gf.NativeData = StdStringSplitIdentityPtr()
	}

	stringTrim := func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'string.trim' (string expected)")
		}
		s := args[0].Str()
		if len(args) >= 2 && args[1].IsString() {
			return []Value{StringValue(strings.Trim(s, args[1].Str()))}, nil
		}
		return []Value{StringValue(strings.TrimSpace(s))}, nil
	}
	stringTrim1 := func(a Value) (Value, error) {
		if !a.IsString() {
			return NilValue(), fmt.Errorf("bad argument #1 to 'string.trim' (string expected)")
		}
		return StringValue(strings.TrimSpace(a.Str())), nil
	}
	stringTrim2 := func(a, b Value) (Value, error) {
		if !a.IsString() || !b.IsString() {
			return NilValue(), fmt.Errorf("bad argument to 'string.trim' (string expected)")
		}
		return StringValue(strings.Trim(a.Str(), b.Str())), nil
	}
	// string.trim(s [, cutset]) -- trim leading/trailing whitespace (or chars in cutset)
	setFastArg2("trim", stringTrim, func(args []Value) (Value, error) {
		if len(args) >= 2 {
			return stringTrim2(args[0], args[1])
		}
		if len(args) == 1 {
			return stringTrim1(args[0])
		}
		return NilValue(), fmt.Errorf("bad argument #1 to 'string.trim' (string expected)")
	}, stringTrim2)
	if v := t.RawGetString("trim"); v.IsFunction() {
		v.GoFunction().FastArg1 = stringTrim1
	}

	stringTrimLeft := func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'string.trimLeft' (string expected)")
		}
		s := args[0].Str()
		if len(args) >= 2 && args[1].IsString() {
			return []Value{StringValue(strings.TrimLeft(s, args[1].Str()))}, nil
		}
		return []Value{StringValue(strings.TrimLeftFunc(s, unicode.IsSpace))}, nil
	}
	stringTrimLeft1 := func(a Value) (Value, error) {
		if !a.IsString() {
			return NilValue(), fmt.Errorf("bad argument #1 to 'string.trimLeft' (string expected)")
		}
		return StringValue(strings.TrimLeftFunc(a.Str(), unicode.IsSpace)), nil
	}
	stringTrimLeft2 := func(a, b Value) (Value, error) {
		if !a.IsString() || !b.IsString() {
			return NilValue(), fmt.Errorf("bad argument to 'string.trimLeft' (string expected)")
		}
		return StringValue(strings.TrimLeft(a.Str(), b.Str())), nil
	}
	// string.trimLeft(s [, cutset])
	setFastArg2("trimLeft", stringTrimLeft, func(args []Value) (Value, error) {
		if len(args) >= 2 {
			return stringTrimLeft2(args[0], args[1])
		}
		if len(args) == 1 {
			return stringTrimLeft1(args[0])
		}
		return NilValue(), fmt.Errorf("bad argument #1 to 'string.trimLeft' (string expected)")
	}, stringTrimLeft2)
	if v := t.RawGetString("trimLeft"); v.IsFunction() {
		v.GoFunction().FastArg1 = stringTrimLeft1
	}

	stringTrimRight := func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'string.trimRight' (string expected)")
		}
		s := args[0].Str()
		if len(args) >= 2 && args[1].IsString() {
			return []Value{StringValue(strings.TrimRight(s, args[1].Str()))}, nil
		}
		return []Value{StringValue(strings.TrimRightFunc(s, unicode.IsSpace))}, nil
	}
	stringTrimRight1 := func(a Value) (Value, error) {
		if !a.IsString() {
			return NilValue(), fmt.Errorf("bad argument #1 to 'string.trimRight' (string expected)")
		}
		return StringValue(strings.TrimRightFunc(a.Str(), unicode.IsSpace)), nil
	}
	stringTrimRight2 := func(a, b Value) (Value, error) {
		if !a.IsString() || !b.IsString() {
			return NilValue(), fmt.Errorf("bad argument to 'string.trimRight' (string expected)")
		}
		return StringValue(strings.TrimRight(a.Str(), b.Str())), nil
	}
	// string.trimRight(s [, cutset])
	setFastArg2("trimRight", stringTrimRight, func(args []Value) (Value, error) {
		if len(args) >= 2 {
			return stringTrimRight2(args[0], args[1])
		}
		if len(args) == 1 {
			return stringTrimRight1(args[0])
		}
		return NilValue(), fmt.Errorf("bad argument #1 to 'string.trimRight' (string expected)")
	}, stringTrimRight2)
	if v := t.RawGetString("trimRight"); v.IsFunction() {
		v.GoFunction().FastArg1 = stringTrimRight1
	}

	stringHasPrefix := func(args []Value) ([]Value, error) {
		if len(args) < 2 || !args[0].IsString() || !args[1].IsString() {
			return nil, fmt.Errorf("bad argument to 'string.hasPrefix' (string expected)")
		}
		return []Value{BoolValue(strings.HasPrefix(args[0].Str(), args[1].Str()))}, nil
	}
	stringHasPrefixFast := func(a, b Value) (Value, error) {
		if !a.IsString() || !b.IsString() {
			return NilValue(), fmt.Errorf("bad argument to 'string.hasPrefix' (string expected)")
		}
		return BoolValue(strings.HasPrefix(a.Str(), b.Str())), nil
	}
	// string.hasPrefix(s, prefix) -> bool
	setFastArg2("hasPrefix", stringHasPrefix, func(args []Value) (Value, error) {
		if len(args) < 2 {
			return NilValue(), fmt.Errorf("bad argument to 'string.hasPrefix' (string expected)")
		}
		return stringHasPrefixFast(args[0], args[1])
	}, stringHasPrefixFast)

	stringHasSuffix := func(args []Value) ([]Value, error) {
		if len(args) < 2 || !args[0].IsString() || !args[1].IsString() {
			return nil, fmt.Errorf("bad argument to 'string.hasSuffix' (string expected)")
		}
		return []Value{BoolValue(strings.HasSuffix(args[0].Str(), args[1].Str()))}, nil
	}
	stringHasSuffixFast := func(a, b Value) (Value, error) {
		if !a.IsString() || !b.IsString() {
			return NilValue(), fmt.Errorf("bad argument to 'string.hasSuffix' (string expected)")
		}
		return BoolValue(strings.HasSuffix(a.Str(), b.Str())), nil
	}
	// string.hasSuffix(s, suffix) -> bool
	setFastArg2("hasSuffix", stringHasSuffix, func(args []Value) (Value, error) {
		if len(args) < 2 {
			return NilValue(), fmt.Errorf("bad argument to 'string.hasSuffix' (string expected)")
		}
		return stringHasSuffixFast(args[0], args[1])
	}, stringHasSuffixFast)

	stringContains := func(args []Value) ([]Value, error) {
		if len(args) < 2 || !args[0].IsString() || !args[1].IsString() {
			return nil, fmt.Errorf("bad argument to 'string.contains' (string expected)")
		}
		return []Value{BoolValue(strings.Contains(args[0].Str(), args[1].Str()))}, nil
	}
	stringContainsFast := func(a, b Value) (Value, error) {
		if !a.IsString() || !b.IsString() {
			return NilValue(), fmt.Errorf("bad argument to 'string.contains' (string expected)")
		}
		return BoolValue(strings.Contains(a.Str(), b.Str())), nil
	}
	// string.contains(s, substr) -> bool
	setFastArg2("contains", stringContains, func(args []Value) (Value, error) {
		if len(args) < 2 {
			return NilValue(), fmt.Errorf("bad argument to 'string.contains' (string expected)")
		}
		return stringContainsFast(args[0], args[1])
	}, stringContainsFast)

	stringCount := func(args []Value) ([]Value, error) {
		if len(args) < 2 || !args[0].IsString() || !args[1].IsString() {
			return nil, fmt.Errorf("bad argument to 'string.count' (string expected)")
		}
		return []Value{IntValue(int64(strings.Count(args[0].Str(), args[1].Str())))}, nil
	}
	stringCountFast := func(a, b Value) (Value, error) {
		if !a.IsString() || !b.IsString() {
			return NilValue(), fmt.Errorf("bad argument to 'string.count' (string expected)")
		}
		return IntValue(int64(strings.Count(a.Str(), b.Str()))), nil
	}
	// string.count(s, substr) -> int
	setFastArg2("count", stringCount, func(args []Value) (Value, error) {
		if len(args) < 2 {
			return NilValue(), fmt.Errorf("bad argument to 'string.count' (string expected)")
		}
		return stringCountFast(args[0], args[1])
	}, stringCountFast)

	stringReplaceAll := func(args []Value) ([]Value, error) {
		if len(args) < 3 || !args[0].IsString() || !args[1].IsString() || !args[2].IsString() {
			return nil, fmt.Errorf("bad argument to 'string.replaceAll' (string expected)")
		}
		return []Value{StringValue(strings.ReplaceAll(args[0].Str(), args[1].Str(), args[2].Str()))}, nil
	}
	stringReplaceAll3 := func(a, b, c Value) (Value, error) {
		if !a.IsString() || !b.IsString() || !c.IsString() {
			return NilValue(), fmt.Errorf("bad argument to 'string.replaceAll' (string expected)")
		}
		return StringValue(strings.ReplaceAll(a.Str(), b.Str(), c.Str())), nil
	}
	// string.replaceAll(s, old, new) -- plain string replace all
	setFastArg23("replaceAll", stringReplaceAll, func(args []Value) (Value, error) {
		if len(args) < 3 {
			return NilValue(), fmt.Errorf("bad argument to 'string.replaceAll' (string expected)")
		}
		return stringReplaceAll3(args[0], args[1], args[2])
	}, nil, stringReplaceAll3)

	// string.join(t, sep) -- join table of strings with separator
	set("join", func(args []Value) ([]Value, error) {
		if len(args) < 2 || !args[0].IsTable() || !args[1].IsString() {
			return nil, fmt.Errorf("bad argument to 'string.join'")
		}
		tbl := args[0].Table()
		sep := args[1].Str()
		length := tbl.Length()
		parts := make([]string, length)
		for i := 0; i < length; i++ {
			parts[i] = tbl.RawGet(IntValue(int64(i + 1))).String()
		}
		total := 0
		for _, part := range parts {
			total += len(part)
		}
		if length > 1 {
			total += len(sep) * (length - 1)
		}
		if err := CheckProjectedHostStringBytes(maxHostResult(), total); err != nil {
			return nil, err
		}
		return []Value{StringValue(strings.Join(parts, sep))}, nil
	})

	// string.title(s) -- capitalize first letter of each word
	set("title", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'string.title' (string expected)")
		}
		s := args[0].Str()
		// Capitalize first letter of each word
		prev := ' '
		result := make([]rune, 0, len(s))
		for _, r := range s {
			if unicode.IsSpace(rune(prev)) {
				result = append(result, unicode.ToUpper(r))
			} else {
				result = append(result, r)
			}
			prev = r
		}
		return []Value{StringValue(string(result))}, nil
	})

	// string.padLeft(s, n [, char]) -- pad with char (default space) on left to width n
	set("padLeft", func(args []Value) ([]Value, error) {
		if len(args) < 2 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument to 'string.padLeft'")
		}
		s := args[0].Str()
		n := int(toInt(args[1]))
		pad := " "
		if len(args) >= 3 && args[2].IsString() {
			pad = args[2].Str()
		}
		if pad == "" {
			pad = " "
		}
		if n > len(s) {
			if err := CheckProjectedHostStringBytes(maxHostResult(), n); err != nil {
				return nil, err
			}
		}
		for len(s) < n {
			s = pad + s
		}
		// Trim to exact width if pad added too much
		if len(s) > n {
			s = s[len(s)-n:]
		}
		return []Value{StringValue(s)}, nil
	})

	// string.padRight(s, n [, char]) -- pad on right
	set("padRight", func(args []Value) ([]Value, error) {
		if len(args) < 2 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument to 'string.padRight'")
		}
		s := args[0].Str()
		n := int(toInt(args[1]))
		pad := " "
		if len(args) >= 3 && args[2].IsString() {
			pad = args[2].Str()
		}
		if pad == "" {
			pad = " "
		}
		if n > len(s) {
			if err := CheckProjectedHostStringBytes(maxHostResult(), n); err != nil {
				return nil, err
			}
		}
		for len(s) < n {
			s = s + pad
		}
		// Trim to exact width
		if len(s) > n {
			s = s[:n]
		}
		return []Value{StringValue(s)}, nil
	})

	// string.repeat(s, n) -- alias for string.rep
	set("repeat", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'string.repeat'")
		}
		if !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'string.repeat' (string expected)")
		}
		s := args[0].Str()
		n := int(toInt(args[1]))
		if n <= 0 {
			return []Value{StringValue("")}, nil
		}
		if err := CheckProjectedRepeatedStringBytes(maxHostResult(), len(s), n, 0); err != nil {
			return nil, err
		}
		return []Value{StringValue(strings.Repeat(s, n))}, nil
	})

	// string.isNumeric(s) -> bool
	set("isNumeric", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'string.isNumeric' (string expected)")
		}
		s := strings.TrimSpace(args[0].Str())
		_, err := strconv.ParseFloat(s, 64)
		return []Value{BoolValue(err == nil && s != "")}, nil
	})

	return t
}

// RefreshStringLibWithCaller updates an existing string library table in place,
// preserving module identity for require/package.loaded users.
func RefreshStringLibWithCaller(t *Table, caller ScriptFunctionCaller, maxHostResults ...func() int64) *Table {
	if t == nil {
		return BuildStringLibWithCaller(caller, maxHostResults...)
	}
	fresh := BuildStringLibWithCaller(caller, maxHostResults...)
	for key, val, ok := fresh.Next(NilValue()); ok; key, val, ok = fresh.Next(key) {
		t.RawSet(key, val)
	}
	return t
}

// buildStringLib creates the "string" standard library table and returns it.
func buildStringLib() *Table {
	return BuildStringLibWithCaller(nil)
}
