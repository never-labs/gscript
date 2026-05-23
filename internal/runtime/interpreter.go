package runtime

import (
	"errors"
	"fmt"
	"math"
	"os"
	goruntime "runtime"
	"strconv"
	"strings"

	"github.com/gscript/gscript/internal/ast"
	"github.com/gscript/gscript/internal/lexer"
	"github.com/gscript/gscript/internal/parser"
)

// Interpreter is the tree-walking evaluator for GScript programs.
type Interpreter struct {
	globals           *Environment
	output            []string         // captured print output (for testing)
	currentCo         *Coroutine       // non-nil when running inside a coroutine
	modules           map[string]Value // require() cache
	stringMeta        *Table           // metatable for string values (__index → string lib)
	scriptDir         string           // directory of the main script (for require path resolution)
	currentSourceName string           // source name for diagnostics while executing parsed source
	args              []string         // current script entrypoint args: [0]=script, [1:]=user args
	callStack         []DebugFrame     // active runtime calls, oldest to newest
	deferStack        [][]deferredCall // active function-scope deferred calls
	debugHook         Value            // optional GScript diagnostic hook
	debugOpts         DebugHookOptions // filters for debugHook
	debugSink         Value            // optional explicit diagnostic sink
	debugBusy         bool             // prevents debug hooks from recursively firing
	gcMode            string           // host-facing collectgarbage mode label
	gcRunning         bool             // host-facing collectgarbage running flag
}

// New creates a new Interpreter with built-in globals.
func New() *Interpreter {
	interp := &Interpreter{
		globals:   NewEnvironment(nil),
		modules:   make(map[string]Value),
		gcMode:    "incremental",
		gcRunning: true,
	}
	interp.registerBuiltins()
	interp.registerStdlib()
	return interp
}

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

// Globals returns the global environment.
func (interp *Interpreter) Globals() *Environment {
	return interp.globals
}

// StringMeta returns the string metatable.
func (interp *Interpreter) StringMeta() *Table {
	return interp.stringMeta
}

// ExportGlobals returns a flat map of all global variables.
// Used by the VM to share stdlib/builtins from the tree-walker.
func (interp *Interpreter) ExportGlobals() map[string]Value {
	m := make(map[string]Value)
	for name, uv := range interp.globals.vars {
		m[name] = uv.Get()
	}
	return m
}

// RestrictStdlib removes standard-library globals not present in allowed.
func (interp *Interpreter) RestrictStdlib(allowed map[string]bool) {
	for _, name := range stdlibModuleNames {
		if allowed[name] {
			continue
		}
		interp.globals.Delete(name)
		delete(interp.modules, name)
		interp.markPackageLoaded(name, NilValue())
		if name == "string" {
			interp.stringMeta = nil
		}
	}
}

// NewInterpreterGlobals creates a fresh globals map with all builtins and stdlib registered.
// This is used by the bytecode VM to get the same standard library as the tree-walker.
func NewInterpreterGlobals() map[string]Value {
	interp := New()
	return interp.ExportGlobals()
}

// SetGlobal defines or overwrites a global variable.
func (interp *Interpreter) SetGlobal(name string, val Value) {
	interp.globals.Define(name, val)
}

// GetGlobal retrieves a global variable.
func (interp *Interpreter) GetGlobal(name string) Value {
	v, _ := interp.globals.Get(name)
	return v
}

// SetScriptDir sets the directory of the main script, used for require path resolution.
func (interp *Interpreter) SetScriptDir(dir string) {
	interp.scriptDir = dir
}

// ScriptDir returns the directory used for relative script/module loading.
func (interp *Interpreter) ScriptDir() string {
	return interp.scriptDir
}

func (interp *Interpreter) builtinModule(name string) (Value, bool) {
	for _, moduleName := range stdlibModuleNames {
		if name != moduleName {
			continue
		}
		v, ok := interp.globals.Get(name)
		return v, ok && v.IsTable()
	}
	return NilValue(), false
}

func (interp *Interpreter) markPackageLoaded(name string, module Value) {
	pkg, ok := interp.globals.Get("package")
	if !ok || !pkg.IsTable() {
		return
	}
	loaded := pkg.Table().RawGetString("loaded")
	if !loaded.IsTable() {
		return
	}
	loaded.Table().RawSetString(name, module)
}

func (interp *Interpreter) packageLoaded(name string) Value {
	pkg, ok := interp.globals.Get("package")
	if !ok || !pkg.IsTable() {
		return NilValue()
	}
	loaded := pkg.Table().RawGetString("loaded")
	if !loaded.IsTable() {
		return NilValue()
	}
	return loaded.Table().RawGetString(name)
}

// SetArgs sets the script entrypoint arguments and updates the global arg table.
// The resulting table follows GScript's Lua-compatible convention:
// arg[0] is the script name, and arg[1..n] are user arguments.
func (interp *Interpreter) SetArgs(script string, args []string) {
	argv := make([]string, 0, len(args)+1)
	argv = append(argv, script)
	argv = append(argv, args...)
	interp.args = argv

	tbl := NewTable()
	tbl.RawSet(IntValue(0), StringValue(script))
	for i, arg := range args {
		tbl.RawSet(IntValue(int64(i+1)), StringValue(arg))
	}
	interp.SetGlobal("arg", TableValue(tbl))
}

// Args returns a copy of the current script entrypoint arguments.
func (interp *Interpreter) Args() []string {
	out := make([]string, len(interp.args))
	copy(out, interp.args)
	return out
}

// Output returns captured print output (for testing).
func (interp *Interpreter) Output() []string {
	return interp.output
}

// registerBuiltins installs standard global functions.
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

			filename := interp.resolveScriptPath(strings.ReplaceAll(name, ".", "/") + ".gs")
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

// ====================================================================
// Metamethod helpers
// ====================================================================

const maxMetaDepth = 50

// getMetamethod returns the metamethod for the given event (__add, etc.)
// Returns the metamethod Value and true if found.
func (interp *Interpreter) getMetamethod(val Value, event string) (Value, bool) {
	var mt *Table
	if val.IsTable() {
		mt = val.Table().GetMetatable()
	}
	if mt == nil {
		return NilValue(), false
	}
	mm := mt.RawGet(StringValue(event))
	if mm.IsNil() {
		return NilValue(), false
	}
	return mm, true
}

func (interp *Interpreter) luaToString(v Value) (string, error) {
	if v.IsTable() {
		if mt := v.Table().GetMetatable(); mt != nil {
			if mm := mt.RawGetString("__tostring"); !mm.IsNil() {
				results, err := interp.callFunction(mm, []Value{v})
				if err != nil {
					return "", err
				}
				if len(results) == 0 || !results[0].IsString() {
					return "", fmt.Errorf("'__tostring' must return a string")
				}
				return results[0].Str(), nil
			}
			if name := mt.RawGetString("__name"); name.IsString() {
				return name.Str() + ": " + strings.TrimPrefix(v.String(), "table: "), nil
			}
		}
	}
	return v.String(), nil
}

// tableGet retrieves a value from a table, with __index metamethod support.
func (interp *Interpreter) tableGet(t Value, key Value) (Value, error) {
	return interp.tableGetDepth(t, key, 0)
}

func (interp *Interpreter) tableGetDepth(t Value, key Value, depth int) (Value, error) {
	if depth > maxMetaDepth {
		return NilValue(), fmt.Errorf("'__index' chain too long; possible loop")
	}
	if !t.IsTable() {
		return NilValue(), fmt.Errorf("attempt to index a %s value", t.TypeName())
	}

	tbl := t.Table()
	val := tbl.RawGet(key)

	if !val.IsNil() {
		return val, nil
	}

	// Try __index
	mt := tbl.GetMetatable()
	if mt == nil {
		return NilValue(), nil
	}

	index := mt.RawGetString("__index")
	if index.IsNil() {
		return NilValue(), nil
	}

	if index.IsTable() {
		return interp.tableGetDepth(TableValue(index.Table()), key, depth+1)
	}

	if index.IsFunction() {
		results, err := interp.callFunction(index, []Value{t, key})
		if err != nil {
			return NilValue(), err
		}
		if len(results) > 0 {
			return results[0], nil
		}
		return NilValue(), nil
	}

	return NilValue(), nil
}

// tableSet assigns a value to a table, with __newindex metamethod support.
func (interp *Interpreter) tableSet(t Value, key, val Value) error {
	return interp.tableSetDepth(t, key, val, 0)
}

func (interp *Interpreter) tableSetDepth(t Value, key, val Value, depth int) error {
	if depth > maxMetaDepth {
		return fmt.Errorf("'__newindex' chain too long; possible loop")
	}
	if !t.IsTable() {
		return fmt.Errorf("attempt to index a %s value", t.TypeName())
	}

	tbl := t.Table()

	// Check if key already exists (rawget) - if so, just set it directly
	existing := tbl.RawGet(key)
	if !existing.IsNil() {
		tbl.RawSet(key, val)
		return nil
	}

	// Check __newindex
	mt := tbl.GetMetatable()
	if mt != nil {
		newindex := mt.RawGetString("__newindex")
		if newindex.IsFunction() {
			_, err := interp.callFunction(newindex, []Value{t, key, val})
			return err
		}
		if newindex.IsTable() {
			return interp.tableSetDepth(TableValue(newindex.Table()), key, val, depth+1)
		}
	}

	tbl.RawSet(key, val)
	return nil
}

func (interp *Interpreter) tableLenInt(t Value) (int64, error) {
	if !t.IsTable() {
		return 0, fmt.Errorf("attempt to get length of a %s value", t.TypeName())
	}
	if mm, ok := interp.getMetamethod(t, "__len"); ok {
		results, err := interp.callFunction(mm, []Value{t})
		if err != nil {
			return 0, err
		}
		if len(results) == 0 {
			return 0, nil
		}
		return toInt(results[0]), nil
	}
	return int64(t.Table().Length()), nil
}

// opToMetamethod maps arithmetic operators to metamethod names.
func opToMetamethod(op string) string {
	switch op {
	case "+":
		return "__add"
	case "-":
		return "__sub"
	case "*":
		return "__mul"
	case "/":
		return "__div"
	case "%":
		return "__mod"
	case "**":
		return "__pow"
	default:
		return ""
	}
}

// ====================================================================
// Exec -- top-level entry
// ====================================================================

// Exec executes a program (top-level statement list).
func (interp *Interpreter) Exec(prog *ast.Program) error {
	if err := ast.ValidateLabelControl(prog); err != nil {
		return err
	}
	interp.pushDeferFrame()
	_, _, _, _, err := interp.execBlockInEnv(&ast.BlockStmt{P: prog.GetPos(), Stmts: prog.Stmts}, interp.globals)
	if err != nil {
		_ = interp.runAndPopDeferFrame()
		var jump *gotoSignal
		if errors.As(err, &jump) {
			return fmt.Errorf("goto %q target not found", jump.name)
		}
		return err
	}
	return interp.runAndPopDeferFrame()
}

// ExecString parses and executes a source string, returning any top-level return values.
func (interp *Interpreter) ExecString(src string) ([]Value, error) {
	tokens, err := lexer.New(src).Tokenize()
	if err != nil {
		return nil, err
	}
	prog, err := parser.New(tokens).Parse()
	if err != nil {
		return nil, err
	}
	if err := ast.ValidateLabelControl(prog); err != nil {
		return nil, err
	}
	// Execute and collect return values from the last return statement.
	var lastRet []Value
	interp.pushDeferFrame()
	retVals, isRet, _, _, err := interp.execBlockInEnv(&ast.BlockStmt{P: prog.GetPos(), Stmts: prog.Stmts}, interp.globals)
	if err != nil {
		_ = interp.runAndPopDeferFrame()
		var jump *gotoSignal
		if errors.As(err, &jump) {
			return nil, fmt.Errorf("goto %q target not found", jump.name)
		}
		return nil, err
	}
	if isRet {
		lastRet = retVals
	}
	if err := interp.runAndPopDeferFrame(); err != nil {
		return nil, err
	}
	return lastRet, nil
}

// ====================================================================
// Statement execution
// ====================================================================

type gotoSignal struct {
	name string
}

func (g *gotoSignal) Error() string {
	return "goto " + g.name
}

// execBlock executes a block of statements in a new child scope.
// Returns (returnValues, isReturn, isBreak, isContinue, error).
func (interp *Interpreter) execBlock(block *ast.BlockStmt, env *Environment) ([]Value, bool, bool, bool, error) {
	child := NewEnvironment(env)
	return interp.execBlockInEnv(block, child)
}

// execBlockInEnv executes a block in the given environment (without creating a new scope).
func (interp *Interpreter) execBlockInEnv(block *ast.BlockStmt, env *Environment) ([]Value, bool, bool, bool, error) {
	labels := make(map[string]int)
	for i, stmt := range block.Stmts {
		if label, ok := stmt.(*ast.LabelStmt); ok {
			labels[label.Name] = i
		}
	}
	for pc := 0; pc < len(block.Stmts); pc++ {
		stmt := block.Stmts[pc]
		retVals, isRet, isBrk, isCont, err := interp.execStmt(stmt, env)
		if err != nil {
			var jump *gotoSignal
			if errors.As(err, &jump) {
				if target, ok := labels[jump.name]; ok {
					pc = target
					continue
				}
			}
			return nil, false, false, false, err
		}
		if isRet || isBrk || isCont {
			return retVals, isRet, isBrk, isCont, nil
		}
	}
	return nil, false, false, false, nil
}

// execStmt dispatches a single statement.
func (interp *Interpreter) execStmt(stmt ast.Stmt, env *Environment) ([]Value, bool, bool, bool, error) {
	retVals, isRet, isBrk, isCont, err := interp.execStmtRaw(stmt, env)
	if err != nil {
		var jump *gotoSignal
		if errors.As(err, &jump) {
			return nil, false, false, false, err
		}
		return nil, false, false, false, interp.wrapRuntimeError(err, stmt.GetPos())
	}
	return retVals, isRet, isBrk, isCont, nil
}

func (interp *Interpreter) execStmtRaw(stmt ast.Stmt, env *Environment) ([]Value, bool, bool, bool, error) {
	switch s := stmt.(type) {
	case *ast.DeclareStmt:
		return interp.execDeclare(s, env)
	case *ast.AssignStmt:
		return interp.execAssign(s, env)
	case *ast.CompoundAssignStmt:
		return interp.execCompoundAssign(s, env)
	case *ast.IncDecStmt:
		return interp.execIncDec(s, env)
	case *ast.CallStmt:
		_, err := interp.evalExpr(s.Call, env)
		return nil, false, false, false, err
	case *ast.IfStmt:
		return interp.execIf(s, env)
	case *ast.ForStmt:
		return interp.execFor(s, env)
	case *ast.ForNumStmt:
		return interp.execForNum(s, env)
	case *ast.ForRangeStmt:
		return interp.execForRange(s, env)
	case *ast.ReturnStmt:
		return interp.execReturn(s, env)
	case *ast.BreakStmt:
		return nil, false, true, false, nil
	case *ast.ContinueStmt:
		return nil, false, false, true, nil
	case *ast.LabelStmt:
		return nil, false, false, false, nil
	case *ast.GotoStmt:
		return nil, false, false, false, &gotoSignal{name: s.Name}
	case *ast.FuncDeclStmt:
		return interp.execFuncDecl(s, env)
	case *ast.BlockStmt:
		return interp.execBlock(s, env)
	case *ast.GoStmt:
		return interp.execGo(s, env)
	case *ast.DeferStmt:
		return interp.execDefer(s, env)
	case *ast.SendStmt:
		return interp.execSend(s, env)
	default:
		return nil, false, false, false, fmt.Errorf("unknown statement type: %T", stmt)
	}
}

type deferredCall struct {
	fn   Value
	args []Value
}

func (interp *Interpreter) pushDeferFrame() {
	interp.deferStack = append(interp.deferStack, nil)
}

func (interp *Interpreter) runAndPopDeferFrame() error {
	if len(interp.deferStack) == 0 {
		return nil
	}
	idx := len(interp.deferStack) - 1
	calls := interp.deferStack[idx]
	interp.deferStack = interp.deferStack[:idx]

	var firstErr error
	for i := len(calls) - 1; i >= 0; i-- {
		if _, err := interp.callFunction(calls[i].fn, calls[i].args); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (interp *Interpreter) execDefer(s *ast.DeferStmt, env *Environment) ([]Value, bool, bool, bool, error) {
	call, err := interp.prepareDeferredCall(s.Call, env)
	if err != nil {
		return nil, false, false, false, err
	}
	if len(interp.deferStack) == 0 {
		if _, err := interp.callFunction(call.fn, call.args); err != nil {
			return nil, false, false, false, err
		}
		return nil, false, false, false, nil
	}
	idx := len(interp.deferStack) - 1
	interp.deferStack[idx] = append(interp.deferStack[idx], call)
	return nil, false, false, false, nil
}

func (interp *Interpreter) prepareDeferredCall(expr ast.Expr, env *Environment) (deferredCall, error) {
	switch call := expr.(type) {
	case *ast.CallExpr:
		fn, err := interp.evalExprSingle(call.Func, env)
		if err != nil {
			return deferredCall{}, err
		}
		args, err := interp.evalExprList(call.Args, env)
		if err != nil {
			return deferredCall{}, err
		}
		return deferredCall{fn: fn, args: args}, nil
	case *ast.MethodCallExpr:
		obj, err := interp.evalExprSingle(call.Object, env)
		if err != nil {
			return deferredCall{}, err
		}
		method, err := interp.methodValue(obj, call.Method)
		if err != nil {
			return deferredCall{}, err
		}
		args, err := interp.evalExprList(call.Args, env)
		if err != nil {
			return deferredCall{}, err
		}
		args = append([]Value{obj}, args...)
		return deferredCall{fn: method, args: args}, nil
	default:
		return deferredCall{}, fmt.Errorf("defer statement requires a function call")
	}
}

func (interp *Interpreter) execGo(s *ast.GoStmt, env *Environment) ([]Value, bool, bool, bool, error) {
	// Evaluate the function and arguments before launching the goroutine.
	switch call := s.Call.(type) {
	case *ast.CallExpr:
		fn, err := interp.evalExprSingle(call.Func, env)
		if err != nil {
			return nil, false, false, false, err
		}
		args, err := interp.evalExprList(call.Args, env)
		if err != nil {
			return nil, false, false, false, err
		}
		go func() {
			childInterp := &Interpreter{
				globals:    interp.globals,
				stringMeta: interp.stringMeta,
			}
			childInterp.callFunction(fn, args)
		}()
	case *ast.MethodCallExpr:
		obj, err := interp.evalExprSingle(call.Object, env)
		if err != nil {
			return nil, false, false, false, err
		}
		method, err := interp.tableGet(obj, StringValue(call.Method))
		if err != nil {
			return nil, false, false, false, err
		}
		args, err := interp.evalExprList(call.Args, env)
		if err != nil {
			return nil, false, false, false, err
		}
		go func() {
			childInterp := &Interpreter{
				globals:    interp.globals,
				stringMeta: interp.stringMeta,
			}
			childInterp.callFunction(method, args)
		}()
	default:
		return nil, false, false, false, fmt.Errorf("go statement requires a function call")
	}
	return nil, false, false, false, nil
}

func (interp *Interpreter) execSend(s *ast.SendStmt, env *Environment) ([]Value, bool, bool, bool, error) {
	chVal, err := interp.evalExprSingle(s.Channel, env)
	if err != nil {
		return nil, false, false, false, err
	}
	if !chVal.IsChannel() {
		return nil, false, false, false, fmt.Errorf("send on non-channel value")
	}
	val, err := interp.evalExprSingle(s.Value, env)
	if err != nil {
		return nil, false, false, false, err
	}
	if err := chVal.Channel().Send(val); err != nil {
		return nil, false, false, false, err
	}
	return nil, false, false, false, nil
}

// ------------------------------------------------------------------
// DeclareStmt: a, b := expr1, expr2
// ------------------------------------------------------------------
func (interp *Interpreter) execDeclare(s *ast.DeclareStmt, env *Environment) ([]Value, bool, bool, bool, error) {
	vals, err := interp.evalExprList(s.Values, env)
	if err != nil {
		return nil, false, false, false, err
	}
	for i, name := range s.Names {
		v := NilValue()
		if i < len(vals) {
			v = vals[i]
		}
		if env.IsLocalReadOnly(name) {
			return nil, false, false, false, fmt.Errorf("cannot redeclare readonly variable %q", name)
		}
		if s.ReadOnly {
			env.DefineReadOnly(name, v)
		} else {
			env.Define(name, v)
		}
	}
	return nil, false, false, false, nil
}

// ------------------------------------------------------------------
// AssignStmt: a, b = expr1, expr2
// ------------------------------------------------------------------
func (interp *Interpreter) execAssign(s *ast.AssignStmt, env *Environment) ([]Value, bool, bool, bool, error) {
	vals, err := interp.evalExprList(s.Values, env)
	if err != nil {
		return nil, false, false, false, err
	}
	for i, target := range s.Targets {
		v := NilValue()
		if i < len(vals) {
			v = vals[i]
		}
		if err := interp.assignTarget(target, v, env); err != nil {
			return nil, false, false, false, err
		}
	}
	return nil, false, false, false, nil
}

// assignTarget assigns a value to an lvalue expression.
func (interp *Interpreter) assignTarget(target ast.Expr, val Value, env *Environment) error {
	switch t := target.(type) {
	case *ast.IdentExpr:
		if env.IsReadOnly(t.Name) {
			return fmt.Errorf("cannot assign to readonly variable %q", t.Name)
		}
		if !env.Set(t.Name, val) {
			// If variable doesn't exist anywhere, create it in the current env
			// (like a global implicit declaration)
			env.Define(t.Name, val)
		}
		return nil
	case *ast.IndexExpr:
		tbl, err := interp.evalExprSingle(t.Table, env)
		if err != nil {
			return err
		}
		key, err := interp.evalExprSingle(t.Index, env)
		if err != nil {
			return err
		}
		return interp.tableSet(tbl, key, val)
	case *ast.FieldExpr:
		tbl, err := interp.evalExprSingle(t.Table, env)
		if err != nil {
			return err
		}
		return interp.tableSet(tbl, StringValue(t.Field), val)
	default:
		return fmt.Errorf("invalid assignment target: %T", target)
	}
}

// ------------------------------------------------------------------
// CompoundAssignStmt: a += b
// ------------------------------------------------------------------
func (interp *Interpreter) execCompoundAssign(s *ast.CompoundAssignStmt, env *Environment) ([]Value, bool, bool, bool, error) {
	lhs, err := interp.evalExprSingle(s.Target, env)
	if err != nil {
		return nil, false, false, false, err
	}
	rhs, err := interp.evalExprSingle(s.Value, env)
	if err != nil {
		return nil, false, false, false, err
	}

	var op string
	switch s.Op {
	case "+=":
		op = "+"
	case "-=":
		op = "-"
	case "*=":
		op = "*"
	case "/=":
		op = "/"
	default:
		return nil, false, false, false, fmt.Errorf("unknown compound operator: %s", s.Op)
	}

	result, err := interp.arith(op, lhs, rhs)
	if err != nil {
		return nil, false, false, false, err
	}

	if err := interp.assignTarget(s.Target, result, env); err != nil {
		return nil, false, false, false, err
	}
	return nil, false, false, false, nil
}

// ------------------------------------------------------------------
// IncDecStmt: a++ / a--
// ------------------------------------------------------------------
func (interp *Interpreter) execIncDec(s *ast.IncDecStmt, env *Environment) ([]Value, bool, bool, bool, error) {
	lhs, err := interp.evalExprSingle(s.Target, env)
	if err != nil {
		return nil, false, false, false, err
	}

	var result Value
	one := IntValue(1)
	if s.Op == "++" {
		result, err = interp.arith("+", lhs, one)
	} else {
		result, err = interp.arith("-", lhs, one)
	}
	if err != nil {
		return nil, false, false, false, err
	}

	if err := interp.assignTarget(s.Target, result, env); err != nil {
		return nil, false, false, false, err
	}
	return nil, false, false, false, nil
}

// ------------------------------------------------------------------
// IfStmt
// ------------------------------------------------------------------
func (interp *Interpreter) execIf(s *ast.IfStmt, env *Environment) ([]Value, bool, bool, bool, error) {
	cond, err := interp.evalExprSingle(s.Cond, env)
	if err != nil {
		return nil, false, false, false, err
	}
	if cond.Truthy() {
		return interp.execBlock(s.Body, env)
	}
	for _, ei := range s.ElseIfs {
		cond, err = interp.evalExprSingle(ei.Cond, env)
		if err != nil {
			return nil, false, false, false, err
		}
		if cond.Truthy() {
			return interp.execBlock(ei.Body, env)
		}
	}
	if s.ElseBody != nil {
		return interp.execBlock(s.ElseBody, env)
	}
	return nil, false, false, false, nil
}

// ------------------------------------------------------------------
// ForStmt (while-style): for cond { }
// ------------------------------------------------------------------
func (interp *Interpreter) execFor(s *ast.ForStmt, env *Environment) ([]Value, bool, bool, bool, error) {
	for {
		if s.Cond != nil {
			cond, err := interp.evalExprSingle(s.Cond, env)
			if err != nil {
				return nil, false, false, false, err
			}
			if !cond.Truthy() {
				break
			}
		}
		retVals, isRet, isBrk, _, err := interp.execBlock(s.Body, env)
		if err != nil {
			return nil, false, false, false, err
		}
		if isRet {
			return retVals, true, false, false, nil
		}
		if isBrk {
			break
		}
		// isContinue just goes to next iteration
	}
	return nil, false, false, false, nil
}

// ------------------------------------------------------------------
// ForNumStmt (C-style): for init; cond; post { }
// ------------------------------------------------------------------
func (interp *Interpreter) execForNum(s *ast.ForNumStmt, env *Environment) ([]Value, bool, bool, bool, error) {
	// The init, cond, and post all share a new scope
	loopEnv := NewEnvironment(env)
	// Execute init
	_, _, _, _, err := interp.execStmt(s.Init, loopEnv)
	if err != nil {
		return nil, false, false, false, err
	}
	for {
		// Evaluate condition
		cond, err := interp.evalExprSingle(s.Cond, loopEnv)
		if err != nil {
			return nil, false, false, false, err
		}
		if !cond.Truthy() {
			break
		}
		// Execute body in a child of loopEnv
		retVals, isRet, isBrk, _, err := interp.execBlock(s.Body, loopEnv)
		if err != nil {
			return nil, false, false, false, err
		}
		if isRet {
			return retVals, true, false, false, nil
		}
		if isBrk {
			break
		}
		// Execute post
		_, _, _, _, err = interp.execStmt(s.Post, loopEnv)
		if err != nil {
			return nil, false, false, false, err
		}
	}
	return nil, false, false, false, nil
}

// ------------------------------------------------------------------
// ForRangeStmt: for k, v := range expr { }
// ------------------------------------------------------------------
func (interp *Interpreter) execForRange(s *ast.ForRangeStmt, env *Environment) ([]Value, bool, bool, bool, error) {
	iterVal, err := interp.evalExprSingle(s.Iter, env)
	if err != nil {
		return nil, false, false, false, err
	}

	if iterVal.IsTable() {
		tbl := iterVal.Table()
		var key Value = NilValue()
		for {
			nextKey, nextVal, ok := tbl.Next(key)
			if !ok {
				break
			}
			key = nextKey

			// Create new scope for each iteration
			iterEnv := NewEnvironment(env)
			iterEnv.Define(s.Key, nextKey)
			if s.Value != "" {
				iterEnv.Define(s.Value, nextVal)
			}

			retVals, isRet, isBrk, _, err := interp.execBlockInEnv(s.Body, iterEnv)
			if err != nil {
				return nil, false, false, false, err
			}
			if isRet {
				return retVals, true, false, false, nil
			}
			if isBrk {
				break
			}
		}
		return nil, false, false, false, nil
	}

	if iterVal.IsFunction() {
		// Iterator function: call repeatedly until nil
		for {
			results, err := interp.callFunction(iterVal, nil)
			if err != nil {
				return nil, false, false, false, err
			}
			if len(results) == 0 || results[0].IsNil() {
				break
			}
			iterEnv := NewEnvironment(env)
			iterEnv.Define(s.Key, results[0])
			if s.Value != "" {
				v := NilValue()
				if len(results) > 1 {
					v = results[1]
				}
				iterEnv.Define(s.Value, v)
			}
			retVals, isRet, isBrk, _, err := interp.execBlockInEnv(s.Body, iterEnv)
			if err != nil {
				return nil, false, false, false, err
			}
			if isRet {
				return retVals, true, false, false, nil
			}
			if isBrk {
				break
			}
		}
		return nil, false, false, false, nil
	}

	if iterVal.IsChannel() {
		ch := iterVal.Channel()
		for {
			val, ok := ch.Recv()
			if !ok {
				break
			}
			iterEnv := NewEnvironment(env)
			iterEnv.Define(s.Key, val)
			retVals, isRet, isBrk, _, err := interp.execBlockInEnv(s.Body, iterEnv)
			if err != nil {
				return nil, false, false, false, err
			}
			if isRet {
				return retVals, true, false, false, nil
			}
			if isBrk {
				break
			}
		}
		return nil, false, false, false, nil
	}

	return nil, false, false, false, fmt.Errorf("cannot range over %s", iterVal.TypeName())
}

// ------------------------------------------------------------------
// ReturnStmt
// ------------------------------------------------------------------
func (interp *Interpreter) execReturn(s *ast.ReturnStmt, env *Environment) ([]Value, bool, bool, bool, error) {
	vals, err := interp.evalExprList(s.Values, env)
	if err != nil {
		return nil, false, false, false, err
	}
	return vals, true, false, false, nil
}

// ------------------------------------------------------------------
// FuncDeclStmt
// ------------------------------------------------------------------
func (interp *Interpreter) execFuncDecl(s *ast.FuncDeclStmt, env *Environment) ([]Value, bool, bool, bool, error) {
	proto := &FuncProto{
		Name:       s.Name,
		Body:       s.Body,
		SourceName: interp.currentSourceName,
		Line:       s.P.Line,
		Column:     s.P.Column,
	}
	paramNames := make([]string, 0, len(s.Params))
	for _, p := range s.Params {
		paramNames = append(paramNames, p.Name)
		proto.Params = append(proto.Params, p.Name)
		if p.IsVarArg {
			proto.HasVarArg = true
		}
	}

	// Define the function name first so it can self-reference (recursion)
	env.Define(s.Name, NilValue())

	// Capture free variables from the enclosing environment
	freeVarNames := FreeVars(s.Body, paramNames)
	upvalues := make(map[string]*Upvalue)
	for _, fv := range freeVarNames {
		if uv, ok := env.GetUpvalue(fv); ok {
			upvalues[fv] = uv
		}
	}

	closure := &Closure{
		Proto:    proto,
		Upvalues: upvalues,
		Env:      env,
	}
	env.Set(s.Name, FunctionValue(closure))
	return nil, false, false, false, nil
}

// ====================================================================
// Expression evaluation
// ====================================================================

// evalExpr evaluates an expression and returns a slice of Values.
// Most expressions return a single-element slice; CallExpr may return multiple.
func (interp *Interpreter) evalExpr(expr ast.Expr, env *Environment) ([]Value, error) {
	vals, err := interp.evalExprRaw(expr, env)
	if err != nil {
		return nil, interp.wrapRuntimeError(err, expr.GetPos())
	}
	return vals, nil
}

func (interp *Interpreter) evalExprRaw(expr ast.Expr, env *Environment) ([]Value, error) {
	switch e := expr.(type) {
	case *ast.NumberLit:
		v, err := parseNumber(e.Value)
		if err != nil {
			return nil, err
		}
		return []Value{v}, nil

	case *ast.StringLit:
		return []Value{StringValue(e.Value)}, nil

	case *ast.BoolLit:
		return []Value{BoolValue(e.Value)}, nil

	case *ast.NilLit:
		return []Value{NilValue()}, nil

	case *ast.VarArgExpr:
		v, ok := env.Get("...")
		if !ok {
			return nil, nil
		}
		// varargs are stored as a table in "..."
		if v.IsTable() {
			tbl := v.Table()
			n := tbl.Length()
			result := make([]Value, n)
			for i := 1; i <= n; i++ {
				result[i-1] = tbl.RawGet(IntValue(int64(i)))
			}
			return result, nil
		}
		return []Value{v}, nil

	case *ast.IdentExpr:
		v, ok := env.Get(e.Name)
		if !ok {
			return nil, fmt.Errorf("undefined variable: %s", e.Name)
		}
		return []Value{v}, nil

	case *ast.BinaryExpr:
		v, err := interp.evalBinary(e, env)
		if err != nil {
			return nil, err
		}
		return []Value{v}, nil

	case *ast.UnaryExpr:
		v, err := interp.evalUnary(e, env)
		if err != nil {
			return nil, err
		}
		return []Value{v}, nil

	case *ast.ParenExpr:
		vals, err := interp.evalExpr(e.Inner, env)
		if err != nil {
			return nil, err
		}
		if len(vals) == 0 {
			return []Value{NilValue()}, nil
		}
		return []Value{vals[0]}, nil

	case *ast.IndexExpr:
		tbl, err := interp.evalExprSingle(e.Table, env)
		if err != nil {
			return nil, err
		}
		key, err := interp.evalExprSingle(e.Index, env)
		if err != nil {
			return nil, err
		}
		val, err := interp.tableGet(tbl, key)
		if err != nil {
			return nil, err
		}
		return []Value{val}, nil

	case *ast.FieldExpr:
		tbl, err := interp.evalExprSingle(e.Table, env)
		if err != nil {
			return nil, err
		}
		val, err := interp.tableGet(tbl, StringValue(e.Field))
		if err != nil {
			return nil, err
		}
		return []Value{val}, nil

	case *ast.CallExpr:
		return interp.evalCall(e, env)

	case *ast.MethodCallExpr:
		return interp.evalMethodCall(e, env)

	case *ast.FuncLitExpr:
		v := interp.makeClosure(e.Params, e.Body, "", env)
		return []Value{v}, nil

	case *ast.TableLitExpr:
		v, err := interp.evalTableLit(e, env)
		if err != nil {
			return nil, err
		}
		return []Value{v}, nil

	case *ast.MakeChanExpr:
		cap := 0
		if e.Size != nil {
			sizeVal, err := interp.evalExprSingle(e.Size, env)
			if err != nil {
				return nil, err
			}
			cap, err = ChannelCapacityFromValue(sizeVal, "make(chan)")
			if err != nil {
				return nil, err
			}
		}
		ch := NewChannel(cap)
		return []Value{ChannelValue(ch)}, nil

	case *ast.RecvExpr:
		chVal, err := interp.evalExprSingle(e.Channel, env)
		if err != nil {
			return nil, err
		}
		if !chVal.IsChannel() {
			return nil, fmt.Errorf("receive from non-channel value")
		}
		val, _ := chVal.Channel().Recv()
		return []Value{val}, nil

	default:
		return nil, fmt.Errorf("unknown expression type: %T", expr)
	}
}

// evalExprSingle evaluates an expression and returns a single Value.
// For VarArgExpr, returns the varargs table itself (not the first expanded element),
// so that #... and ...[i] work correctly.
func (interp *Interpreter) evalExprSingle(expr ast.Expr, env *Environment) (Value, error) {
	// Special case: VarArgExpr in single-value context returns the table.
	if _, ok := expr.(*ast.VarArgExpr); ok {
		v, ok := env.Get("...")
		if !ok {
			return NilValue(), nil
		}
		return v, nil
	}
	vals, err := interp.evalExpr(expr, env)
	if err != nil {
		return NilValue(), err
	}
	if len(vals) == 0 {
		return NilValue(), nil
	}
	return vals[0], nil
}

// evalExprList evaluates a list of expressions, expanding the last one for
// multiple return values.
func (interp *Interpreter) evalExprList(exprs []ast.Expr, env *Environment) ([]Value, error) {
	if len(exprs) == 0 {
		return nil, nil
	}
	var result []Value
	for i, expr := range exprs {
		if spreadExpr, ok := explicitSpreadExpr(expr); ok {
			vals, err := interp.evalExpr(spreadExpr, env)
			if err != nil {
				return nil, err
			}
			result = append(result, vals...)
			continue
		}
		vals, err := interp.evalExpr(expr, env)
		if err != nil {
			return nil, err
		}
		if i == len(exprs)-1 {
			// Last expression: expand all return values
			result = append(result, vals...)
		} else {
			// Not last: take only first value
			if len(vals) > 0 {
				result = append(result, vals[0])
			} else {
				result = append(result, NilValue())
			}
		}
	}
	return result, nil
}

// explicitSpreadExpr recognizes GScript's explicit multi-value expansion forms.
// spread(expr) expands expr's values, while table.spread is an ordinary
// multi-return call that opts in to expansion at any list position.
func explicitSpreadExpr(expr ast.Expr) (ast.Expr, bool) {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return nil, false
	}
	if ident, ok := call.Func.(*ast.IdentExpr); ok && ident.Name == "spread" && len(call.Args) == 1 {
		return call.Args[0], true
	}
	if field, ok := call.Func.(*ast.FieldExpr); ok {
		if ident, ok := field.Table.(*ast.IdentExpr); ok && ident.Name == "table" &&
			field.Field == "spread" {
			return call, true
		}
	}
	return nil, false
}

// ------------------------------------------------------------------
// Binary expressions
// ------------------------------------------------------------------
func (interp *Interpreter) evalBinary(e *ast.BinaryExpr, env *Environment) (Value, error) {
	// Short-circuit operators
	if e.Op == "&&" {
		left, err := interp.evalExprSingle(e.Left, env)
		if err != nil {
			return NilValue(), err
		}
		if !left.Truthy() {
			return left, nil
		}
		return interp.evalExprSingle(e.Right, env)
	}
	if e.Op == "||" {
		left, err := interp.evalExprSingle(e.Left, env)
		if err != nil {
			return NilValue(), err
		}
		if left.Truthy() {
			return left, nil
		}
		return interp.evalExprSingle(e.Right, env)
	}

	left, err := interp.evalExprSingle(e.Left, env)
	if err != nil {
		return NilValue(), err
	}
	right, err := interp.evalExprSingle(e.Right, env)
	if err != nil {
		return NilValue(), err
	}

	switch e.Op {
	case "+", "-", "*", "/", "%", "**":
		return interp.arith(e.Op, left, right)
	case "&", "|", "^", "&^", "<<", ">>":
		return interp.bitwise(e.Op, left, right)
	case "..":
		return interp.concat(left, right)
	case "==":
		eq, err := interp.valEqual(left, right)
		if err != nil {
			return NilValue(), err
		}
		return BoolValue(eq), nil
	case "!=":
		eq, err := interp.valEqual(left, right)
		if err != nil {
			return NilValue(), err
		}
		return BoolValue(!eq), nil
	case "<":
		res, err := interp.valLessThan(left, right)
		if err != nil {
			return NilValue(), err
		}
		return BoolValue(res), nil
	case "<=":
		res, err := interp.valLessEqual(left, right)
		if err != nil {
			return NilValue(), err
		}
		return BoolValue(res), nil
	case ">":
		// a > b is equivalent to b < a
		res, err := interp.valLessThan(right, left)
		if err != nil {
			return NilValue(), err
		}
		return BoolValue(res), nil
	case ">=":
		// a >= b is equivalent to b <= a
		res, err := interp.valLessEqual(right, left)
		if err != nil {
			return NilValue(), err
		}
		return BoolValue(res), nil
	default:
		return NilValue(), fmt.Errorf("unknown binary operator: %s", e.Op)
	}
}

func (interp *Interpreter) bitwise(op string, left, right Value) (Value, error) {
	l, lok := left.ToNumber()
	r, rok := right.ToNumber()
	if !lok || !rok {
		return NilValue(), fmt.Errorf("attempt to perform bitwise operation on a %s value", map[bool]string{true: right.TypeName(), false: left.TypeName()}[lok])
	}
	a, b := toInt(l), toInt(r)
	switch op {
	case "&":
		return IntValue(a & b), nil
	case "|":
		return IntValue(a | b), nil
	case "^":
		return IntValue(a ^ b), nil
	case "&^":
		return IntValue(a &^ b), nil
	case "<<":
		if b < 0 {
			return NilValue(), fmt.Errorf("negative shift count")
		}
		if b >= 64 {
			return IntValue(0), nil
		}
		return IntValue(int64(uint64(a) << uint(b))), nil
	case ">>":
		if b < 0 {
			return NilValue(), fmt.Errorf("negative shift count")
		}
		if b >= 64 {
			if a < 0 {
				return IntValue(-1), nil
			}
			return IntValue(0), nil
		}
		return IntValue(a >> uint(b)), nil
	default:
		return NilValue(), fmt.Errorf("unknown bitwise operator: %s", op)
	}
}

// arith performs arithmetic on two values, with metamethod fallback.
func (interp *Interpreter) arith(op string, left, right Value) (Value, error) {
	// Try to coerce strings to numbers
	l, lok := left.ToNumber()
	r, rok := right.ToNumber()
	if !lok || !rok {
		// Try metamethod before giving up
		mmName := opToMetamethod(op)
		if mmName != "" {
			if mm, ok := interp.getMetamethod(left, mmName); ok {
				results, err := interp.callFunction(mm, []Value{left, right})
				if err != nil {
					return NilValue(), err
				}
				if len(results) > 0 {
					return results[0], nil
				}
				return NilValue(), nil
			}
			if mm, ok := interp.getMetamethod(right, mmName); ok {
				results, err := interp.callFunction(mm, []Value{left, right})
				if err != nil {
					return NilValue(), err
				}
				if len(results) > 0 {
					return results[0], nil
				}
				return NilValue(), nil
			}
		}
		return NilValue(), fmt.Errorf("attempt to perform arithmetic on a %s value", map[bool]string{true: right.TypeName(), false: left.TypeName()}[lok])
	}
	left, right = l, r

	// If both are ints and the op keeps integer domain, use int arithmetic
	if left.IsInt() && right.IsInt() {
		a, b := left.Int(), right.Int()
		switch op {
		case "+":
			return IntValue(a + b), nil
		case "-":
			return IntValue(a - b), nil
		case "*":
			return IntValue(a * b), nil
		case "/":
			// Integer division produces float (like Lua 5.3 / operator)
			if b == 0 {
				return NilValue(), fmt.Errorf("attempt to divide by zero")
			}
			// If evenly divisible, keep int
			if a%b == 0 {
				return IntValue(a / b), nil
			}
			return FloatValue(float64(a) / float64(b)), nil
		case "%":
			if b == 0 {
				return NilValue(), fmt.Errorf("attempt to perform modulo by zero")
			}
			r := a % b
			if r != 0 && (r^b) < 0 {
				r += b
			}
			return IntValue(r), nil
		case "**":
			if b >= 0 && b < 64 {
				return IntValue(intPow(a, b)), nil
			}
			return FloatValue(math.Pow(float64(a), float64(b))), nil
		}
	}

	// Float arithmetic
	a, b := left.Number(), right.Number()
	switch op {
	case "+":
		return FloatValue(a + b), nil
	case "-":
		return FloatValue(a - b), nil
	case "*":
		return FloatValue(a * b), nil
	case "/":
		if b == 0 {
			return NilValue(), fmt.Errorf("attempt to divide by zero")
		}
		return FloatValue(a / b), nil
	case "%":
		if b == 0 {
			return NilValue(), fmt.Errorf("attempt to perform modulo by zero")
		}
		r := math.Mod(a, b)
		if r != 0 && (r < 0) != (b < 0) {
			r += b
		}
		return FloatValue(r), nil
	case "**":
		return FloatValue(math.Pow(a, b)), nil
	default:
		return NilValue(), fmt.Errorf("unknown arithmetic operator: %s", op)
	}
}

// intPow computes a^b for non-negative b using exponentiation by squaring.
func intPow(a, b int64) int64 {
	result := int64(1)
	for b > 0 {
		if b&1 == 1 {
			result *= a
		}
		a *= a
		b >>= 1
	}
	return result
}

// concat performs string concatenation, with __concat metamethod fallback.
func (interp *Interpreter) concat(left, right Value) (Value, error) {
	ls := valueToStr(left)
	rs := valueToStr(right)
	canL := canConcatType(left)
	canR := canConcatType(right)
	if canL && canR {
		return StringValue(ls + rs), nil
	}
	// Try __concat metamethod
	if mm, ok := interp.getMetamethod(left, "__concat"); ok {
		results, err := interp.callFunction(mm, []Value{left, right})
		if err != nil {
			return NilValue(), err
		}
		if len(results) > 0 {
			return results[0], nil
		}
		return NilValue(), nil
	}
	if mm, ok := interp.getMetamethod(right, "__concat"); ok {
		results, err := interp.callFunction(mm, []Value{left, right})
		if err != nil {
			return NilValue(), err
		}
		if len(results) > 0 {
			return results[0], nil
		}
		return NilValue(), nil
	}
	if !canL {
		return NilValue(), fmt.Errorf("attempt to concatenate a %s value", left.TypeName())
	}
	return NilValue(), fmt.Errorf("attempt to concatenate a %s value", right.TypeName())
}

// valEqual compares two values for equality, with __eq metamethod support.
func (interp *Interpreter) valEqual(a, b Value) (bool, error) {
	// For primitive types, use raw equality
	if a.IsTable() && b.IsTable() {
		if a.Table() == b.Table() {
			return true, nil
		}
		// Try __eq from a's metatable, then b's
		if mm, ok := interp.getMetamethod(a, "__eq"); ok {
			results, err := interp.callFunction(mm, []Value{a, b})
			if err != nil {
				return false, err
			}
			if len(results) > 0 {
				return results[0].Truthy(), nil
			}
			return false, nil
		}
		if mm, ok := interp.getMetamethod(b, "__eq"); ok {
			results, err := interp.callFunction(mm, []Value{a, b})
			if err != nil {
				return false, err
			}
			if len(results) > 0 {
				return results[0].Truthy(), nil
			}
			return false, nil
		}
		return false, nil
	}
	return a.Equal(b), nil
}

// valLessThan compares two values with < operator, with __lt metamethod support.
func (interp *Interpreter) valLessThan(a, b Value) (bool, error) {
	// Try normal comparison first
	ok, valid := a.LessThan(b)
	if valid {
		return ok, nil
	}
	// Try __lt metamethod
	if mm, found := interp.getMetamethod(a, "__lt"); found {
		results, err := interp.callFunction(mm, []Value{a, b})
		if err != nil {
			return false, err
		}
		if len(results) > 0 {
			return results[0].Truthy(), nil
		}
		return false, nil
	}
	if mm, found := interp.getMetamethod(b, "__lt"); found {
		results, err := interp.callFunction(mm, []Value{a, b})
		if err != nil {
			return false, err
		}
		if len(results) > 0 {
			return results[0].Truthy(), nil
		}
		return false, nil
	}
	return false, fmt.Errorf("attempt to compare %s with %s", a.TypeName(), b.TypeName())
}

// valLessEqual compares two values with <= operator, with __le metamethod support.
func (interp *Interpreter) valLessEqual(a, b Value) (bool, error) {
	// Try normal comparison first
	less, valid := a.LessThan(b)
	if valid {
		return less || a.Equal(b), nil
	}
	// Try __le metamethod
	if mm, found := interp.getMetamethod(a, "__le"); found {
		results, err := interp.callFunction(mm, []Value{a, b})
		if err != nil {
			return false, err
		}
		if len(results) > 0 {
			return results[0].Truthy(), nil
		}
		return false, nil
	}
	if mm, found := interp.getMetamethod(b, "__le"); found {
		results, err := interp.callFunction(mm, []Value{a, b})
		if err != nil {
			return false, err
		}
		if len(results) > 0 {
			return results[0].Truthy(), nil
		}
		return false, nil
	}
	return false, fmt.Errorf("attempt to compare %s with %s", a.TypeName(), b.TypeName())
}

func canConcatType(v Value) bool {
	return v.IsString() || v.IsNumber()
}

func valueToStr(v Value) string {
	switch v.Type() {
	case TypeString:
		return v.Str()
	case TypeInt:
		return strconv.FormatInt(v.Int(), 10)
	case TypeFloat:
		return strconv.FormatFloat(v.Float(), 'g', -1, 64)
	default:
		return ""
	}
}

// ------------------------------------------------------------------
// Unary expressions
// ------------------------------------------------------------------
func (interp *Interpreter) evalUnary(e *ast.UnaryExpr, env *Environment) (Value, error) {
	operand, err := interp.evalExprSingle(e.Operand, env)
	if err != nil {
		return NilValue(), err
	}
	switch e.Op {
	case "-":
		n, ok := operand.ToNumber()
		if !ok {
			// Try __unm metamethod
			if mm, found := interp.getMetamethod(operand, "__unm"); found {
				results, err := interp.callFunction(mm, []Value{operand})
				if err != nil {
					return NilValue(), err
				}
				if len(results) > 0 {
					return results[0], nil
				}
				return NilValue(), nil
			}
			return NilValue(), fmt.Errorf("attempt to perform arithmetic on a %s value", operand.TypeName())
		}
		if n.IsInt() {
			return IntValue(-n.Int()), nil
		}
		return FloatValue(-n.Float()), nil
	case "!":
		return BoolValue(!operand.Truthy()), nil
	case "#":
		// Check __len metamethod first for tables
		if operand.IsTable() {
			if mm, ok := interp.getMetamethod(operand, "__len"); ok {
				results, err := interp.callFunction(mm, []Value{operand})
				if err != nil {
					return NilValue(), err
				}
				if len(results) > 0 {
					return results[0], nil
				}
				return NilValue(), nil
			}
		}
		switch operand.Type() {
		case TypeString:
			return IntValue(int64(StringLen(operand))), nil
		case TypeTable:
			return IntValue(int64(operand.Table().Length())), nil
		default:
			return NilValue(), fmt.Errorf("attempt to get length of a %s value", operand.TypeName())
		}
	case "^":
		n, ok := operand.ToNumber()
		if !ok {
			return NilValue(), fmt.Errorf("attempt to perform bitwise operation on a %s value", operand.TypeName())
		}
		return IntValue(^toInt(n)), nil
	default:
		return NilValue(), fmt.Errorf("unknown unary operator: %s", e.Op)
	}
}

// ------------------------------------------------------------------
// Call expressions
// ------------------------------------------------------------------
func (interp *Interpreter) evalCall(e *ast.CallExpr, env *Environment) ([]Value, error) {
	fn, err := interp.evalExprSingle(e.Func, env)
	if err != nil {
		return nil, err
	}

	// Build args with last-arg expansion
	args, err := interp.evalExprList(e.Args, env)
	if err != nil {
		return nil, err
	}

	return interp.callFunction(fn, args)
}

func (interp *Interpreter) evalMethodCall(e *ast.MethodCallExpr, env *Environment) ([]Value, error) {
	obj, err := interp.evalExprSingle(e.Object, env)
	if err != nil {
		return nil, err
	}

	method, err := interp.methodValue(obj, e.Method)
	if err != nil {
		return nil, err
	}

	if !method.IsFunction() {
		return nil, fmt.Errorf("attempt to call a %s value (method '%s' not found)", method.TypeName(), e.Method)
	}

	args, err := interp.evalExprList(e.Args, env)
	if err != nil {
		return nil, err
	}
	// Prepend self as first argument
	args = append([]Value{obj}, args...)

	return interp.callFunction(method, args)
}

func (interp *Interpreter) methodValue(obj Value, methodName string) (Value, error) {
	if obj.IsTable() {
		return interp.tableGet(obj, StringValue(methodName))
	}
	if obj.IsString() && interp.stringMeta != nil {
		idx := interp.stringMeta.RawGet(StringValue("__index"))
		if idx.IsTable() {
			return idx.Table().RawGet(StringValue(methodName)), nil
		}
	}
	return NilValue(), fmt.Errorf("attempt to call method on a %s value", obj.TypeName())
}

// callFunction invokes a function value with the given arguments.
// If fn is a table with a __call metamethod, invokes that instead.
func (interp *Interpreter) callFunction(fn Value, args []Value) ([]Value, error) {
	if !fn.IsFunction() {
		// Try __call metamethod for tables
		if fn.IsTable() {
			if mm, ok := interp.getMetamethod(fn, "__call"); ok {
				// Prepend the table itself as first arg (Lua convention)
				newArgs := make([]Value, 0, len(args)+1)
				newArgs = append(newArgs, fn)
				newArgs = append(newArgs, args...)
				return interp.callFunction(mm, newArgs)
			}
		}
		return nil, fmt.Errorf("attempt to call a %s value", fn.TypeName())
	}

	if gf := fn.GoFunction(); gf != nil {
		interp.pushDebugFrame(gf.Name, "native")
		defer interp.popDebugFrame()
		if err := interp.emitDebugHook("call", "native", gf.Name, NilValue()); err != nil {
			return nil, err
		}
		results, err := gf.Fn(args)
		if err != nil {
			_ = interp.emitDebugHook("error", "native", gf.Name, StringValue(err.Error()))
			return nil, err
		}
		if err := interp.emitDebugHook("return", "native", gf.Name, NilValue()); err != nil {
			return nil, err
		}
		return results, nil
	}

	cl := fn.Closure()
	if cl == nil {
		return nil, fmt.Errorf("attempt to call a nil function")
	}

	// Create a new environment for the function body.
	// Parent is the globals so that built-in functions are accessible.
	// Captured upvalues are injected directly -- they provide lexical scoping.
	callEnv := NewEnvironment(interp.globals)

	// Inject captured upvalues: these share the same *Upvalue pointer as the
	// enclosing scope, so mutations are visible to all closures that share them.
	for name, uv := range cl.Upvalues {
		callEnv.DefineUpvalue(name, uv)
	}

	proto := cl.Proto
	name := proto.Name
	if name == "" {
		name = "<anonymous>"
	}
	interp.pushDebugFrameWithSource(name, "script", proto.SourceName, proto.Line, proto.Column)
	defer interp.popDebugFrame()
	interp.pushDeferFrame()
	oldSourceName := interp.currentSourceName
	if proto.SourceName != "" {
		interp.currentSourceName = proto.SourceName
	}
	defer func() {
		interp.currentSourceName = oldSourceName
	}()
	if err := interp.emitDebugHook("call", "script", name, NilValue()); err != nil {
		_ = interp.runAndPopDeferFrame()
		return nil, err
	}

	// Bind parameters (as new local variables -- these shadow any captured upvalues)
	nParams := len(proto.Params)
	if proto.HasVarArg {
		nParams-- // last param is the vararg collector name
	}

	for i := 0; i < nParams; i++ {
		v := NilValue()
		if i < len(args) {
			v = args[i]
		}
		callEnv.Define(proto.Params[i], v)
	}

	if proto.HasVarArg {
		// Collect remaining args into a table stored as "..."
		varargs := NewTable()
		start := nParams
		for i := start; i < len(args); i++ {
			varargs.RawSet(IntValue(int64(i-start+1)), args[i])
		}
		callEnv.Define("...", TableValue(varargs))
	}

	retVals, isRet, _, _, err := interp.execBlockInEnv(proto.Body, callEnv)
	deferErr := interp.runAndPopDeferFrame()
	if err != nil {
		var jump *gotoSignal
		if errors.As(err, &jump) {
			err = fmt.Errorf("goto %q target not found", jump.name)
		}
		_ = interp.emitDebugHook("error", "script", name, StringValue(err.Error()))
		return nil, err
	}
	if deferErr != nil {
		_ = interp.emitDebugHook("error", "script", name, StringValue(deferErr.Error()))
		return nil, deferErr
	}
	if err := interp.emitDebugHook("return", "script", name, NilValue()); err != nil {
		return nil, err
	}
	if isRet {
		return retVals, nil
	}
	return nil, nil
}

// CallFunction calls a GScript function value with the given args.
// This is a public method for embedding use.
func (interp *Interpreter) CallFunction(fn Value, args []Value) ([]Value, error) {
	return interp.callFunction(fn, args)
}

// ------------------------------------------------------------------
// Function literal
// ------------------------------------------------------------------
func (interp *Interpreter) makeClosure(params []ast.FuncParam, body *ast.BlockStmt, name string, env *Environment) Value {
	proto := &FuncProto{
		Name:       name,
		Body:       body,
		SourceName: interp.currentSourceName,
		Line:       body.P.Line,
		Column:     body.P.Column,
	}
	paramNames := make([]string, 0, len(params))
	for _, p := range params {
		paramNames = append(paramNames, p.Name)
		proto.Params = append(proto.Params, p.Name)
		if p.IsVarArg {
			proto.HasVarArg = true
		}
	}

	// Capture free variables from the enclosing environment
	freeVarNames := FreeVars(body, paramNames)
	upvalues := make(map[string]*Upvalue)
	for _, fv := range freeVarNames {
		if uv, ok := env.GetUpvalue(fv); ok {
			upvalues[fv] = uv
		}
		// If not found in any scope, it's a global or builtin -- don't capture.
		// It will be resolved via interp.globals at call time.
	}

	closure := &Closure{
		Proto:    proto,
		Upvalues: upvalues,
		Env:      env,
	}
	return FunctionValue(closure)
}

// ------------------------------------------------------------------
// Table literal
// ------------------------------------------------------------------
func (interp *Interpreter) evalTableLit(e *ast.TableLitExpr, env *Environment) (Value, error) {
	tbl := NewTable()
	arrayIdx := int64(1) // 1-indexed auto-incrementing key for positional fields

	for i, field := range e.Fields {
		if field.Key == nil {
			// Array-style (positional)
			if spreadExpr, ok := explicitSpreadExpr(field.Value); ok {
				vals, err := interp.evalExpr(spreadExpr, env)
				if err != nil {
					return NilValue(), err
				}
				for _, v := range vals {
					tbl.RawSet(IntValue(arrayIdx), v)
					arrayIdx++
				}
				continue
			}
			var val Value
			var err error
			if i == len(e.Fields)-1 {
				// Last field: expand multiple returns
				vals, err2 := interp.evalExpr(field.Value, env)
				if err2 != nil {
					return NilValue(), err2
				}
				for _, v := range vals {
					tbl.RawSet(IntValue(arrayIdx), v)
					arrayIdx++
				}
				continue
			}
			val, err = interp.evalExprSingle(field.Value, env)
			if err != nil {
				return NilValue(), err
			}
			tbl.RawSet(IntValue(arrayIdx), val)
			arrayIdx++
		} else {
			// Keyed field
			key, err := interp.evalExprSingle(field.Key, env)
			if err != nil {
				return NilValue(), err
			}
			val, err := interp.evalExprSingle(field.Value, env)
			if err != nil {
				return NilValue(), err
			}
			tbl.RawSet(key, val)
		}
	}

	return TableValue(tbl), nil
}

// ------------------------------------------------------------------
// Number parsing
// ------------------------------------------------------------------
func tonumberWithBase(value, baseValue Value) (Value, bool) {
	if !value.IsString() {
		return NilValue(), false
	}
	base := toInt(baseValue)
	if base < 2 || base > 36 {
		return NilValue(), false
	}
	s := strings.TrimSpace(value.Str())
	if s == "" {
		return NilValue(), false
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
		return NilValue(), false
	}

	var acc uint64
	for i := 0; i < len(s); i++ {
		d := digitValue(s[i])
		if d < 0 || int64(d) >= base {
			return NilValue(), false
		}
		acc = acc*uint64(base) + uint64(d)
	}
	if sign < 0 {
		if acc <= uint64(math.MaxInt64)+1 {
			return IntValue(-int64(acc)), true
		}
		return FloatValue(-float64(acc)), true
	}
	if acc <= uint64(math.MaxInt64) {
		return IntValue(int64(acc)), true
	}
	return FloatValue(float64(acc)), true
}

func digitValue(c byte) int {
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

func parseNumber(s string) (Value, error) {
	if i, err := strconv.ParseInt(s, 0, 64); err == nil {
		return IntValue(i), nil
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return FloatValue(f), nil
	}
	return NilValue(), fmt.Errorf("invalid number: %s", s)
}
