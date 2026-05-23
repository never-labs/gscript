package runtime

import (
	"fmt"
	"math/bits"
)

// buildBit32Lib creates the "bit32" standard library table.
func buildBit32Lib() *Table {
	t := NewTable()

	set := func(name string, fn func([]Value) ([]Value, error)) {
		t.RawSet(StringValue(name), FunctionValue(&GoFunction{
			Name: "bit32." + name,
			Fn:   fn,
		}))
	}

	// bit32.band(...) → uint32
	set("band", func(args []Value) ([]Value, error) {
		v, err := bit32FoldValues(args, "bit32.band", uint32(0xFFFFFFFF), func(a, b uint32) uint32 { return a & b })
		if err != nil {
			return nil, err
		}
		return []Value{v}, nil
	})
	installBit32FoldFastPaths(t.RawGetString("band").GoFunction(), "bit32.band", uint32(0xFFFFFFFF), func(a, b uint32) uint32 { return a & b })

	// bit32.bor(...) → uint32
	set("bor", func(args []Value) ([]Value, error) {
		v, err := bit32FoldValues(args, "bit32.bor", 0, func(a, b uint32) uint32 { return a | b })
		if err != nil {
			return nil, err
		}
		return []Value{v}, nil
	})
	installBit32FoldFastPaths(t.RawGetString("bor").GoFunction(), "bit32.bor", 0, func(a, b uint32) uint32 { return a | b })

	// bit32.bxor(...) → uint32
	set("bxor", func(args []Value) ([]Value, error) {
		v, err := bit32FoldValues(args, "bit32.bxor", 0, func(a, b uint32) uint32 { return a ^ b })
		if err != nil {
			return nil, err
		}
		return []Value{v}, nil
	})
	installBit32FoldFastPaths(t.RawGetString("bxor").GoFunction(), "bit32.bxor", 0, func(a, b uint32) uint32 { return a ^ b })

	// bit32.bnot(n) → uint32
	set("bnot", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'bit32.bnot'")
		}
		v, err := bit32BnotValue(args[0])
		if err != nil {
			return nil, err
		}
		return []Value{v}, nil
	})
	if gf := t.RawGetString("bnot").GoFunction(); gf != nil {
		gf.FastArg1 = bit32BnotValue
		gf.Fast1 = func(args []Value) (Value, error) {
			if len(args) < 1 {
				return NilValue(), fmt.Errorf("bad argument #1 to 'bit32.bnot'")
			}
			return bit32BnotValue(args[0])
		}
	}

	// bit32.btest(...) -> bool
	set("btest", func(args []Value) ([]Value, error) {
		if len(args) == 0 {
			return []Value{BoolValue(true)}, nil
		}
		first, err := bit32IntArg(args, 0, "bit32.btest")
		if err != nil {
			return nil, err
		}
		result := uint32(first)
		for i := 1; i < len(args); i++ {
			n, err := bit32IntArg(args, i, "bit32.btest")
			if err != nil {
				return nil, err
			}
			result &= uint32(n)
		}
		return []Value{BoolValue(result != 0)}, nil
	})

	// bit32.lshift(n, disp) → uint32
	set("lshift", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'bit32.lshift'")
		}
		v, err := bit32LshiftValue(args[0], args[1])
		if err != nil {
			return nil, err
		}
		return []Value{v}, nil
	})
	installBit32BinaryFastPath(t.RawGetString("lshift").GoFunction(), bit32LshiftValue, "bit32.lshift")

	// bit32.rshift(n, disp) → uint32
	set("rshift", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'bit32.rshift'")
		}
		v, err := bit32RshiftValue(args[0], args[1])
		if err != nil {
			return nil, err
		}
		return []Value{v}, nil
	})
	installBit32BinaryFastPath(t.RawGetString("rshift").GoFunction(), bit32RshiftValue, "bit32.rshift")

	// bit32.lrotate(n, disp) -> uint32
	set("lrotate", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'bit32.lrotate'")
		}
		return []Value{bit32LrotateValue(args[0], args[1])}, nil
	})
	installBit32BinaryFastPath(t.RawGetString("lrotate").GoFunction(), bit32LrotateValueNoError, "bit32.lrotate")

	// bit32.rrotate(n, disp) -> uint32
	set("rrotate", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'bit32.rrotate'")
		}
		return []Value{bit32RrotateValue(args[0], args[1])}, nil
	})
	installBit32BinaryFastPath(t.RawGetString("rrotate").GoFunction(), bit32RrotateValueNoError, "bit32.rrotate")

	// bit32.arshift(n, disp) → int32 (arithmetic right shift, sign-extending)
	set("arshift", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'bit32.arshift'")
		}
		v, err := bit32ArshiftValue(args[0], args[1])
		if err != nil {
			return nil, err
		}
		return []Value{v}, nil
	})
	installBit32BinaryFastPath(t.RawGetString("arshift").GoFunction(), bit32ArshiftValue, "bit32.arshift")

	// bit32.test(n, pos) → bool
	set("test", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'bit32.test'")
		}
		n := uint32(toInt(args[0]))
		pos := uint(toInt(args[1]))
		return []Value{BoolValue((n & (1 << pos)) != 0)}, nil
	})

	// bit32.set(n, pos) → uint32
	set("set", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'bit32.set'")
		}
		n := uint32(toInt(args[0]))
		pos := uint(toInt(args[1]))
		return []Value{IntValue(int64(n | (1 << pos)))}, nil
	})

	// bit32.clear(n, pos) → uint32
	set("clear", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'bit32.clear'")
		}
		n := uint32(toInt(args[0]))
		pos := uint(toInt(args[1]))
		return []Value{IntValue(int64(n &^ (1 << pos)))}, nil
	})

	// bit32.toggle(n, pos) → uint32
	set("toggle", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'bit32.toggle'")
		}
		n := uint32(toInt(args[0]))
		pos := uint(toInt(args[1]))
		return []Value{IntValue(int64(n ^ (1 << pos)))}, nil
	})

	// bit32.extract(n, field [, width]) → uint32
	set("extract", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'bit32.extract'")
		}
		v, err := bit32ExtractArgs(args)
		if err != nil {
			return nil, err
		}
		return []Value{v}, nil
	})
	if gf := t.RawGetString("extract").GoFunction(); gf != nil {
		gf.FastArg2 = func(a, b Value) (Value, error) { return bit32ExtractValue(a, b, IntValue(1)) }
		gf.FastArg3 = bit32ExtractValue
		gf.Fast1 = bit32ExtractArgs
	}

	// bit32.replace(n, v, field [, width]) → uint32
	set("replace", func(args []Value) ([]Value, error) {
		if len(args) < 3 {
			return nil, fmt.Errorf("bad argument to 'bit32.replace'")
		}
		v, err := bit32ReplaceArgs(args)
		if err != nil {
			return nil, err
		}
		return []Value{v}, nil
	})
	if gf := t.RawGetString("replace").GoFunction(); gf != nil {
		gf.FastArg3 = func(a, b, c Value) (Value, error) { return bit32ReplaceValue(a, b, c, IntValue(1)) }
		gf.FastArg4 = bit32ReplaceValue
		gf.Fast1 = bit32ReplaceArgs
	}

	// bit32.countbits(n) → int (popcount)
	set("countbits", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'bit32.countbits'")
		}
		n := uint32(toInt(args[0]))
		return []Value{IntValue(int64(bits.OnesCount32(n)))}, nil
	})

	// bit32.highbit(n) → int (position of highest set bit, 0-based; -1 if n=0)
	set("highbit", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'bit32.highbit'")
		}
		n := uint32(toInt(args[0]))
		if n == 0 {
			return []Value{IntValue(-1)}, nil
		}
		return []Value{IntValue(int64(31 - bits.LeadingZeros32(n)))}, nil
	})

	// bit32.toHex(n [, digits]) → string
	set("toHex", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'bit32.toHex'")
		}
		n := uint32(toInt(args[0]))
		if len(args) >= 2 {
			digits := int(toInt(args[1]))
			return []Value{StringValue(fmt.Sprintf("0x%0*X", digits, n))}, nil
		}
		return []Value{StringValue(fmt.Sprintf("0x%X", n))}, nil
	})

	return t
}

func bit32Mask(width uint) uint32 {
	if width >= 32 {
		return ^uint32(0)
	}
	return uint32((uint64(1) << width) - 1)
}

func installBit32FoldFastPaths(gf *GoFunction, name string, zero uint32, op func(uint32, uint32) uint32) {
	if gf == nil {
		return
	}
	gf.FastArg1 = func(a Value) (Value, error) {
		n, err := bit32ValueArg(a, 0, name)
		if err != nil {
			return NilValue(), err
		}
		return IntValue(int64(uint32(n))), nil
	}
	gf.FastArg2 = func(a, b Value) (Value, error) {
		return bit32FoldFixed(name, zero, op, a, b)
	}
	gf.FastArg3 = func(a, b, c Value) (Value, error) {
		return bit32FoldFixed(name, zero, op, a, b, c)
	}
	gf.FastArg4 = func(a, b, c, d Value) (Value, error) {
		return bit32FoldFixed(name, zero, op, a, b, c, d)
	}
	gf.Fast1 = func(args []Value) (Value, error) {
		return bit32FoldValues(args, name, zero, op)
	}
}

func installBit32BinaryFastPath(gf *GoFunction, fast func(Value, Value) (Value, error), name string) {
	if gf == nil {
		return
	}
	gf.FastArg2 = fast
	gf.Fast1 = func(args []Value) (Value, error) {
		if len(args) < 2 {
			return NilValue(), fmt.Errorf("bad argument to '%s'", name)
		}
		return fast(args[0], args[1])
	}
}

func bit32FoldFixed(name string, zero uint32, op func(uint32, uint32) uint32, args ...Value) (Value, error) {
	return bit32FoldValues(args, name, zero, op)
}

func bit32FoldValues(args []Value, name string, zero uint32, op func(uint32, uint32) uint32) (Value, error) {
	if len(args) == 0 {
		return IntValue(int64(zero)), nil
	}
	first, err := bit32ValueArg(args[0], 0, name)
	if err != nil {
		return NilValue(), err
	}
	result := uint32(first)
	for i := 1; i < len(args); i++ {
		n, err := bit32ValueArg(args[i], i, name)
		if err != nil {
			return NilValue(), err
		}
		result = op(result, uint32(n))
	}
	return IntValue(int64(result)), nil
}

func bit32BnotValue(v Value) (Value, error) {
	n, err := bit32ValueArg(v, 0, "bit32.bnot")
	if err != nil {
		return NilValue(), err
	}
	return IntValue(int64(^uint32(n))), nil
}

func bit32LshiftValue(nv, dispv Value) (Value, error) {
	n64, err := bit32ValueArg(nv, 0, "bit32.lshift")
	if err != nil {
		return NilValue(), err
	}
	disp, err := bit32ValueArg(dispv, 1, "bit32.lshift")
	if err != nil {
		return NilValue(), err
	}
	n := uint32(n64)
	if disp < 0 {
		return IntValue(int64(n >> uint(-disp))), nil
	}
	if disp >= 32 {
		return IntValue(0), nil
	}
	return IntValue(int64(n << uint(disp))), nil
}

func bit32RshiftValue(nv, dispv Value) (Value, error) {
	n64, err := bit32ValueArg(nv, 0, "bit32.rshift")
	if err != nil {
		return NilValue(), err
	}
	disp, err := bit32ValueArg(dispv, 1, "bit32.rshift")
	if err != nil {
		return NilValue(), err
	}
	n := uint32(n64)
	if disp < 0 {
		return IntValue(int64(n << uint(-disp))), nil
	}
	if disp >= 32 {
		return IntValue(0), nil
	}
	return IntValue(int64(n >> uint(disp))), nil
}

func bit32LrotateValue(a, b Value) Value {
	n := uint32(toInt(a))
	disp := int(toInt(b) & 31)
	return IntValue(int64(bits.RotateLeft32(n, disp)))
}

func bit32LrotateValueNoError(a, b Value) (Value, error) {
	return bit32LrotateValue(a, b), nil
}

func bit32RrotateValue(a, b Value) Value {
	n := uint32(toInt(a))
	disp := int(toInt(b) & 31)
	return IntValue(int64(bits.RotateLeft32(n, -disp)))
}

func bit32RrotateValueNoError(a, b Value) (Value, error) {
	return bit32RrotateValue(a, b), nil
}

func bit32ArshiftValue(nv, dispv Value) (Value, error) {
	n64, err := bit32ValueArg(nv, 0, "bit32.arshift")
	if err != nil {
		return NilValue(), err
	}
	disp, err := bit32ValueArg(dispv, 1, "bit32.arshift")
	if err != nil {
		return NilValue(), err
	}
	n := int32(uint32(n64))
	if disp < 0 {
		return IntValue(int64(uint32(n) << uint(-disp))), nil
	}
	if disp >= 32 {
		if n < 0 {
			return IntValue(int64(uint32(^uint32(0)))), nil
		}
		return IntValue(0), nil
	}
	return IntValue(int64(uint32(n >> uint(disp)))), nil
}

func bit32ExtractArgs(args []Value) (Value, error) {
	if len(args) < 2 {
		return NilValue(), fmt.Errorf("bad argument to 'bit32.extract'")
	}
	width := IntValue(1)
	if len(args) >= 3 {
		width = args[2]
	}
	return bit32ExtractValue(args[0], args[1], width)
}

func bit32ExtractValue(nv, fieldv, widthv Value) (Value, error) {
	n := uint32(toInt(nv))
	field := toInt(fieldv)
	width := toInt(widthv)
	if field < 0 || field >= 32 || width <= 0 || width > 32-field {
		return NilValue(), fmt.Errorf("bad field or width")
	}
	mask := bit32Mask(uint(width))
	return IntValue(int64((n >> field) & mask)), nil
}

func bit32ReplaceArgs(args []Value) (Value, error) {
	if len(args) < 3 {
		return NilValue(), fmt.Errorf("bad argument to 'bit32.replace'")
	}
	width := IntValue(1)
	if len(args) >= 4 {
		width = args[3]
	}
	return bit32ReplaceValue(args[0], args[1], args[2], width)
}

func bit32ReplaceValue(nv, valuev, fieldv, widthv Value) (Value, error) {
	n := uint32(toInt(nv))
	v := uint32(toInt(valuev))
	field := toInt(fieldv)
	width := toInt(widthv)
	if field < 0 || field >= 32 || width <= 0 || width > 32-field {
		return NilValue(), fmt.Errorf("bad field or width")
	}
	mask := bit32Mask(uint(width))
	n = (n &^ (mask << field)) | ((v & mask) << field)
	return IntValue(int64(n)), nil
}

func bit32ValueArg(v Value, index int, name string) (int64, error) {
	n, ok := v.ToNumber()
	if !ok {
		return 0, fmt.Errorf("bad argument #%d to '%s' (number expected)", index+1, name)
	}
	return toInt(n), nil
}

func bit32IntArg(args []Value, index int, name string) (int64, error) {
	if index >= len(args) {
		return 0, fmt.Errorf("bad argument #%d to '%s'", index+1, name)
	}
	return bit32ValueArg(args[index], index, name)
}
