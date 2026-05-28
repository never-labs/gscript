package gscript

import (
	"fmt"
	"reflect"

	"github.com/gscript/gscript/internal/runtime"
)

// Kind identifies the public kind of a GScript value.
type Kind string

const (
	KindNil       Kind = "nil"
	KindBool      Kind = "bool"
	KindInt       Kind = "int"
	KindFloat     Kind = "float"
	KindString    Kind = "string"
	KindTable     Kind = "table"
	KindFunction  Kind = "function"
	KindCoroutine Kind = "coroutine"
	KindChannel   Kind = "channel"
	KindUnknown   Kind = "unknown"
)

// Value is the public representation of a GScript value.
//
// It intentionally hides the internal runtime value layout. Existing APIs that
// still accept interface{} can also accept Value directly.
type Value struct {
	value runtime.Value
	valid bool
}

// Nil constructs a nil GScript value.
func Nil() Value {
	return toPublic(runtime.NilValue())
}

// Bool constructs a boolean GScript value.
func Bool(v bool) Value {
	return toPublic(runtime.BoolValue(v))
}

// Int constructs an integer GScript value.
func Int(v int64) Value {
	return toPublic(runtime.IntValue(v))
}

// Float constructs a floating-point GScript value.
func Float(v float64) Value {
	return toPublic(runtime.FloatValue(v))
}

// String constructs a string GScript value.
func String(v string) Value {
	return toPublic(runtime.StringValue(v))
}

// Decode converts a Go value into a public GScript value using the existing
// reflection conversion rules.
func Decode(v interface{}) (Value, error) {
	rv, err := ToValue(v)
	if err != nil {
		return Value{}, err
	}
	return toPublic(rv), nil
}

// Encode converts a public GScript value back into its default Go
// representation.
func Encode(v Value) (interface{}, error) {
	rv, err := FromValue(fromPublic(v), nil)
	if err != nil {
		return nil, err
	}
	if !rv.IsValid() {
		return nil, nil
	}
	return rv.Interface(), nil
}

func toPublic(v runtime.Value) Value {
	return Value{value: v, valid: true}
}

func fromPublic(v Value) runtime.Value {
	if !v.valid {
		return runtime.NilValue()
	}
	return v.value
}

// Kind returns the value kind.
func (v Value) Kind() Kind {
	switch fromPublic(v).Type() {
	case runtime.TypeNil:
		return KindNil
	case runtime.TypeBool:
		return KindBool
	case runtime.TypeInt:
		return KindInt
	case runtime.TypeFloat:
		return KindFloat
	case runtime.TypeString:
		return KindString
	case runtime.TypeTable:
		return KindTable
	case runtime.TypeFunction:
		return KindFunction
	case runtime.TypeCoroutine:
		return KindCoroutine
	case runtime.TypeChannel:
		return KindChannel
	default:
		return KindUnknown
	}
}

// IsNil reports whether the value is nil.
func (v Value) IsNil() bool {
	return fromPublic(v).IsNil()
}

// Bool returns the value as a bool. Non-bool values return false.
func (v Value) Bool() bool {
	rv := fromPublic(v)
	if !rv.IsBool() {
		return false
	}
	return rv.Bool()
}

// Int returns the value as an int64. Non-int values return 0.
func (v Value) Int() int64 {
	rv := fromPublic(v)
	if !rv.IsInt() {
		return 0
	}
	return rv.Int()
}

// Float returns the value as a float64. Int values are converted to float64;
// other non-float values return 0.
func (v Value) Float() float64 {
	rv := fromPublic(v)
	if rv.IsInt() {
		return float64(rv.Int())
	}
	if !rv.IsFloat() {
		return 0
	}
	return rv.Float()
}

// String returns the value's string form.
func (v Value) String() string {
	return fromPublic(v).String()
}

// Encode converts the value back into its default Go representation.
func (v Value) Encode() (interface{}, error) {
	return Encode(v)
}

// Decode stores the Go value converted with the existing reflection rules.
func (v *Value) Decode(src interface{}) error {
	if v == nil {
		return fmt.Errorf("gscript.Value.Decode: nil receiver")
	}
	decoded, err := Decode(src)
	if err != nil {
		return err
	}
	*v = decoded
	return nil
}

// To stores the value in target using the existing FromValue conversion rules.
func (v Value) To(target reflect.Type) (reflect.Value, error) {
	return FromValue(fromPublic(v), target)
}
