package runtime

// Builtin and standard-global registration for the tree-walking interpreter:
// the setmetatable helper, registerBuiltins, and the coroutine library
// (registerCoroutineLib / resumeCoroutine / yieldFromCoroutine).
// Moved verbatim from interpreter.go (pure code movement).

import (
	"errors"
	"fmt"
	"math"
	"os"
	goruntime "runtime"
	"strings"
)

func setMetatableValue(table, metatable Value) (Value, error) {
	if !table.IsTable() {
		return NilValue(), fmt.Errorf("bad argument #1 to 'setmetatable' (table expected, got %s)", table.TypeName())
	}
	tbl := table.Table()
	if mt := tbl.GetMetatable(); mt != nil && !mt.RawGetString("__metatable").IsNil() {
		return NilValue(), fmt.Errorf("cannot change a protected metatable")
	}
	if metatable.IsNil() {
		tbl.SetMetatable(nil)
	} else if metatable.IsTable() {
		tbl.SetMetatable(metatable.Table())
	} else {
		return NilValue(), fmt.Errorf("bad argument #2 to 'setmetatable' (nil or table expected, got %s)", metatable.TypeName())
	}
	return table, nil
}
func (interp *Interpreter) registerBuiltins() {
	interp.globals.Define("print", FunctionValue(&GoFunction{
		Name: "print",
		Fn: func(args []Value) ([]Value, error) {
			parts := make([]string, len(args))
			for i, a := range args {
				parts[i] = a.String()
			}
			line := strings.Join(parts, "\t")
			fmt.Println(line)
			interp.output = append(interp.output, line)
			return nil, nil
		},
	}))

	typeNames := [TypeChannel + 1]Value{
		TypeNil:       StringValue("nil"),
		TypeBool:      StringValue("boolean"),
		TypeInt:       StringValue("number"),
		TypeFloat:     StringValue("number"),
		TypeString:    StringValue("string"),
		TypeTable:     StringValue("table"),
		TypeFunction:  StringValue("function"),
		TypeCoroutine: StringValue("coroutine"),
		TypeChannel:   StringValue("channel"),
	}
	unknownTypeName := StringValue("unknown")
	typeNameValue := func(v Value) Value {
		t := v.Type()
		if int(t) < len(typeNames) {
			if tv := typeNames[t]; !tv.IsNil() {
				return tv
			}
		}
		return unknownTypeName
	}
	interp.globals.Define("type", FunctionValue(&GoFunction{
		Name: "type",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) == 0 {
				return nil, fmt.Errorf("bad argument #1 to 'type' (value expected)")
			}
			return []Value{typeNameValue(args[0])}, nil
		},
		FastArg1: func(arg Value) (Value, error) {
			return typeNameValue(arg), nil
		},
	}))

	interp.globals.Define("tostring", FunctionValue(&GoFunction{
		Name: "tostring",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) == 0 {
				return nil, fmt.Errorf("bad argument #1 to 'tostring' (value expected)")
			}
			if args[0].IsInt() {
				if v, ok := CachedIntStringValue(args[0].Int()); ok {
					return []Value{v}, nil
				}
			}
			s, err := interp.luaToString(args[0])
			if err != nil {
				return nil, err
			}
			return []Value{StringValue(s)}, nil
		},
		FastArg1: func(arg Value) (Value, error) {
			if arg.IsInt() {
				if v, ok := CachedIntStringValue(arg.Int()); ok {
					return v, nil
				}
			}
			s, err := interp.luaToString(arg)
			if err != nil {
				return NilValue(), err
			}
			return StringValue(s), nil
		},
		Fast1: func(args []Value) (Value, error) {
			if len(args) == 0 {
				return NilValue(), fmt.Errorf("bad argument #1 to 'tostring' (value expected)")
			}
			if args[0].IsInt() {
				if v, ok := CachedIntStringValue(args[0].Int()); ok {
					return v, nil
				}
			}
			s, err := interp.luaToString(args[0])
			if err != nil {
				return NilValue(), err
			}
			return StringValue(s), nil
		},
	}))

	interp.globals.Define("tonumber", FunctionValue(&GoFunction{
		Name: "tonumber",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) == 0 {
				return nil, fmt.Errorf("bad argument #1 to 'tonumber' (value expected)")
			}
			if len(args) >= 2 {
				v, ok := tonumberWithBase(args[0], args[1])
				if !ok {
					return []Value{NilValue()}, nil
				}
				return []Value{v}, nil
			}
			v, ok := args[0].ToNumber()
			if !ok {
				return []Value{NilValue()}, nil
			}
			return []Value{v}, nil
		},
		Fast1: func(args []Value) (Value, error) {
			if len(args) == 0 {
				return NilValue(), fmt.Errorf("bad argument #1 to 'tonumber' (value expected)")
			}
			if len(args) >= 2 {
				v, ok := tonumberWithBase(args[0], args[1])
				if !ok {
					return NilValue(), nil
				}
				return v, nil
			}
			v, ok := args[0].ToNumber()
			if !ok {
				return NilValue(), nil
			}
			return v, nil
		},
		FastArg1: func(arg Value) (Value, error) {
			v, ok := arg.ToNumber()
			if !ok {
				return NilValue(), nil
			}
			return v, nil
		},
		FastArg2: func(value, base Value) (Value, error) {
			v, ok := tonumberWithBase(value, base)
			if !ok {
				return NilValue(), nil
			}
			return v, nil
		},
		NativeKind: NativeKindStdToNumber,
		NativeData: StdToNumberIdentityPtr(),
	}))

	interp.globals.Define("collectgarbage", FunctionValue(&GoFunction{
		Name: "collectgarbage",
		Fn: func(args []Value) ([]Value, error) {
			option := "collect"
			if len(args) > 0 && !args[0].IsNil() {
				if args[0].Type() != TypeString {
					return nil, fmt.Errorf("bad argument #1 to 'collectgarbage' (string expected, got %s)", args[0].TypeName())
				}
				option = args[0].Str()
			}

			switch option {
			case "collect":
				goruntime.GC()
				return []Value{IntValue(0)}, nil
			case "stop":
				interp.gcRunning = false
				return []Value{IntValue(0)}, nil
			case "restart":
				interp.gcRunning = true
				return []Value{IntValue(0)}, nil
			case "isrunning":
				return []Value{BoolValue(interp.gcRunning)}, nil
			case "count":
				var stats goruntime.MemStats
				goruntime.ReadMemStats(&stats)
				return []Value{FloatValue(float64(stats.Alloc) / 1024)}, nil
			case "stats":
				var stats goruntime.MemStats
				goruntime.ReadMemStats(&stats)
				tbl := NewTable()
				tbl.RawSetString("allocBytes", IntValue(int64(stats.Alloc)))
				tbl.RawSetString("allocKB", FloatValue(float64(stats.Alloc)/1024))
				tbl.RawSetString("sysBytes", IntValue(int64(stats.Sys)))
				tbl.RawSetString("heapObjects", IntValue(int64(stats.HeapObjects)))
				tbl.RawSetString("numGC", IntValue(int64(stats.NumGC)))
				tbl.RawSetString("rootLog", IntValue(GCRootLogSize()))
				tbl.RawSetString("running", BoolValue(interp.gcRunning))
				tbl.RawSetString("mode", StringValue(interp.gcMode))
				return []Value{TableValue(tbl)}, nil
			case "step":
				goruntime.GC()
				return []Value{BoolValue(false)}, nil
			case "incremental", "generational":
				old := interp.gcMode
				interp.gcMode = option
				return []Value{StringValue(old)}, nil
			default:
				return nil, fmt.Errorf("bad argument #1 to 'collectgarbage' (invalid option '%s')", option)
			}
		},
	}))

	interp.globals.Define("setmetatable", FunctionValue(&GoFunction{
		Name: "setmetatable",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) < 2 {
				return nil, fmt.Errorf("bad argument to 'setmetatable' (table expected)")
			}
			v, err := setMetatableValue(args[0], args[1])
			if err != nil {
				return nil, err
			}
			return []Value{v}, nil
		},
		FastArg2: func(table, metatable Value) (Value, error) {
			return setMetatableValue(table, metatable)
		},
	}))

	interp.globals.Define("getmetatable", FunctionValue(&GoFunction{
		Name: "getmetatable",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) == 0 {
				return nil, fmt.Errorf("bad argument to 'getmetatable' (value expected)")
			}
			if !args[0].IsTable() {
				return []Value{NilValue()}, nil
			}
			mt := args[0].Table().GetMetatable()
			if mt == nil {
				return []Value{NilValue()}, nil
			}
			if protected := mt.RawGetString("__metatable"); !protected.IsNil() {
				return []Value{protected}, nil
			}
			return []Value{TableValue(mt)}, nil
		},
		FastArg1: func(v Value) (Value, error) {
			if !v.IsTable() {
				return NilValue(), nil
			}
			mt := v.Table().GetMetatable()
			if mt == nil {
				return NilValue(), nil
			}
			if protected := mt.RawGetString("__metatable"); !protected.IsNil() {
				return protected, nil
			}
			return TableValue(mt), nil
		},
		NativeKind: NativeKindStdGetMetatable,
		NativeData: StdGetMetatableIdentityPtr(),
	}))

	interp.globals.Define("rawget", FunctionValue(&GoFunction{
		Name: "rawget",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) < 2 {
				return nil, fmt.Errorf("bad argument to 'rawget' (table expected)")
			}
			if !args[0].IsTable() {
				return nil, fmt.Errorf("bad argument #1 to 'rawget' (table expected, got %s)", args[0].TypeName())
			}
			return []Value{args[0].Table().RawGet(args[1])}, nil
		},
		FastArg2: func(table, key Value) (Value, error) {
			if !table.IsTable() {
				return NilValue(), fmt.Errorf("bad argument #1 to 'rawget' (table expected, got %s)", table.TypeName())
			}
			return table.Table().RawGet(key), nil
		},
		NativeKind: NativeKindStdRawGet,
		NativeData: StdRawGetIdentityPtr(),
	}))

	interp.globals.Define("rawset", FunctionValue(&GoFunction{
		Name: "rawset",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) < 3 {
				return nil, fmt.Errorf("bad argument to 'rawset' (table expected)")
			}
			if !args[0].IsTable() {
				return nil, fmt.Errorf("bad argument #1 to 'rawset' (table expected, got %s)", args[0].TypeName())
			}
			if args[1].IsNil() {
				return nil, fmt.Errorf("table index is nil")
			}
			if args[1].IsFloat() && math.IsNaN(args[1].Float()) {
				return nil, fmt.Errorf("table index is NaN")
			}
			args[0].Table().RawSet(args[1], args[2])
			return []Value{args[0]}, nil
		},
		FastArg3: func(table, key, value Value) (Value, error) {
			if !table.IsTable() {
				return NilValue(), fmt.Errorf("bad argument #1 to 'rawset' (table expected, got %s)", table.TypeName())
			}
			if key.IsNil() {
				return NilValue(), fmt.Errorf("table index is nil")
			}
			if key.IsFloat() && math.IsNaN(key.Float()) {
				return NilValue(), fmt.Errorf("table index is NaN")
			}
			table.Table().RawSet(key, value)
			return table, nil
		},
		NativeKind: NativeKindStdRawSet,
		NativeData: StdRawSetIdentityPtr(),
	}))

	interp.globals.Define("rawequal", FunctionValue(&GoFunction{
		Name: "rawequal",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) < 2 {
				return nil, fmt.Errorf("bad argument to 'rawequal' (two values expected)")
			}
			return []Value{BoolValue(args[0].Equal(args[1]))}, nil
		},
	}))

	interp.globals.Define("rawlen", FunctionValue(&GoFunction{
		Name: "rawlen",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) == 0 {
				return nil, fmt.Errorf("bad argument to 'rawlen' (value expected)")
			}
			a := args[0]
			switch a.Type() {
			case TypeString:
				return []Value{IntValue(int64(StringLen(a)))}, nil
			case TypeTable:
				return []Value{IntValue(int64(a.Table().Length()))}, nil
			default:
				return nil, fmt.Errorf("bad argument to 'rawlen' (table or string expected, got %s)", a.TypeName())
			}
		},
		FastArg1: func(a Value) (Value, error) {
			switch a.Type() {
			case TypeString:
				return IntValue(int64(StringLen(a))), nil
			case TypeTable:
				return IntValue(int64(a.Table().Length())), nil
			default:
				return NilValue(), fmt.Errorf("bad argument to 'rawlen' (table or string expected, got %s)", a.TypeName())
			}
		},
		NativeKind: NativeKindStdRawLen,
		NativeData: StdRawLenIdentityPtr(),
	}))

	interp.globals.Define("len", FunctionValue(&GoFunction{
		Name: "len",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) == 0 {
				return nil, fmt.Errorf("bad argument to 'len' (value expected)")
			}
			a := args[0]
			switch a.Type() {
			case TypeString:
				return []Value{IntValue(int64(StringLen(a)))}, nil
			case TypeTable:
				return []Value{IntValue(int64(a.Table().Length()))}, nil
			default:
				return nil, fmt.Errorf("bad argument to 'len' (table or string expected, got %s)", a.TypeName())
			}
		},
		FastArg1: func(a Value) (Value, error) {
			switch a.Type() {
			case TypeString:
				return IntValue(int64(StringLen(a))), nil
			case TypeTable:
				return IntValue(int64(a.Table().Length())), nil
			default:
				return NilValue(), fmt.Errorf("bad argument to 'len' (table or string expected, got %s)", a.TypeName())
			}
		},
	}))

	// ----------------------------------------------------------------
	// Error handling: error, pcall, xpcall, assert
	// ----------------------------------------------------------------

	interp.globals.Define("error", FunctionValue(&GoFunction{
		Name: "error",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) == 0 {
				return nil, &LuaError{Value: StringValue("<no error object>")}
			}
			return nil, &LuaError{Value: args[0]}
		},
	}))

	interp.globals.Define("pcall", FunctionValue(&GoFunction{
		Name: "pcall",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) == 0 {
				return nil, fmt.Errorf("bad argument #1 to 'pcall' (value expected)")
			}
			fn := args[0]
			fnArgs := args[1:]

			results, err := interp.callFunction(fn, fnArgs)
			if err != nil {
				var luaErr *LuaError
				if errors.As(err, &luaErr) {
					return []Value{BoolValue(false), luaErr.Value}, nil
				}
				return []Value{BoolValue(false), StringValue(err.Error())}, nil
			}
			return append([]Value{BoolValue(true)}, results...), nil
		},
	}))

	interp.globals.Define("xpcall", FunctionValue(&GoFunction{
		Name: "xpcall",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) < 2 {
				return nil, fmt.Errorf("bad argument #%d to 'xpcall' (value expected)", len(args)+1)
			}
			fn := args[0]
			handler := args[1]
			fnArgs := args[2:]

			results, err := interp.callFunction(fn, fnArgs)
			if err != nil {
				var errVal Value
				var luaErr *LuaError
				if errors.As(err, &luaErr) {
					errVal = luaErr.Value
				} else {
					errVal = StringValue(err.Error())
				}
				handlerResult, _ := interp.callFunction(handler, []Value{errVal})
				msg := NilValue()
				if len(handlerResult) > 0 {
					msg = handlerResult[0]
				}
				return []Value{BoolValue(false), msg}, nil
			}
			return append([]Value{BoolValue(true)}, results...), nil
		},
	}))

	interp.globals.Define("assert", FunctionValue(&GoFunction{
		Name: "assert",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) == 0 {
				return nil, fmt.Errorf("bad argument #1 to 'assert' (value expected)")
			}
			if !args[0].Truthy() {
				errVal := StringValue("assertion failed")
				if len(args) > 1 {
					errVal = args[1]
				}
				return nil, &LuaError{Value: errVal}
			}
			return args, nil // return all args on success
		},
		FastArg1: func(arg Value) (Value, error) {
			if !arg.Truthy() {
				return NilValue(), &LuaError{Value: StringValue("assertion failed")}
			}
			return arg, nil
		},
	}))

	interp.globals.Define("spread", FunctionValue(&GoFunction{
		Name: "spread",
		Fn: func(args []Value) ([]Value, error) {
			return args, nil
		},
	}))

	// ----------------------------------------------------------------
	// Iteration: ipairs, pairs, next, select, unpack
	// ----------------------------------------------------------------

	interp.globals.Define("ipairs", FunctionValue(&GoFunction{
		Name: "ipairs",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) < 1 || !args[0].IsTable() {
				return nil, fmt.Errorf("bad argument #1 to 'ipairs' (table expected)")
			}
			tbl := args[0].Table()
			i := int64(0)
			iter := &GoFunction{
				Name: "ipairs_iterator",
				Fn: func(_ []Value) ([]Value, error) {
					i++
					v, err := interp.tableGet(TableValue(tbl), IntValue(i))
					if err != nil {
						return nil, err
					}
					if v.IsNil() {
						return []Value{NilValue()}, nil
					}
					return []Value{IntValue(i), v}, nil
				},
			}
			return []Value{FunctionValue(iter)}, nil
		},
		NativeKind: NativeKindStdIPairs,
		NativeData: StdIPairsIdentityPtr(),
	}))

	interp.globals.Define("pairs", FunctionValue(&GoFunction{
		Name: "pairs",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) < 1 || !args[0].IsTable() {
				return nil, fmt.Errorf("bad argument #1 to 'pairs' (table expected)")
			}
			tbl := args[0].Table()
			if mt := tbl.GetMetatable(); mt != nil {
				mm := mt.RawGetString("__pairs")
				if !mm.IsNil() {
					return interp.callFunction(mm, []Value{args[0]})
				}
			}
			keys := tbl.PairsKeysSnapshot()
			idx := 0
			iter := &GoFunction{
				Name: "pairs_iterator",
				Fn: func(_ []Value) ([]Value, error) {
					if idx >= len(keys) {
						return []Value{NilValue()}, nil
					}
					k := keys[idx]
					idx++
					v := tbl.RawGet(k)
					return []Value{k, v}, nil
				},
				FastArg2Ret2: func(_, _ Value) (Value, Value, int, error) {
					if idx >= len(keys) {
						return NilValue(), NilValue(), 1, nil
					}
					k := keys[idx]
					idx++
					return k, tbl.RawGet(k), 2, nil
				},
			}
			return []Value{FunctionValue(iter), args[0], NilValue()}, nil
		},
		NativeKind: NativeKindStdPairs,
		NativeData: StdPairsIdentityPtr(),
	}))

	interp.globals.Define("next", FunctionValue(&GoFunction{
		Name: "next",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) < 1 || !args[0].IsTable() {
				return nil, fmt.Errorf("bad argument #1 to 'next' (table expected)")
			}
			tbl := args[0].Table()
			key := NilValue()
			if len(args) > 1 {
				key = args[1]
			}
			nk, nv, ok := tbl.Next(key)
			if !ok {
				if !key.IsNil() && tbl.RawGet(key).IsNil() {
					return nil, fmt.Errorf("invalid key to 'next'")
				}
				return []Value{NilValue()}, nil
			}
			return []Value{nk, nv}, nil
		},
		FastArg2Ret2: func(table, key Value) (Value, Value, int, error) {
			if !table.IsTable() {
				return NilValue(), NilValue(), 0, fmt.Errorf("bad argument #1 to 'next' (table expected)")
			}
			tbl := table.Table()
			nk, nv, ok := tbl.Next(key)
			if !ok {
				if !key.IsNil() && tbl.RawGet(key).IsNil() {
					return NilValue(), NilValue(), 0, fmt.Errorf("invalid key to 'next'")
				}
				return NilValue(), NilValue(), 1, nil
			}
			return nk, nv, 2, nil
		},
		NativeKind: NativeKindStdNext,
		NativeData: StdNextIdentityPtr(),
	}))

	interp.globals.Define("select", FunctionValue(&GoFunction{
		Name: "select",
		Fn: func(args []Value) ([]Value, error) {
			return SelectResults(args)
		},
		NativeKind: NativeKindStdSelect,
		NativeData: StdSelectIdentityPtr(),
	}))

	interp.globals.Define("unpack", FunctionValue(&GoFunction{
		Name: "unpack",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) < 1 || !args[0].IsTable() {
				return nil, fmt.Errorf("bad argument #1 to 'unpack' (table expected)")
			}
			tbl := args[0].Table()
			i := int64(1)
			j := int64(tbl.Length())
			if len(args) >= 2 {
				if n, ok := args[1].ToNumber(); ok {
					i = int64(n.Number())
				}
			}
			if len(args) >= 3 {
				if n, ok := args[2].ToNumber(); ok {
					j = int64(n.Number())
				}
			}
			var result []Value
			for idx := i; idx <= j; idx++ {
				result = append(result, tbl.RawGet(IntValue(idx)))
			}
			return result, nil
		},
	}))

	// ----------------------------------------------------------------
	// Module system: require, dofile, loadstring
	// ----------------------------------------------------------------

	interp.globals.Define("require", FunctionValue(&GoFunction{
		Name: "require",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) < 1 || !args[0].IsString() {
				return nil, fmt.Errorf("bad argument #1 to 'require' (string expected)")
			}
			name := args[0].Str()

			if loaded := interp.packageLoaded(name); !loaded.IsNil() {
				interp.modules[name] = loaded
				return []Value{loaded}, nil
			}

			// Check loaded cache
			if loaded, ok := interp.modules[name]; ok {
				return []Value{loaded}, nil
			}
			if module, ok := interp.builtinModule(name); ok {
				interp.modules[name] = module
				interp.markPackageLoaded(name, module)
				return []Value{module}, nil
			}
			if !interp.moduleLoading {
				return nil, fmt.Errorf("module loading disabled")
			}

			filename := interp.resolveScriptPath(strings.ReplaceAll(name, ".", "/") + ".gs")
			resolved, err := interp.resolveFilesystemPath(filename)
			if err != nil {
				return nil, err
			}
			filename = resolved
			if _, err := os.Stat(filename); err != nil {
				return nil, fmt.Errorf("module '%s' not found", name)
			}
			result, err := interp.RunFile(filename)
			if err != nil {
				return nil, err
			}

			if len(result) > 0 {
				interp.modules[name] = result[0]
			} else {
				interp.modules[name] = BoolValue(true)
			}
			interp.markPackageLoaded(name, interp.modules[name])
			return []Value{interp.modules[name]}, nil
		},
	}))

	interp.globals.Define("dofile", FunctionValue(&GoFunction{
		Name: "dofile",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) < 1 || !args[0].IsString() {
				return nil, fmt.Errorf("bad argument #1 to 'dofile' (string expected)")
			}
			if !interp.filesystemEnabled {
				return nil, fmt.Errorf("filesystem access disabled")
			}
			filename := args[0].Str()
			return interp.RunFile(filename)
		},
	}))

	interp.globals.Define("load", FunctionValue(&GoFunction{
		Name: "load",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) < 1 || !args[0].IsString() {
				return nil, fmt.Errorf("bad argument #1 to 'load' (string expected)")
			}
			var opt Value
			if len(args) >= 2 {
				opt = args[1]
			}
			fn, err := interp.compileStringWithConfig(args[0].Str(), opt, "<load>")
			if err != nil {
				return []Value{NilValue(), StringValue(err.Error())}, nil
			}
			return []Value{fn}, nil
		},
	}))

	interp.globals.Define("loadfile", FunctionValue(&GoFunction{
		Name: "loadfile",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) < 1 || !args[0].IsString() {
				return nil, fmt.Errorf("bad argument #1 to 'loadfile' (string expected)")
			}
			if !interp.filesystemEnabled {
				return []Value{NilValue(), StringValue("filesystem access disabled")}, nil
			}
			var opt Value
			if len(args) >= 2 {
				opt = args[1]
			}
			fn, err := interp.loadFileWithConfig(args[0].Str(), opt)
			if err != nil {
				return []Value{NilValue(), StringValue(err.Error())}, nil
			}
			return []Value{fn}, nil
		},
	}))

	interp.globals.Define("loadstring", FunctionValue(&GoFunction{
		Name: "loadstring",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) < 1 || !args[0].IsString() {
				return nil, fmt.Errorf("bad argument #1 to 'loadstring' (string expected)")
			}
			src := args[0].Str()
			return interp.ExecString(src)
		},
	}))

	// ----------------------------------------------------------------
	// Coroutine library
	// ----------------------------------------------------------------
	interp.registerCoroutineLib()

	// ----------------------------------------------------------------
	// Channel builtins
	// ----------------------------------------------------------------
	interp.globals.Define("close", FunctionValue(&GoFunction{
		Name: "close",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) < 1 || !args[0].IsChannel() {
				return nil, fmt.Errorf("close expects a channel")
			}
			ch := args[0].Channel()
			if err := ch.Close(); err != nil {
				return nil, err
			}
			return nil, nil
		},
	}))
}

// registerCoroutineLib installs the "coroutine" global table with
// create, resume, yield, status, wrap, and isyieldable.
func (interp *Interpreter) registerCoroutineLib() {
	coLib := NewTable()

	coLib.RawSet(StringValue("create"), FunctionValue(&GoFunction{
		Name: "coroutine.create",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) < 1 || !args[0].IsFunction() {
				return nil, fmt.Errorf("coroutine.create expects a function")
			}
			co := NewCoroutine(args[0])
			return []Value{CoroutineValue(co)}, nil
		},
	}))

	coLib.RawSet(StringValue("resume"), FunctionValue(&GoFunction{
		Name: "coroutine.resume",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) < 1 || !args[0].IsCoroutine() {
				return nil, fmt.Errorf("coroutine.resume expects a coroutine")
			}
			co := args[0].Coroutine()
			resumeArgs := args[1:]
			return interp.resumeCoroutine(co, resumeArgs)
		},
	}))

	coLib.RawSet(StringValue("yield"), FunctionValue(&GoFunction{
		Name: "coroutine.yield",
		Fn: func(args []Value) ([]Value, error) {
			return interp.yieldFromCoroutine(args)
		},
	}))

	coLib.RawSet(StringValue("status"), FunctionValue(&GoFunction{
		Name: "coroutine.status",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) < 1 || !args[0].IsCoroutine() {
				return nil, fmt.Errorf("coroutine.status expects a coroutine")
			}
			return []Value{StringValue(args[0].Coroutine().Status())}, nil
		},
	}))

	coLib.RawSet(StringValue("wrap"), FunctionValue(&GoFunction{
		Name: "coroutine.wrap",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) < 1 || !args[0].IsFunction() {
				return nil, fmt.Errorf("coroutine.wrap expects a function")
			}
			co := NewCoroutine(args[0])
			wrapper := &GoFunction{
				Name: "wrapped_coroutine",
				Fn: func(wargs []Value) ([]Value, error) {
					results, err := interp.resumeCoroutine(co, wargs)
					if err != nil {
						return nil, err
					}
					// results[0] is bool success flag
					if len(results) > 0 && !results[0].Bool() {
						if len(results) > 1 {
							return nil, fmt.Errorf("%s", results[1].String())
						}
						return nil, fmt.Errorf("cannot resume dead coroutine")
					}
					// Strip the success flag, return remaining values
					return results[1:], nil
				},
			}
			return []Value{FunctionValue(wrapper)}, nil
		},
	}))

	coLib.RawSet(StringValue("isyieldable"), FunctionValue(&GoFunction{
		Name: "coroutine.isyieldable",
		Fn: func(args []Value) ([]Value, error) {
			return []Value{BoolValue(getCurrentCoroutine() != nil)}, nil
		},
	}))

	interp.globals.Define("coroutine", TableValue(coLib))
}

// resumeCoroutine resumes a suspended coroutine with the given arguments.
// Returns (true, values...) on success/yield, or (false, error_message) on failure.
func (interp *Interpreter) resumeCoroutine(co *Coroutine, args []Value) ([]Value, error) {
	if co.status == CoroutineDead {
		return []Value{BoolValue(false), StringValue("cannot resume dead coroutine")}, nil
	}
	if co.status == CoroutineRunning {
		return []Value{BoolValue(false), StringValue("cannot resume running coroutine")}, nil
	}

	// Save and restore the previous coroutine context so that nested
	// resume calls (coroutine resuming another coroutine) work correctly.
	prevCo := interp.currentCo

	co.status = CoroutineRunning

	if !co.started {
		co.started = true
		// Launch the goroutine. It creates its own Interpreter that shares
		// globals but has its own currentCo, avoiding data races.
		go func() {
			// Register this coroutine in the goroutine-local map so that
			// coroutine.yield can find it from within GoFunction closures.
			setCurrentCoroutine(co)
			defer setCurrentCoroutine(nil)

			coInterp := &Interpreter{
				globals:   interp.globals,
				currentCo: co,
			}
			// Wait for initial args from the first resume.
			initArgs := <-co.resumeCh
			// Call the coroutine body function.
			results, err := coInterp.callFunction(co.fn, initArgs)
			if results == nil {
				results = []Value{}
			}
			co.yieldCh <- yieldResult{values: results, err: err, done: true}
		}()
	}

	// Send args to the coroutine (initial args on first resume, or
	// values returned from yield on subsequent resumes).
	co.resumeCh <- args

	// Wait for the coroutine to yield or finish.
	result := <-co.yieldCh

	if result.done || result.err != nil {
		co.status = CoroutineDead
	} else {
		co.status = CoroutineSuspended
	}

	interp.currentCo = prevCo

	if result.err != nil {
		return []Value{BoolValue(false), StringValue(result.err.Error())}, nil
	}

	// Prepend true to indicate success.
	return append([]Value{BoolValue(true)}, result.values...), nil
}

// yieldFromCoroutine yields values from the currently running coroutine back
// to the caller of resume. It blocks until the coroutine is resumed again,
// and returns the values passed to the next resume call.
// It uses goroutine-local storage to find the correct coroutine, so it works
// correctly even though the GoFunction closure captures the main interpreter.
func (interp *Interpreter) yieldFromCoroutine(values []Value) ([]Value, error) {
	co := getCurrentCoroutine()
	if co == nil {
		return nil, fmt.Errorf("cannot yield from outside a coroutine")
	}
	// Send yielded values to the resume caller.
	co.yieldCh <- yieldResult{values: values}
	// Block until resumed.
	resumeVals := <-co.resumeCh
	return resumeVals, nil
}
