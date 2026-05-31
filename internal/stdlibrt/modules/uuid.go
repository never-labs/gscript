package modules

import (
	"fmt"

	"github.com/never-labs/gscript/internal/runtime"
	uuidlib "github.com/never-labs/gscript/internal/stdlib/uuid"
)

// buildUUIDLib adapts the low-coupling uuid stdlib implementation to runtime values.
func BuildUUID() *runtime.Table {
	t := runtime.NewTable()

	set := func(name string, fn func([]runtime.Value) ([]runtime.Value, error)) {
		t.RawSet(runtime.StringValue(name), runtime.FunctionValue(&runtime.GoFunction{
			Name: "uuid." + name,
			Fn:   fn,
		}))
	}

	// uuid.v4() -- generate random UUID v4 string
	set("v4", func(args []runtime.Value) ([]runtime.Value, error) {
		s, err := uuidlib.V4()
		if err != nil {
			return nil, fmt.Errorf("uuid.v4: %w", err)
		}
		return []runtime.Value{runtime.StringValue(s)}, nil
	})

	// uuid.v4Raw() -- UUID v4 without hyphens (32 hex chars)
	set("v4Raw", func(args []runtime.Value) ([]runtime.Value, error) {
		s, err := uuidlib.V4Raw()
		if err != nil {
			return nil, fmt.Errorf("uuid.v4Raw: %w", err)
		}
		return []runtime.Value{runtime.StringValue(s)}, nil
	})

	// uuid.isValid(s) -- bool (validates UUID format)
	set("isValid", func(args []runtime.Value) ([]runtime.Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'uuid.isValid' (string expected)")
		}
		return []runtime.Value{runtime.BoolValue(uuidlib.IsValid(args[0].Str()))}, nil
	})

	// uuid.parse(s) -- parse UUID string -> {version, variant, bytes (hex string)}
	set("parse", func(args []runtime.Value) ([]runtime.Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'uuid.parse' (string expected)")
		}
		s := args[0].Str()
		parsed, ok := uuidlib.Parse(s)
		if !ok {
			return []runtime.Value{runtime.NilValue(), runtime.StringValue("invalid UUID format")}, nil
		}

		result := runtime.NewTable()
		result.RawSet(runtime.StringValue("version"), runtime.IntValue(parsed.Version))
		result.RawSet(runtime.StringValue("variant"), runtime.StringValue(parsed.Variant))
		result.RawSet(runtime.StringValue("bytes"), runtime.StringValue(parsed.Bytes))

		return []runtime.Value{runtime.TableValue(result)}, nil
	})

	// uuid.nil() -- return the nil UUID
	set("nil", func(args []runtime.Value) ([]runtime.Value, error) {
		return []runtime.Value{runtime.StringValue(uuidlib.Nil())}, nil
	})

	return t
}
