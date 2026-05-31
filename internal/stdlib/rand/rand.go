package rand

import (
	"fmt"
	"math"
)

const Int48Mask int64 = 0x7FFFFFFFFFFF
const seedMixMultiplier uint64 = 0x9e3779b97f4a7c15

func MaskInt48(n int64) int64 {
	return n & Int48Mask
}

func MixSeedPair(seed1, seed2 int64) int64 {
	return seed1 ^ int64(uint64(seed2)*seedMixMultiplier)
}

func InclusiveSpan(min, max int64) (int64, error) {
	if min > max {
		return 0, fmt.Errorf("min > max")
	}
	return max - min + 1, nil
}

func IntBelow(nextInt63n func(int64) int64, max int64) (int64, error) {
	if max <= 0 {
		return 0, fmt.Errorf("positive number expected")
	}
	return nextInt63n(max), nil
}

func IntRange(nextInt63n func(int64) int64, min, max int64) (int64, error) {
	if min > max {
		return 0, fmt.Errorf("min > max")
	}
	return min + nextInt63n(max-min+1), nil
}

func Normal(unitNormal func() float64, mean, stddev float64) (float64, error) {
	if stddev < 0 {
		return 0, fmt.Errorf("non-negative stddev expected")
	}
	return unitNormal()*stddev + mean, nil
}

func Exponential(unitExp func() float64, rate float64) (float64, error) {
	if rate <= 0 {
		return 0, fmt.Errorf("positive rate expected")
	}
	return unitExp() / rate, nil
}

func Bytes(n int, nextByte func() byte) ([]byte, error) {
	if n < 0 {
		return nil, fmt.Errorf("non-negative number expected")
	}
	buf := make([]byte, n)
	for i := range buf {
		buf[i] = nextByte()
	}
	return buf, nil
}

func ClampSampleCount(n, length int) (int, error) {
	if n < 0 {
		return 0, fmt.Errorf("negative count")
	}
	if n > length {
		return length, nil
	}
	return n, nil
}

func PrepareUUIDV4(bytes *[16]byte) {
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80
}

func FormatUUID(bytes [16]byte) string {
	return fmt.Sprintf("%02x%02x%02x%02x-%02x%02x-%02x%02x-%02x%02x-%02x%02x%02x%02x%02x%02x",
		bytes[0], bytes[1], bytes[2], bytes[3],
		bytes[4], bytes[5],
		bytes[6], bytes[7],
		bytes[8], bytes[9],
		bytes[10], bytes[11], bytes[12], bytes[13], bytes[14], bytes[15])
}

func UUIDV4(nextByte func() byte) string {
	var uuid [16]byte
	for i := range uuid {
		uuid[i] = nextByte()
	}
	PrepareUUIDV4(&uuid)
	return FormatUUID(uuid)
}

func ValidWeightTotal(total float64) bool {
	return total != 0 && !math.IsInf(total, 0) && !math.IsNaN(total)
}

func ValidateWeights(weights []float64) (float64, error) {
	total := 0.0
	for i, w := range weights {
		if math.IsNaN(w) || w < 0 {
			return 0, fmt.Errorf("negative weight at index %d", i+1)
		}
		total += w
	}
	if !ValidWeightTotal(total) {
		return 0, fmt.Errorf("invalid total weight")
	}
	return total, nil
}

func WeightedIndex(weights []float64, point float64) int {
	cumulative := 0.0
	for i, w := range weights {
		cumulative += w
		if point < cumulative {
			return i
		}
	}
	return len(weights) - 1
}
