package modules

import (
	"fmt"

	"github.com/never-labs/leia/internal/runtime"
	stdlibstring "github.com/never-labs/leia/internal/support/stringlib"
)

// BuildString creates the "string" standard-library module.
//
// Most pure string helpers are assembled in stdlibrt. The runtime-owned
// native string substrate entries below are intentionally reused as GoFunction
// objects because VM/JIT guards, Lua pattern caches, simple-format caches,
// split projections, and function-valued gsub replacements still depend on
// runtime-native identities and execution-engine callbacks.
func BuildString(caller ScriptFunctionCaller, maxHostResult func() int64) *Table {
	t := NewTable()
	max := func() int64 { return hostResultLimit(maxHostResult) }

	var runtimeLib *runtime.Table
	runtimeEntry := func(name string) Value {
		if runtimeLib == nil {
			runtimeLib = runtime.BuildStringLibWithCaller(caller, maxHostResult)
		}
		return runtimeLib.RawGetString(name)
	}
	useRuntimeEntry := func(name string) {
		t.RawSetString(name, runtimeEntry(name))
	}
	set := func(name string, fn func([]Value) ([]Value, error)) {
		t.RawSet(StringValue(name), FunctionValue(&GoFunction{
			Name: "string." + name,
			Fn:   fn,
		}))
	}
	useRuntimeEntry("sub")
	useRuntimeEntry("byte")
	useRuntimeEntry("find")
	useRuntimeEntry("match")
	useRuntimeEntry("gmatch")
	useRuntimeEntry("gsub")
	useRuntimeEntry("format")
	useRuntimeEntry("split")

	set("len", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'string.len' (string expected)")
		}
		return []Value{IntValue(int64(StringLen(args[0])))}, nil
	})

	set("pack", func(args []Value) ([]Value, error) { return binaryPackValues("string.pack", args, max()) })
	set("unpack", func(args []Value) ([]Value, error) { return binaryUnpackValues("string.unpack", args) })
	set("packsize", func(args []Value) ([]Value, error) { return binarySizeValues("string.packsize", args) })

	set("upper", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'string.upper' (string expected)")
		}
		return []Value{StringValue(stdlibstring.Upper(args[0].Str()))}, nil
	})
	set("lower", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'string.lower' (string expected)")
		}
		return []Value{StringValue(stdlibstring.Lower(args[0].Str()))}, nil
	})
	set("rep", func(args []Value) ([]Value, error) {
		return repeatString("string.rep", args, max(), true)
	})
	set("reverse", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'string.reverse' (string expected)")
		}
		return []Value{StringValue(stdlibstring.Reverse(args[0].Str()))}, nil
	})
	set("char", func(args []Value) ([]Value, error) {
		if err := CheckProjectedHostStringBytes(max(), len(args)); err != nil {
			return nil, err
		}
		values := make([]int64, 0, len(args))
		for _, arg := range args {
			values = append(values, toInt(arg))
		}
		buf, ok := stdlibstring.CharBytes(values)
		if !ok {
			return nil, fmt.Errorf("bad argument to 'string.char' (value out of range)")
		}
		return []Value{StringValue(string(buf))}, nil
	})

	installTrim(t)
	installPredicates(t)
	installReplaceAll(t)

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
		if err := CheckProjectedHostStringBytes(max(), stdlibstring.JoinProjectedLen(parts, sep)); err != nil {
			return nil, err
		}
		return []Value{StringValue(stdlibstring.Join(parts, sep))}, nil
	})
	set("title", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'string.title' (string expected)")
		}
		return []Value{StringValue(stdlibstring.Title(args[0].Str()))}, nil
	})
	set("padLeft", func(args []Value) ([]Value, error) {
		return padString("string.padLeft", args, max(), true)
	})
	set("padRight", func(args []Value) ([]Value, error) {
		return padString("string.padRight", args, max(), false)
	})
	set("repeat", func(args []Value) ([]Value, error) {
		return repeatString("string.repeat", args, max(), false)
	})
	set("isNumeric", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'string.isNumeric' (string expected)")
		}
		return []Value{BoolValue(stdlibstring.IsNumeric(args[0].Str()))}, nil
	})

	return t
}

// RefreshString updates an existing string module in place, preserving module
// identity for require/package.loaded users.
func RefreshString(t *Table, caller ScriptFunctionCaller, maxHostResult func() int64) *Table {
	if t == nil {
		return BuildString(caller, maxHostResult)
	}
	fresh := BuildString(caller, maxHostResult)
	for key, val, ok := fresh.Next(NilValue()); ok; key, val, ok = fresh.Next(key) {
		t.RawSet(key, val)
	}
	return t
}

func repeatString(apiName string, args []Value, max int64, allowSep bool) ([]Value, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("bad argument to '%s'", apiName)
	}
	if !args[0].IsString() {
		return nil, fmt.Errorf("bad argument #1 to '%s' (string expected)", apiName)
	}
	s := args[0].Str()
	n := int(toInt(args[1]))
	if n <= 0 {
		return []Value{StringValue("")}, nil
	}
	sep := ""
	if allowSep && len(args) >= 3 && args[2].IsString() {
		sep = args[2].Str()
	}
	if err := runtime.CheckProjectedRepeatedStringBytes(max, len(s), n, len(sep)); err != nil {
		return nil, err
	}
	if sep != "" {
		return []Value{StringValue(stdlibstring.RepeatJoin(s, n, sep))}, nil
	}
	return []Value{StringValue(stdlibstring.Repeat(s, n))}, nil
}

func installTrim(t *Table) {
	install := func(name string, trimDefault func(string) string, trimCutset func(string, string) string) {
		fn := func(args []Value) ([]Value, error) {
			if len(args) < 1 || !args[0].IsString() {
				return nil, fmt.Errorf("bad argument #1 to 'string.%s' (string expected)", name)
			}
			if len(args) >= 2 && args[1].IsString() {
				return []Value{StringValue(trimCutset(args[0].Str(), args[1].Str()))}, nil
			}
			return []Value{StringValue(trimDefault(args[0].Str()))}, nil
		}
		fast1 := func(a Value) (Value, error) {
			if !a.IsString() {
				return NilValue(), fmt.Errorf("bad argument #1 to 'string.%s' (string expected)", name)
			}
			return StringValue(trimDefault(a.Str())), nil
		}
		fast2 := func(a, b Value) (Value, error) {
			if !a.IsString() || !b.IsString() {
				return NilValue(), fmt.Errorf("bad argument to 'string.%s' (string expected)", name)
			}
			return StringValue(trimCutset(a.Str(), b.Str())), nil
		}
		gf := &GoFunction{
			Name: "string." + name,
			Fn:   fn,
			Fast1: func(args []Value) (Value, error) {
				if len(args) >= 2 {
					return fast2(args[0], args[1])
				}
				if len(args) == 1 {
					return fast1(args[0])
				}
				return NilValue(), fmt.Errorf("bad argument #1 to 'string.%s' (string expected)", name)
			},
			FastArg1: fast1,
			FastArg2: fast2,
		}
		t.RawSetString(name, FunctionValue(gf))
	}
	install("trim", stdlibstring.TrimSpace, stdlibstring.Trim)
	install("trimLeft", stdlibstring.TrimLeftSpace, stdlibstring.TrimLeft)
	install("trimRight", stdlibstring.TrimRightSpace, stdlibstring.TrimRight)
}

func installPredicates(t *Table) {
	install := func(name string, pred func(string, string) bool) {
		fn := func(args []Value) ([]Value, error) {
			if len(args) < 2 || !args[0].IsString() || !args[1].IsString() {
				return nil, fmt.Errorf("bad argument to 'string.%s' (string expected)", name)
			}
			return []Value{BoolValue(pred(args[0].Str(), args[1].Str()))}, nil
		}
		fast := func(a, b Value) (Value, error) {
			if !a.IsString() || !b.IsString() {
				return NilValue(), fmt.Errorf("bad argument to 'string.%s' (string expected)", name)
			}
			return BoolValue(pred(a.Str(), b.Str())), nil
		}
		t.RawSetString(name, FunctionValue(&GoFunction{
			Name: "string." + name,
			Fn:   fn,
			Fast1: func(args []Value) (Value, error) {
				if len(args) < 2 {
					return NilValue(), fmt.Errorf("bad argument to 'string.%s' (string expected)", name)
				}
				return fast(args[0], args[1])
			},
			FastArg2: fast,
		}))
	}
	install("hasPrefix", stdlibstring.HasPrefix)
	install("hasSuffix", stdlibstring.HasSuffix)
	install("contains", stdlibstring.Contains)

	fn := func(args []Value) ([]Value, error) {
		if len(args) < 2 || !args[0].IsString() || !args[1].IsString() {
			return nil, fmt.Errorf("bad argument to 'string.count' (string expected)")
		}
		return []Value{IntValue(int64(stdlibstring.Count(args[0].Str(), args[1].Str())))}, nil
	}
	fast := func(a, b Value) (Value, error) {
		if !a.IsString() || !b.IsString() {
			return NilValue(), fmt.Errorf("bad argument to 'string.count' (string expected)")
		}
		return IntValue(int64(stdlibstring.Count(a.Str(), b.Str()))), nil
	}
	t.RawSetString("count", FunctionValue(&GoFunction{
		Name: "string.count",
		Fn:   fn,
		Fast1: func(args []Value) (Value, error) {
			if len(args) < 2 {
				return NilValue(), fmt.Errorf("bad argument to 'string.count' (string expected)")
			}
			return fast(args[0], args[1])
		},
		FastArg2: fast,
	}))
}

func installReplaceAll(t *Table) {
	fn := func(args []Value) ([]Value, error) {
		if len(args) < 3 || !args[0].IsString() || !args[1].IsString() || !args[2].IsString() {
			return nil, fmt.Errorf("bad argument to 'string.replaceAll' (string expected)")
		}
		return []Value{StringValue(stdlibstring.ReplaceAll(args[0].Str(), args[1].Str(), args[2].Str()))}, nil
	}
	fast3 := func(a, b, c Value) (Value, error) {
		if !a.IsString() || !b.IsString() || !c.IsString() {
			return NilValue(), fmt.Errorf("bad argument to 'string.replaceAll' (string expected)")
		}
		return StringValue(stdlibstring.ReplaceAll(a.Str(), b.Str(), c.Str())), nil
	}
	t.RawSetString("replaceAll", FunctionValue(&GoFunction{
		Name: "string.replaceAll",
		Fn:   fn,
		Fast1: func(args []Value) (Value, error) {
			if len(args) < 3 {
				return NilValue(), fmt.Errorf("bad argument to 'string.replaceAll' (string expected)")
			}
			return fast3(args[0], args[1], args[2])
		},
		FastArg3: fast3,
	}))
}

func padString(apiName string, args []Value, max int64, left bool) ([]Value, error) {
	if len(args) < 2 || !args[0].IsString() {
		return nil, fmt.Errorf("bad argument to '%s'", apiName)
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
		if err := CheckProjectedHostStringBytes(max, n); err != nil {
			return nil, err
		}
	}
	if left {
		return []Value{StringValue(stdlibstring.PadLeft(s, n, pad))}, nil
	}
	return []Value{StringValue(stdlibstring.PadRight(s, n, pad))}, nil
}
