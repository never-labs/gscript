package runtime

import (
	"errors"
	"fmt"
	"strings"
)

func protectedErrorValue(err error) Value {
	var luaErr *LuaError
	if errors.As(err, &luaErr) {
		return luaErr.Value
	}
	return StringValue(err.Error())
}

func BuildPCallFunction(call ScriptFunctionCaller) *GoFunction {
	return &GoFunction{
		Name: "pcall",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) == 0 {
				return nil, fmt.Errorf("bad argument #1 to 'pcall' (value expected)")
			}
			results, err := call(args[0], args[1:])
			if err != nil {
				return []Value{BoolValue(false), protectedErrorValue(err)}, nil
			}
			return append([]Value{BoolValue(true)}, results...), nil
		},
	}
}

func BuildXPCallFunction(call ScriptFunctionCaller) *GoFunction {
	return &GoFunction{
		Name: "xpcall",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) < 2 {
				return nil, fmt.Errorf("bad argument #%d to 'xpcall' (value expected)", len(args)+1)
			}
			results, err := call(args[0], args[2:])
			if err == nil {
				return append([]Value{BoolValue(true)}, results...), nil
			}
			handlerResults, handlerErr := call(args[1], []Value{protectedErrorValue(err)})
			if handlerErr != nil {
				return []Value{BoolValue(false), protectedErrorValue(handlerErr)}, nil
			}
			msg := NilValue()
			if len(handlerResults) > 0 {
				msg = handlerResults[0]
			}
			return []Value{BoolValue(false), msg}, nil
		},
	}
}

func BuildTypeFunction(typeNameValue func(Value) Value) *GoFunction {
	return &GoFunction{
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
		NativeKind: NativeKindStdType,
		NativeData: StdTypeIdentityPtr(),
	}
}

func BuildToStringFunction(call ScriptFunctionCaller) *GoFunction {
	return &GoFunction{
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
			s, err := LuaToStringWithCaller(args[0], call)
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
			s, err := LuaToStringWithCaller(arg, call)
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
			s, err := LuaToStringWithCaller(args[0], call)
			if err != nil {
				return NilValue(), err
			}
			return StringValue(s), nil
		},
	}
}

func LuaToStringWithCaller(v Value, call ScriptFunctionCaller) (string, error) {
	if v.IsTable() {
		if mt := v.Table().GetMetatable(); mt != nil {
			if mm := mt.RawGetString("__tostring"); !mm.IsNil() {
				results, err := call(mm, []Value{v})
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

func BuildTestkitProtectFunction(call ScriptFunctionCaller, accessAllowed func() bool) *GoFunction {
	return &GoFunction{
		Name: "testkit.protect",
		Fn: func(args []Value) ([]Value, error) {
			if accessAllowed != nil && !accessAllowed() {
				return nil, fmt.Errorf("testkit access disabled")
			}
			if len(args) < 1 {
				return nil, fmt.Errorf("bad argument #1 to 'testkit.protect' (function expected)")
			}
			if !args[0].IsFunction() {
				return nil, fmt.Errorf("bad argument #1 to 'testkit.protect' (function expected)")
			}
			results, err := call(args[0], args[1:])
			out := NewTable()
			if err != nil {
				out.RawSetString("ok", BoolValue(false))
				out.RawSetString("error", protectedErrorValue(err))
				return []Value{TableValue(out)}, nil
			}
			out.RawSetString("ok", BoolValue(true))
			out.RawSetString("values", TableValue(testkitArray(results)))
			out.RawSetString("n", IntValue(int64(len(results))))
			return []Value{TableValue(out)}, nil
		},
	}
}
