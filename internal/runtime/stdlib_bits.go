package runtime

import (
	"fmt"
	gobits "math/bits"
)

// buildBitsLib creates the GScript-native "bits" standard library.
//
// The API intentionally uses Go-style names and 64-bit integer operations
// instead of mirroring Lua bitwise operator syntax. Lua compatibility can be
// layered over this library where needed.
func buildBitsLib() *Table {
	t := NewTable()

	set := func(name string, fn func([]Value) ([]Value, error)) {
		t.RawSet(StringValue(name), FunctionValue(&GoFunction{
			Name: "bits." + name,
			Fn:   fn,
		}))
	}

	set("and", func(args []Value) ([]Value, error) {
		if len(args) == 0 {
			return []Value{IntValue(-1)}, nil
		}
		result, err := bitsIntArg(args, 0, "bits.and")
		if err != nil {
			return nil, err
		}
		for i := 1; i < len(args); i++ {
			n, err := bitsIntArg(args, i, "bits.and")
			if err != nil {
				return nil, err
			}
			result &= n
		}
		return []Value{IntValue(result)}, nil
	})

	set("or", func(args []Value) ([]Value, error) {
		var result int64
		for i := range args {
			n, err := bitsIntArg(args, i, "bits.or")
			if err != nil {
				return nil, err
			}
			result |= n
		}
		return []Value{IntValue(result)}, nil
	})

	set("xor", func(args []Value) ([]Value, error) {
		var result int64
		for i := range args {
			n, err := bitsIntArg(args, i, "bits.xor")
			if err != nil {
				return nil, err
			}
			result ^= n
		}
		return []Value{IntValue(result)}, nil
	})

	set("not", func(args []Value) ([]Value, error) {
		n, err := bitsIntArg(args, 0, "bits.not")
		if err != nil {
			return nil, err
		}
		return []Value{IntValue(^n)}, nil
	})

	set("shl", func(args []Value) ([]Value, error) {
		n, shift, err := bitsShiftArgs(args, "bits.shl")
		if err != nil {
			return nil, err
		}
		if shift >= 64 {
			return []Value{IntValue(0)}, nil
		}
		return []Value{IntValue(int64(uint64(n) << shift))}, nil
	})

	set("shr", func(args []Value) ([]Value, error) {
		n, shift, err := bitsShiftArgs(args, "bits.shr")
		if err != nil {
			return nil, err
		}
		if shift >= 64 {
			return []Value{IntValue(0)}, nil
		}
		return []Value{IntValue(int64(uint64(n) >> shift))}, nil
	})

	set("sar", func(args []Value) ([]Value, error) {
		n, shift, err := bitsShiftArgs(args, "bits.sar")
		if err != nil {
			return nil, err
		}
		if shift >= 64 {
			if n < 0 {
				return []Value{IntValue(-1)}, nil
			}
			return []Value{IntValue(0)}, nil
		}
		return []Value{IntValue(n >> shift)}, nil
	})

	set("rotl", func(args []Value) ([]Value, error) {
		n, err := bitsIntArg(args, 0, "bits.rotl")
		if err != nil {
			return nil, err
		}
		shift, err := bitsIntArg(args, 1, "bits.rotl")
		if err != nil {
			return nil, err
		}
		return []Value{IntValue(int64(gobits.RotateLeft64(uint64(n), int(shift))))}, nil
	})

	set("rotr", func(args []Value) ([]Value, error) {
		n, err := bitsIntArg(args, 0, "bits.rotr")
		if err != nil {
			return nil, err
		}
		shift, err := bitsIntArg(args, 1, "bits.rotr")
		if err != nil {
			return nil, err
		}
		return []Value{IntValue(int64(gobits.RotateLeft64(uint64(n), -int(shift))))}, nil
	})

	set("test", func(args []Value) ([]Value, error) {
		n, pos, err := bitsPositionArgs(args, "bits.test")
		if err != nil {
			return nil, err
		}
		return []Value{BoolValue((uint64(n) & (uint64(1) << pos)) != 0)}, nil
	})

	set("set", func(args []Value) ([]Value, error) {
		n, pos, err := bitsPositionArgs(args, "bits.set")
		if err != nil {
			return nil, err
		}
		return []Value{IntValue(int64(uint64(n) | (uint64(1) << pos)))}, nil
	})

	set("clear", func(args []Value) ([]Value, error) {
		n, pos, err := bitsPositionArgs(args, "bits.clear")
		if err != nil {
			return nil, err
		}
		return []Value{IntValue(int64(uint64(n) &^ (uint64(1) << pos)))}, nil
	})

	set("toggle", func(args []Value) ([]Value, error) {
		n, pos, err := bitsPositionArgs(args, "bits.toggle")
		if err != nil {
			return nil, err
		}
		return []Value{IntValue(int64(uint64(n) ^ (uint64(1) << pos)))}, nil
	})

	set("ones", func(args []Value) ([]Value, error) {
		n, err := bitsIntArg(args, 0, "bits.ones")
		if err != nil {
			return nil, err
		}
		return []Value{IntValue(int64(gobits.OnesCount64(uint64(n))))}, nil
	})

	set("leadingZeros", func(args []Value) ([]Value, error) {
		n, err := bitsIntArg(args, 0, "bits.leadingZeros")
		if err != nil {
			return nil, err
		}
		return []Value{IntValue(int64(gobits.LeadingZeros64(uint64(n))))}, nil
	})

	set("trailingZeros", func(args []Value) ([]Value, error) {
		n, err := bitsIntArg(args, 0, "bits.trailingZeros")
		if err != nil {
			return nil, err
		}
		return []Value{IntValue(int64(gobits.TrailingZeros64(uint64(n))))}, nil
	})

	return t
}

func bitsIntArg(args []Value, index int, name string) (int64, error) {
	if index >= len(args) {
		return 0, fmt.Errorf("bad argument #%d to '%s' (integer expected)", index+1, name)
	}
	n, ok := args[index].ToNumber()
	if !ok {
		return 0, fmt.Errorf("bad argument #%d to '%s' (integer expected)", index+1, name)
	}
	return toInt(n), nil
}

func bitsShiftArgs(args []Value, name string) (int64, uint, error) {
	n, err := bitsIntArg(args, 0, name)
	if err != nil {
		return 0, 0, err
	}
	shift, err := bitsIntArg(args, 1, name)
	if err != nil {
		return 0, 0, err
	}
	if shift < 0 {
		return 0, 0, fmt.Errorf("bad argument #2 to '%s' (non-negative shift expected)", name)
	}
	return n, uint(shift), nil
}

func bitsPositionArgs(args []Value, name string) (int64, uint, error) {
	n, err := bitsIntArg(args, 0, name)
	if err != nil {
		return 0, 0, err
	}
	pos, err := bitsIntArg(args, 1, name)
	if err != nil {
		return 0, 0, err
	}
	if pos < 0 || pos >= 64 {
		return 0, 0, fmt.Errorf("bad argument #2 to '%s' (bit position out of range)", name)
	}
	return n, uint(pos), nil
}
