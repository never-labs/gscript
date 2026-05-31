package runtime

import (
	"fmt"

	uuidlib "github.com/never-labs/gscript/internal/stdlib/base/uuid"
)

// buildUUIDLib adapts the low-coupling uuid stdlib implementation to runtime values.
func buildUUIDLib() *Table {
	t := NewTable()

	set := func(name string, fn func([]Value) ([]Value, error)) {
		t.RawSet(StringValue(name), FunctionValue(&GoFunction{
			Name: "uuid." + name,
			Fn:   fn,
		}))
	}

	// uuid.v4() -- generate random UUID v4 string
	set("v4", func(args []Value) ([]Value, error) {
		s, err := uuidlib.V4()
		if err != nil {
			return nil, fmt.Errorf("uuid.v4: %w", err)
		}
		return []Value{StringValue(s)}, nil
	})

	// uuid.v4Raw() -- UUID v4 without hyphens (32 hex chars)
	set("v4Raw", func(args []Value) ([]Value, error) {
		s, err := uuidlib.V4Raw()
		if err != nil {
			return nil, fmt.Errorf("uuid.v4Raw: %w", err)
		}
		return []Value{StringValue(s)}, nil
	})

	// uuid.isValid(s) -- bool (validates UUID format)
	set("isValid", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'uuid.isValid' (string expected)")
		}
		return []Value{BoolValue(uuidlib.IsValid(args[0].Str()))}, nil
	})

	// uuid.parse(s) -- parse UUID string -> {version, variant, bytes (hex string)}
	set("parse", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'uuid.parse' (string expected)")
		}
		s := args[0].Str()
		parsed, ok := uuidlib.Parse(s)
		if !ok {
			return []Value{NilValue(), StringValue("invalid UUID format")}, nil
		}

		result := NewTable()
		result.RawSet(StringValue("version"), IntValue(parsed.Version))
		result.RawSet(StringValue("variant"), StringValue(parsed.Variant))
		result.RawSet(StringValue("bytes"), StringValue(parsed.Bytes))

		return []Value{TableValue(result)}, nil
	})

	// uuid.nil() -- return the nil UUID
	set("nil", func(args []Value) ([]Value, error) {
		return []Value{StringValue(uuidlib.Nil())}, nil
	})

	return t
}
