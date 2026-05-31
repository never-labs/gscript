package runtime

import (
	"fmt"

	basebits "github.com/never-labs/gscript/internal/stdlib/bits"
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
		nums, err := bitsIntArgs(args, "bits.and")
		if err != nil {
			return nil, err
		}
		return []Value{IntValue(basebits.And(nums...))}, nil
	})

	set("or", func(args []Value) ([]Value, error) {
		nums, err := bitsIntArgs(args, "bits.or")
		if err != nil {
			return nil, err
		}
		return []Value{IntValue(basebits.Or(nums...))}, nil
	})

	set("xor", func(args []Value) ([]Value, error) {
		nums, err := bitsIntArgs(args, "bits.xor")
		if err != nil {
			return nil, err
		}
		return []Value{IntValue(basebits.Xor(nums...))}, nil
	})

	set("not", func(args []Value) ([]Value, error) {
		n, err := bitsIntArg(args, 0, "bits.not")
		if err != nil {
			return nil, err
		}
		return []Value{IntValue(basebits.Not(n))}, nil
	})

	set("shl", func(args []Value) ([]Value, error) {
		n, shift, err := bitsShiftArgs(args, "bits.shl")
		if err != nil {
			return nil, err
		}
		return []Value{IntValue(basebits.Shl(n, shift))}, nil
	})

	set("shr", func(args []Value) ([]Value, error) {
		n, shift, err := bitsShiftArgs(args, "bits.shr")
		if err != nil {
			return nil, err
		}
		return []Value{IntValue(basebits.Shr(n, shift))}, nil
	})

	set("sar", func(args []Value) ([]Value, error) {
		n, shift, err := bitsShiftArgs(args, "bits.sar")
		if err != nil {
			return nil, err
		}
		return []Value{IntValue(basebits.Sar(n, shift))}, nil
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
		return []Value{IntValue(basebits.Rotl(n, shift))}, nil
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
		return []Value{IntValue(basebits.Rotr(n, shift))}, nil
	})

	set("test", func(args []Value) ([]Value, error) {
		n, pos, err := bitsPositionArgs(args, "bits.test")
		if err != nil {
			return nil, err
		}
		return []Value{BoolValue(basebits.Test(n, pos))}, nil
	})

	set("set", func(args []Value) ([]Value, error) {
		n, pos, err := bitsPositionArgs(args, "bits.set")
		if err != nil {
			return nil, err
		}
		return []Value{IntValue(basebits.Set(n, pos))}, nil
	})

	set("clear", func(args []Value) ([]Value, error) {
		n, pos, err := bitsPositionArgs(args, "bits.clear")
		if err != nil {
			return nil, err
		}
		return []Value{IntValue(basebits.Clear(n, pos))}, nil
	})

	set("toggle", func(args []Value) ([]Value, error) {
		n, pos, err := bitsPositionArgs(args, "bits.toggle")
		if err != nil {
			return nil, err
		}
		return []Value{IntValue(basebits.Toggle(n, pos))}, nil
	})

	set("ones", func(args []Value) ([]Value, error) {
		n, err := bitsIntArg(args, 0, "bits.ones")
		if err != nil {
			return nil, err
		}
		return []Value{IntValue(int64(basebits.Ones(n)))}, nil
	})

	set("leadingZeros", func(args []Value) ([]Value, error) {
		n, err := bitsIntArg(args, 0, "bits.leadingZeros")
		if err != nil {
			return nil, err
		}
		return []Value{IntValue(int64(basebits.LeadingZeros(n)))}, nil
	})

	set("trailingZeros", func(args []Value) ([]Value, error) {
		n, err := bitsIntArg(args, 0, "bits.trailingZeros")
		if err != nil {
			return nil, err
		}
		return []Value{IntValue(int64(basebits.TrailingZeros(n)))}, nil
	})

	return t
}

func bitsIntArgs(args []Value, name string) ([]int64, error) {
	nums := make([]int64, len(args))
	for i := range args {
		n, err := bitsIntArg(args, i, name)
		if err != nil {
			return nil, err
		}
		nums[i] = n
	}
	return nums, nil
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
