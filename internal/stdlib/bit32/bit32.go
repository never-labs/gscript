package bit32

import (
	"fmt"
	"math/bits"
)

func Mask(width uint) uint32 {
	if width >= 32 {
		return ^uint32(0)
	}
	return uint32((uint64(1) << width) - 1)
}

func And(a, b uint32) uint32 {
	return a & b
}

func Or(a, b uint32) uint32 {
	return a | b
}

func Xor(a, b uint32) uint32 {
	return a ^ b
}

func Bnot(n int64) int64 {
	return int64(^uint32(n))
}

func BtestResult(result uint32) bool {
	return result != 0
}

func Lshift(n, disp int64) int64 {
	u := uint32(n)
	if disp < 0 {
		return int64(u >> uint(-disp))
	}
	if disp >= 32 {
		return 0
	}
	return int64(u << uint(disp))
}

func Rshift(n, disp int64) int64 {
	u := uint32(n)
	if disp < 0 {
		return int64(u << uint(-disp))
	}
	if disp >= 32 {
		return 0
	}
	return int64(u >> uint(disp))
}

func Lrotate(n, disp int64) int64 {
	return int64(bits.RotateLeft32(uint32(n), int(disp&31)))
}

func Rrotate(n, disp int64) int64 {
	return int64(bits.RotateLeft32(uint32(n), -int(disp&31)))
}

func Arshift(n, disp int64) int64 {
	i := int32(uint32(n))
	if disp < 0 {
		return int64(uint32(i) << uint(-disp))
	}
	if disp >= 32 {
		if i < 0 {
			return int64(^uint32(0))
		}
		return 0
	}
	return int64(uint32(i >> uint(disp)))
}

func Test(n, pos int64) bool {
	return (uint32(n) & (1 << uint(pos))) != 0
}

func Set(n, pos int64) int64 {
	return int64(uint32(n) | (1 << uint(pos)))
}

func Clear(n, pos int64) int64 {
	return int64(uint32(n) &^ (1 << uint(pos)))
}

func Toggle(n, pos int64) int64 {
	return int64(uint32(n) ^ (1 << uint(pos)))
}

func Extract(n, field, width int64) (int64, error) {
	if field < 0 || field >= 32 || width <= 0 || width > 32-field {
		return 0, fmt.Errorf("bad field or width")
	}
	mask := Mask(uint(width))
	return int64((uint32(n) >> field) & mask), nil
}

func Replace(n, value, field, width int64) (int64, error) {
	if field < 0 || field >= 32 || width <= 0 || width > 32-field {
		return 0, fmt.Errorf("bad field or width")
	}
	mask := Mask(uint(width))
	result := (uint32(n) &^ (mask << field)) | ((uint32(value) & mask) << field)
	return int64(result), nil
}

func Countbits(n int64) int64 {
	return int64(bits.OnesCount32(uint32(n)))
}

func Highbit(n int64) int64 {
	u := uint32(n)
	if u == 0 {
		return -1
	}
	return int64(31 - bits.LeadingZeros32(u))
}
