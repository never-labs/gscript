package bits

import gobits "math/bits"

// And returns the bitwise AND of nums. With no arguments it returns -1.
func And(nums ...int64) int64 {
	if len(nums) == 0 {
		return -1
	}
	result := nums[0]
	for _, n := range nums[1:] {
		result &= n
	}
	return result
}

// Or returns the bitwise OR of nums. With no arguments it returns 0.
func Or(nums ...int64) int64 {
	var result int64
	for _, n := range nums {
		result |= n
	}
	return result
}

// Xor returns the bitwise XOR of nums. With no arguments it returns 0.
func Xor(nums ...int64) int64 {
	var result int64
	for _, n := range nums {
		result ^= n
	}
	return result
}

func Not(n int64) int64 {
	return ^n
}

func Shl(n int64, shift uint) int64 {
	if shift >= 64 {
		return 0
	}
	return int64(uint64(n) << shift)
}

func Shr(n int64, shift uint) int64 {
	if shift >= 64 {
		return 0
	}
	return int64(uint64(n) >> shift)
}

func Sar(n int64, shift uint) int64 {
	if shift >= 64 {
		if n < 0 {
			return -1
		}
		return 0
	}
	return n >> shift
}

func Rotl(n int64, shift int64) int64 {
	return int64(gobits.RotateLeft64(uint64(n), int(shift)))
}

func Rotr(n int64, shift int64) int64 {
	return int64(gobits.RotateLeft64(uint64(n), -int(shift)))
}

func Test(n int64, pos uint) bool {
	return (uint64(n) & (uint64(1) << pos)) != 0
}

func Set(n int64, pos uint) int64 {
	return int64(uint64(n) | (uint64(1) << pos))
}

func Clear(n int64, pos uint) int64 {
	return int64(uint64(n) &^ (uint64(1) << pos))
}

func Toggle(n int64, pos uint) int64 {
	return int64(uint64(n) ^ (uint64(1) << pos))
}

func Ones(n int64) int {
	return gobits.OnesCount64(uint64(n))
}

func LeadingZeros(n int64) int {
	return gobits.LeadingZeros64(uint64(n))
}

func TrailingZeros(n int64) int {
	return gobits.TrailingZeros64(uint64(n))
}
