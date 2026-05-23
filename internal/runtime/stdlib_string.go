package runtime

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"unicode"
	"unsafe"
)

const (
	NativeKindStdStringFormat uint8 = 2
	NativeKindStdStringSplit  uint8 = 100
	NativeKindStdStringSub    uint8 = 101
	NativeKindStdToNumber     uint8 = 102
	NativeKindStdSelect       uint8 = 103
	NativeKindStdPairs        uint8 = 104
	NativeKindStdIPairs       uint8 = 105
	NativeKindStdStringFind   uint8 = 106
	NativeKindStdStringMatch  uint8 = 107
	NativeKindStdRawGet       uint8 = 108
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
var stdRawGetIdentity byte

type compiledLuaPatternCacheEntry struct {
	prog luaPatternProgram
	re   *regexp.Regexp
}

var compiledLuaPatternCache sync.Map

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

// FunctionCaller invokes a GScript function value from the active execution
// engine. Stdlib functions that accept callbacks use it to stay VM-aware.
type FunctionCaller func(Value, []Value) ([]Value, error)

// BuildStringLibWithCaller creates the "string" standard library table using
// caller for function-valued replacements. A nil caller still supports native
// GoFunction callbacks.
func BuildStringLibWithCaller(caller FunctionCaller) *Table {
	t := NewTable()

	set := func(name string, fn func([]Value) ([]Value, error)) {
		t.RawSet(StringValue(name), FunctionValue(&GoFunction{
			Name: "string." + name,
			Fn:   fn,
		}))
	}
	setFast1 := func(name string, fn func([]Value) ([]Value, error), fast func([]Value) (Value, error)) {
		t.RawSet(StringValue(name), FunctionValue(&GoFunction{
			Name:  "string." + name,
			Fn:    fn,
			Fast1: fast,
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
	set("pack", func(args []Value) ([]Value, error) { return binaryPackValues("string.pack", args) })
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
			return []Value{StringValue(strings.Repeat(s, n))}, nil
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

	// string.format(fmt, ...) -> string
	setFast1("format", func(args []Value) ([]Value, error) {
		v, err := stringFormatValue(args)
		if err != nil {
			return nil, err
		}
		return []Value{v}, nil
	}, stringFormatValue)
	if v := t.RawGetString("format"); v.IsFunction() {
		gf := v.GoFunction()
		gf.FastArg2 = stringFormat2Value
		gf.FastArg3 = stringFormat3Value
		gf.FastArg4 = stringFormat4Value
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
func RefreshStringLibWithCaller(t *Table, caller FunctionCaller) *Table {
	if t == nil {
		return BuildStringLibWithCaller(caller)
	}
	fresh := BuildStringLibWithCaller(caller)
	for key, val, ok := fresh.Next(NilValue()); ok; key, val, ok = fresh.Next(key) {
		t.RawSet(key, val)
	}
	return t
}

// buildStringLib creates the "string" standard library table and returns it.
func buildStringLib() *Table {
	return BuildStringLibWithCaller(nil)
}

func stringSubValue(args []Value) (Value, error) {
	if len(args) < 2 {
		return NilValue(), fmt.Errorf("bad argument to 'string.sub'")
	}
	if !args[0].IsString() {
		return NilValue(), fmt.Errorf("bad argument #1 to 'string.sub' (string expected)")
	}
	s := args[0].Str()
	slen := len(s)

	i := int(toInt(args[1]))
	j := slen
	if len(args) >= 3 {
		j = int(toInt(args[2]))
	}

	// Convert Lua 1-based indexes to Go byte offsets.
	if i < 0 {
		i = slen + i + 1
	}
	if i < 1 {
		i = 1
	}
	if j < 0 {
		j = slen + j + 1
	}
	if j > slen {
		j = slen
	}
	if i > j {
		return StringValue(""), nil
	}
	return StringValue(s[i-1 : j]), nil
}

func stringSub2Value(sv, iv Value) (Value, error) {
	if !sv.IsString() {
		return NilValue(), fmt.Errorf("bad argument #1 to 'string.sub' (string expected)")
	}
	s := sv.Str()
	slen := len(s)
	i := int(toInt(iv))
	if i < 0 {
		i = slen + i + 1
	}
	if i < 1 {
		i = 1
	}
	if i > slen {
		return StringValue(""), nil
	}
	return StringValue(s[i-1:]), nil
}

func stringSub3Value(sv, iv, jv Value) (Value, error) {
	if !sv.IsString() {
		return NilValue(), fmt.Errorf("bad argument #1 to 'string.sub' (string expected)")
	}
	s := sv.Str()
	slen := len(s)
	i := int(toInt(iv))
	j := int(toInt(jv))
	if i < 0 {
		i = slen + i + 1
	}
	if i < 1 {
		i = 1
	}
	if j < 0 {
		j = slen + j + 1
	}
	if j > slen {
		j = slen
	}
	if i > j {
		return StringValue(""), nil
	}
	return StringValue(s[i-1 : j]), nil
}

func stringByte1Value(sv Value) (Value, error) {
	if !sv.IsString() {
		return NilValue(), fmt.Errorf("bad argument #1 to 'string.byte' (string expected)")
	}
	s := sv.Str()
	if len(s) == 0 {
		return NilValue(), nil
	}
	return IntValue(int64(s[0])), nil
}

func stringByte2Value(sv, iv Value) (Value, error) {
	if !sv.IsString() {
		return NilValue(), fmt.Errorf("bad argument #1 to 'string.byte' (string expected)")
	}
	s := sv.Str()
	i := int(toInt(iv))
	if i < 0 {
		i = len(s) + i + 1
	}
	if i < 1 || i > len(s) {
		return NilValue(), nil
	}
	return IntValue(int64(s[i-1])), nil
}

func stringSplitValue(args []Value) (Value, error) {
	if len(args) < 2 {
		return NilValue(), fmt.Errorf("bad argument to 'string.split'")
	}
	if !args[0].IsString() || !args[1].IsString() {
		return NilValue(), fmt.Errorf("bad argument to 'string.split' (string expected)")
	}
	return stringSplitStrings(args[0].Str(), args[1].Str()), nil
}

func stringSplit2Value(sv, sepv Value) (Value, error) {
	if !sv.IsString() || !sepv.IsString() {
		return NilValue(), fmt.Errorf("bad argument to 'string.split' (string expected)")
	}
	return stringSplitStrings(sv.Str(), sepv.Str()), nil
}

func stringSplitStrings(s, sep string) Value {
	if sep == "" {
		tbl := NewSequentialArrayTable(len(s))
		for i := 0; i < len(s); i++ {
			tbl.array[i+1] = StringValue(string(s[i]))
		}
		return TableValue(tbl)
	}

	tbl := NewAppendArrayTable(8)
	if len(sep) == 1 {
		sepByte := sep[0]
		start := 0
		for i := 0; i < len(s); i++ {
			if s[i] != sepByte {
				continue
			}
			arenaAppendValue(DefaultHeap, &tbl.array, StringValue(s[start:i]))
			start = i + 1
		}
		arenaAppendValue(DefaultHeap, &tbl.array, StringValue(s[start:]))
		return TableValue(tbl)
	}

	start := 0
	for {
		next := strings.Index(s[start:], sep)
		if next < 0 {
			arenaAppendValue(DefaultHeap, &tbl.array, StringValue(s[start:]))
			break
		}
		end := start + next
		arenaAppendValue(DefaultHeap, &tbl.array, StringValue(s[start:end]))
		start = end + len(sep)
	}
	return TableValue(tbl)
}

func StdStringSplitIdentityPtr() unsafe.Pointer {
	return unsafe.Pointer(&stdStringSplitIdentity)
}

func StdStringSubIdentityPtr() unsafe.Pointer {
	return unsafe.Pointer(&stdStringSubIdentity)
}

func StdToNumberIdentityPtr() unsafe.Pointer {
	return unsafe.Pointer(&stdToNumberIdentity)
}

func StdSelectIdentityPtr() unsafe.Pointer {
	return unsafe.Pointer(&stdSelectIdentity)
}

func StdPairsIdentityPtr() unsafe.Pointer {
	return unsafe.Pointer(&stdPairsIdentity)
}

func StdIPairsIdentityPtr() unsafe.Pointer {
	return unsafe.Pointer(&stdIPairsIdentity)
}

func StdStringFindIdentityPtr() unsafe.Pointer {
	return unsafe.Pointer(&stdStringFindIdentity)
}

func StdStringMatchIdentityPtr() unsafe.Pointer {
	return unsafe.Pointer(&stdStringMatchIdentity)
}

func StdRawGetIdentityPtr() unsafe.Pointer {
	return unsafe.Pointer(&stdRawGetIdentity)
}

// FastStringFindRet2 computes the first two string.find return values without
// allocating a result slice. It only handles cases where dropping captures is
// semantically safe for the caller's fixed result count.
func FastStringFindRet2(sv, pv, initv, plainv Value, nArgs, rawC int) (Value, Value, int, bool, error) {
	if nArgs != 2 && nArgs != 3 && nArgs != 4 {
		return NilValue(), NilValue(), 0, false, nil
	}
	if !sv.IsString() || !pv.IsString() {
		return NilValue(), NilValue(), 0, true, fmt.Errorf("bad argument to 'string.find' (string expected)")
	}
	s := sv.Str()
	pattern := pv.Str()
	init := 1
	if nArgs >= 3 {
		init = int(toInt(initv))
	}
	if init < 0 {
		init = len(s) + init + 1
	}
	if init < 1 {
		init = 1
	}
	if init > len(s)+1 {
		return NilValue(), NilValue(), 1, true, nil
	}
	searchStr := s[init-1:]
	if nArgs >= 4 && plainv.Truthy() {
		idx := strings.Index(searchStr, pattern)
		if idx < 0 {
			return NilValue(), NilValue(), 1, true, nil
		}
		start := idx + init
		end := start + len(pattern) - 1
		return IntValue(int64(start)), IntValue(int64(end)), 2, true, nil
	}

	if balanced, open, close := parseStandaloneBalancedPattern(pattern); balanced {
		loc := findBalancedRange(searchStr, open, close, 0)
		if loc == nil {
			return NilValue(), NilValue(), 1, true, nil
		}
		return IntValue(int64(loc[0] + init)), IntValue(int64(loc[1] + init - 1)), 2, true, nil
	}
	prog, re, err := cachedLuaPatternRegexp(pattern)
	if err != nil {
		return NilValue(), NilValue(), 0, true, fmt.Errorf("invalid pattern: %s", err)
	}
	if len(prog.captureSlots) > 0 && (rawC == 0 || rawC-1 > 2) {
		return NilValue(), NilValue(), 0, false, nil
	}
	loc := prog.findSubmatchIndex(re, searchStr)
	if loc == nil {
		return NilValue(), NilValue(), 1, true, nil
	}
	return IntValue(int64(loc[0] + init)), IntValue(int64(loc[1] + init - 1)), 2, true, nil
}

// FastStringMatchRet2 computes string.match results without allocating a result
// slice when the caller's fixed result count cannot observe later captures.
func FastStringMatchRet2(sv, pv, initv Value, nArgs, rawC int) (Value, Value, int, bool, error) {
	if nArgs != 2 && nArgs != 3 {
		return NilValue(), NilValue(), 0, false, nil
	}
	if !sv.IsString() || !pv.IsString() {
		return NilValue(), NilValue(), 0, true, fmt.Errorf("bad argument to 'string.match' (string expected)")
	}
	s := sv.Str()
	pattern := pv.Str()
	init := 1
	if nArgs >= 3 {
		init = int(toInt(initv))
	}
	if init < 0 {
		init = len(s) + init + 1
	}
	if init < 1 {
		init = 1
	}
	if init > len(s)+1 {
		return NilValue(), NilValue(), 1, true, nil
	}
	searchStr := s[init-1:]

	if balanced, open, close := parseStandaloneBalancedPattern(pattern); balanced {
		loc := findBalancedRange(searchStr, open, close, 0)
		if loc == nil {
			return NilValue(), NilValue(), 1, true, nil
		}
		return StringValue(searchStr[loc[0]:loc[1]]), NilValue(), 1, true, nil
	}
	prog, re, err := cachedLuaPatternRegexp(pattern)
	if err != nil {
		return NilValue(), NilValue(), 0, true, fmt.Errorf("invalid pattern: %s", err)
	}
	if len(prog.captureSlots) > 2 && (rawC == 0 || rawC-1 > 2) {
		return NilValue(), NilValue(), 0, false, nil
	}
	loc := prog.findSubmatchIndex(re, searchStr)
	if loc == nil {
		return NilValue(), NilValue(), 1, true, nil
	}
	if len(prog.captureSlots) == 0 {
		return StringValue(searchStr[loc[0]:loc[1]]), NilValue(), 1, true, nil
	}
	var r0, r1 Value
	for i, slot := range prog.captureSlots {
		if i >= 2 {
			break
		}
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
		} else {
			r1 = v
		}
	}
	if len(prog.captureSlots) == 1 {
		return r0, NilValue(), 1, true, nil
	}
	return r0, r1, 2, true, nil
}

func IsStdStringSplitFunction(v Value) bool {
	gf := v.GoFunction()
	return gf != nil &&
		gf.NativeKind == NativeKindStdStringSplit &&
		gf.NativeData == StdStringSplitIdentityPtr() &&
		gf.FastArg2 != nil
}

func IsStdStringSubFunction(v Value) bool {
	gf := v.GoFunction()
	return gf != nil &&
		gf.NativeKind == NativeKindStdStringSub &&
		gf.NativeData == StdStringSubIdentityPtr() &&
		gf.FastArg2 != nil &&
		gf.FastArg3 != nil
}

func IsStdToNumberFunction(v Value) bool {
	gf := v.GoFunction()
	return gf != nil &&
		gf.NativeKind == NativeKindStdToNumber &&
		gf.NativeData == StdToNumberIdentityPtr() &&
		gf.FastArg1 != nil
}

func IsStdSelectFunction(v Value) bool {
	gf := v.GoFunction()
	return gf != nil &&
		gf.NativeKind == NativeKindStdSelect &&
		gf.NativeData == StdSelectIdentityPtr()
}

func IsStdPairsFunction(v Value) bool {
	gf := v.GoFunction()
	return gf != nil &&
		gf.NativeKind == NativeKindStdPairs &&
		gf.NativeData == StdPairsIdentityPtr()
}

func IsStdIPairsFunction(v Value) bool {
	gf := v.GoFunction()
	return gf != nil &&
		gf.NativeKind == NativeKindStdIPairs &&
		gf.NativeData == StdIPairsIdentityPtr()
}

func SelectReturnRange(selector Value, argCount int) (start int, countOnly bool, err error) {
	if argCount == 0 {
		return 0, false, fmt.Errorf("bad argument #1 to 'select'")
	}
	if selector.IsString() && selector.Str() == "#" {
		return argCount - 1, true, nil
	}
	n, ok := selector.ToNumber()
	if !ok {
		return 0, false, fmt.Errorf("bad argument #1 to 'select' (number or string expected)")
	}
	idx := int(n.Number())
	if idx < 0 {
		idx = argCount + idx
	}
	if idx < 1 {
		return 0, false, fmt.Errorf("bad argument #1 to 'select' (index out of range)")
	}
	if idx >= argCount {
		return argCount, false, nil
	}
	return idx, false, nil
}

func SelectResults(args []Value) ([]Value, error) {
	start, countOnly, err := SelectReturnRange(args[0], len(args))
	if err != nil {
		return nil, err
	}
	if countOnly {
		return []Value{IntValue(int64(start))}, nil
	}
	if start >= len(args) {
		return nil, nil
	}
	return args[start:], nil
}

func StringSplitProject(sv, sepv Value, index int64) (Value, error) {
	if !sv.IsString() || !sepv.IsString() {
		return NilValue(), fmt.Errorf("bad argument to 'string.split' (string expected)")
	}
	if index < 1 {
		return NilValue(), nil
	}
	return stringSplitProjectStrings(sv.Str(), sepv.Str(), index), nil
}

func StringSplitProjectSub(sv, sepv Value, index, start, end int64, hasEnd bool) (Value, error) {
	if !sv.IsString() || !sepv.IsString() {
		return NilValue(), fmt.Errorf("bad argument to 'string.split' (string expected)")
	}
	if index < 1 {
		return NilValue(), fmt.Errorf("bad argument #1 to 'string.sub' (string expected)")
	}
	token, ok := stringSplitProjectSlice(sv.Str(), sepv.Str(), index)
	if !ok {
		return NilValue(), fmt.Errorf("bad argument #1 to 'string.sub' (string expected)")
	}
	return StringValue(stringSubRaw(token, start, end, hasEnd)), nil
}

func StringSplitProjectSubToNumber(sv, sepv Value, index, start, end int64, hasEnd bool) (Value, error) {
	if !sv.IsString() || !sepv.IsString() {
		return NilValue(), fmt.Errorf("bad argument to 'string.split' (string expected)")
	}
	if index < 1 {
		return NilValue(), fmt.Errorf("bad argument #1 to 'string.sub' (string expected)")
	}
	token, ok := stringSplitProjectSlice(sv.Str(), sepv.Str(), index)
	if !ok {
		return NilValue(), fmt.Errorf("bad argument #1 to 'string.sub' (string expected)")
	}
	if v, ok := stringToNumberRaw(stringSubRaw(token, start, end, hasEnd)); ok {
		return v, nil
	}
	return NilValue(), nil
}

func stringSubRaw(s string, start, end int64, hasEnd bool) string {
	slen := len(s)
	i := int(start)
	j := slen
	if hasEnd {
		j = int(end)
	}
	if i < 0 {
		i = slen + i + 1
	}
	if i < 1 {
		i = 1
	}
	if j < 0 {
		j = slen + j + 1
	}
	if j > slen {
		j = slen
	}
	if i > j {
		return ""
	}
	return s[i-1 : j]
}

func stringToNumberRaw(raw string) (Value, bool) {
	if v, ok := parseFastDecimalInt(raw); ok {
		return v, true
	}
	s := strings.TrimSpace(raw)
	if s != raw {
		if v, ok := parseFastDecimalInt(s); ok {
			return v, true
		}
	}
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return IntValue(i), true
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return FloatValue(f), true
	}
	return NilValue(), false
}

func stringSplitProjectSlice(s, sep string, index int64) (string, bool) {
	if sep == "" {
		i := int(index) - 1
		if i < 0 || i >= len(s) {
			return "", false
		}
		return string(s[i]), true
	}

	token := int64(1)
	start := 0
	if len(sep) == 1 {
		sepByte := sep[0]
		for i := 0; i < len(s); i++ {
			if s[i] != sepByte {
				continue
			}
			if token == index {
				return s[start:i], true
			}
			token++
			start = i + 1
		}
		if token == index {
			return s[start:], true
		}
		return "", false
	}

	for {
		next := strings.Index(s[start:], sep)
		if next < 0 {
			if token == index {
				return s[start:], true
			}
			return "", false
		}
		end := start + next
		if token == index {
			return s[start:end], true
		}
		token++
		start = end + len(sep)
	}
}

func stringSplitProjectStrings(s, sep string, index int64) Value {
	if sep == "" {
		i := int(index) - 1
		if i < 0 || i >= len(s) {
			return NilValue()
		}
		return StringValue(string(s[i]))
	}

	token := int64(1)
	start := 0
	if len(sep) == 1 {
		sepByte := sep[0]
		for i := 0; i < len(s); i++ {
			if s[i] != sepByte {
				continue
			}
			if token == index {
				return StringValue(s[start:i])
			}
			token++
			start = i + 1
		}
		if token == index {
			return StringValue(s[start:])
		}
		return NilValue()
	}

	for {
		next := strings.Index(s[start:], sep)
		if next < 0 {
			if token == index {
				return StringValue(s[start:])
			}
			return NilValue()
		}
		end := start + next
		if token == index {
			return StringValue(s[start:end])
		}
		token++
		start = end + len(sep)
	}
}

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

		start := i - 1 // include the %
		for i < len(formatStr) && isFormatFlag(formatStr[i]) {
			i++
		}
		for i < len(formatStr) && formatStr[i] >= '0' && formatStr[i] <= '9' {
			i++
		}
		if i < len(formatStr) && formatStr[i] == '.' {
			i++
			for i < len(formatStr) && formatStr[i] >= '0' && formatStr[i] <= '9' {
				i++
			}
		}

		if i >= len(formatStr) {
			return NilValue(), fmt.Errorf("invalid format string")
		}
		spec := formatStr[i]
		i++
		fmtSpec := formatStr[start:i]

		if argIdx >= len(args) {
			return NilValue(), fmt.Errorf("bad argument #%d to 'string.format' (no value)", argIdx+1)
		}
		arg := args[argIdx]
		argIdx++

		switch spec {
		case 'd', 'i', 'u':
			n := toInt(arg)
			if !writeFastIntegerFormat(&buf, fmtSpec, spec, n) {
				goFmt := strings.Replace(fmtSpec, string(spec), "d", 1)
				buf.WriteString(fmt.Sprintf(goFmt, n))
			}
		case 'f', 'e', 'E', 'g', 'G':
			buf.WriteString(fmt.Sprintf(fmtSpec, toFloat(arg)))
		case 'x':
			n := toInt(arg)
			if !writeFastIntegerFormat(&buf, fmtSpec, spec, n) {
				goFmt := strings.Replace(fmtSpec, "x", "x", 1)
				buf.WriteString(fmt.Sprintf(goFmt, n))
			}
		case 'X':
			n := toInt(arg)
			if !writeFastIntegerFormat(&buf, fmtSpec, spec, n) {
				goFmt := strings.Replace(fmtSpec, "X", "X", 1)
				buf.WriteString(fmt.Sprintf(goFmt, n))
			}
		case 'o':
			n := toInt(arg)
			if !writeFastIntegerFormat(&buf, fmtSpec, spec, n) {
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
		return "nil", nil
	case TypeBool:
		if v.Bool() {
			return "true", nil
		}
		return "false", nil
	case TypeInt:
		return strconv.FormatInt(v.Int(), 10), nil
	case TypeFloat:
		f := v.Float()
		if math.IsInf(f, 1) {
			return "1e9999", nil
		}
		if math.IsInf(f, -1) {
			return "-1e9999", nil
		}
		if math.IsNaN(f) {
			return "(0/0)", nil
		}
		return strconv.FormatFloat(f, 'g', -1, 64), nil
	case TypeString:
		var buf strings.Builder
		buf.WriteByte('"')
		for _, c := range v.Str() {
			switch c {
			case '"':
				buf.WriteString(`\"`)
			case '\\':
				buf.WriteString(`\\`)
			case '\n':
				buf.WriteString(`\n`)
			case '\r':
				buf.WriteString(`\r`)
			case '\000':
				buf.WriteString(`\0`)
			default:
				buf.WriteRune(c)
			}
		}
		buf.WriteByte('"')
		return buf.String(), nil
	default:
		return "", fmt.Errorf("bad argument to 'string.format' (value has no literal form)")
	}
}

// StringFormatValue applies the stdlib string.format implementation to a
// pre-built argument slice. It is used by JIT op-exit paths after guarding the
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

// IsStdStringFormatFunction reports whether v is the stdlib string.format
// GoFunction installed by buildStringLib. This is intentionally an identity
// style guard for JIT fast paths: scripts cannot create GoFunction values.
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

func writeFastIntegerFormat(buf *strings.Builder, fmtSpec string, spec byte, n int64) bool {
	if len(fmtSpec) < 2 || fmtSpec[0] != '%' || fmtSpec[len(fmtSpec)-1] != spec {
		return false
	}
	pos := 1
	pad := byte(' ')
	if pos < len(fmtSpec)-1 && fmtSpec[pos] == '0' {
		pad = '0'
		pos++
	}
	width := 0
	for pos < len(fmtSpec)-1 && fmtSpec[pos] >= '0' && fmtSpec[pos] <= '9' {
		width = width*10 + int(fmtSpec[pos]-'0')
		pos++
	}
	if pos != len(fmtSpec)-1 {
		return false
	}

	var scratch [64]byte
	digits := scratch[:0]
	switch spec {
	case 'd', 'i', 'u':
		digits = strconv.AppendInt(digits, n, 10)
	case 'x':
		digits = strconv.AppendInt(digits, n, 16)
	case 'X':
		digits = strconv.AppendInt(digits, n, 16)
		for i, b := range digits {
			if b >= 'a' && b <= 'f' {
				digits[i] = b - ('a' - 'A')
			}
		}
	case 'o':
		digits = strconv.AppendInt(digits, n, 8)
	default:
		return false
	}

	if width <= len(digits) {
		buf.Write(digits)
		return true
	}
	padCount := width - len(digits)
	if pad == '0' && len(digits) > 0 && digits[0] == '-' {
		buf.WriteByte('-')
		for i := 0; i < padCount; i++ {
			buf.WriteByte('0')
		}
		buf.Write(digits[1:])
		return true
	}
	for i := 0; i < padCount; i++ {
		buf.WriteByte(pad)
	}
	buf.Write(digits)
	return true
}

type simpleFormatPart struct {
	lit   string
	spec  string
	verb  byte
	pad   byte
	width int
	prec  int
}

type simpleFormatProgram struct {
	formatStr string
	parts     []simpleFormatPart
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
	slot := simpleFormatFastSlot(formatStr)
	prog := simpleFormatFastCache[slot].Load()
	if prog != nil && prog.formatStr == formatStr {
		return prog
	}
	return nil
}

func storeSimpleFormatFast(formatStr string, prog *simpleFormatProgram) {
	if prog == nil {
		return
	}
	simpleFormatFastCache[simpleFormatFastSlot(formatStr)].Store(prog)
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
	parts := make([]simpleFormatPart, 0, 4)
	litStart := 0
	argCount := 0
	litBytes := 0
	for i := 0; i < len(formatStr); {
		if formatStr[i] != '%' {
			i++
			continue
		}
		if i+1 >= len(formatStr) {
			return nil, false, fmt.Errorf("invalid format string (ends with %%)")
		}
		if formatStr[i+1] == '%' {
			return nil, false, nil
		}
		if i > litStart {
			lit := formatStr[litStart:i]
			parts = append(parts, simpleFormatPart{lit: lit})
			litBytes += len(lit)
		}

		start := i
		i++
		for i < len(formatStr) && isFormatFlag(formatStr[i]) {
			if formatStr[i] != '0' {
				return nil, false, nil
			}
			i++
		}
		for i < len(formatStr) && formatStr[i] >= '0' && formatStr[i] <= '9' {
			i++
		}
		precisionStart := -1
		if i < len(formatStr) && formatStr[i] == '.' {
			precisionStart = i
			i++
			for i < len(formatStr) && formatStr[i] >= '0' && formatStr[i] <= '9' {
				i++
			}
		}
		if i >= len(formatStr) {
			return nil, false, fmt.Errorf("invalid format string")
		}
		verb := formatStr[i]
		i++
		switch verb {
		case 'd', 'i', 'u', 'x', 'X', 'o':
			if precisionStart >= 0 {
				return nil, false, nil
			}
			part, ok := compileSimpleIntegerFormatPart(formatStr[start:i], verb)
			if !ok {
				return nil, false, nil
			}
			parts = append(parts, part)
		case 'f':
			part, ok := compileSimpleFloatFormatPart(formatStr[start:i])
			if !ok {
				return nil, false, nil
			}
			parts = append(parts, part)
		case 's':
			if precisionStart >= 0 || i-start != 2 {
				return nil, false, nil
			}
			parts = append(parts, simpleFormatPart{spec: "%s", verb: verb})
		default:
			return nil, false, nil
		}
		argCount++
		litStart = i
	}
	if argCount == 0 {
		return nil, false, nil
	}
	if litStart < len(formatStr) {
		lit := formatStr[litStart:]
		parts = append(parts, simpleFormatPart{lit: lit})
		litBytes += len(lit)
	}
	return &simpleFormatProgram{
		formatStr: formatStr,
		parts:     parts,
		minArgs:   argCount + 1,
		litBytes:  litBytes,
		singleInt: argCount == 1 && simpleFormatHasSingleIntegerArg(parts),
	}, true, nil
}

func compileSimpleIntegerFormatPart(fmtSpec string, verb byte) (simpleFormatPart, bool) {
	if len(fmtSpec) < 2 || fmtSpec[0] != '%' || fmtSpec[len(fmtSpec)-1] != verb {
		return simpleFormatPart{}, false
	}
	pos := 1
	pad := byte(' ')
	if pos < len(fmtSpec)-1 && fmtSpec[pos] == '0' {
		pad = '0'
		pos++
	}
	width := 0
	for pos < len(fmtSpec)-1 && fmtSpec[pos] >= '0' && fmtSpec[pos] <= '9' {
		width = width*10 + int(fmtSpec[pos]-'0')
		pos++
	}
	if pos != len(fmtSpec)-1 {
		return simpleFormatPart{}, false
	}
	return simpleFormatPart{spec: fmtSpec, verb: verb, pad: pad, width: width}, true
}

func compileSimpleFloatFormatPart(fmtSpec string) (simpleFormatPart, bool) {
	if len(fmtSpec) < 3 || fmtSpec[0] != '%' || fmtSpec[len(fmtSpec)-1] != 'f' {
		return simpleFormatPart{}, false
	}
	pos := 1
	width := 0
	for pos < len(fmtSpec)-1 && fmtSpec[pos] >= '0' && fmtSpec[pos] <= '9' {
		width = width*10 + int(fmtSpec[pos]-'0')
		pos++
	}
	prec := 6
	if pos < len(fmtSpec)-1 && fmtSpec[pos] == '.' {
		pos++
		prec = 0
		if pos >= len(fmtSpec)-1 || fmtSpec[pos] < '0' || fmtSpec[pos] > '9' {
			return simpleFormatPart{}, false
		}
		for pos < len(fmtSpec)-1 && fmtSpec[pos] >= '0' && fmtSpec[pos] <= '9' {
			prec = prec*10 + int(fmtSpec[pos]-'0')
			pos++
		}
	}
	if pos != len(fmtSpec)-1 || prec > 9 {
		return simpleFormatPart{}, false
	}
	return simpleFormatPart{spec: fmtSpec, verb: 'f', width: width, prec: prec}, true
}

func simpleFormatHasSingleIntegerArg(parts []simpleFormatPart) bool {
	seen := false
	for _, part := range parts {
		if part.verb == 0 {
			continue
		}
		switch part.verb {
		case 'd', 'i', 'u', 'x', 'X', 'o':
			if seen {
				return false
			}
			seen = true
		default:
			return false
		}
	}
	return seen
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
		if part.verb == 0 {
			buf.WriteString(part.lit)
			continue
		}
		arg := args[argIdx]
		argIdx++
		switch part.verb {
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
		if part.verb == 0 {
			buf.WriteString(part.lit)
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
		switch part.verb {
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
		if part.verb == 0 {
			buf.WriteString(part.lit)
			continue
		}
		if argIdx >= len(args) {
			return "", fmt.Errorf("bad argument #%d to 'string.format' (no value)", argIdx+2)
		}
		arg := args[argIdx]
		argIdx++
		switch part.verb {
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
		if part.verb == 0 {
			buf.WriteString(part.lit)
			continue
		}
		writeCompiledIntegerFormat(&buf, part, n)
	}
	return buf.String()
}

func writeCompiledFloatFormat(buf *strings.Builder, part simpleFormatPart, f float64) {
	var scratch [128]byte
	digits := strconv.AppendFloat(scratch[:0], f, 'f', part.prec, 64)
	if part.width <= len(digits) {
		buf.Write(digits)
		return
	}
	for i := 0; i < part.width-len(digits); i++ {
		buf.WriteByte(' ')
	}
	buf.Write(digits)
}

func writeCompiledIntegerFormat(buf *strings.Builder, part simpleFormatPart, n int64) {
	var scratch [64]byte
	digits := scratch[:0]
	switch part.verb {
	case 'd', 'i', 'u':
		digits = strconv.AppendInt(digits, n, 10)
	case 'x':
		digits = strconv.AppendInt(digits, n, 16)
	case 'X':
		digits = strconv.AppendInt(digits, n, 16)
		for i, b := range digits {
			if b >= 'a' && b <= 'f' {
				digits[i] = b - ('a' - 'A')
			}
		}
	case 'o':
		digits = strconv.AppendInt(digits, n, 8)
	default:
		return
	}

	if part.width <= len(digits) {
		buf.Write(digits)
		return
	}
	padCount := part.width - len(digits)
	if part.pad == '0' && len(digits) > 0 && digits[0] == '-' {
		buf.WriteByte('-')
		for i := 0; i < padCount; i++ {
			buf.WriteByte('0')
		}
		buf.Write(digits[1:])
		return
	}
	for i := 0; i < padCount; i++ {
		buf.WriteByte(part.pad)
	}
	buf.Write(digits)
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

func isFormatFlag(b byte) bool {
	switch b {
	case '-', '+', ' ', '#', '0':
		return true
	default:
		return false
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

type luaPatternCaptureKind uint8

const (
	luaPatternCaptureText luaPatternCaptureKind = iota
	luaPatternCapturePosition
)

type luaPatternFrontier struct {
	slot  int
	class string
}

type luaPatternProgram struct {
	regex        string
	captureKinds []luaPatternCaptureKind
	captureSlots []int
	frontiers    []luaPatternFrontier
}

// luaPatternToRegex converts a Lua-style pattern string to a Go regex string.
func luaPatternToRegex(pattern string) string {
	return compileLuaPattern(pattern).regex
}

func luaPatternToRegexWithCaptures(pattern string) (string, []luaPatternCaptureKind) {
	prog := compileLuaPattern(pattern)
	return prog.regex, prog.captureKinds
}

func cachedLuaPatternRegexp(pattern string) (luaPatternProgram, *regexp.Regexp, error) {
	if cached, ok := compiledLuaPatternCache.Load(pattern); ok {
		entry := cached.(compiledLuaPatternCacheEntry)
		return entry.prog, entry.re, nil
	}
	prog := compileLuaPattern(pattern)
	re, err := regexp.Compile(prog.regex)
	if err != nil {
		return luaPatternProgram{}, nil, err
	}
	entry := compiledLuaPatternCacheEntry{prog: prog, re: re}
	actual, _ := compiledLuaPatternCache.LoadOrStore(pattern, entry)
	entry = actual.(compiledLuaPatternCacheEntry)
	return entry.prog, entry.re, nil
}

func compileLuaPattern(pattern string) luaPatternProgram {
	var buf strings.Builder
	var captureKinds []luaPatternCaptureKind
	var captureSlots []int
	var frontiers []luaPatternFrontier
	captureSlot := 1
	i := 0
	n := len(pattern)

	// Handle anchors
	if n > 0 && pattern[0] == '^' {
		buf.WriteByte('^')
		i++
	}

	// Track whether the previous item was a matchable item (can have a quantifier)
	prevMatchable := false

	for i < n {
		c := pattern[i]
		switch c {
		case '%':
			i++
			if i >= n {
				buf.WriteByte('%')
				prevMatchable = false
				continue
			}
			next := pattern[i]
			switch next {
			case 'd':
				buf.WriteString("[0-9]")
			case 'D':
				buf.WriteString("[^0-9]")
			case 'x':
				buf.WriteString("[0-9A-Fa-f]")
			case 'X':
				buf.WriteString("[^0-9A-Fa-f]")
			case 'a':
				buf.WriteString("[a-zA-Z]")
			case 'A':
				buf.WriteString("[^a-zA-Z]")
			case 'l':
				buf.WriteString("[a-z]")
			case 'L':
				buf.WriteString("[^a-z]")
			case 'u':
				buf.WriteString("[A-Z]")
			case 'U':
				buf.WriteString("[^A-Z]")
			case 's':
				buf.WriteString("[\\t\\n\\r\\f\\v ]")
			case 'S':
				buf.WriteString("[^\\t\\n\\r\\f\\v ]")
			case 'w':
				buf.WriteString("[a-zA-Z0-9]")
			case 'W':
				buf.WriteString("[^a-zA-Z0-9]")
			case 'p':
				buf.WriteString("[!-/:-@\\[-`{-~]")
			case 'P':
				buf.WriteString("[^!-/:-@\\[-`{-~]")
			case 'c':
				buf.WriteString("[\\x00-\\x1f\\x7f]")
			case 'C':
				buf.WriteString("[^\\x00-\\x1f\\x7f]")
			case 'z':
				buf.WriteString("\\x00")
			case 'Z':
				buf.WriteString("[^\\x00]")
			case 'f':
				if i+1 < n && pattern[i+1] == '[' {
					start := i + 1
					j := start + 1
					if j < n && pattern[j] == '^' {
						j++
					}
					if j < n && pattern[j] == ']' && j+1 < n {
						j++
					}
					for j < n && pattern[j] != ']' {
						j++
					}
					if j < n {
						buf.WriteString("()")
						frontiers = append(frontiers, luaPatternFrontier{slot: captureSlot, class: pattern[start : j+1]})
						captureSlot++
						i = j + 1
						prevMatchable = false
						continue
					}
				}
				buf.WriteString("[")
			case 'b':
				buf.WriteString("[")
			default:
				// Escape the literal character
				buf.WriteString(regexp.QuoteMeta(string(next)))
			}
			i++
			prevMatchable = true
		case '[':
			start := i
			i++
			if i < n && pattern[i] == '^' {
				i++
			}
			if i < n && pattern[i] == ']' && i+1 < n {
				i++
			}
			for i < n && pattern[i] != ']' {
				i++
			}
			if i < n {
				i++
			}
			buf.WriteString(luaBracketClassToRegex(pattern[start:i]))
			prevMatchable = true
		case '(':
			if i+1 < n && pattern[i+1] == ')' {
				buf.WriteString("()")
				captureKinds = append(captureKinds, luaPatternCapturePosition)
				captureSlots = append(captureSlots, captureSlot)
				captureSlot++
				i += 2
			} else {
				buf.WriteByte('(')
				captureKinds = append(captureKinds, luaPatternCaptureText)
				captureSlots = append(captureSlots, captureSlot)
				captureSlot++
				i++
			}
			prevMatchable = false
		case ')':
			buf.WriteByte(')')
			i++
			prevMatchable = false // In Lua, groups cannot be quantified
		case '.':
			// Lua patterns match any byte for '.', including newlines.
			buf.WriteString("(?s:.)")
			i++
			prevMatchable = true
		case '*', '+', '?':
			buf.WriteByte(c)
			i++
			prevMatchable = false
		case '-':
			// In Lua, '-' is a non-greedy repetition modifier (like *? in regex)
			// but only when it follows a matchable item. Otherwise it's a literal '-'.
			if prevMatchable {
				buf.WriteString("*?")
				prevMatchable = false
			} else {
				buf.WriteString("\\-")
				prevMatchable = true
			}
			i++
		case '$':
			if i == n-1 {
				buf.WriteByte('$')
				prevMatchable = false
			} else {
				buf.WriteString(regexp.QuoteMeta("$"))
				prevMatchable = true
			}
			i++
		default:
			// Check if the char needs escaping for Go regex
			if isRegexMeta(c) {
				buf.WriteByte('\\')
			}
			buf.WriteByte(c)
			i++
			prevMatchable = true
		}
	}

	return luaPatternProgram{
		regex:        buf.String(),
		captureKinds: captureKinds,
		captureSlots: captureSlots,
		frontiers:    frontiers,
	}
}

func luaBracketClassToRegex(class string) string {
	if len(class) < 2 || class[0] != '[' || class[len(class)-1] != ']' {
		return "["
	}
	negated := len(class) > 2 && class[1] == '^'
	bodyStart := 1
	if negated {
		bodyStart = 2
	}
	body := class[bodyStart : len(class)-1]
	if body == "" || body[len(body)-1] == '%' {
		return "["
	}
	if kind, ok := luaStandaloneClassEscape(body); ok {
		return luaPatternClassRegex(kind, negated)
	}

	var buf strings.Builder
	buf.WriteByte('[')
	if negated {
		buf.WriteByte('^')
	}
	for i := 0; i < len(body); i++ {
		if body[i] == '%' && i+1 < len(body) {
			i++
			ch := body[i]
			if content, ok := luaPositiveClassContent(ch); ok {
				buf.WriteString(content)
			} else {
				buf.WriteString(regexp.QuoteMeta(string(ch)))
			}
			continue
		}
		buf.WriteByte(body[i])
	}
	buf.WriteByte(']')
	return buf.String()
}

func luaStandaloneClassEscape(body string) (byte, bool) {
	if len(body) == 2 && body[0] == '%' {
		return body[1], true
	}
	return 0, false
}

func luaPatternClassRegex(kind byte, outerNegated bool) string {
	if positive, ok := luaPositiveClassContent(kind); ok {
		if outerNegated {
			return "[^" + positive + "]"
		}
		return "[" + positive + "]"
	}
	if positive, ok := luaNegatedClassContent(kind); ok {
		if outerNegated {
			return "[" + positive + "]"
		}
		return "[^" + positive + "]"
	}
	quoted := regexp.QuoteMeta(string(kind))
	if outerNegated {
		return "[^" + quoted + "]"
	}
	return "[" + quoted + "]"
}

func (p luaPatternProgram) findSubmatchIndex(re *regexp.Regexp, s string) []int {
	if len(p.frontiers) == 0 {
		return re.FindStringSubmatchIndex(s)
	}
	for _, loc := range re.FindAllStringSubmatchIndex(s, -1) {
		if p.frontiersMatch(s, loc) {
			return loc
		}
	}
	return nil
}

func (p luaPatternProgram) findAllSubmatchIndex(re *regexp.Regexp, s string) [][]int {
	matches := re.FindAllStringSubmatchIndex(s, -1)
	if len(p.frontiers) == 0 || len(matches) == 0 {
		return matches
	}
	out := matches[:0]
	for _, loc := range matches {
		if p.frontiersMatch(s, loc) {
			out = append(out, loc)
		}
	}
	return out
}

func (p luaPatternProgram) frontiersMatch(s string, loc []int) bool {
	for _, frontier := range p.frontiers {
		posSlot := frontier.slot * 2
		if posSlot >= len(loc) || loc[posSlot] < 0 {
			return false
		}
		if !luaFrontierMatches(s, loc[posSlot], frontier.class) {
			return false
		}
	}
	return true
}

func luaFrontierMatches(s string, pos int, class string) bool {
	prev := byte(0)
	if pos > 0 {
		prev = s[pos-1]
	}
	next := byte(0)
	if pos < len(s) {
		next = s[pos]
	}
	return !luaBracketClassByteContains(class, prev) && luaBracketClassByteContains(class, next)
}

func luaBracketClassByteContains(class string, b byte) bool {
	if len(class) < 2 || class[0] != '[' || class[len(class)-1] != ']' {
		return false
	}
	negated := len(class) > 2 && class[1] == '^'
	bodyStart := 1
	if negated {
		bodyStart = 2
	}
	body := class[bodyStart : len(class)-1]
	matched := luaClassBodyByteContains(body, b)
	if negated {
		return !matched
	}
	return matched
}

func luaClassBodyByteContains(body string, b byte) bool {
	for i := 0; i < len(body); i++ {
		ch := body[i]
		if ch == '%' && i+1 < len(body) {
			i++
			if luaClassEscapeByteContains(body[i], b) {
				return true
			}
			continue
		}
		if i+2 < len(body) && body[i+1] == '-' && body[i+2] != ']' {
			end := body[i+2]
			if ch <= b && b <= end {
				return true
			}
			i += 2
			continue
		}
		if ch == b {
			return true
		}
	}
	return false
}

func luaClassEscapeByteContains(kind byte, b byte) bool {
	switch kind {
	case 'd':
		return b >= '0' && b <= '9'
	case 'D':
		return !(b >= '0' && b <= '9')
	case 'x':
		return (b >= '0' && b <= '9') || (b >= 'A' && b <= 'F') || (b >= 'a' && b <= 'f')
	case 'X':
		return !luaClassEscapeByteContains('x', b)
	case 'a':
		return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
	case 'A':
		return !luaClassEscapeByteContains('a', b)
	case 'l':
		return b >= 'a' && b <= 'z'
	case 'L':
		return !luaClassEscapeByteContains('l', b)
	case 'u':
		return b >= 'A' && b <= 'Z'
	case 'U':
		return !luaClassEscapeByteContains('u', b)
	case 's':
		return b == '\t' || b == '\n' || b == '\r' || b == '\f' || b == '\v' || b == ' '
	case 'S':
		return !luaClassEscapeByteContains('s', b)
	case 'w':
		return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9')
	case 'W':
		return !luaClassEscapeByteContains('w', b)
	case 'p':
		return (b >= '!' && b <= '/') || (b >= ':' && b <= '@') || (b >= '[' && b <= '`') || (b >= '{' && b <= '~')
	case 'P':
		return !luaClassEscapeByteContains('p', b)
	case 'c':
		return b <= 0x1f || b == 0x7f
	case 'C':
		return !luaClassEscapeByteContains('c', b)
	case 'z':
		return b == 0
	case 'Z':
		return b != 0
	default:
		return b == kind
	}
}

func luaPositiveClassContent(kind byte) (string, bool) {
	switch kind {
	case 'd':
		return "0-9", true
	case 'x':
		return "0-9A-Fa-f", true
	case 'a':
		return "a-zA-Z", true
	case 'l':
		return "a-z", true
	case 'u':
		return "A-Z", true
	case 's':
		return "\\t\\n\\r\\f\\v ", true
	case 'w':
		return "a-zA-Z0-9", true
	case 'p':
		return "!-/:-@\\[-`{-~", true
	case 'c':
		return "\\x00-\\x1f\\x7f", true
	case 'z':
		return "\\x00", true
	default:
		return "", false
	}
}

func luaNegatedClassContent(kind byte) (string, bool) {
	switch kind {
	case 'D':
		return "0-9", true
	case 'X':
		return "0-9A-Fa-f", true
	case 'A':
		return "a-zA-Z", true
	case 'L':
		return "a-z", true
	case 'U':
		return "A-Z", true
	case 'S':
		return "\\t\\n\\r\\f\\v ", true
	case 'W':
		return "a-zA-Z0-9", true
	case 'P':
		return "!-/:-@\\[-`{-~", true
	case 'C':
		return "\\x00-\\x1f\\x7f", true
	case 'Z':
		return "\\x00", true
	default:
		return "", false
	}
}

func parseStandaloneBalancedPattern(pattern string) (bool, byte, byte) {
	if len(pattern) != 4 || pattern[0] != '%' || pattern[1] != 'b' {
		return false, 0, 0
	}
	return true, pattern[2], pattern[3]
}

func findBalancedRange(s string, open, close byte, from int) []int {
	for i := from; i < len(s); i++ {
		if s[i] != open {
			continue
		}
		if open == close {
			for j := i + 1; j < len(s); j++ {
				if s[j] == close {
					return []int{i, j + 1}
				}
			}
			return nil
		}
		depth := 1
		for j := i + 1; j < len(s); j++ {
			if s[j] == open {
				depth++
			}
			if s[j] == close {
				depth--
				if depth == 0 {
					return []int{i, j + 1}
				}
			}
		}
		return nil
	}
	return nil
}

func findAllBalancedRanges(s string, open, close byte) [][]int {
	var ranges [][]int
	next := 0
	for next < len(s) {
		loc := findBalancedRange(s, open, close, next)
		if loc == nil {
			break
		}
		ranges = append(ranges, loc)
		if loc[1] > next {
			next = loc[1]
		} else {
			next++
		}
	}
	return ranges
}

func replaceBalancedPatternString(s string, open, close byte, repl string, maxRepl int, count *int) string {
	result, _ := replaceBalancedPattern(s, open, close, maxRepl, count, func(loc []int) (string, error) {
		return expandLuaReplacement(s, loc, luaPatternProgram{}, repl), nil
	})
	return result
}

func replaceBalancedPatternRaw(s string, open, close byte, repl string, maxRepl int, count *int) string {
	result, _ := replaceBalancedPattern(s, open, close, maxRepl, count, func(_ []int) (string, error) {
		return repl, nil
	})
	return result
}

func replaceBalancedPatternTable(s string, open, close byte, repl *Table, maxRepl int, count *int) (string, error) {
	return replaceBalancedPattern(s, open, close, maxRepl, count, func(loc []int) (string, error) {
		key := StringValue(s[loc[0]:loc[1]])
		val := repl.RawGet(key)
		if val.IsNil() || (val.IsBool() && !val.Bool()) {
			return s[loc[0]:loc[1]], nil
		}
		if val.IsString() || val.IsNumber() {
			return val.String(), nil
		}
		return "", fmt.Errorf("invalid replacement value (a %s)", val.TypeName())
	})
}

func replaceBalancedPatternFunction(s string, open, close byte, fn Value, caller FunctionCaller, maxRepl int, count *int) (string, error) {
	return replaceBalancedPattern(s, open, close, maxRepl, count, func(loc []int) (string, error) {
		return callLuaReplacementFunction(s, loc, luaPatternProgram{}, fn, caller)
	})
}

func replaceBalancedPattern(s string, open, close byte, maxRepl int, count *int, repl func([]int) (string, error)) (string, error) {
	var b strings.Builder
	last := 0
	next := 0
	for next < len(s) {
		if maxRepl >= 0 && *count >= maxRepl {
			break
		}
		loc := findBalancedRange(s, open, close, next)
		if loc == nil {
			break
		}
		replacement, err := repl(loc)
		if err != nil {
			return "", err
		}
		b.WriteString(s[last:loc[0]])
		b.WriteString(replacement)
		last = loc[1]
		next = loc[1]
		(*count)++
	}
	if *count == 0 {
		return s, nil
	}
	b.WriteString(s[last:])
	return b.String(), nil
}

func replaceLuaPatternFunction(s string, re *regexp.Regexp, prog luaPatternProgram, fn Value, caller FunctionCaller, maxRepl int, count *int) (string, error) {
	matches := prog.findAllSubmatchIndex(re, s)
	if len(matches) == 0 {
		return s, nil
	}
	var b strings.Builder
	last := 0
	for _, loc := range matches {
		if maxRepl >= 0 && *count >= maxRepl {
			break
		}
		start, end := loc[0], loc[1]
		if start < last {
			continue
		}
		replacement, err := callLuaReplacementFunction(s, loc, prog, fn, caller)
		if err != nil {
			return "", err
		}
		b.WriteString(s[last:start])
		b.WriteString(replacement)
		last = end
		(*count)++
	}
	b.WriteString(s[last:])
	return b.String(), nil
}

func replaceLuaPatternString(s string, re *regexp.Regexp, prog luaPatternProgram, repl string, maxRepl int, count *int) string {
	matches := prog.findAllSubmatchIndex(re, s)
	if len(matches) == 0 {
		return s
	}
	var b strings.Builder
	last := 0
	for _, loc := range matches {
		if maxRepl >= 0 && *count >= maxRepl {
			break
		}
		start, end := loc[0], loc[1]
		if start < last {
			continue
		}
		b.WriteString(s[last:start])
		b.WriteString(expandLuaReplacement(s, loc, prog, repl))
		last = end
		(*count)++
	}
	b.WriteString(s[last:])
	return b.String()
}

func replaceLuaPatternTable(s string, re *regexp.Regexp, prog luaPatternProgram, repl *Table, maxRepl int, count *int) (string, error) {
	matches := prog.findAllSubmatchIndex(re, s)
	if len(matches) == 0 {
		return s, nil
	}
	var b strings.Builder
	last := 0
	for _, loc := range matches {
		if maxRepl >= 0 && *count >= maxRepl {
			break
		}
		start, end := loc[0], loc[1]
		if start < last {
			continue
		}
		key := luaReplacementTableKey(s, loc, prog)
		val := repl.RawGet(key)
		replacement := s[start:end]
		if !val.IsNil() && !(val.IsBool() && !val.Bool()) {
			if val.IsString() || val.IsNumber() {
				replacement = val.String()
			} else {
				return "", fmt.Errorf("invalid replacement value (a %s)", val.TypeName())
			}
		}
		b.WriteString(s[last:start])
		b.WriteString(replacement)
		last = end
		(*count)++
	}
	b.WriteString(s[last:])
	return b.String(), nil
}

func callLuaReplacementFunction(s string, loc []int, prog luaPatternProgram, fn Value, caller FunctionCaller) (string, error) {
	results, err := callGScriptFunction(fn, luaReplacementFunctionArgs(s, loc, prog), caller)
	if err != nil {
		return "", err
	}
	replacement := s[loc[0]:loc[1]]
	if len(results) == 0 {
		return replacement, nil
	}
	val := results[0]
	if val.IsNil() || (val.IsBool() && !val.Bool()) {
		return replacement, nil
	}
	if val.IsString() || val.IsNumber() {
		return val.String(), nil
	}
	return "", fmt.Errorf("invalid replacement value (a %s)", val.TypeName())
}

func callGScriptFunction(fn Value, args []Value, caller FunctionCaller) ([]Value, error) {
	if caller != nil {
		return caller(fn, args)
	}
	if gf := fn.GoFunction(); gf != nil {
		return gf.Fn(args)
	}
	return nil, fmt.Errorf("attempt to call a %s value", fn.TypeName())
}

func luaReplacementFunctionArgs(s string, loc []int, prog luaPatternProgram) []Value {
	if len(prog.captureKinds) == 0 || len(prog.captureSlots) == 0 {
		return []Value{StringValue(s[loc[0]:loc[1]])}
	}
	args := make([]Value, 0, len(prog.captureSlots))
	for i, slot := range prog.captureSlots {
		pos := slot * 2
		if pos+1 >= len(loc) || loc[pos] < 0 {
			args = append(args, NilValue())
			continue
		}
		if prog.captureKinds[i] == luaPatternCapturePosition {
			args = append(args, IntValue(int64(loc[pos]+1)))
		} else {
			args = append(args, StringValue(s[loc[pos]:loc[pos+1]]))
		}
	}
	return args
}

func luaReplacementTableKey(s string, loc []int, prog luaPatternProgram) Value {
	if len(prog.captureKinds) == 0 || len(prog.captureSlots) == 0 {
		return StringValue(s[loc[0]:loc[1]])
	}
	slot := prog.captureSlots[0]
	pos := slot * 2
	if pos+1 >= len(loc) || loc[pos] < 0 {
		return StringValue("")
	}
	if prog.captureKinds[0] == luaPatternCapturePosition {
		return IntValue(int64(loc[pos] + 1))
	}
	return StringValue(s[loc[pos]:loc[pos+1]])
}

func validateLuaReplacementString(repl string, captureCount int) error {
	for i := 0; i < len(repl); i++ {
		if repl[i] != '%' {
			continue
		}
		if i+1 >= len(repl) {
			return fmt.Errorf("invalid use of '%%' in replacement string")
		}
		i++
		ch := repl[i]
		if ch == '%' {
			continue
		}
		if ch >= '0' && ch <= '9' {
			idx := int(ch - '0')
			if idx == 0 {
				continue
			}
			if captureCount == 0 {
				if idx == 1 {
					continue
				}
			} else if idx <= captureCount {
				continue
			}
			return fmt.Errorf("invalid capture index %%%c", ch)
		}
		return fmt.Errorf("invalid use of '%%' in replacement string")
	}
	return nil
}

func expandLuaReplacement(s string, loc []int, prog luaPatternProgram, repl string) string {
	var b strings.Builder
	for i := 0; i < len(repl); i++ {
		if repl[i] != '%' || i+1 >= len(repl) {
			b.WriteByte(repl[i])
			continue
		}
		i++
		ch := repl[i]
		if ch == '%' {
			b.WriteByte('%')
			continue
		}
		if ch >= '0' && ch <= '9' {
			idx := int(ch - '0')
			if idx == 0 {
				b.WriteString(s[loc[0]:loc[1]])
			} else if idx == 1 && len(prog.captureSlots) == 0 {
				b.WriteString(s[loc[0]:loc[1]])
			} else if idx > 0 && idx <= len(prog.captureSlots) {
				slot := prog.captureSlots[idx-1]
				pos := slot * 2
				if prog.captureKinds[idx-1] == luaPatternCapturePosition && pos+1 < len(loc) && loc[pos] >= 0 {
					b.WriteString(strconv.Itoa(loc[pos] + 1))
				} else if pos+1 < len(loc) && loc[pos] >= 0 {
					b.WriteString(s[loc[pos]:loc[pos+1]])
				}
			}
			continue
		}
		b.WriteByte('%')
		b.WriteByte(ch)
	}
	return b.String()
}

// isRegexMeta returns true if the byte is a Go regex metacharacter that
// needs escaping in a literal context.
func isRegexMeta(c byte) bool {
	switch c {
	case '\\', '{', '}', '|':
		return true
	}
	return false
}
