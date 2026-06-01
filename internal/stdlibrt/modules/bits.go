package modules

import (
	"fmt"
	"github.com/never-labs/leia/internal/runtime"

	basebits "github.com/never-labs/leia/internal/stdlib/bits"
)

// buildBitsLib creates the Leia-native "bits" standard library.
//
// The API intentionally uses Go-style names and 64-bit integer operations
// instead of mirroring Lua bitwise operator syntax. Lua compatibility can be
// layered over this library where needed.
func BuildBits() *runtime.Table {
	t := runtime.NewTable()

	set := func(name string, fn func([]runtime.Value) ([]runtime.Value, error)) {
		t.RawSet(runtime.StringValue(name), runtime.FunctionValue(&runtime.GoFunction{
			Name: "bits." + name,
			Fn:   fn,
		}))
	}

	set("and", func(args []runtime.Value) ([]runtime.Value, error) {
		nums, err := bitsIntArgs(args, "bits.and")
		if err != nil {
			return nil, err
		}
		return []runtime.Value{runtime.IntValue(basebits.And(nums...))}, nil
	})

	set("or", func(args []runtime.Value) ([]runtime.Value, error) {
		nums, err := bitsIntArgs(args, "bits.or")
		if err != nil {
			return nil, err
		}
		return []runtime.Value{runtime.IntValue(basebits.Or(nums...))}, nil
	})

	set("xor", func(args []runtime.Value) ([]runtime.Value, error) {
		nums, err := bitsIntArgs(args, "bits.xor")
		if err != nil {
			return nil, err
		}
		return []runtime.Value{runtime.IntValue(basebits.Xor(nums...))}, nil
	})

	set("not", func(args []runtime.Value) ([]runtime.Value, error) {
		n, err := bitsIntArg(args, 0, "bits.not")
		if err != nil {
			return nil, err
		}
		return []runtime.Value{runtime.IntValue(basebits.Not(n))}, nil
	})

	set("shl", func(args []runtime.Value) ([]runtime.Value, error) {
		n, shift, err := bitsShiftArgs(args, "bits.shl")
		if err != nil {
			return nil, err
		}
		return []runtime.Value{runtime.IntValue(basebits.Shl(n, shift))}, nil
	})

	set("shr", func(args []runtime.Value) ([]runtime.Value, error) {
		n, shift, err := bitsShiftArgs(args, "bits.shr")
		if err != nil {
			return nil, err
		}
		return []runtime.Value{runtime.IntValue(basebits.Shr(n, shift))}, nil
	})

	set("sar", func(args []runtime.Value) ([]runtime.Value, error) {
		n, shift, err := bitsShiftArgs(args, "bits.sar")
		if err != nil {
			return nil, err
		}
		return []runtime.Value{runtime.IntValue(basebits.Sar(n, shift))}, nil
	})

	set("rotl", func(args []runtime.Value) ([]runtime.Value, error) {
		n, err := bitsIntArg(args, 0, "bits.rotl")
		if err != nil {
			return nil, err
		}
		shift, err := bitsIntArg(args, 1, "bits.rotl")
		if err != nil {
			return nil, err
		}
		return []runtime.Value{runtime.IntValue(basebits.Rotl(n, shift))}, nil
	})

	set("rotr", func(args []runtime.Value) ([]runtime.Value, error) {
		n, err := bitsIntArg(args, 0, "bits.rotr")
		if err != nil {
			return nil, err
		}
		shift, err := bitsIntArg(args, 1, "bits.rotr")
		if err != nil {
			return nil, err
		}
		return []runtime.Value{runtime.IntValue(basebits.Rotr(n, shift))}, nil
	})

	set("test", func(args []runtime.Value) ([]runtime.Value, error) {
		n, pos, err := bitsPositionArgs(args, "bits.test")
		if err != nil {
			return nil, err
		}
		return []runtime.Value{runtime.BoolValue(basebits.Test(n, pos))}, nil
	})

	set("set", func(args []runtime.Value) ([]runtime.Value, error) {
		n, pos, err := bitsPositionArgs(args, "bits.set")
		if err != nil {
			return nil, err
		}
		return []runtime.Value{runtime.IntValue(basebits.Set(n, pos))}, nil
	})

	set("clear", func(args []runtime.Value) ([]runtime.Value, error) {
		n, pos, err := bitsPositionArgs(args, "bits.clear")
		if err != nil {
			return nil, err
		}
		return []runtime.Value{runtime.IntValue(basebits.Clear(n, pos))}, nil
	})

	set("toggle", func(args []runtime.Value) ([]runtime.Value, error) {
		n, pos, err := bitsPositionArgs(args, "bits.toggle")
		if err != nil {
			return nil, err
		}
		return []runtime.Value{runtime.IntValue(basebits.Toggle(n, pos))}, nil
	})

	set("ones", func(args []runtime.Value) ([]runtime.Value, error) {
		n, err := bitsIntArg(args, 0, "bits.ones")
		if err != nil {
			return nil, err
		}
		return []runtime.Value{runtime.IntValue(int64(basebits.Ones(n)))}, nil
	})

	set("leadingZeros", func(args []runtime.Value) ([]runtime.Value, error) {
		n, err := bitsIntArg(args, 0, "bits.leadingZeros")
		if err != nil {
			return nil, err
		}
		return []runtime.Value{runtime.IntValue(int64(basebits.LeadingZeros(n)))}, nil
	})

	set("trailingZeros", func(args []runtime.Value) ([]runtime.Value, error) {
		n, err := bitsIntArg(args, 0, "bits.trailingZeros")
		if err != nil {
			return nil, err
		}
		return []runtime.Value{runtime.IntValue(int64(basebits.TrailingZeros(n)))}, nil
	})

	return t
}

func bitsIntArgs(args []runtime.Value, name string) ([]int64, error) {
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

func bitsIntArg(args []runtime.Value, index int, name string) (int64, error) {
	if index >= len(args) {
		return 0, fmt.Errorf("bad argument #%d to '%s' (integer expected)", index+1, name)
	}
	n, ok := args[index].ToNumber()
	if !ok {
		return 0, fmt.Errorf("bad argument #%d to '%s' (integer expected)", index+1, name)
	}
	return bitsToInt(n), nil
}

func bitsToInt(v runtime.Value) int64 {
	switch v.Type() {
	case runtime.TypeInt:
		return v.Int()
	case runtime.TypeFloat:
		return int64(v.Float())
	case runtime.TypeString:
		n, ok := v.ToNumber()
		if ok {
			return bitsToInt(n)
		}
	}
	return 0
}

func bitsShiftArgs(args []runtime.Value, name string) (int64, uint, error) {
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

func bitsPositionArgs(args []runtime.Value, name string) (int64, uint, error) {
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
