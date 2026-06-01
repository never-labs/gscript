package runtime

import (
	"errors"
	"fmt"

	binfmt "github.com/never-labs/leia/internal/support/binaryfmt"
)

// These adapters back the runtime-native string.pack/unpack/packsize entries.
// The binary stdlib module has its own thin Value adapter in stdlibrt/modules;
// both paths share the format codec in internal/support/binaryfmt.
func binaryPackValues(apiName string, args []Value, maxHostResult int64) ([]Value, error) {
	if len(args) < 1 || !args[0].IsString() {
		return nil, fmt.Errorf("bad argument #1 to '%s' (format string expected)", apiName)
	}
	values := make([]binfmt.PackValue, 0, len(args)-1)
	for _, arg := range args[1:] {
		values = append(values, binaryPackValue(arg))
	}
	s, err := binfmt.Pack(apiName, args[0].Str(), values, maxHostResult)
	if err != nil {
		return nil, err
	}
	return []Value{StringValue(s)}, nil
}

func binaryUnpackValues(apiName string, args []Value) ([]Value, error) {
	if len(args) < 2 || !args[0].IsString() || !args[1].IsString() {
		return nil, fmt.Errorf("bad argument to '%s' (format and data strings expected)", apiName)
	}
	offset := 1
	if len(args) >= 3 {
		offset = int(toInt(args[2]))
	}
	values, next, err := binfmt.Unpack(apiName, args[0].Str(), args[1].Str(), offset)
	if err != nil {
		var resultErr binfmt.ResultError
		if errors.As(err, &resultErr) {
			return []Value{NilValue(), StringValue(resultErr.Error())}, nil
		}
		return nil, err
	}
	results := make([]Value, 0, len(values)+1)
	for _, value := range values {
		results = append(results, binaryUnpackedValue(value))
	}
	results = append(results, IntValue(int64(next)))
	return results, nil
}

func binarySizeValues(apiName string, args []Value) ([]Value, error) {
	if len(args) < 1 || !args[0].IsString() {
		return nil, fmt.Errorf("bad argument #1 to '%s' (format string expected)", apiName)
	}
	total, fixed, err := binfmt.Size(apiName, args[0].Str())
	if err != nil {
		return nil, err
	}
	if !fixed {
		return []Value{NilValue(), StringValue(apiName + ": variable-size field in format")}, nil
	}
	return []Value{IntValue(int64(total))}, nil
}

func binaryPackValue(v Value) binfmt.PackValue {
	return binfmt.PackValue{
		IsNumber: v.IsNumber(),
		IsString: v.IsString(),
		IsFloat:  v.IsFloat(),
		Int:      toInt(v),
		Float:    toFloat(v),
		String:   binaryString(v),
	}
}

func binaryString(v Value) string {
	if !v.IsString() {
		return ""
	}
	return v.Str()
}

func binaryUnpackedValue(v binfmt.UnpackedValue) Value {
	switch v.Kind {
	case binfmt.UnpackedInt:
		return IntValue(v.Int)
	case binfmt.UnpackedFloat:
		return FloatValue(v.Float)
	case binfmt.UnpackedString:
		return StringValue(v.String)
	default:
		return NilValue()
	}
}
