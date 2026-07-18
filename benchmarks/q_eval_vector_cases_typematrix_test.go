//go:build leia_q

package benchmarks

// Per-type x null matrix and compound-predicate-depth q.eval cases.
//
// This file closes the "operation x element-type x null-mix" and "compound
// qSQL-style predicate depth" gaps from benchmarks/q_benchmark_coverage.md.
// Semantics encoded here were verified against the live evaluator:
//   - sum (+/) and avg skip nulls for every numeric element kind
//   - elementwise arithmetic propagates nulls (0Nh+1 -> 0N, typed)
//   - ordered compares follow canonical q null semantics: null sorts before
//     every value (0N<x is true for non-null x; 0N>x, x within a b with null
//     x are false); <> treats null-vs-value as true; =0N matches nulls; null
//     verb flags them
//   - all integer arithmetic promotes to long; any float operand promotes the
//     result to float64 (real+real is float64, not real)
//   - boolean vectors are not numeric operands (+/mask and mask+1 error), so
//     boolean participation goes through b$ casts and where/count
// Float pipelines stick to power-of-two scale factors (0.25/0.5 steps) so the
// q float64 results and the Go baselines are bit-exact under any summation
// order and the int64 checksum truncation is deterministic.

import (
	"fmt"
	"math"
	"testing"
)

// --- typed data builders -------------------------------------------------

//go:noinline
func qTypeMatrixShortPatternData(rows int, vals [4]int16, nulls [4]bool) ([]int16, []bool) {
	v := make([]int16, rows)
	n := make([]bool, rows)
	for i := 0; i < rows; i++ {
		v[i] = vals[i%4]
		n[i] = nulls[i%4]
	}
	return v, n
}

//go:noinline
func qTypeMatrixIntPatternData(rows int, vals [4]int32, nulls [4]bool) ([]int32, []bool) {
	v := make([]int32, rows)
	n := make([]bool, rows)
	for i := 0; i < rows; i++ {
		v[i] = vals[i%4]
		n[i] = nulls[i%4]
	}
	return v, n
}

//go:noinline
func qTypeMatrixLongPatternData(rows int, vals [4]int64, nulls [4]bool) ([]int64, []bool) {
	v := make([]int64, rows)
	n := make([]bool, rows)
	for i := 0; i < rows; i++ {
		v[i] = vals[i%4]
		n[i] = nulls[i%4]
	}
	return v, n
}

//go:noinline
func qTypeMatrixRealPatternData(rows int, vals [4]float32, nulls [4]bool) ([]float32, []bool) {
	v := make([]float32, rows)
	n := make([]bool, rows)
	for i := 0; i < rows; i++ {
		v[i] = vals[i%4]
		n[i] = nulls[i%4]
	}
	return v, n
}

//go:noinline
func qTypeMatrixFloatPatternData(rows int, vals [4]float64, nulls [4]bool) ([]float64, []bool) {
	v := make([]float64, rows)
	n := make([]bool, rows)
	for i := 0; i < rows; i++ {
		v[i] = vals[i%4]
		n[i] = nulls[i%4]
	}
	return v, n
}

//go:noinline
func qTypeMatrixShortModData(rows, mod int) []int16 {
	v := make([]int16, rows)
	for i := 0; i < rows; i++ {
		v[i] = int16(i % mod)
	}
	return v
}

//go:noinline
func qTypeMatrixIntModData(rows, mod int) []int32 {
	v := make([]int32, rows)
	for i := 0; i < rows; i++ {
		v[i] = int32(i % mod)
	}
	return v
}

//go:noinline
func qTypeMatrixRealModData(rows, mod int) []float32 {
	v := make([]float32, rows)
	for i := 0; i < rows; i++ {
		v[i] = float32(i % mod)
	}
	return v
}

// --- typed compute kernels ------------------------------------------------

//go:noinline
func qTypeMatrixShortAffineNullSum(v []int16, n []bool, mul, add int64) (int64, int64) {
	var sum, nullCount int64
	for i := range v {
		if n[i] {
			nullCount++
			continue
		}
		sum += int64(v[i])*mul + add
	}
	return sum, nullCount
}

//go:noinline
func qTypeMatrixIntAffineNullSum(v []int32, n []bool, mul, add int64) (int64, int64) {
	var sum, nullCount int64
	for i := range v {
		if n[i] {
			nullCount++
			continue
		}
		sum += int64(v[i])*mul + add
	}
	return sum, nullCount
}

//go:noinline
func qTypeMatrixRealAffineNullSum(v []float32, n []bool, mul, add float64) (float64, int64) {
	var sum float64
	var nullCount int64
	for i := range v {
		if n[i] {
			nullCount++
			continue
		}
		sum += float64(v[i])*mul + add
	}
	return sum, nullCount
}

//go:noinline
func qTypeMatrixFloatAffineNullSum(v []float64, n []bool, mul, add float64) (float64, int64) {
	var sum float64
	var nullCount int64
	for i := range v {
		if n[i] {
			nullCount++
			continue
		}
		sum += v[i]*mul + add
	}
	return sum, nullCount
}

//go:noinline
func qTypeMatrixLongVectorAddNullSum(rows int, pv []int64, pn []bool) int64 {
	var sum int64
	for i := 0; i < rows; i++ {
		if pn[i] {
			continue
		}
		sum += int64(i) + pv[i]
	}
	return sum
}

//go:noinline
func qTypeMatrixShortVectorMulNullSum(xv []int16, xn []bool, wv []int16, wn []bool) (int64, int64) {
	var sum, nullCount int64
	for i := range xv {
		if xn[i] || wn[i] {
			nullCount++
			continue
		}
		sum += int64(xv[i]) * int64(wv[i])
	}
	return sum, nullCount
}

//go:noinline
func qTypeMatrixLongDivModNullSum(v []int64, n []bool, div int64, useDiv bool) (int64, int64) {
	var sum, nullCount int64
	for i := range v {
		if n[i] {
			nullCount++
			continue
		}
		if useDiv {
			sum += v[i] / div
		} else {
			sum += v[i] % div
		}
	}
	return sum, nullCount
}

//go:noinline
func qTypeMatrixShortGatherCompare(v []int16, n []bool, threshold int16, greater bool) (int64, int64) {
	var sum, count int64
	for i := range v {
		hit := false
		switch {
		case n[i]:
			// Canonical q: null sorts before every value, so null<t hits
			// and null>t misses for a non-null threshold.
			hit = !greater
		case greater:
			hit = v[i] > threshold
		default:
			hit = v[i] < threshold
		}
		if hit {
			sum += int64(i)
			count++
		}
	}
	return sum, count
}

//go:noinline
func qTypeMatrixIntGatherCompare(v []int32, n []bool, threshold int32, greater bool) (int64, int64) {
	var sum, count int64
	for i := range v {
		hit := false
		switch {
		case n[i]:
			// Canonical q: null sorts before every value, so null<t hits
			// and null>t misses for a non-null threshold.
			hit = !greater
		case greater:
			hit = v[i] > threshold
		default:
			hit = v[i] < threshold
		}
		if hit {
			sum += int64(i)
			count++
		}
	}
	return sum, count
}

//go:noinline
func qTypeMatrixRealGatherGreater(v []float32, n []bool, threshold float64) (int64, int64) {
	var sum, nullCount int64
	for i := range v {
		if n[i] {
			nullCount++
			continue
		}
		if float64(v[i]) > threshold {
			sum += int64(i)
		}
	}
	return sum, nullCount
}

//go:noinline
func qTypeMatrixLongWithinGather(v []int64, n []bool, lo, hi int64) (int64, int64) {
	var sum, count int64
	for i := range v {
		if n[i] {
			continue
		}
		if v[i] >= lo && v[i] <= hi {
			sum += int64(i)
			count++
		}
	}
	return sum, count
}

//go:noinline
func qTypeMatrixLongNotEqualNullCount(v []int64, n []bool, exclude int64) int64 {
	var c1, c2 int64
	for i := range v {
		if n[i] || v[i] != exclude {
			c1++
		}
		if n[i] {
			c2++
		}
	}
	return c1 + c2
}

// --- case list -------------------------------------------------------------

func qEvalVectorTypeNullMatrixCases() []qEvalVectorCase {
	cases := make([]qEvalVectorCase, 0, 48)

	// A. Dyadic arithmetic: op x element type x {dense, null-mixed}.
	cases = append(cases,
		qEvalVectorCase{
			name:   "TypeMatrixShortDenseAddScalarSum",
			tags:   []string{"cast", "promotion", "numeric-vector", "sum"},
			matrix: []string{"cast:typed-numeric:scalar-vector", "numeric-arithmetic:int-vector:hot"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:`short$(til %d) mod 1000;y:x+7;+/y", rows)
			},
			goFn: func(rows int) int64 {
				v := qTypeMatrixShortModData(rows, 1000)
				sum, _ := qTypeMatrixShortAffineNullSum(v, make([]bool, rows), 1, 7)
				return sum
			},
		},
		qEvalVectorCase{
			name:   "TypeMatrixShortNullAddScalarSum",
			tags:   []string{"typed-suffix", "typed-null", "null-verb", "where", "sum"},
			matrix: []string{"numeric-arithmetic:typed-null:hot"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:%d#1h 0Nh 3h 9h;y:x+5;(+/y)+count where null y", rows)
			},
			goFn: func(rows int) int64 {
				v, n := qTypeMatrixShortPatternData(rows, [4]int16{1, 0, 3, 9}, [4]bool{false, true, false, false})
				sum, nullCount := qTypeMatrixShortAffineNullSum(v, n, 1, 5)
				return sum + nullCount
			},
		},
		qEvalVectorCase{
			name:   "TypeMatrixIntNullMulScalarSum",
			tags:   []string{"typed-suffix", "typed-null", "null-verb", "where", "sum"},
			matrix: []string{"numeric-arithmetic:typed-null:hot"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:%d#2i 0Ni 4i 7i;y:x*3;(+/y)+count where null y", rows)
			},
			goFn: func(rows int) int64 {
				v, n := qTypeMatrixIntPatternData(rows, [4]int32{2, 0, 4, 7}, [4]bool{false, true, false, false})
				sum, nullCount := qTypeMatrixIntAffineNullSum(v, n, 3, 0)
				return sum + nullCount
			},
		},
		qEvalVectorCase{
			name:   "TypeMatrixIntDenseSubScalarSum",
			tags:   []string{"cast", "promotion", "numeric-vector", "sum"},
			matrix: []string{"cast:typed-numeric:scalar-vector", "numeric-arithmetic:int-vector:hot"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:`int$(til %d) mod 512;y:x-9;+/y", rows)
			},
			goFn: func(rows int) int64 {
				v := qTypeMatrixIntModData(rows, 512)
				sum, _ := qTypeMatrixIntAffineNullSum(v, make([]bool, rows), 1, -9)
				return sum
			},
		},
		qEvalVectorCase{
			name:   "TypeMatrixRealDenseMulQuarterSum",
			tags:   []string{"cast", "promotion", "numeric-vector", "sum"},
			matrix: []string{"cast:typed-numeric:scalar-vector", "numeric-arithmetic:float-vector:hot"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:\"e\"$(til %d) mod 64;y:x*0.25;+/y", rows)
			},
			goFn: func(rows int) int64 {
				v := qTypeMatrixRealModData(rows, 64)
				sum, _ := qTypeMatrixRealAffineNullSum(v, make([]bool, rows), 0.25, 0)
				return int64(sum)
			},
		},
		qEvalVectorCase{
			name:   "TypeMatrixRealNullAddScalarSum",
			tags:   []string{"typed-suffix", "typed-null", "null-verb", "where", "sum"},
			matrix: []string{"numeric-arithmetic:typed-null:hot", "numeric-arithmetic:float-vector:hot"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:%d#1.5e 0Ne 2.5e 6e;y:x+2;(+/y)+count where null x", rows)
			},
			goFn: func(rows int) int64 {
				v, n := qTypeMatrixRealPatternData(rows, [4]float32{1.5, 0, 2.5, 6}, [4]bool{false, true, false, false})
				sum, nullCount := qTypeMatrixRealAffineNullSum(v, n, 1, 2)
				return int64(sum + float64(nullCount))
			},
		},
		qEvalVectorCase{
			name:   "TypeMatrixFloatNullDivideHalfSum",
			tags:   []string{"typed-null", "numeric-vector", "sum"},
			matrix: []string{"numeric-arithmetic:typed-null:hot", "numeric-arithmetic:float-vector:hot"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:%d#1.5 0Nf 2.5 6.5;y:x%%0.5;+/y", rows)
			},
			goFn: func(rows int) int64 {
				v, n := qTypeMatrixFloatPatternData(rows, [4]float64{1.5, 0, 2.5, 6.5}, [4]bool{false, true, false, false})
				sum, _ := qTypeMatrixFloatAffineNullSum(v, n, 2, 0)
				return int64(sum)
			},
		},
		qEvalVectorCase{
			name:   "TypeMatrixLongNullVectorAddVectorSum",
			tags:   []string{"typed-null", "numeric-vector", "sum"},
			matrix: []string{"numeric-arithmetic:typed-null:hot"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;n:%d#1 0N 3 4;y:x+n;+/y", rows, rows)
			},
			goFn: func(rows int) int64 {
				pv, pn := qTypeMatrixLongPatternData(rows, [4]int64{1, 0, 3, 4}, [4]bool{false, true, false, false})
				return qTypeMatrixLongVectorAddNullSum(rows, pv, pn)
			},
		},
		qEvalVectorCase{
			name:   "TypeMatrixShortNullVectorMulVectorSum",
			tags:   []string{"typed-suffix", "typed-null", "null-verb", "where", "sum"},
			matrix: []string{"numeric-arithmetic:typed-null:hot"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:%d#1h 0Nh 3h 4h;w:%d#2h 3h 0Nh 1h;y:x*w;(+/y)+count where null y", rows, rows)
			},
			goFn: func(rows int) int64 {
				xv, xn := qTypeMatrixShortPatternData(rows, [4]int16{1, 0, 3, 4}, [4]bool{false, true, false, false})
				wv, wn := qTypeMatrixShortPatternData(rows, [4]int16{2, 3, 0, 1}, [4]bool{false, false, true, false})
				sum, nullCount := qTypeMatrixShortVectorMulNullSum(xv, xn, wv, wn)
				return sum + nullCount
			},
		},
		qEvalVectorCase{
			name:   "TypeMatrixLongPercentDivideSum",
			tags:   []string{"numeric-vector", "promotion", "sum"},
			matrix: []string{"numeric-arithmetic:float-vector:hot"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;y:x%%4;+/y", rows)
			},
			goFn: func(rows int) int64 {
				var sum float64
				for i := 0; i < rows; i++ {
					sum += float64(i) / 4
				}
				return int64(sum)
			},
		},
		qEvalVectorCase{
			name:   "TypeMatrixFloatModSum",
			tags:   []string{"numeric-vector", "word-dyadic", "sum"},
			matrix: []string{"numeric-arithmetic:float-vector:hot"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:(til %d)*0.25;y:x mod 2;+/y", rows)
			},
			goFn: func(rows int) int64 {
				var sum float64
				for i := 0; i < rows; i++ {
					sum += math.Mod(float64(i)*0.25, 2)
				}
				return int64(sum)
			},
		},
		qEvalVectorCase{
			name:   "TypeMatrixIntDivModComboSum",
			tags:   []string{"cast", "word-dyadic", "numeric-vector", "sum"},
			matrix: []string{"cast:typed-numeric:scalar-vector", "numeric-arithmetic:int-vector:hot"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:`int$(til %d) mod 90;d:x div 7;m:x mod 7;s:d+m;+/s", rows)
			},
			goFn: func(rows int) int64 {
				v := qTypeMatrixIntModData(rows, 90)
				var sum int64
				for i := range v {
					sum += int64(v[i]/7) + int64(v[i]%7)
				}
				return sum
			},
		},
		qEvalVectorCase{
			name:   "TypeMatrixLongNullModSum",
			tags:   []string{"typed-null", "word-dyadic", "null-verb", "where", "sum"},
			matrix: []string{"numeric-arithmetic:typed-null:hot"},
			expr: func(rows int) string {
				return fmt.Sprintf("y:%d#0N 5 7 9;z:y mod 3;(+/z)+count where null z", rows)
			},
			goFn: func(rows int) int64 {
				v, n := qTypeMatrixLongPatternData(rows, [4]int64{0, 5, 7, 9}, [4]bool{true, false, false, false})
				sum, nullCount := qTypeMatrixLongDivModNullSum(v, n, 3, false)
				return sum + nullCount
			},
		},
		qEvalVectorCase{
			name:   "TypeMatrixLongNullDivSum",
			tags:   []string{"typed-null", "word-dyadic", "null-verb", "where", "sum"},
			matrix: []string{"numeric-arithmetic:typed-null:hot"},
			expr: func(rows int) string {
				return fmt.Sprintf("y:%d#0N 5 7 9;z:y div 3;(+/z)+count where null z", rows)
			},
			goFn: func(rows int) int64 {
				v, n := qTypeMatrixLongPatternData(rows, [4]int64{0, 5, 7, 9}, [4]bool{true, false, false, false})
				sum, nullCount := qTypeMatrixLongDivModNullSum(v, n, 3, true)
				return sum + nullCount
			},
		},
	)

	// B. Comparison matrix feeding where + count/sum gather.
	cases = append(cases,
		qEvalVectorCase{
			name:   "TypeMatrixFloatCompareGtGatherSum",
			tags:   []string{"composite-compare", "where", "sum"},
			matrix: []string{"numeric-arithmetic:float-vector:hot"},
			shapes: []string{"where:compare-count-sum:row-scaled", "index:gather-after-where:row-scaled"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:(til %d)*0.25;idx:where x>16.5;c:count idx;v:x[idx];s:+/v;s+c", rows)
			},
			goFn: func(rows int) int64 {
				var sum float64
				var count int64
				for i := 0; i < rows; i++ {
					val := float64(i) * 0.25
					if val > 16.5 {
						sum += val
						count++
					}
				}
				return int64(sum + float64(count))
			},
		},
		qEvalVectorCase{
			name:   "TypeMatrixFloatCompareEqNotEqCount",
			tags:   []string{"composite-compare", "where", "numeric-vector"},
			matrix: []string{"numeric-arithmetic:float-vector:hot"},
			shapes: []string{"where:compare-count-sum:row-scaled"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:((til %d) mod 8)*0.25;c1:count where x=1.25;c2:count where x<>0.5;c1+c2", rows)
			},
			goFn: func(rows int) int64 {
				var c1, c2 int64
				for i := 0; i < rows; i++ {
					val := float64(i%8) * 0.25
					if val == 1.25 {
						c1++
					}
					if val != 0.5 {
						c2++
					}
				}
				return c1 + c2
			},
		},
		qEvalVectorCase{
			name:   "TypeMatrixFloatWithinGatherSum",
			tags:   []string{"bin-within-xrank", "where", "sum"},
			matrix: []string{"numeric-arithmetic:float-vector:hot"},
			shapes: []string{"where:gather-reduce-selectivity:row-scaled"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:((til %d) mod 16)*0.5;idx:where x within 2.5 5.0;(+/x[idx])+count idx", rows)
			},
			goFn: func(rows int) int64 {
				var sum float64
				var count int64
				for i := 0; i < rows; i++ {
					val := float64(i%16) * 0.5
					if val >= 2.5 && val <= 5.0 {
						sum += val
						count++
					}
				}
				return int64(sum + float64(count))
			},
		},
		qEvalVectorCase{
			name:   "TypeMatrixShortNullCompareGtGatherSum",
			tags:   []string{"typed-suffix", "typed-null", "composite-compare", "where", "sum"},
			matrix: []string{"numeric-arithmetic:typed-null:hot", "compare:int-vector:where"},
			shapes: []string{"index:gather-after-where:row-scaled"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:%d#0Nh 1h 5h 9h;v:til %d;idx:where x>2h;(+/v[idx])+count idx", rows, rows)
			},
			goFn: func(rows int) int64 {
				v, n := qTypeMatrixShortPatternData(rows, [4]int16{0, 1, 5, 9}, [4]bool{true, false, false, false})
				sum, count := qTypeMatrixShortGatherCompare(v, n, 2, true)
				return sum + count
			},
		},
		qEvalVectorCase{
			name:   "TypeMatrixIntNullCompareLtGatherSum",
			tags:   []string{"typed-suffix", "typed-null", "composite-compare", "where", "sum"},
			matrix: []string{"numeric-arithmetic:typed-null:hot", "compare:int-vector:where"},
			shapes: []string{"index:gather-after-where:row-scaled"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:%d#0Ni 1i 5i 9i;v:til %d;idx:where x<6i;(+/v[idx])+count idx", rows, rows)
			},
			goFn: func(rows int) int64 {
				v, n := qTypeMatrixIntPatternData(rows, [4]int32{0, 1, 5, 9}, [4]bool{true, false, false, false})
				sum, count := qTypeMatrixIntGatherCompare(v, n, 6, false)
				return sum + count
			},
		},
		qEvalVectorCase{
			name:   "TypeMatrixLongNullNotEqualEqualNullCount",
			tags:   []string{"typed-null", "composite-compare", "where"},
			matrix: []string{"numeric-arithmetic:typed-null:hot", "compare:int-vector:where"},
			shapes: []string{"where:compare-count-sum:row-scaled"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:%d#0N 1 2 1;c1:count where x<>1;c2:count where x=0N;c1+c2", rows)
			},
			goFn: func(rows int) int64 {
				v, n := qTypeMatrixLongPatternData(rows, [4]int64{0, 1, 2, 1}, [4]bool{true, false, false, false})
				return qTypeMatrixLongNotEqualNullCount(v, n, 1)
			},
		},
		qEvalVectorCase{
			name:   "TypeMatrixRealNullCompareGtGatherSum",
			tags:   []string{"typed-suffix", "typed-null", "composite-compare", "null-verb", "where", "sum"},
			matrix: []string{"numeric-arithmetic:typed-null:hot"},
			shapes: []string{"index:gather-after-where:row-scaled"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:%d#0Ne 1.5e 2.5e 6e;v:til %d;idx:where x>2;(+/v[idx])+count where null x", rows, rows)
			},
			goFn: func(rows int) int64 {
				v, n := qTypeMatrixRealPatternData(rows, [4]float32{0, 1.5, 2.5, 6}, [4]bool{true, false, false, false})
				sum, nullCount := qTypeMatrixRealGatherGreater(v, n, 2)
				return sum + nullCount
			},
		},
		qEvalVectorCase{
			name:   "TypeMatrixLongNullWithinGatherSum",
			tags:   []string{"typed-null", "bin-within-xrank", "where", "sum"},
			matrix: []string{"numeric-arithmetic:typed-null:hot", "compare:int-vector:where"},
			shapes: []string{"where:gather-reduce-selectivity:row-scaled"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:%d#0N 2 5 9;v:til %d;idx:where x within 2 8;(+/v[idx])+count idx", rows, rows)
			},
			goFn: func(rows int) int64 {
				v, n := qTypeMatrixLongPatternData(rows, [4]int64{0, 2, 5, 9}, [4]bool{true, false, false, false})
				sum, count := qTypeMatrixLongWithinGather(v, n, 2, 8)
				return sum + count
			},
		},
		qEvalVectorCase{
			name:   "TypeMatrixRealCompareGeGatherSum",
			tags:   []string{"cast", "composite-compare", "where", "sum"},
			matrix: []string{"cast:typed-numeric:scalar-vector"},
			shapes: []string{"where:compare-count-sum:row-scaled"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:\"e\"$(til %d) mod 32;idx:where x>=8;(+/x[idx])+count idx", rows)
			},
			goFn: func(rows int) int64 {
				v := qTypeMatrixRealModData(rows, 32)
				var sum float64
				var count int64
				for i := range v {
					if v[i] >= 8 {
						sum += float64(v[i])
						count++
					}
				}
				return int64(sum + float64(count))
			},
		},
	)

	// C. Cast round-trips chained with arithmetic + reduce.
	cases = append(cases,
		qEvalVectorCase{
			name:   "TypeMatrixCastShortLetterRoundTripSum",
			tags:   []string{"cast", "promotion", "numeric-vector", "sum"},
			matrix: []string{"cast:typed-numeric:scalar-vector"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;b:\"h\"$x mod 95;c:`long$b;y:c*3;+/y", rows)
			},
			goFn: func(rows int) int64 {
				v := qTypeMatrixShortModData(rows, 95)
				sum, _ := qTypeMatrixShortAffineNullSum(v, make([]bool, rows), 3, 0)
				return sum
			},
		},
		qEvalVectorCase{
			name:   "TypeMatrixCastIntLetterModSum",
			tags:   []string{"cast", "word-dyadic", "numeric-vector", "sum"},
			matrix: []string{"cast:typed-numeric:scalar-vector", "numeric-arithmetic:int-vector:hot"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;y:\"i\"$x mod 257;z:y mod 5;(+/z)+count z", rows)
			},
			goFn: func(rows int) int64 {
				v := qTypeMatrixIntModData(rows, 257)
				var sum int64
				for i := range v {
					sum += int64(v[i] % 5)
				}
				return sum + int64(rows)
			},
		},
		qEvalVectorCase{
			name:   "TypeMatrixCastFloatLetterHalfSum",
			tags:   []string{"cast", "promotion", "numeric-vector", "sum"},
			matrix: []string{"cast:typed-numeric:scalar-vector", "numeric-arithmetic:float-vector:hot"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;y:\"f\"$x mod 128;z:y%%2;+/z", rows)
			},
			goFn: func(rows int) int64 {
				var sum float64
				for i := 0; i < rows; i++ {
					sum += float64(i%128) / 2
				}
				return int64(sum)
			},
		},
		qEvalVectorCase{
			name:   "TypeMatrixCastRealLetterQuarterSum",
			tags:   []string{"cast", "promotion", "numeric-vector", "sum"},
			matrix: []string{"cast:typed-numeric:scalar-vector", "numeric-arithmetic:float-vector:hot"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;y:\"e\"$x mod 256;z:y*0.25;+/z", rows)
			},
			goFn: func(rows int) int64 {
				v := qTypeMatrixRealModData(rows, 256)
				sum, _ := qTypeMatrixRealAffineNullSum(v, make([]bool, rows), 0.25, 0)
				return int64(sum)
			},
		},
		qEvalVectorCase{
			name:   "TypeMatrixCastBoolMaskGatherSum",
			tags:   []string{"cast", "boolean-logical", "where", "sum"},
			matrix: []string{"cast:typed-numeric:scalar-vector", "compare:int-vector:where"},
			shapes: []string{"index:gather-after-where:row-scaled"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;y:x mod 2;m:\"b\"$y;idx:where m;g:x[idx];(+/g)+count idx", rows)
			},
			goFn: func(rows int) int64 {
				var sum, count int64
				for i := 0; i < rows; i++ {
					if i%2 == 1 {
						sum += int64(i)
						count++
					}
				}
				return sum + count
			},
		},
		qEvalVectorCase{
			name:   "TypeMatrixCastRealLongRoundTripSum",
			tags:   []string{"cast", "promotion", "numeric-vector", "sum"},
			matrix: []string{"cast:typed-numeric:scalar-vector"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;y:`real$x mod 4096;z:`long$y;w:z+2;+/w", rows)
			},
			goFn: func(rows int) int64 {
				v := qTypeMatrixRealModData(rows, 4096)
				var sum int64
				for i := range v {
					sum += int64(v[i]) + 2
				}
				return sum
			},
		},
		qEvalVectorCase{
			name:   "TypeMatrixCastShortIntLongChainSum",
			tags:   []string{"cast", "promotion", "numeric-vector", "sum"},
			matrix: []string{"cast:typed-numeric:scalar-vector"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;s:`short$x mod 100;y:`int$s;z:`long$y;w:z+11;+/w", rows)
			},
			goFn: func(rows int) int64 {
				v := qTypeMatrixShortModData(rows, 100)
				var sum int64
				for i := range v {
					sum += int64(int32(v[i])) + 11
				}
				return sum
			},
		},
	)

	// D. Compound predicates at depth (3+ clauses) feeding gather+reduce.
	cases = append(cases,
		qEvalVectorCase{
			name:   "TypeMatrixWhereModCompareMembershipGatherSum",
			tags:   []string{"where", "membership", "word-dyadic", "boolean-logical", "sum"},
			matrix: []string{"compare:int-vector:where", "membership:in-differ-ratios:vector"},
			shapes: []string{"logical:mask-composition:row-scaled", "index:gather-after-where:row-scaled"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;y:(x*3)+7;idx:where ((x mod 3)=1) and (x>64) and (not (x in 70 73 76));(+/y[idx])+count idx", rows)
			},
			goFn: func(rows int) int64 {
				var sum, count int64
				for i := 0; i < rows; i++ {
					if i%3 == 1 && i > 64 && i != 70 && i != 73 && i != 76 {
						sum += int64(i)*3 + 7
						count++
					}
				}
				return sum + count
			},
		},
		qEvalVectorCase{
			name:   "TypeMatrixWhereWithinModNotEqGatherSum",
			tags:   []string{"where", "bin-within-xrank", "word-dyadic", "boolean-logical", "sum"},
			matrix: []string{"compare:int-vector:where"},
			shapes: []string{"logical:mask-composition:row-scaled", "where:gather-reduce-selectivity:row-scaled"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;v:(x*2)+1;idx:where (x within 100 7000) and ((x mod 5)>2) and ((x mod 7)<>0);(+/v[idx])+count idx", rows)
			},
			goFn: func(rows int) int64 {
				var sum, count int64
				for i := 0; i < rows; i++ {
					if i >= 100 && i <= 7000 && i%5 > 2 && i%7 != 0 {
						sum += int64(i)*2 + 1
						count++
					}
				}
				return sum + count
			},
		},
		qEvalVectorCase{
			name:   "TypeMatrixWhereSymbolMembershipNumericGatherSum",
			tags:   []string{"where", "membership", "symbol", "boolean-logical", "sum"},
			matrix: []string{"compare:symbol-vector:where", "membership:in-differ-ratios:vector"},
			shapes: []string{"membership:symbol-filter:row-scaled", "logical:mask-composition:row-scaled"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;sym:%d#`AAPL`MSFT`NVDA`TSLA;v:x*3;idx:where (sym in `AAPL`NVDA) and (x>128) and ((x mod 3)=0);(+/v[idx])+count idx", rows, rows)
			},
			goFn: func(rows int) int64 {
				var sum, count int64
				for i := 0; i < rows; i++ {
					if (i%4 == 0 || i%4 == 2) && i > 128 && i%3 == 0 {
						sum += int64(i) * 3
						count++
					}
				}
				return sum + count
			},
		},
		qEvalVectorCase{
			name:   "TypeMatrixWhereNegatedModWithinGatherSum",
			tags:   []string{"where", "bin-within-xrank", "boolean-logical", "word-dyadic", "sum"},
			matrix: []string{"compare:int-vector:where"},
			shapes: []string{"logical:mask-composition:row-scaled", "index:gather-after-where:row-scaled"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;v:x+13;idx:where (not ((x mod 11)=3)) and (x within 50 8000) and (x>100);(+/v[idx])+count idx", rows)
			},
			goFn: func(rows int) int64 {
				var sum, count int64
				for i := 0; i < rows; i++ {
					if i%11 != 3 && i >= 50 && i <= 8000 && i > 100 {
						sum += int64(i) + 13
						count++
					}
				}
				return sum + count
			},
		},
		qEvalVectorCase{
			name:   "TypeMatrixWhereFloatIntMixedPredicateGatherSum",
			tags:   []string{"where", "composite-compare", "word-dyadic", "boolean-logical", "sum"},
			matrix: []string{"compare:int-vector:where", "numeric-arithmetic:float-vector:hot"},
			shapes: []string{"logical:mask-composition:row-scaled", "index:gather-after-where:row-scaled"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;p:(x mod 64)*0.25;idx:where (p>4.5) and ((x mod 3)=0) and (x<7500);g:x[idx];(+/g)+count idx", rows)
			},
			goFn: func(rows int) int64 {
				var sum, count int64
				for i := 0; i < rows; i++ {
					p := float64(i%64) * 0.25
					if p > 4.5 && i%3 == 0 && i < 7500 {
						sum += int64(i)
						count++
					}
				}
				return sum + count
			},
		},
		qEvalVectorCase{
			name:   "TypeMatrixWhereNullGuardCompoundGatherSum",
			tags:   []string{"where", "typed-null", "null-verb", "boolean-logical", "sum"},
			matrix: []string{"numeric-arithmetic:typed-null:hot", "compare:int-vector:where"},
			shapes: []string{"logical:mask-composition:row-scaled", "null:fill-arithmetic-where:row-scaled"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:%d#0N 3 6 9;v:til %d;idx:where (not null x) and (x>4) and ((x mod 3)=0);(+/v[idx])+count idx", rows, rows)
			},
			goFn: func(rows int) int64 {
				v, n := qTypeMatrixLongPatternData(rows, [4]int64{0, 3, 6, 9}, [4]bool{true, false, false, false})
				var sum, count int64
				for i := range v {
					if !n[i] && v[i] > 4 && v[i]%3 == 0 {
						sum += int64(i)
						count++
					}
				}
				return sum + count
			},
		},
		qEvalVectorCase{
			name:   "TypeMatrixWhereFourClauseMembershipGatherSum",
			tags:   []string{"where", "membership", "word-dyadic", "boolean-logical", "sum"},
			matrix: []string{"compare:int-vector:where", "membership:in-differ-ratios:vector"},
			shapes: []string{"logical:mask-composition:row-scaled", "where:gather-reduce-selectivity:row-scaled"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;y:(x mod 97)+5;idx:where (y>20) and (y<90) and ((y mod 4)=1) and (not (y in 25 33 41));(+/y[idx])+count idx", rows)
			},
			goFn: func(rows int) int64 {
				var sum, count int64
				for i := 0; i < rows; i++ {
					y := int64(i%97) + 5
					if y > 20 && y < 90 && y%4 == 1 && y != 25 && y != 33 && y != 41 {
						sum += y
						count++
					}
				}
				return sum + count
			},
		},
		qEvalVectorCase{
			name:   "TypeMatrixWhereXbarWithinMembershipGatherSum",
			tags:   []string{"where", "xbar", "bin-within-xrank", "membership", "boolean-logical", "sum"},
			matrix: []string{"compare:int-vector:where", "membership:in-differ-ratios:vector", "temporal:xbar:bucket"},
			shapes: []string{"logical:mask-composition:row-scaled", "index:gather-after-where:row-scaled"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;r:x mod 200;bkt:10 xbar r;par:x mod 2;idx:where (bkt within 40 160) and (not (bkt in 50 100 150)) and (par=1);g:x[idx];(+/g)+count idx", rows)
			},
			goFn: func(rows int) int64 {
				var sum, count int64
				for i := 0; i < rows; i++ {
					b := int64((i % 200) / 10 * 10)
					if b >= 40 && b <= 160 && b != 50 && b != 100 && b != 150 && i%2 == 1 {
						sum += int64(i)
						count++
					}
				}
				return sum + count
			},
		},
	)

	// E. Promotion boundaries proved by reduce values.
	cases = append(cases,
		qEvalVectorCase{
			name:   "TypeMatrixPromoteShortPlusLongScalarSum",
			tags:   []string{"cast", "promotion", "numeric-vector", "sum"},
			matrix: []string{"cast:typed-numeric:scalar-vector", "numeric-arithmetic:int-vector:hot"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:`short$(til %d) mod 50;y:x+100000;+/y", rows)
			},
			goFn: func(rows int) int64 {
				v := qTypeMatrixShortModData(rows, 50)
				sum, _ := qTypeMatrixShortAffineNullSum(v, make([]bool, rows), 1, 100000)
				return sum
			},
		},
		qEvalVectorCase{
			name:   "TypeMatrixPromoteIntPlusFloatScalarSum",
			tags:   []string{"cast", "promotion", "numeric-vector", "sum"},
			matrix: []string{"cast:typed-numeric:scalar-vector", "numeric-arithmetic:float-vector:hot"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:`int$(til %d) mod 1024;y:x+0.5;+/y", rows)
			},
			goFn: func(rows int) int64 {
				v := qTypeMatrixIntModData(rows, 1024)
				var sum float64
				for i := range v {
					sum += float64(v[i]) + 0.5
				}
				return int64(sum)
			},
		},
		qEvalVectorCase{
			name:   "TypeMatrixPromoteRealPlusIntScalarSum",
			tags:   []string{"cast", "promotion", "numeric-vector", "sum"},
			matrix: []string{"cast:typed-numeric:scalar-vector", "numeric-arithmetic:float-vector:hot"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:\"e\"$(til %d) mod 128;y:x+3;+/y", rows)
			},
			goFn: func(rows int) int64 {
				v := qTypeMatrixRealModData(rows, 128)
				sum, _ := qTypeMatrixRealAffineNullSum(v, make([]bool, rows), 1, 3)
				return int64(sum)
			},
		},
		qEvalVectorCase{
			name:   "TypeMatrixPromoteShortPlusIntVectorSum",
			tags:   []string{"cast", "promotion", "numeric-vector", "sum"},
			matrix: []string{"cast:typed-numeric:scalar-vector", "numeric-arithmetic:int-vector:hot"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;s:`short$x mod 11;w:`int$x mod 13;y:s+w;+/y", rows)
			},
			goFn: func(rows int) int64 {
				a := qTypeMatrixShortModData(rows, 11)
				b := qTypeMatrixIntModData(rows, 13)
				var sum int64
				for i := range a {
					sum += int64(a[i]) + int64(b[i])
				}
				return sum
			},
		},
		qEvalVectorCase{
			name:   "TypeMatrixPromoteShortMulOverflowSum",
			tags:   []string{"cast", "promotion", "numeric-vector", "sum"},
			matrix: []string{"cast:typed-numeric:scalar-vector", "numeric-arithmetic:int-vector:hot"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:`short$(til %d) mod 300;y:x*1000;+/y", rows)
			},
			goFn: func(rows int) int64 {
				v := qTypeMatrixShortModData(rows, 300)
				sum, _ := qTypeMatrixShortAffineNullSum(v, make([]bool, rows), 1000, 0)
				return sum
			},
		},
		qEvalVectorCase{
			name:   "TypeMatrixPromoteIntMulOverflowSum",
			tags:   []string{"cast", "promotion", "numeric-vector", "sum"},
			matrix: []string{"cast:typed-numeric:scalar-vector", "numeric-arithmetic:int-vector:hot"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:`int$til %d;y:x*1000000;+/y", rows)
			},
			goFn: func(rows int) int64 {
				v := qTypeMatrixIntModData(rows, rows+1)
				sum, _ := qTypeMatrixIntAffineNullSum(v, make([]bool, rows), 1000000, 0)
				return sum
			},
		},
	)

	return cases
}

// --- permanent validation ----------------------------------------------------

// TestQEvalVectorTypeNullMatrixCaseValues proves every type/null-matrix case's
// Go baseline equals the q.eval result at the benchmark row count and that
// every case is row-scaled (the q expression result differs between the full
// and half row counts).
func TestQEvalVectorTypeNullMatrixCaseValues(t *testing.T) {
	eval := qEvalVectorEval(t)
	seen := make(map[string]struct{})
	for _, tc := range qEvalVectorTypeNullMatrixCases() {
		tc := tc
		if _, dup := seen[tc.name]; dup {
			t.Fatalf("duplicate type-matrix case name %q", tc.name)
		}
		seen[tc.name] = struct{}{}
		t.Run(tc.name, func(t *testing.T) {
			got := qEvalVectorRun(t, eval, tc.expr(qEvalVectorRows))
			want := tc.goFn(qEvalVectorRows)
			if got != want {
				t.Fatalf("q.eval(%q) = %d, Go baseline = %d", tc.expr(qEvalVectorRows), got, want)
			}
			gotHalf := qEvalVectorRun(t, eval, tc.expr(qEvalVectorRows/2))
			wantHalf := tc.goFn(qEvalVectorRows / 2)
			if gotHalf != wantHalf {
				t.Fatalf("q.eval(%q) = %d, Go baseline = %d", tc.expr(qEvalVectorRows/2), gotHalf, wantHalf)
			}
			if got == gotHalf {
				t.Fatalf("case %s is row-invariant: %d at both %d and %d rows", tc.name, got, qEvalVectorRows, qEvalVectorRows/2)
			}
		})
	}
}
