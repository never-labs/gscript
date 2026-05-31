package gscript

import (
	"reflect"

	"github.com/never-labs/gscript/internal/binding"
	"github.com/never-labs/gscript/internal/runtime"
)

var hostBinding = binding.Converter{
	PublicValueType: reflect.TypeOf(Value{}),
	FromPublic: func(v any) runtime.Value {
		return fromPublic(v.(Value))
	},
	ToPublic: func(v runtime.Value) any {
		return toPublic(v)
	},
	HostCallbackError: func(name string, err error) error {
		return &HostCallbackError{Name: name, Err: err}
	},
	HostCallbackPanicErr: func(name string, value any) error {
		return &HostCallbackPanicError{Name: name, Value: value}
	},
}

func toValue(v interface{}) (runtime.Value, error) {
	return hostBinding.ToValue(v)
}

func fromValue(val runtime.Value, target reflect.Type) (reflect.Value, error) {
	return hostBinding.FromValue(val, target)
}

func fromValueDefault(val runtime.Value) (reflect.Value, error) {
	return hostBinding.FromValueDefault(val)
}

func wrapGoFunc(fn reflect.Value) (*runtime.GoFunction, error) {
	return hostBinding.WrapGoFunc(fn)
}

func bindStructToInterp(interp *runtime.Interpreter, name string, proto interface{}, customCtor interface{}) error {
	return hostBinding.BindStructToInterp(interp, name, proto, customCtor)
}
