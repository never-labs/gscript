package binding

import (
	"fmt"
	"reflect"

	"github.com/never-labs/leia/internal/runtime"
)

// Converter owns Go host-value conversion without depending on the public SDK
// package. The root package supplies public Value and error adapters.
type Converter struct {
	PublicValueType      reflect.Type
	FromPublic           func(any) runtime.Value
	ToPublic             func(runtime.Value) any
	HostCallbackError    func(name string, err error) error
	HostCallbackPanicErr func(name string, value any) error
}

func (c Converter) isPublicValue(v any) bool {
	if c.PublicValueType == nil || c.FromPublic == nil || v == nil {
		return false
	}
	return reflect.TypeOf(v) == c.PublicValueType
}

func (c Converter) publicValue(val runtime.Value) (reflect.Value, bool) {
	if c.PublicValueType == nil || c.ToPublic == nil {
		return reflect.Value{}, false
	}
	return reflect.ValueOf(c.ToPublic(val)), true
}

func (c Converter) hostCallbackError(name string, err error) error {
	if c.HostCallbackError != nil {
		return c.HostCallbackError(name, err)
	}
	return fmt.Errorf("%s: %w", name, err)
}

func (c Converter) hostCallbackPanicError(name string, value any) error {
	if c.HostCallbackPanicErr != nil {
		return c.HostCallbackPanicErr(name, value)
	}
	return fmt.Errorf("%s panic: %v", name, value)
}
