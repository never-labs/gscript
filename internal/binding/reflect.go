package binding

import (
	"fmt"
	"reflect"

	"github.com/never-labs/leia/internal/runtime"
)

// toValue converts a Go value to an internal Leia value using reflection.
// Supported types:
//
//	nil               -> nil
//	bool              -> bool
//	int/int8/.../int64 -> int
//	uint/uint8/.../uint64 -> int (truncated)
//	float32/float64   -> float
//	string            -> string
//	[]T               -> table (1-based array)
//	map[string]T      -> table (hash)
//	struct / *struct   -> table (fields + methods via metatable)
//	func              -> function (reflected, see wrapGoFunc)
func (c Converter) ToValue(v interface{}) (runtime.Value, error) {
	if v == nil {
		return runtime.NilValue(), nil
	}

	if c.isPublicValue(v) {
		return c.FromPublic(v), nil
	}

	// Pass through if already a runtime.Value
	if rv, ok := v.(runtime.Value); ok {
		return rv, nil
	}

	return c.ReflectToValue(reflect.ValueOf(v))
}

func (c Converter) ReflectToValue(rv reflect.Value) (runtime.Value, error) {
	if !rv.IsValid() {
		return runtime.NilValue(), nil
	}

	// Dereference pointers
	for rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return runtime.NilValue(), nil
		}
		rv = rv.Elem()
	}

	switch rv.Kind() {
	case reflect.Bool:
		return runtime.BoolValue(rv.Bool()), nil

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return runtime.IntValue(rv.Int()), nil

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return runtime.IntValue(int64(rv.Uint())), nil

	case reflect.Float32, reflect.Float64:
		return runtime.FloatValue(rv.Float()), nil

	case reflect.String:
		return runtime.StringValue(rv.String()), nil

	case reflect.Slice, reflect.Array:
		if rv.Kind() == reflect.Slice && rv.IsNil() {
			return runtime.TableValue(runtime.NewTable()), nil
		}
		t := runtime.NewTable()
		for i := 0; i < rv.Len(); i++ {
			elem, err := c.ReflectToValue(rv.Index(i))
			if err != nil {
				return runtime.NilValue(), err
			}
			t.RawSet(runtime.IntValue(int64(i+1)), elem)
		}
		return runtime.TableValue(t), nil

	case reflect.Map:
		if rv.IsNil() {
			return runtime.TableValue(runtime.NewTable()), nil
		}
		t := runtime.NewTable()
		for _, key := range rv.MapKeys() {
			k, err := c.ReflectToValue(key)
			if err != nil {
				continue
			}
			v, err := c.ReflectToValue(rv.MapIndex(key))
			if err != nil {
				continue
			}
			t.RawSet(k, v)
		}
		return runtime.TableValue(t), nil

	case reflect.Struct:
		return c.structToValue(rv)

	case reflect.Func:
		fn, err := c.WrapGoFunc(rv)
		if err != nil {
			return runtime.NilValue(), err
		}
		return runtime.FunctionValue(fn), nil

	case reflect.Interface:
		if rv.IsNil() {
			return runtime.NilValue(), nil
		}
		return c.ReflectToValue(rv.Elem())
	}

	return runtime.NilValue(), fmt.Errorf("unsupported Go type: %s", rv.Type())
}

// fromValue converts an internal Leia value to a Go value of the target type.
// If target is nil, uses a default mapping (int64, float64, string, map, etc.)
func (c Converter) FromValue(val runtime.Value, target reflect.Type) (reflect.Value, error) {
	if target == nil {
		return c.FromValueDefault(val)
	}
	if target == c.PublicValueType {
		if out, ok := c.publicValue(val); ok {
			return out, nil
		}
	}

	// Handle pointer targets
	if target.Kind() == reflect.Ptr {
		elem, err := c.FromValue(val, target.Elem())
		if err != nil {
			return reflect.Value{}, err
		}
		ptr := reflect.New(target.Elem())
		ptr.Elem().Set(elem)
		return ptr, nil
	}

	switch target.Kind() {
	case reflect.Bool:
		return reflect.ValueOf(val.Truthy()).Convert(target), nil

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		var n int64
		if val.IsInt() {
			n = val.Int()
		} else if val.IsFloat() {
			n = int64(val.Float())
		} else if val.IsString() {
			nv, ok := val.ToNumber()
			if !ok {
				return reflect.Value{}, fmt.Errorf("cannot convert string %q to %s", val.Str(), target)
			}
			if nv.IsInt() {
				n = nv.Int()
			} else {
				n = int64(nv.Float())
			}
		} else {
			return reflect.Value{}, fmt.Errorf("cannot convert %s to %s", val.TypeName(), target)
		}
		return reflect.ValueOf(n).Convert(target), nil

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		var n uint64
		if val.IsInt() {
			n = uint64(val.Int())
		} else if val.IsFloat() {
			n = uint64(val.Float())
		} else {
			return reflect.Value{}, fmt.Errorf("cannot convert %s to %s", val.TypeName(), target)
		}
		return reflect.ValueOf(n).Convert(target), nil

	case reflect.Float32, reflect.Float64:
		var f float64
		if val.IsInt() {
			f = float64(val.Int())
		} else if val.IsFloat() {
			f = val.Float()
		} else if val.IsString() {
			nv, ok := val.ToNumber()
			if !ok {
				return reflect.Value{}, fmt.Errorf("cannot convert string %q to %s", val.Str(), target)
			}
			f = nv.Number()
		} else {
			return reflect.Value{}, fmt.Errorf("cannot convert %s to %s", val.TypeName(), target)
		}
		return reflect.ValueOf(f).Convert(target), nil

	case reflect.String:
		if val.IsString() {
			return reflect.ValueOf(val.Str()).Convert(target), nil
		}
		return reflect.ValueOf(val.String()).Convert(target), nil

	case reflect.Slice:
		if !val.IsTable() {
			return reflect.Value{}, fmt.Errorf("cannot convert %s to slice", val.TypeName())
		}
		t := val.Table()
		n := t.Length()
		slice := reflect.MakeSlice(target, n, n)
		for i := 1; i <= n; i++ {
			elem := t.RawGet(runtime.IntValue(int64(i)))
			goElem, err := c.FromValue(elem, target.Elem())
			if err != nil {
				return reflect.Value{}, err
			}
			slice.Index(i - 1).Set(goElem)
		}
		return slice, nil

	case reflect.Map:
		if !val.IsTable() {
			return reflect.Value{}, fmt.Errorf("cannot convert %s to map", val.TypeName())
		}
		m := reflect.MakeMap(target)
		t := val.Table()
		key := runtime.NilValue()
		for {
			k, v, ok := t.Next(key)
			if !ok {
				break
			}
			goKey, err := c.FromValue(k, target.Key())
			if err == nil {
				goVal, err := c.FromValue(v, target.Elem())
				if err == nil {
					m.SetMapIndex(goKey, goVal)
				}
			}
			key = k
		}
		return m, nil

	case reflect.Struct:
		return c.fromValueStruct(val, target)

	case reflect.Interface:
		if target.NumMethod() == 0 {
			// interface{}
			rv, err := c.FromValueDefault(val)
			if err != nil {
				return reflect.Value{}, err
			}
			result := reflect.New(target).Elem()
			if rv.IsValid() {
				result.Set(rv)
			}
			return result, nil
		}
		return reflect.Value{}, fmt.Errorf("cannot convert to interface %s", target)
	}

	return reflect.Value{}, fmt.Errorf("unsupported target type: %s", target)
}

// fromValueDefault converts without a target type hint.
func (c Converter) FromValueDefault(val runtime.Value) (reflect.Value, error) {
	switch {
	case val.IsNil():
		return reflect.ValueOf(nil), nil
	case val.IsBool():
		return reflect.ValueOf(val.Bool()), nil
	case val.IsInt():
		return reflect.ValueOf(val.Int()), nil
	case val.IsFloat():
		return reflect.ValueOf(val.Float()), nil
	case val.IsString():
		return reflect.ValueOf(val.Str()), nil
	case val.IsTable():
		// Try array first, then map
		t := val.Table()
		n := t.Length()
		if n > 0 {
			arr := make([]interface{}, n)
			for i := 1; i <= n; i++ {
				elem := t.RawGet(runtime.IntValue(int64(i)))
				rv, err := c.FromValueDefault(elem)
				if err == nil && rv.IsValid() {
					arr[i-1] = rv.Interface()
				}
			}
			return reflect.ValueOf(arr), nil
		}
		m := make(map[string]interface{})
		key := runtime.NilValue()
		for {
			k, v, ok := t.Next(key)
			if !ok {
				break
			}
			rv, err := c.FromValueDefault(v)
			if err == nil && rv.IsValid() {
				m[k.String()] = rv.Interface()
			}
			key = k
		}
		return reflect.ValueOf(m), nil
	case val.IsFunction():
		return reflect.ValueOf(val), nil
	}
	return reflect.ValueOf(nil), nil
}

// wrapGoFunc wraps a Go function (reflect.Value) as a GoFunction callable from Leia.
// It uses reflection to convert arguments and return values automatically.
func (c Converter) WrapGoFunc(fn reflect.Value) (*runtime.GoFunction, error) {
	fnType := fn.Type()
	if fnType.Kind() != reflect.Func {
		return nil, fmt.Errorf("expected func, got %s", fnType.Kind())
	}

	// Check if last return type is error
	hasError := fnType.NumOut() > 0 && fnType.Out(fnType.NumOut()-1).Implements(errorType)

	gf := &runtime.GoFunction{Name: fnType.String()}
	gf.Fn = func(args []runtime.Value) (results []runtime.Value, err error) {
		defer func() {
			if r := recover(); r != nil {
				results = nil
				err = c.hostCallbackPanicError(gf.Name, r)
			}
		}()

		numIn := fnType.NumIn()
		isVariadic := fnType.IsVariadic()

		in := make([]reflect.Value, numIn)

		for i := 0; i < numIn; i++ {
			if isVariadic && i == numIn-1 {
				// Variadic last param: collect remaining args
				sliceType := fnType.In(i)
				elemType := sliceType.Elem()
				remaining := len(args) - i
				if remaining < 0 {
					remaining = 0
				}
				slice := reflect.MakeSlice(sliceType, remaining, remaining)
				for j := 0; j < remaining; j++ {
					var scriptVal runtime.Value
					if i+j < len(args) {
						scriptVal = args[i+j]
					} else {
						scriptVal = runtime.NilValue()
					}
					rv, err := c.FromValue(scriptVal, elemType)
					if err != nil {
						return nil, fmt.Errorf("arg %d: %v", i+j, err)
					}
					slice.Index(j).Set(rv)
				}
				in[i] = slice
				break
			}
			argType := fnType.In(i)
			var scriptVal runtime.Value
			if i < len(args) {
				scriptVal = args[i]
			} else {
				scriptVal = runtime.NilValue()
			}
			rv, err := c.FromValue(scriptVal, argType)
			if err != nil {
				return nil, fmt.Errorf("arg %d: %v", i, err)
			}
			in[i] = rv
		}

		// Call the function
		var out []reflect.Value
		if isVariadic {
			out = fn.Call(in)
		} else {
			// Pad missing args with zero values
			for len(in) < numIn {
				in = append(in, reflect.Zero(fnType.In(len(in))))
			}
			out = fn.Call(in[:numIn])
		}

		// Process output
		numOut := len(out)
		if hasError && numOut > 0 {
			lastOut := out[numOut-1]
			if !lastOut.IsNil() {
				return nil, c.hostCallbackError(gf.Name, lastOut.Interface().(error))
			}
			out = out[:numOut-1]
		}

		result := make([]runtime.Value, 0, len(out))
		for _, rv := range out {
			scriptVal, err := c.ReflectToValue(rv)
			if err != nil {
				return nil, err
			}
			result = append(result, scriptVal)
		}
		return result, nil
	}
	return gf, nil
}

var errorType = reflect.TypeOf((*error)(nil)).Elem()
