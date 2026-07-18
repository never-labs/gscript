//go:build leia_q

package benchmarks

// Complex-combination q.eval benchmark cases: depth>=3 pipelines, mixed-type
// arithmetic, null-mixed pipelines, nested adverbs, compound where predicates,
// dict->table->aggregate chains, and adverb-over-apply/index fusion. Each case
// carries one (or two, when genuinely both) of the qEvalRequiredComboShapes
// tags, with at least qEvalComboMinCases cases per tag.

import (
	"fmt"
	"math"
	"sort"
	"testing"
)

// ---------------------------------------------------------------------------
// Go baseline helpers.
// ---------------------------------------------------------------------------

//go:noinline
func qEvalComboGoDeltasAbsSqrtFloorSum(rows int, mod int64) int64 {
	var prev, sum int64
	for i := 0; i < rows; i++ {
		v := qEvalVectorGoBaselineInput[i] % mod
		d := v - prev
		if i == 0 {
			d = v
		}
		prev = v
		if d < 0 {
			d = -d
		}
		sum += int64(math.Floor(math.Sqrt(float64(d))))
	}
	return sum
}

// qEvalComboGoFillsPattern materializes rows values from a repeating
// null-bearing pattern (math.MinInt64 marks 0N), applies fills (carry last
// non-null forward) and then a scalar fill for any leading nulls.
//
//go:noinline
func qEvalComboGoFillsPattern(rows int, pattern []int64, leadingFill int64) []int64 {
	values := make([]int64, rows)
	last := int64(math.MinInt64)
	for i := 0; i < rows; i++ {
		v := pattern[i%len(pattern)]
		if v != math.MinInt64 {
			last = v
		}
		if last == math.MinInt64 {
			values[i] = leadingFill
		} else {
			values[i] = last
		}
	}
	qEvalVectorAnyBenchSink = values
	return values
}

//go:noinline
func qEvalComboGoMovingSumTotal(values []int64, width int) int64 {
	var window, total int64
	for i := 0; i < len(values); i++ {
		window += values[i]
		if i >= width {
			window -= values[i-width]
		}
		total += window
	}
	return total
}

//go:noinline
func qEvalComboGoRotateDeltasAbsSum(rows, shift int) int64 {
	shift %= rows
	var prev, sum int64
	for i := 0; i < rows; i++ {
		v := qEvalVectorGoBaselineInput[(shift+i)%rows]
		d := v - prev
		if i == 0 {
			d = v
		}
		prev = v
		if d < 0 {
			d = -d
		}
		sum += d
	}
	return sum
}

//go:noinline
func qEvalComboGoSumsModXbarSum(rows int, mod, width int64) int64 {
	var prefix, sum int64
	for i := 0; i < rows; i++ {
		prefix += qEvalVectorGoBaselineInput[i] % mod
		sum += (prefix / width) * width
	}
	return sum
}

//go:noinline
func qEvalComboGoFloatNotionalFloor(rows int) int64 {
	var sum float64
	for i := 0; i < rows; i++ {
		px := 100.5 + 0.25*float64(i%400)
		sz := float64(1 + i%50)
		sum += px * sz
	}
	return int64(math.Floor(sum))
}

//go:noinline
func qEvalComboGoShortIntFloatPromotion(rows int) int64 {
	var sum float64
	for i := 0; i < rows; i++ {
		s := float64(2 * int64(i%9))
		f := 0.25 * float64(i)
		sum += s + f
	}
	return int64(math.Floor(sum))
}

//go:noinline
func qEvalComboGoFloatXbarBucketEnvelope(rows int) int64 {
	var bucketSum float64
	var count int64
	for i := 0; i < rows; i++ {
		px := 0.5 * float64(i%19)
		b := 2 * math.Floor(px/2)
		bucketSum += b
		if b >= 4 {
			count++
		}
	}
	return int64(bucketSum + float64(count))
}

//go:noinline
func qEvalComboGoNullFillsWhereGatherReduce(rows int) int64 {
	pattern := []int64{math.MinInt64, 100, math.MinInt64, 105, 110}
	filled := qEvalComboGoFillsPattern(rows, pattern, 0)
	var sum, count int64
	for _, v := range filled {
		if v > 104 {
			sum += v
			count++
		}
	}
	return sum + count
}

//go:noinline
func qEvalComboGoNullMaskArithWithinEnvelope(rows int) int64 {
	pattern := []int64{3, math.MinInt64, 5, math.MinInt64, 7, 9}
	var withinCount, modSum int64
	for i := 0; i < rows; i++ {
		f := pattern[i%len(pattern)]
		if f == math.MinInt64 {
			f = 1
		}
		v := f * 2
		if v >= 6 && v <= 14 {
			withinCount++
		}
		modSum += v % 11
	}
	return withinCount + modSum
}

//go:noinline
func qEvalComboGoCutSegmentSums(rows int, cuts []int) int64 {
	var total int64
	for seg := 0; seg < len(cuts); seg++ {
		start := cuts[seg]
		end := rows
		if seg+1 < len(cuts) {
			end = cuts[seg+1]
		}
		var segment int64
		for i := start; i < end; i++ {
			segment += qEvalVectorGoBaselineInput[i]
		}
		total += segment
	}
	return total
}

//go:noinline
func qEvalComboGoCutSegmentScansRazeSum(rows int, mod int64, cuts []int) int64 {
	var total int64
	for seg := 0; seg < len(cuts); seg++ {
		start := cuts[seg]
		end := rows
		if seg+1 < len(cuts) {
			end = cuts[seg+1]
		}
		var prefix int64
		for i := start; i < end; i++ {
			prefix += qEvalVectorGoBaselineInput[i] % mod
			total += prefix
		}
	}
	return total
}

//go:noinline
func qEvalComboGoEachPriorAbsSum(rows int, mod int64) int64 {
	var prev, sum int64
	for i := 0; i < rows; i++ {
		v := qEvalVectorGoBaselineInput[i] % mod
		d := v - prev
		if i == 0 {
			d = v
		}
		prev = v
		if d < 0 {
			d = -d
		}
		sum += d
	}
	return sum
}

//go:noinline
func qEvalComboGoWherePriceBandSizeWithin(rows int) int64 {
	var sum, count int64
	for i := 0; i < rows; i++ {
		px := 100 + int64(i%90)
		sz := 1 + int64(i%64)
		if px > 120 && sz >= 8 && sz <= 32 {
			sum += px
			count++
		}
	}
	return sum + count
}

//go:noinline
func qEvalComboGoWhereSymbolInOrNot(rows int) int64 {
	syms := []string{"AAPL", "MSFT", "NVDA", "TSLA", "AMD"}
	var sum, count int64
	for i := 0; i < rows; i++ {
		sym := syms[i%len(syms)]
		px := int64(i % 97)
		if sym == "AAPL" || sym == "NVDA" || !(px < 60) {
			sum += px
			count++
		}
	}
	return sum + count
}

//go:noinline
func qEvalComboGoWhereModCompareMembership(rows int) int64 {
	var sum int64
	for i := 0; i < rows; i++ {
		m := int64(i % 7)
		r := int64(i % 11)
		if m == 3 && (r == 2 || r == 5 || r == 7) {
			sum += int64(i)
		}
	}
	return sum
}

//go:noinline
func qEvalComboGoDictTableXgroupUngroupSum(rows int) int64 {
	syms := []string{"AAPL", "MSFT", "NVDA", "TSLA"}
	groups := make(map[string]int64, len(syms))
	var sum int64
	for i := 0; i < rows; i++ {
		sym := syms[i%len(syms)]
		px := 100 + int64(i%90)
		groups[sym] += px
		sum += px
	}
	return sum + int64(len(groups))
}

//go:noinline
func qEvalComboGoDictTableFbyNotionalSum(rows int) int64 {
	groupSums := make([]int64, 3)
	for i := 0; i < rows; i++ {
		px := 100 + int64(i%50)
		sz := 1 + int64(i%16)
		groupSums[i%3] += px * sz
	}
	var total int64
	for i := 0; i < rows; i++ {
		total += groupSums[i%3]
	}
	return total
}

//go:noinline
func qEvalComboGoDictTableXascHeadSum(rows, head int) int64 {
	px := make([]int64, rows)
	for i := 0; i < rows; i++ {
		px[i] = 100 + (int64(i)*37)%401
	}
	sorted := append([]int64(nil), px...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	var sum int64
	for i := 0; i < head && i < len(sorted); i++ {
		sum += sorted[i]
	}
	qEvalVectorAnyBenchSink = sorted
	return sum + int64(rows)
}

//go:noinline
func qEvalComboGoOverGatherModuloSum(rows int) int64 {
	var sum, count int64
	for i := 0; i < rows; i++ {
		v := int64(i % 101)
		if v%3 == 0 {
			sum += v
			count++
		}
	}
	return sum + count
}

//go:noinline
func qEvalComboGoScanGatherLastEnvelope(rows int) int64 {
	var last, count int64
	for i := 0; i < rows; i++ {
		if i%7 < 2 {
			last += int64(i)
			count++
		}
	}
	return last + count
}

//go:noinline
func qEvalComboGoEachIndexedMatrixRows(rows, cols int) int64 {
	matrixRows := rows / cols
	var sum int64
	for c := 0; c < cols; c++ {
		sum += qEvalVectorGoBaselineInput[c]
		sum += qEvalVectorGoBaselineInput[(matrixRows-1)*cols+c]
	}
	return sum
}

// ---------------------------------------------------------------------------
// Cases.
// ---------------------------------------------------------------------------

func qEvalVectorComboCases() []qEvalVectorCase {
	cases := []qEvalVectorCase{
		// -- combo:producer-transform-reducer:depth3 ------------------------
		{
			name: "ComboDepth3DeltasAbsSqrtFloorSum",
			tags: []string{
				"combo:producer-transform-reducer:depth3",
				"numeric-vector", "numeric-monad", "math-transcendental", "sum",
			},
			matrix: []string{"math:transcendental-vector:hot"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:(til %d) mod 97;+/floor sqrt abs deltas x", rows)
			},
			goFn: func(rows int) int64 {
				return qEvalComboGoDeltasAbsSqrtFloorSum(rows, 97)
			},
		},
		{
			name: "ComboDepth3FillsMsumNullPipeline",
			tags: []string{
				"combo:producer-transform-reducer:depth3",
				"combo:null-mixed-pipeline",
				"typed-null", "fill", "moving-window", "sum",
			},
			matrix: []string{"list:prev-next-deltas-fills:typed-null"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:%d#0N 3 0N 7 5;y:0^fills x;+/32 msum y", rows)
			},
			goFn: func(rows int) int64 {
				pattern := []int64{math.MinInt64, 3, math.MinInt64, 7, 5}
				filled := qEvalComboGoFillsPattern(rows, pattern, 0)
				return qEvalComboGoMovingSumTotal(filled, 32)
			},
		},
		{
			name: "ComboDepth3RotateDeltasAbsSum",
			tags: []string{
				"combo:producer-transform-reducer:depth3",
				"numeric-vector", "rotate", "numeric-monad", "sum",
			},
			expr: func(rows int) string {
				return fmt.Sprintf("x:1024 rotate til %d;+/abs deltas x", rows)
			},
			goFn: func(rows int) int64 {
				return qEvalComboGoRotateDeltasAbsSum(rows, 1024)
			},
		},
		{
			name: "ComboDepth3SumsModXbarReduce",
			tags: []string{
				"combo:producer-transform-reducer:depth3",
				"numeric-vector", "sums", "xbar", "sum",
			},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;m:x mod 13;y:sums m;+/10 xbar y", rows)
			},
			goFn: func(rows int) int64 {
				return qEvalComboGoSumsModXbarSum(rows, 13, 10)
			},
		},

		// -- combo:mixed-type-arith -----------------------------------------
		{
			name: "ComboMixedFloatIntNotionalFloor",
			tags: []string{
				"combo:mixed-type-arith",
				"numeric-vector", "promotion", "sum",
			},
			matrix: []string{"numeric-arithmetic:float-vector:hot"},
			expr: func(rows int) string {
				return fmt.Sprintf("n:til %d;px:100.5+0.25*(n mod 400);sz:1+(n mod 50);t:px*sz;s:+/t;floor s", rows)
			},
			goFn: func(rows int) int64 {
				return qEvalComboGoFloatNotionalFloor(rows)
			},
		},
		{
			name: "ComboMixedTypedSuffixPromotionSum",
			tags: []string{
				"combo:mixed-type-arith",
				"typed-suffix", "cast", "promotion", "sum",
			},
			matrix: []string{"cast:typed-numeric:scalar-vector"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;s:`short$(x mod 9);f:0.25*x;t:(s*2i)+f;v:+/t;floor v", rows)
			},
			goFn: func(rows int) int64 {
				return qEvalComboGoShortIntFloatPromotion(rows)
			},
		},
		{
			name: "ComboMixedFloatXbarBucketEnvelope",
			tags: []string{
				"combo:mixed-type-arith",
				"promotion", "xbar", "where", "sum",
			},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;px:0.5*(x mod 19);b:2 xbar px;(+/b)+count where b>=4", rows)
			},
			goFn: func(rows int) int64 {
				return qEvalComboGoFloatXbarBucketEnvelope(rows)
			},
		},

		// -- combo:null-mixed-pipeline ---------------------------------------
		{
			name: "ComboNullFillsWhereGatherReduce",
			tags: []string{
				"combo:null-mixed-pipeline",
				"typed-null", "fill", "where", "apply-index", "sum",
			},
			matrix: []string{"numeric-arithmetic:typed-null:hot"},
			expr: func(rows int) string {
				return fmt.Sprintf("px:%d#0N 100 0N 105 110;c:0^fills px;idx:where c>104;(+/c[idx])+count idx", rows)
			},
			goFn: func(rows int) int64 {
				return qEvalComboGoNullFillsWhereGatherReduce(rows)
			},
		},
		{
			name: "ComboNullMaskArithWithinEnvelope",
			tags: []string{
				"combo:null-mixed-pipeline",
				"typed-null", "fill", "where", "bin-within-xrank", "sum",
			},
			expr: func(rows int) string {
				return fmt.Sprintf("sz:%d#3 0N 5 0N 7 9;f:1^sz;v:f*2;(count where v within 6 14)+(+/(v mod 11))", rows)
			},
			goFn: func(rows int) int64 {
				return qEvalComboGoNullMaskArithWithinEnvelope(rows)
			},
		},

		// -- combo:nested-adverb ----------------------------------------------
		{
			name: "ComboNestedOverEachCutSegments",
			tags: []string{
				"combo:nested-adverb",
				"adverb-each", "adverb-over-scan", "cut", "sum",
			},
			matrix: []string{"adverb:over-scan:projection"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;g:0 1024 2048 4096 cut x;s:(+/)'g;+/s", rows)
			},
			goFn: func(rows int) int64 {
				return qEvalComboGoCutSegmentSums(rows, []int{0, 1024, 2048, 4096})
			},
		},
		{
			name: "ComboNestedScanEachRazeSum",
			tags: []string{
				"combo:nested-adverb",
				"adverb-each", "adverb-over-scan", "cut", "raze", "sum",
			},
			expr: func(rows int) string {
				return fmt.Sprintf("x:(til %d) mod 23;g:0 2048 4096 cut x;+/raze (+\\)'g", rows)
			},
			goFn: func(rows int) int64 {
				return qEvalComboGoCutSegmentScansRazeSum(rows, 23, []int{0, 2048, 4096})
			},
		},
		{
			name: "ComboNestedEachPriorOverAbs",
			tags: []string{
				"combo:nested-adverb",
				"adverb-each-prior", "adverb-over-scan", "numeric-monad", "sum",
			},
			matrix: []string{"adverb:each-prior:vector"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:(til %d) mod 53;d:(-':)[x];+/abs d", rows)
			},
			goFn: func(rows int) int64 {
				return qEvalComboGoEachPriorAbsSum(rows, 53)
			},
		},

		// -- combo:where-compound-predicate -----------------------------------
		{
			name: "ComboWherePriceBandSizeWithin",
			tags: []string{
				"combo:where-compound-predicate",
				"where", "boolean-logical", "bin-within-xrank", "apply-index", "sum",
			},
			matrix: []string{"compare:int-vector:where"},
			expr: func(rows int) string {
				return fmt.Sprintf("n:til %d;px:100+(n mod 90);sz:1+(n mod 64);idx:where (px>120) and (sz within 8 32);(+/px[idx])+count idx", rows)
			},
			goFn: func(rows int) int64 {
				return qEvalComboGoWherePriceBandSizeWithin(rows)
			},
		},
		{
			name: "ComboWhereSymbolInOrNotCompare",
			tags: []string{
				"combo:where-compound-predicate",
				"where", "boolean-logical", "membership", "symbol", "sum",
			},
			matrix: []string{"compare:symbol-vector:where"},
			expr: func(rows int) string {
				return fmt.Sprintf("n:til %d;sym:%d#`AAPL`MSFT`NVDA`TSLA`AMD;px:n mod 97;idx:where (sym in `AAPL`NVDA) or (not (px<60));(+/px[idx])+count idx", rows, rows)
			},
			goFn: func(rows int) int64 {
				return qEvalComboGoWhereSymbolInOrNot(rows)
			},
		},
		{
			name: "ComboWhereModCompareMembershipSum",
			tags: []string{
				"combo:where-compound-predicate",
				"where", "boolean-logical", "membership", "composite-compare", "sum",
			},
			matrix: []string{"membership:in-differ-ratios:vector"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;m:x mod 7;idx:where (m=3) and ((x mod 11) in 2 5 7);+/x[idx]", rows)
			},
			goFn: func(rows int) int64 {
				return qEvalComboGoWhereModCompareMembership(rows)
			},
		},

		// -- combo:dict-table-chain --------------------------------------------
		{
			name: "ComboDictTableXgroupUngroupSum",
			tags: []string{
				"combo:dict-table-chain",
				"dict", "table-literal", "table-group-ungroup", "symbol", "sum",
			},
			matrix: []string{"table:xkey-xgroup-ungroup:keyed-frame"},
			expr: func(rows int) string {
				return fmt.Sprintf("n:til %d;sym:%d#`AAPL`MSFT`NVDA`TSLA;px:100+(n mod 90);t:flip `sym`px!(sym;px);g:`sym xgroup t;u:ungroup g;(+/u`px)+count g", rows, rows)
			},
			goFn: func(rows int) int64 {
				return qEvalComboGoDictTableXgroupUngroupSum(rows)
			},
		},
		{
			name: "ComboDictTableFbyNotionalSum",
			tags: []string{
				"combo:dict-table-chain",
				"dict", "table-literal", "fby", "group", "sum",
			},
			matrix: []string{"table:literal-meta-cols:frame"},
			expr: func(rows int) string {
				return fmt.Sprintf("n:til %d;sym:%d#`AAPL`MSFT`NVDA;px:100+(n mod 50);sz:1+(n mod 16);t:flip `sym`px`sz!(sym;px;sz);v:(t`px)*t`sz;sy:t`sym;s:sum v fby sy;+/s", rows, rows)
			},
			goFn: func(rows int) int64 {
				return qEvalComboGoDictTableFbyNotionalSum(rows)
			},
		},
		{
			name: "ComboDictTableXascHeadGatherSum",
			tags: []string{
				"combo:dict-table-chain",
				"dict", "table-literal", "table-sort", "take", "sum",
			},
			matrix: []string{"table:xcols-xasc-xdesc:frame"},
			expr: func(rows int) string {
				return fmt.Sprintf("n:til %d;px:100+((n*37) mod 401);t:flip `id`px!(n;px);a:`px xasc t;y:a`px;(+/256#y)+count y", rows)
			},
			goFn: func(rows int) int64 {
				return qEvalComboGoDictTableXascHeadSum(rows, 256)
			},
		},

		// -- combo:adverb-over-apply-index --------------------------------------
		{
			name: "ComboOverGatherModuloSum",
			tags: []string{
				"combo:adverb-over-apply-index",
				"adverb-over-scan", "where", "apply-index", "sum",
			},
			matrix: []string{"apply-index:list-dict-callable:hot"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:(til %d) mod 101;idx:where 0=(x mod 3);(+/x[idx])+count idx", rows)
			},
			goFn: func(rows int) int64 {
				return qEvalComboGoOverGatherModuloSum(rows)
			},
		},
		{
			name: "ComboScanGatherLastEnvelope",
			tags: []string{
				"combo:adverb-over-apply-index",
				"adverb-over-scan", "where", "apply-index", "sum",
			},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;idx:where (x mod 7)<2;s:+\\x[idx];(last s)+count s", rows)
			},
			goFn: func(rows int) int64 {
				return qEvalComboGoScanGatherLastEnvelope(rows)
			},
		},
		{
			name: "ComboEachIndexedMatrixRowsSum",
			tags: []string{
				"combo:adverb-over-apply-index",
				"combo:nested-adverb",
				"adverb-each", "adverb-over-scan", "matrix-reshape", "apply-index", "sum",
			},
			matrix: []string{"matrix:reshape-flip:vector"},
			expr: func(rows int) string {
				return fmt.Sprintf("m:%d 32#til %d;r:m[0 %d];s:(+/)'r;+/s", rows/32, rows, rows/32-1)
			},
			goFn: func(rows int) int64 {
				return qEvalComboGoEachIndexedMatrixRows(rows, 32)
			},
		},
	}
	return cases
}

func TestQEvalVectorComboCaseValues(t *testing.T) {
	eval := qEvalVectorEval(t)
	for _, tc := range qEvalVectorComboCases() {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := qEvalVectorRun(t, eval, tc.expr(qEvalVectorRows))
			if want := tc.goFn(qEvalVectorRows); got != want {
				t.Fatalf("q.eval checksum = %d, want Go baseline %d", got, want)
			}
		})
	}
}
