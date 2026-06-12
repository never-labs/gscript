package benchmarks

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"testing"

	"github.com/never-labs/leia/internal/stdlib/bind"
	stdq "github.com/never-labs/leia/internal/stdlib/lib/q"
)

const qEvalVectorRows = 8192

var qEvalVectorBenchSink int64
var qEvalVectorFloatBenchSink float64
var qEvalVectorAnyBenchSink any
var qEvalVectorGoBaselineInput = buildQEvalVectorGoBaselineInput(qEvalVectorRows + 1024)
var qEvalVectorGoBaselineTakeExtra = 128

func buildQEvalVectorGoBaselineInput(rows int) []int64 {
	values := make([]int64, rows)
	for i := range values {
		values[i] = int64(i)
	}
	return values
}

//go:noinline
func qEvalVectorGoBaselineMaterializeTil(rows int) []int64 {
	values := make([]int64, rows)
	copy(values, qEvalVectorGoBaselineInput[:rows])
	qEvalVectorAnyBenchSink = values
	return values
}

//go:noinline
func qEvalVectorGoBaselineMaterializeReverse(rows int) []int64 {
	values := make([]int64, rows)
	for i := 0; i < rows; i++ {
		values[i] = qEvalVectorGoBaselineInput[rows-1-i]
	}
	qEvalVectorAnyBenchSink = values
	return values
}

//go:noinline
func qEvalVectorGoBaselineMaterializeRotate(rows, shift int) []int64 {
	shift %= rows
	if shift < 0 {
		shift += rows
	}
	values := make([]int64, rows)
	for i := 0; i < rows; i++ {
		values[i] = qEvalVectorGoBaselineInput[(shift+i)%rows]
	}
	qEvalVectorAnyBenchSink = values
	return values
}

//go:noinline
func qEvalVectorGoBaselineMaterializeDrop(rows, drop int) []int64 {
	values := make([]int64, rows-drop)
	copy(values, qEvalVectorGoBaselineInput[drop:rows])
	qEvalVectorAnyBenchSink = values
	return values
}

//go:noinline
func qEvalVectorGoBaselineXrankModuloCount(rows int, mod int64) int64 {
	x := make([]int64, rows)
	for i := 0; i < rows; i++ {
		x[i] = qEvalVectorGoBaselineInput[i] % mod
	}
	idx := make([]int, rows)
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(i, j int) bool { return x[idx[i]] < x[idx[j]] })
	buckets := make([]int64, rows)
	for rank, original := range idx {
		buckets[original] = int64(10 * rank / rows)
	}
	qEvalVectorAnyBenchSink = buckets
	return int64(len(buckets))
}

//go:noinline
func qEvalVectorGoBaselineSumsTailChecksum(rows int, values []int64) int64 {
	var sum int64
	for i := 0; i < rows; i++ {
		sum += values[i]
	}
	return sum + int64(rows)
}

//go:noinline
func qEvalVectorGoBaselineRunningMinMaxTail(rows int, values []int64) int64 {
	if rows <= 0 || rows > len(values) {
		return 0
	}
	minTail := values[0]
	for i := 1; i < rows; i++ {
		if values[i] < minTail {
			minTail = values[i]
		}
	}
	reverseMaxTail := values[rows-1]
	for i := rows - 2; i >= 0; i-- {
		if values[i] > reverseMaxTail {
			reverseMaxTail = values[i]
		}
	}
	maxTail := values[0]
	for i := 1; i < rows; i++ {
		if values[i] > maxTail {
			maxTail = values[i]
		}
	}
	return minTail + reverseMaxTail + maxTail
}

//go:noinline
func qEvalVectorGoBaselineAvgsTail(rows int, values []int64) int64 {
	var sum int64
	for i := 0; i < rows; i++ {
		sum += values[i]
	}
	return int64(float64(sum) / float64(rows))
}

//go:noinline
func qEvalVectorGoBaselineProductOnes(rows int, values []int64) int64 {
	product := int64(1)
	for i := 0; i < rows; i++ {
		if values[i] >= 0 {
			product *= 1
		}
	}
	return product
}

//go:noinline
func qEvalVectorGoBaselineAggregateAvgMedRange(rows int, values []int64) int64 {
	if rows <= 0 || rows > len(values) {
		return 0
	}
	minimum := values[0] + 1
	maximum := minimum
	var sum int64
	for i := 0; i < rows; i++ {
		value := values[i] + 1
		sum += value
		if value < minimum {
			minimum = value
		}
		if value > maximum {
			maximum = value
		}
	}
	average := int64(float64(sum) / float64(rows))
	median := values[rows/2] + 1
	return average + median + minimum + maximum + int64(rows)
}

//go:noinline
func qEvalVectorGoBaselineAggregateProductWavg(rows int, values []int64) int64 {
	product := int64(1)
	for i := 0; i < rows; i++ {
		if values[i] >= 0 {
			product *= 1
		}
	}
	prefixCount := int64(0)
	for i := 0; i < rows; i++ {
		if values[i] >= 0 {
			prefixCount++
		}
	}
	weights := [...]int64{1, 2, 3}
	payload := [...]int64{10, 20, 30}
	var weightSum, weightedSum int64
	for i := range weights {
		weightSum += weights[i]
		weightedSum += weights[i] * payload[i]
	}
	return product + prefixCount + weightedSum/weightSum
}

//go:noinline
func qEvalVectorGoBaselineRunningMinMaxAvgEnvelope(rows int, values []int64) int64 {
	if rows <= 0 || rows > len(values) {
		return 0
	}
	minTail := values[0] + 1
	maxTail := minTail
	var sum int64
	for i := 0; i < rows; i++ {
		value := values[i] + 1
		sum += value
		if value < minTail {
			minTail = value
		}
		if value > maxTail {
			maxTail = value
		}
	}
	averageTail := int64(float64(sum) / float64(rows))
	return sum + minTail + maxTail + averageTail
}

//go:noinline
func qEvalVectorGoBaselineApplyAtGather(rows, width int, values []int64) int64 {
	if rows <= 0 || width*3 >= rows || rows > len(values) {
		return 0
	}
	x := make([]int64, rows)
	copy(x, values[:rows])
	qEvalVectorAnyBenchSink = x
	return x[width] + x[width*2] + x[width*3] + int64(rows)
}

//go:noinline
func qEvalVectorGoBaselineCallableDotApply(rows, width int, values []int64) int64 {
	var sum int64
	for i := 0; i < rows; i++ {
		sum += values[i]
	}
	var count int64
	for i := 0; i < width; i++ {
		if values[i] >= 0 {
			count++
		}
	}
	return sum + count
}

//go:noinline
func qEvalVectorGoBaselineMatrixCellProbe(matrixRows, cols, cellIndex int, values []int64) int64 {
	total := matrixRows * cols
	offset := (matrixRows-1)*cols + cellIndex
	if offset < 0 || total > len(values) || offset >= total {
		return 0
	}
	cells := make([]int64, total)
	copy(cells, values[:total])
	qEvalVectorAnyBenchSink = cells
	return cells[offset] + int64(matrixRows)
}

func qEvalVectorMatrixMmuSide(rows int) int {
	side := rows / 512
	if side < 4 {
		return 4
	}
	if side > 32 {
		return 32
	}
	return side
}

//go:noinline
func qEvalVectorGoBaselineMatrixMmuInv(rows int) int64 {
	side := qEvalVectorMatrixMmuSide(rows)
	n := side * side
	left := make([]float64, n)
	right := make([]float64, n)
	for i := 0; i < n; i++ {
		left[i] = float64(i + 1)
		right[i] = float64(1 + (i % 17))
	}
	var total float64
	for row := 0; row < side; row++ {
		for col := 0; col < side; col++ {
			var cell float64
			for inner := 0; inner < side; inner++ {
				cell += left[row*side+inner] * right[inner*side+col]
			}
			total += cell
		}
	}
	qEvalVectorAnyBenchSink = []any{left, right}
	qEvalVectorFloatBenchSink = total
	return int64(total + 2)
}

func qEvalVectorCrossSide(rows int) int {
	side := rows / 64
	if side < 32 {
		return 32
	}
	if side > 256 {
		return 256
	}
	return side
}

//go:noinline
func qEvalVectorGoBaselineCrossApplyIndex(rows int) int64 {
	side := qEvalVectorCrossSide(rows)
	firstIndex := side + 3
	lastIndex := side*side - 1
	var count int64
	var a, b [2]int64
	for left := 0; left < side; left++ {
		for right := 0; right < side; right++ {
			if int(count) == firstIndex {
				a = [2]int64{int64(left), int64(right)}
			}
			if int(count) == lastIndex {
				b = [2]int64{int64(left), int64(right)}
			}
			count++
		}
	}
	return count + a[0] + a[1] + b[0] + b[1]
}

//go:noinline
func qEvalVectorGoBaselineSortRankTypeMatrix(values []int64) int64 {
	ints := []int64{3, 1, 2, 1}
	ascIdx := sortedIndexI64ForQEvalVectorGoBaseline(ints, true)
	descIdx := sortedIndexI64ForQEvalVectorGoBaseline(ints, false)
	rankVals := rankI64ForQEvalVectorGoBaseline([]int64{2, 7, 3, 2, 5})
	ascVals := append([]int64(nil), []int64{3, 1, 2}...)
	descVals := append([]int64(nil), ascVals...)
	sort.Slice(ascVals, func(i, j int) bool { return ascVals[i] < ascVals[j] })
	sort.Slice(descVals, func(i, j int) bool { return descVals[i] > descVals[j] })
	symbolIdx := sortedIndexStringForQEvalVectorGoBaseline([]string{"MSFT", "AAPL", "AAPL"}, true)
	symbolRank := rankStringForQEvalVectorGoBaseline([]string{"x", "a", "b", "z", "c"})
	dateIdx := sortedIndexI64ForQEvalVectorGoBaseline([]int64{20260607, math.MinInt64, 20260606}, true)
	dateRank := rankI64ForQEvalVectorGoBaseline([]int64{20260607, math.MinInt64, 20260606})
	return sumIntSliceForQEvalVectorGoBaseline(ascIdx) +
		sumIntSliceForQEvalVectorGoBaseline(descIdx) +
		sumIntSliceForQEvalVectorGoBaseline(rankVals) +
		ascVals[0] + descVals[0] +
		sumIntSliceForQEvalVectorGoBaseline(symbolIdx) +
		sumIntSliceForQEvalVectorGoBaseline(symbolRank) +
		sumIntSliceForQEvalVectorGoBaseline(dateIdx) +
		sumIntSliceForQEvalVectorGoBaseline(dateRank) +
		values[0]*0
}

//go:noinline
func qEvalVectorGoBaselineTemporalTypedXbarAndSort(values []int64) int64 {
	dates := []int64{20260606, math.MinInt64, 20260607}
	var dateCount int64
	for range dates {
		dateCount++
	}
	times := []int64{9*60*60 + 30*60, 9*60*60 + 30*60 + 59, 9*60*60 + 31*60}
	var bucketCount int64
	for _, t := range times {
		_ = (t / 60) * 60
		bucketCount++
	}
	var whereCount int64
	for _, d := range []int64{20260606, 20260607} {
		if d >= 20260607 {
			whereCount++
		}
	}
	return dateCount + bucketCount + whereCount + values[0]*0
}

func sortedIndexI64ForQEvalVectorGoBaseline(values []int64, asc bool) []int64 {
	idx := make([]int, len(values))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(i, j int) bool {
		if asc {
			return values[idx[i]] < values[idx[j]]
		}
		return values[idx[i]] > values[idx[j]]
	})
	out := make([]int64, len(idx))
	for i, v := range idx {
		out[i] = int64(v)
	}
	return out
}

func sortedIndexStringForQEvalVectorGoBaseline(values []string, asc bool) []int64 {
	idx := make([]int, len(values))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(i, j int) bool {
		if asc {
			return values[idx[i]] < values[idx[j]]
		}
		return values[idx[i]] > values[idx[j]]
	})
	out := make([]int64, len(idx))
	for i, v := range idx {
		out[i] = int64(v)
	}
	return out
}

func rankI64ForQEvalVectorGoBaseline(values []int64) []int64 {
	idx := sortedIndexI64ForQEvalVectorGoBaseline(values, true)
	out := make([]int64, len(values))
	for rank, original := range idx {
		out[original] = int64(rank)
	}
	return out
}

func rankStringForQEvalVectorGoBaseline(values []string) []int64 {
	idx := sortedIndexStringForQEvalVectorGoBaseline(values, true)
	out := make([]int64, len(values))
	for rank, original := range idx {
		out[original] = int64(rank)
	}
	return out
}

func sumIntSliceForQEvalVectorGoBaseline(values []int64) int64 {
	var sum int64
	for _, value := range values {
		sum += value
	}
	return sum
}

//go:noinline
func qEvalVectorGoBaselineTakeCount(rows int, values []int64) int64 {
	if rows <= 0 || rows > len(values) {
		return 0
	}
	n := rows + qEvalVectorGoBaselineTakeExtra
	taken := make([]int64, n)
	for i := 0; i < n; i++ {
		taken[i] = values[i%rows]
	}
	qEvalVectorAnyBenchSink = taken
	return int64(len(taken))
}

//go:noinline
func qEvalVectorGoBaselineMovingStdDevEmaCount(rows int, alpha float64, width int) int64 {
	values := make([]float64, rows)
	mdev := make([]float64, rows)
	ema := make([]float64, rows)
	var emaPrev float64
	var checksum float64
	for i := 0; i < rows; i++ {
		values[i] = float64(i + 1)
		if i == 0 {
			emaPrev = values[i]
		} else {
			emaPrev = alpha*values[i] + (1-alpha)*emaPrev
		}
		ema[i] = emaPrev
		start := i - width + 1
		if start < 0 {
			start = 0
		}
		n := float64(i - start + 1)
		var sum, sumSq float64
		for j := start; j <= i; j++ {
			sum += values[j]
			sumSq += values[j] * values[j]
		}
		mean := sum / n
		variance := sumSq/n - mean*mean
		if variance < 0 {
			variance = 0
		}
		mdev[i] = math.Sqrt(variance)
		checksum += mdev[i] + ema[i]
	}
	qEvalVectorFloatBenchSink = checksum
	return int64(len(mdev) + len(ema))
}

//go:noinline
func qEvalVectorGoBaselineSampleStatsCorrelationCount(rows int) int64 {
	x := make([]float64, rows)
	y := make([]float64, rows)
	var sumX, sumY, sumYWeighted float64
	for i := 0; i < rows; i++ {
		x[i] = float64(i + 1)
		y[i] = x[i] * 2
		sumX += x[i]
		sumY += y[i]
		sumYWeighted += x[i] * y[i]
	}
	n := float64(rows)
	meanX := sumX / n
	meanY := sumY / n
	var sumDX2, sumDY2, sumDXDY float64
	for i := 0; i < rows; i++ {
		dx := x[i] - meanX
		dy := y[i] - meanY
		sumDX2 += dx * dx
		sumDY2 += dy * dy
		sumDXDY += dx * dy
	}
	svar := sumDX2 / (n - 1)
	sdev := math.Sqrt(svar)
	cov := sumDXDY / n
	scov := sumDXDY / (n - 1)
	cor := sumDXDY / math.Sqrt(sumDX2*sumDY2)
	qEvalVectorFloatBenchSink = svar + sdev + cov + scov + cor + sumYWeighted
	return 6
}

//go:noinline
func qEvalVectorGoBaselineSublistCount(rows int) int64 {
	x := make([]int64, rows)
	for i := range x {
		x[i] = int64(i)
	}
	sublist := x
	if len(sublist) > 100 {
		sublist = sublist[:100]
	}
	return int64(len(sublist))
}

//go:noinline
func qEvalVectorGoBaselineCutCount(rows int) int64 {
	x := make([]int64, rows)
	for i := range x {
		x[i] = int64(i)
	}
	cutPoints := []int{0, 100, 200}
	cutCount := 0
	for i := 0; i < len(cutPoints); i++ {
		start := cutPoints[i]
		end := len(x)
		if i+1 < len(cutPoints) {
			end = cutPoints[i+1]
		}
		if start < 0 {
			start = 0
		}
		if end > len(x) {
			end = len(x)
		}
		if end < start {
			end = start
		}
		qEvalVectorBenchSink += int64(len(x[start:end]))
		cutCount++
	}
	return int64(cutCount)
}

//go:noinline
func qEvalVectorGoBaselineCrossCount(rows int) int64 {
	side := qEvalVectorCrossSide(rows)
	var count int64
	for left := 0; left < side; left++ {
		for right := 0; right < side; right++ {
			count++
		}
	}
	return count
}

//go:noinline
func qEvalVectorGoBaselineStringTrimCount(rows int) int64 {
	pattern := " AAPL "
	spacePadded := make([]byte, rows)
	for i := 0; i < rows; i++ {
		spacePadded[i] = pattern[i%len(pattern)]
	}
	trimmed := strings.TrimSpace(string(spacePadded))
	qEvalVectorFloatBenchSink = float64(len(trimmed))
	return int64(len(trimmed))
}

//go:noinline
func qEvalVectorGoBaselineBooleanWhereLiteralCount() int64 {
	mask := []bool{true, false, true, false, false, true}
	indexes := make([]int64, 0, len(mask))
	for i, value := range mask {
		if value {
			indexes = append(indexes, int64(i))
		}
	}
	qEvalVectorAnyBenchSink = indexes
	return int64(len(indexes))
}

//go:noinline
func qEvalVectorGoBaselineListStringBooleanCount(rows int) int64 {
	return qEvalVectorGoBaselineSublistCount(rows) +
		qEvalVectorGoBaselineCutCount(rows) +
		qEvalVectorGoBaselineCrossCount(rows) +
		qEvalVectorGoBaselineStringTrimCount(rows) +
		qEvalVectorGoBaselineBooleanWhereLiteralCount()
}

type qEvalVectorCase struct {
	name   string
	tags   []string
	matrix []string
	shapes []string
	expr   func(rows int) string
	goFn   func(rows int) int64
}

var qEvalVectorCases = buildQEvalVectorCases()

var qEvalRequiredCoverageTags = []string{
	"numeric-vector",
	"typed-suffix",
	"typed-null",
	"cast",
	"promotion",
	"numeric-monad",
	"word-dyadic",
	"boolean-logical",
	"composite-compare",
	"where",
	"take",
	"drop",
	"cut",
	"enlist",
	"raze",
	"reverse",
	"rotate",
	"sum",
	"sums",
	"min-max",
	"avg-var-dev-med",
	"running-aggregate",
	"product",
	"moving-window",
	"adverb-each",
	"adverb-each-prior",
	"adverb-each-left-right",
	"adverb-over-scan",
	"projection",
	"composition",
	"distinct",
	"group",
	"set-verb",
	"bin-within-xrank",
	"dict",
	"dict-amend-upsert",
	"symbol",
	"enum",
	"string",
	"temporal",
	"xbar",
	"table-literal",
	"keyed-table",
	"metadata",
	"table-reorder",
	"table-sort",
	"table-group-ungroup",
	"fby",
	"membership",
	"find-bin",
	"weighted-aggregate",
	"fill",
	"match",
	"attrs",
	"keys-value",
	"match-like",
	"null-verb",
	"safe-system",
	"ipc-loopback",
	"math-transcendental",
	"matrix-reshape",
	"apply-index",
}

var qEvalRequiredMatrixCoverage = []string{
	"numeric-arithmetic:int-vector:hot",
	"numeric-arithmetic:float-vector:hot",
	"numeric-arithmetic:typed-null:hot",
	"cast:typed-numeric:scalar-vector",
	"compare:int-vector:where",
	"compare:symbol-vector:where",
	"compare:string-vector:where",
	"compare:temporal-vector:where",
	"set:int-vector:union-inter-except",
	"set:symbol-vector:union-inter-except",
	"adverb:dyadic-each:vector",
	"adverb:each-prior:vector",
	"adverb:each-left-right:vector",
	"adverb:over-scan:projection",
	"list:cut-raze-enlist:nested",
	"list:prev-next-deltas-fills:typed-null",
	"aggregate:running-prd-min-max-avg:vector",
	"membership:in-differ-ratios:vector",
	"sort:int-vector:index-rank",
	"sort:symbol-vector:index-rank",
	"sort:temporal-vector:index-rank",
	"search:bin-binr-find:vector",
	"aggregate:wavg-xprev:vector",
	"dict:lookup-amend-upsert:symbol-key",
	"dict:keys-value-attrs:meta",
	"table:literal-meta-cols:frame",
	"table:xcols-xasc-xdesc:frame",
	"table:xkey-xgroup-ungroup:keyed-frame",
	"temporal:xbar:bucket",
	"ipc:loopback:session",
	"math:transcendental-vector:hot",
	"matrix:reshape-flip:vector",
	"apply-index:list-dict-callable:hot",
}

var qEvalRequiredSemanticShapes = []string{
	"verb:exp-reciprocal-signum-not:row-scaled",
	"adverb:initial-over-scan:row-scaled",
	"index:gather-after-where:row-scaled",
	"where:gather-reduce-selectivity:row-scaled",
	"where:compare-count-sum:row-scaled",
	"amend:functional-vector-where:row-scaled",
	"string:symbol-string-like:row-scaled",
	"logical:mask-composition:row-scaled",
	"membership:symbol-filter:row-scaled",
	"sort:index-gather:row-scaled",
	"group:fby-aggregate:row-scaled",
	"moving:sum-avg:row-scaled",
	"null:fill-arithmetic-where:row-scaled",
}

var qEvalRequiredPipelineCategories = []string{
	"xbar_within",
	"where_project_reduce",
	"group_fby",
	"sort_gather",
	"ordinary_list_adverb",
	"vector_numeric",
	"math_transcendental",
	"matrix_reshape",
	"apply_index",
}

func buildQEvalVectorCases() []qEvalVectorCase {
	cases := make([]qEvalVectorCase, 0, 128)

	for _, p := range []struct {
		name string
		mul  int64
		add  int64
	}{
		{"Small", 2, 3},
		{"MarketPrice", 3, 7},
		{"Wide", 5, 11},
		{"SignedOffset", 7, -13},
		{"LargeScale", 17, 101},
		{"OddScale", 31, -29},
	} {
		p := p
		cases = append(cases, qEvalVectorCase{
			name:   "VectorAffineSum" + p.name,
			tags:   []string{"numeric-vector", "sum"},
			matrix: []string{"numeric-arithmetic:int-vector:hot"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;y:(x*%d)+%d;+/y", rows, p.mul, p.add)
			},
			goFn: func(rows int) int64 {
				var sum int64
				for i := int64(0); i < int64(rows); i++ {
					sum += i*p.mul + p.add
				}
				return sum
			},
		})
		cases = append(cases, qEvalVectorCase{
			name:   "VectorSquareSum" + p.name,
			tags:   []string{"numeric-vector", "sum"},
			matrix: []string{"numeric-arithmetic:int-vector:hot"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;y:(x*%d)+%d;+/y*y", rows, p.mul, p.add)
			},
			goFn: func(rows int) int64 {
				var sum int64
				for i := int64(0); i < int64(rows); i++ {
					y := i*p.mul + p.add
					sum += y * y
				}
				return sum
			},
		})
	}

	for _, p := range []struct {
		name string
		mul  int64
		add  int64
		sub  int64
	}{
		{"TwoStageA", 3, 7, 2},
		{"TwoStageB", 9, -5, 4},
		{"TwoStageC", 15, 19, 8},
		{"TwoStageD", 21, -31, 13},
	} {
		p := p
		cases = append(cases, qEvalVectorCase{
			name: "VectorMixedArithmetic" + p.name,
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;y:(x*%d)+%d;z:y-(x*%d);+/z", rows, p.mul, p.add, p.sub)
			},
			goFn: func(rows int) int64 {
				var sum int64
				for i := int64(0); i < int64(rows); i++ {
					y := i*p.mul + p.add
					z := y - i*p.sub
					sum += z
				}
				return sum
			},
		})
	}

	for _, threshold := range []int64{0, 1, 17, 128, 999, 1000, 2048, 4096, 7000, 8191} {
		threshold := threshold
		cases = append(cases, qEvalVectorCase{
			name:   fmt.Sprintf("WhereIndexSumGE%d", threshold),
			matrix: []string{"compare:int-vector:where"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;idx:where x>=%d;+/idx", rows, threshold)
			},
			goFn: func(rows int) int64 {
				var sum int64
				for i := int64(0); i < int64(rows); i++ {
					if i >= threshold {
						sum += i
					}
				}
				return sum
			},
		})
		cases = append(cases, qEvalVectorCase{
			name:   fmt.Sprintf("WhereIndexCountGE%d", threshold),
			matrix: []string{"compare:int-vector:where"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;idx:where x>=%d;count idx", rows, threshold)
			},
			goFn: func(rows int) int64 {
				var count int64
				for i := int64(0); i < int64(rows); i++ {
					if i >= threshold {
						count++
					}
				}
				return count
			},
		})
	}

	for _, p := range []struct {
		name       string
		selectRows func(rows int) int
		expr       func(rows int, selected int) string
	}{
		{"Pct0", func(rows int) int { return 0 }, func(rows int, selected int) string {
			return fmt.Sprintf("x:til %d;y:(x*3)+7;lo:0;hi:%d;idx:where (x>=lo) and x<hi;count y[idx]", rows, selected)
		}},
		{"Pct1", func(rows int) int { return rows / 100 }, func(rows int, selected int) string {
			return fmt.Sprintf("x:til %d;y:(x*3)+7;lo:0;hi:%d;idx:where (x>=lo) and x<hi;+/y[idx]", rows, selected)
		}},
		{"Pct50", func(rows int) int { return rows / 2 }, func(rows int, selected int) string {
			return fmt.Sprintf("x:til %d;y:(x*3)+7;lo:0;hi:%d;idx:where (x>=lo) and x<hi;+/y[idx]", rows, selected)
		}},
		{"Pct99", func(rows int) int { return rows * 99 / 100 }, func(rows int, selected int) string {
			return fmt.Sprintf("x:til %d;y:(x*3)+7;lo:0;hi:%d;idx:where (x>=lo) and x<hi;+/y[idx]", rows, selected)
		}},
		{"Pct100", func(rows int) int { return rows }, func(rows int, selected int) string {
			return fmt.Sprintf("x:til %d;y:(x*3)+7;lo:0;hi:%d;idx:where (x>=lo) and x<hi;+/y[idx]", rows, selected)
		}},
	} {
		p := p
		cases = append(cases, qEvalVectorCase{
			name:   "WhereValueGatherReduceSelectivity" + p.name,
			tags:   []string{"where", "projection", "numeric-vector", "sum", "composite-compare"},
			matrix: []string{"compare:int-vector:where"},
			shapes: []string{"where:gather-reduce-selectivity:row-scaled"},
			expr: func(rows int) string {
				selected := p.selectRows(rows)
				return p.expr(rows, selected)
			},
			goFn: func(rows int) int64 {
				selected := p.selectRows(rows)
				var sum int64
				for i := 0; i < rows; i++ {
					if i >= 0 && i < selected {
						x := int64(i)
						sum += x*3 + 7
					}
				}
				return sum
			},
		})
	}

	for _, n := range []int{1, 8, 64, 128, 1024, 4096} {
		n := n
		cases = append(cases, qEvalVectorCase{
			name: fmt.Sprintf("TakeHead%d", n),
			tags: []string{"take", "sum"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;a:%d#x;+/a", rows, n)
			},
			goFn: func(rows int) int64 {
				x := qEvalVectorGoBaselineMaterializeTil(rows)
				var sum int64
				for i := 0; i < n; i++ {
					sum += x[i%rows]
				}
				return sum
			},
		})
	}

	for _, n := range []int{1, 8, 64, 256, 1024, 4096} {
		n := n
		cases = append(cases, qEvalVectorCase{
			name: fmt.Sprintf("DropPrefix%d", n),
			tags: []string{"drop", "sum"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;a:drop %d x;+/a", rows, n)
			},
			goFn: func(rows int) int64 {
				var sum int64
				for i := n; i < rows; i++ {
					sum += int64(i)
				}
				return sum
			},
		})
	}

	cases = append(cases, qEvalVectorCase{
		name: "ReverseFirstLastChecksum",
		tags: []string{"reverse", "sum"},
		expr: func(rows int) string {
			return fmt.Sprintf("x:til %d;r:reverse x;(first r)+last r+(+/r)", rows)
		},
		goFn: func(rows int) int64 {
			var sum int64
			var first, last int64
			for i := rows - 1; i >= 0; i-- {
				value := int64(i)
				if i == rows-1 {
					first = value
				}
				if i == 0 {
					last = value
				}
				sum += value
			}
			return first + last + sum
		},
	})

	for _, n := range []int{-1024, -257, -1, 1, 257, 1024} {
		n := n
		cases = append(cases, qEvalVectorCase{
			name: fmt.Sprintf("RotateSum%s", qEvalVectorNameInt(n)),
			tags: []string{"rotate", "sum"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;r:%d rotate x;(+/r)+first r+last r", rows, n)
			},
			goFn: func(rows int) int64 {
				shift := n % rows
				if shift < 0 {
					shift += rows
				}
				var sum int64
				var first, last int64
				for i := 0; i < rows; i++ {
					value := int64((shift + i) % rows)
					if i == 0 {
						first = value
					}
					if i == rows-1 {
						last = value
					}
					sum += value
				}
				return sum + first + last
			},
		})
	}

	cases = append(cases,
		qEvalVectorCase{
			name: "AdverbSumScanAndNamedSums",
			tags: []string{"sum", "sums", "adverb-over-scan"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:1+til %d;s:+/x;scan:+\\x;named:sums x;s+last scan+last named", rows)
			},
			goFn: func(rows int) int64 {
				var sum, scanLast, namedLast int64
				for i := int64(1); i <= int64(rows); i++ {
					sum += i
					scanLast += i
				}
				for i := int64(1); i <= int64(rows); i++ {
					namedLast += i
				}
				return sum + scanLast + namedLast
			},
		},
		qEvalVectorCase{
			name: "AdverbDeltasSum",
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;+/deltas x", rows)
			},
			goFn: func(rows int) int64 {
				var sum int64
				var prev int64
				for i := 0; i < rows; i++ {
					value := int64(i)
					if i == 0 {
						sum += value
					} else {
						sum += value - prev
					}
					prev = value
				}
				return sum
			},
		},
		qEvalVectorCase{
			name: "ListReductionsInt",
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;s:+/x;named:sum x;mx:max x;mn:min x;cnt:count x;s+named+mx+mn+cnt", rows)
			},
			goFn: func(rows int) int64 {
				var sum, named int64
				var max int64
				for i := 0; i < rows; i++ {
					value := int64(i)
					sum += value
					if i == 0 || value > max {
						max = value
					}
				}
				for i := 0; i < rows; i++ {
					named += int64(i)
				}
				return sum + named + max + int64(rows)
			},
		},
		qEvalVectorCase{
			name: "CompositionCountDistinctAndReverse",
			tags: []string{"composition", "distinct"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:%d#10 20 10 30;(count distinct)[x]+(first reverse)[til %d]", rows, rows)
			},
			goFn: func(rows int) int64 {
				seen := make(map[int64]struct{}, 4)
				pattern := []int64{10, 20, 10, 30}
				for i := 0; i < rows; i++ {
					seen[pattern[i%len(pattern)]] = struct{}{}
				}
				return int64(len(seen)) + int64(rows-1)
			},
		},
		qEvalVectorCase{
			name: "DictEachCountDistinct",
			expr: func(rows int) string {
				return "d:(count distinct)'`a`b`c!(1 1 2;3 3 3;9 8 9 7);a:d`a;b:d`b;c:d`c;a+b+c"
			},
			goFn: func(rows int) int64 {
				countDistinct := func(values []int64) int64 {
					seen := make(map[int64]struct{}, len(values))
					for _, value := range values {
						seen[value] = struct{}{}
					}
					return int64(len(seen))
				}
				return countDistinct([]int64{1, 1, 2}) +
					countDistinct([]int64{3, 3, 3}) +
					countDistinct([]int64{9, 8, 9, 7})
			},
		},
	)

	for _, width := range []int64{2, 3, 5, 10, 60, 1000} {
		width := width
		cases = append(cases, qEvalVectorCase{
			name: fmt.Sprintf("XbarIntWidth%d", width),
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;b:%d xbar x;+/b", rows, width)
			},
			goFn: func(rows int) int64 {
				var sum int64
				for i := int64(0); i < int64(rows); i++ {
					sum += floorBucket(i, width)
				}
				return sum
			},
		})
	}

	cases = append(cases,
		qEvalVectorCase{
			name:   "FloatVectorArithmetic",
			matrix: []string{"numeric-arithmetic:float-vector:hot"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;y:(x*0.5)+1.5;+/y", rows)
			},
			goFn: func(rows int) int64 {
				var sum float64
				for i := 0; i < rows; i++ {
					sum += float64(i)*0.5 + 1.5
				}
				return int64(sum)
			},
		},
		qEvalVectorCase{
			name: "FloatXbarCount",
			expr: func(rows int) string {
				return fmt.Sprintf("x:(til %d)*0.5;count 0.5 xbar x", rows)
			},
			goFn: func(rows int) int64 {
				var count int64
				for i := 0; i < rows; i++ {
					_ = floorBucketFloat(float64(i)*0.5, 0.5)
					count++
				}
				return count
			},
		},
		qEvalVectorCase{
			name: "SymbolDistinctCount",
			expr: func(rows int) string {
				return fmt.Sprintf("count distinct %d#`AAPL`MSFT`AAPL`NVDA`MSFT`TSLA", rows)
			},
			goFn: func(rows int) int64 {
				pattern := []string{"AAPL", "MSFT", "AAPL", "NVDA", "MSFT", "TSLA"}
				seen := make(map[string]struct{}, len(pattern))
				for i := 0; i < rows; i++ {
					seen[pattern[i%len(pattern)]] = struct{}{}
				}
				return int64(len(seen))
			},
		},
		qEvalVectorCase{
			name: "SymbolRotateDistinctCount",
			expr: func(rows int) string {
				return fmt.Sprintf("count distinct -1 rotate %d#`AAPL`MSFT`NVDA`AAPL`TSLA", rows)
			},
			goFn: func(rows int) int64 {
				pattern := []string{"AAPL", "MSFT", "NVDA", "AAPL", "TSLA"}
				seen := make(map[string]struct{}, len(pattern))
				shift := rows - 1
				for i := 0; i < rows; i++ {
					seen[pattern[(shift+i)%rows%len(pattern)]] = struct{}{}
				}
				return int64(len(seen))
			},
		},
		qEvalVectorCase{
			name:   "TemporalXbarCount",
			matrix: []string{"temporal:xbar:bucket"},
			expr: func(rows int) string {
				return "count 00:01 xbar 09:30 09:30:59 09:31:00 09:31:30"
			},
			goFn: func(rows int) int64 {
				return 4
			},
		},
		qEvalVectorCase{
			name: "TimestampXbarCount",
			expr: func(rows int) string {
				return "count 0D00:01:00 xbar 2026.06.06D09:30:15 2026.06.06D09:31:45 2026.06.06D09:32:01"
			},
			goFn: func(rows int) int64 {
				return 3
			},
		},
		qEvalVectorCase{
			name: "TableLiteralCount",
			expr: func(rows int) string {
				return fmt.Sprintf("syms:%d#`AAPL`MSFT`NVDA;price:til %d;size:1+til %d;count flip `sym`price`size!(syms;price;size)", rows, rows, rows)
			},
			goFn: func(rows int) int64 {
				syms := make([]string, rows)
				price := make([]int64, rows)
				size := make([]int64, rows)
				pattern := []string{"AAPL", "MSFT", "NVDA"}
				for i := 0; i < rows; i++ {
					syms[i] = pattern[i%len(pattern)]
					price[i] = int64(i)
					size[i] = int64(i + 1)
				}
				if len(syms) != len(price) || len(price) != len(size) {
					return 0
				}
				return int64(len(price))
			},
		},
		qEvalVectorCase{
			name: "XbarStaticMixedSignCount",
			expr: func(rows int) string {
				return fmt.Sprintf("x:(til %d)-%d;count 10 xbar x", rows, rows/2)
			},
			goFn: func(rows int) int64 {
				var count int64
				offset := int64(rows / 2)
				for i := 0; i < rows; i++ {
					_ = floorBucket(int64(i)-offset, 10)
					count++
				}
				return count
			},
		},
	)

	cases = append(cases,
		qEvalVectorCase{
			name:   "NumericMonadExpReciprocalSignumNot",
			tags:   []string{"numeric-monad", "numeric-vector", "sum", "where"},
			matrix: []string{"numeric-arithmetic:float-vector:hot"},
			shapes: []string{"verb:exp-reciprocal-signum-not:row-scaled"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:%d#0 1 2 3;r:x mod 2;(+/exp x)+(+/reciprocal 1+x)+(+/signum x-2)+(count where not r)", rows)
			},
			goFn: func(rows int) int64 {
				var total float64
				var signum, notCount int64
				for i := 0; i < rows; i++ {
					x := int64(i % 4)
					total += math.Exp(float64(x))
					total += 1 / float64(1+x)
					switch {
					case x-2 < 0:
						signum--
					case x-2 > 0:
						signum++
					}
					if x%2 == 0 {
						notCount++
					}
				}
				return int64(total) + signum + notCount
			},
		},
		qEvalVectorCase{
			name:   "AdverbInitialOverScanProducts",
			tags:   []string{"adverb-over-scan", "sum", "sums", "product", "pipeline-vector-last-scan"},
			matrix: []string{"adverb:over-scan:projection", "aggregate:running-prd-min-max-avg:vector"},
			shapes: []string{"adverb:initial-over-scan:row-scaled"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:1+til %d;p:%d#1;({x+y}/[10;x])+(last ({x+y}\\[10;x]))+(prd p)+(last prds p)", rows, rows)
			},
			goFn: func(rows int) int64 {
				var sum int64 = 10
				for i := 1; i <= rows; i++ {
					sum += int64(i)
				}
				return sum + sum + 1 + 1
			},
		},
		qEvalVectorCase{
			name:   "WhereGatherProjectionSum",
			tags:   []string{"where", "projection", "numeric-vector", "sum"},
			matrix: []string{"compare:int-vector:where"},
			shapes: []string{"index:gather-after-where:row-scaled"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;idx:where x>=%d;y:x[idx];(+/y)+first y+last y+count idx", rows, rows/2)
			},
			goFn: func(rows int) int64 {
				start := rows / 2
				var sum int64
				for i := start; i < rows; i++ {
					sum += int64(i)
				}
				return sum + int64(start) + int64(rows-1) + int64(rows-start)
			},
		},
		qEvalVectorCase{
			name:   "WhereCompareCountSumDirect",
			tags:   []string{"where", "numeric-vector", "sum"},
			matrix: []string{"compare:int-vector:where"},
			shapes: []string{"where:compare-count-sum:row-scaled"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;(count where x>=%d)+(+/where x>=%d)", rows, rows/2, rows/2)
			},
			goFn: func(rows int) int64 {
				start := rows / 2
				var sum, count int64
				for i := start; i < rows; i++ {
					count++
					sum += int64(i)
				}
				return count + sum
			},
		},
		qEvalVectorCase{
			name:   "FunctionalAmendWhereVector",
			tags:   []string{"dict-amend-upsert", "where", "numeric-vector", "sum"},
			matrix: []string{"compare:int-vector:where"},
			shapes: []string{"amend:functional-vector-where:row-scaled"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:%d#0;idx:where (til %d)<128;y:@[x;idx;+;128#1];(+/y)+count idx", rows, rows)
			},
			goFn: func(rows int) int64 {
				var sum, count int64
				for i := 0; i < rows; i++ {
					if i < 128 {
						sum++
						count++
					}
				}
				return sum + count
			},
		},
		qEvalVectorCase{
			name:   "SymbolStringLikeRowScaled",
			tags:   []string{"symbol", "string", "match-like", "where"},
			matrix: []string{"compare:symbol-vector:where", "compare:string-vector:where"},
			shapes: []string{"string:symbol-string-like:row-scaled"},
			expr: func(rows int) string {
				return fmt.Sprintf("syms:%d#`aapl`msft`amd`ask;names:upper string syms;(count where names like \"A*\")+count reverse names", rows)
			},
			goFn: func(rows int) int64 {
				var aCount int64
				for i := 0; i < rows; i++ {
					switch i % 4 {
					case 0, 2, 3:
						aCount++
					}
				}
				return aCount + int64(rows)
			},
		},
		qEvalVectorCase{
			name:   "LogicalMaskFilterRowScaled",
			tags:   []string{"boolean-logical", "composite-compare", "where", "numeric-vector"},
			matrix: []string{"compare:int-vector:where"},
			shapes: []string{"logical:mask-composition:row-scaled"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;lo:%d;hi:%d;m:(x>=lo) and x<hi;count where m", rows, rows/4, rows*3/4)
			},
			goFn: func(rows int) int64 {
				lo := rows / 4
				hi := rows * 3 / 4
				var count int64
				for i := 0; i < rows; i++ {
					if i >= lo && i < hi {
						count++
					}
				}
				return count
			},
		},
		qEvalVectorCase{
			name:   "MembershipSymbolFilterRowScaled",
			tags:   []string{"membership", "symbol", "where"},
			matrix: []string{"membership:in-differ-ratios:vector", "compare:symbol-vector:where"},
			shapes: []string{"membership:symbol-filter:row-scaled"},
			expr: func(rows int) string {
				return fmt.Sprintf("syms:%d#`AAPL`MSFT`NVDA`TSLA;count where syms in `AAPL`MSFT", rows)
			},
			goFn: func(rows int) int64 {
				var count int64
				for i := 0; i < rows; i++ {
					if i%4 == 0 || i%4 == 1 {
						count++
					}
				}
				return count
			},
		},
		qEvalVectorCase{
			name:   "SortIndexGatherRowScaled",
			tags:   []string{"table-sort", "projection", "sum"},
			matrix: []string{"sort:int-vector:index-rank"},
			shapes: []string{"sort:index-gather:row-scaled"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:reverse til %d;idx:iasc x;y:x[idx];f:first y;l:last y;f+l+count y", rows)
			},
			goFn: func(rows int) int64 {
				x := make([]int64, rows)
				idx := make([]int, rows)
				for i := 0; i < rows; i++ {
					x[i] = int64(rows - 1 - i)
					idx[i] = i
				}
				sort.Slice(idx, func(i, j int) bool {
					return x[idx[i]] < x[idx[j]]
				})
				first := x[idx[0]]
				last := x[idx[len(idx)-1]]
				return first + last + int64(len(idx))
			},
		},
		qEvalVectorCase{
			name:   "FbyGroupedAggregateRowScaled",
			tags:   []string{"fby", "group", "sum", "symbol"},
			matrix: []string{"aggregate:running-prd-min-max-avg:vector"},
			shapes: []string{"group:fby-aggregate:row-scaled"},
			expr: func(rows int) string {
				return fmt.Sprintf("v:til %d;g:%d#`a`b`c`d;s:sum v fby g;+/s", rows, rows)
			},
			goFn: func(rows int) int64 {
				groupSums := [4]int64{}
				groupCounts := [4]int64{}
				for i := 0; i < rows; i++ {
					group := i % 4
					groupSums[group] += int64(i)
					groupCounts[group]++
				}
				var total int64
				for group := 0; group < 4; group++ {
					total += groupSums[group] * groupCounts[group]
				}
				return total
			},
		},
		qEvalVectorCase{
			name:   "FbyCountTerminalProjection",
			tags:   []string{"fby", "group", "sum", "symbol"},
			matrix: []string{"aggregate:running-prd-min-max-avg:vector"},
			shapes: []string{"group:fby-aggregate:row-scaled"},
			expr: func(rows int) string {
				return fmt.Sprintf("v:til %d;g:%d#`a`b`c`d;count (sum v fby g)", rows, rows)
			},
			goFn: func(rows int) int64 {
				var groupSums [4]int64
				for i := 0; i < rows; i++ {
					groupSums[i%4] += qEvalVectorGoBaselineInput[i]
				}
				perRow := make([]int64, rows)
				for i := 0; i < rows; i++ {
					perRow[i] = groupSums[i%4]
				}
				qEvalVectorAnyBenchSink = perRow
				return int64(len(perRow))
			},
		},
		qEvalVectorCase{
			name:   "MovingSumAvgRowScaled",
			tags:   []string{"moving-window", "sum", "avg-var-dev-med"},
			matrix: []string{"aggregate:running-prd-min-max-avg:vector"},
			shapes: []string{"moving:sum-avg:row-scaled"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:1+til %d;(+/20 msum x)+(+/20 mavg x)", rows)
			},
			goFn: func(rows int) int64 {
				var sumWindow int64
				var avgWindow float64
				for i := 0; i < rows; i++ {
					start := i - 19
					if start < 0 {
						start = 0
					}
					var window int64
					for row := start; row <= i; row++ {
						window += int64(row + 1)
					}
					sumWindow += window
					avgWindow += float64(window) / float64(i-start+1)
				}
				return sumWindow + int64(avgWindow)
			},
		},
		qEvalVectorCase{
			name:   "TypedNullFillWhereRowScaled",
			tags:   []string{"typed-null", "fill", "null-verb", "where", "sum"},
			matrix: []string{"numeric-arithmetic:typed-null:hot", "list:prev-next-deltas-fills:typed-null"},
			shapes: []string{"null:fill-arithmetic-where:row-scaled"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:%d#1 0Ni 3 0Ni;y:0^x;(+/y)+count where null x", rows)
			},
			goFn: func(rows int) int64 {
				var sum, nullCount int64
				for i := 0; i < rows; i++ {
					switch i % 4 {
					case 0:
						sum += 1
					case 1, 3:
						nullCount++
					case 2:
						sum += 3
					}
				}
				return sum + nullCount
			},
		},
	)

	cases = appendQEvalExpressionCombinationCases(cases)
	cases = appendQEvalOrdinaryExpressionCoverageCases(cases)
	cases = appendQEvalTaskDListMathMatrixApplyIndexCases(cases)
	cases = appendQEvalTaskDColumnarAnalyticsWorkloadCases(cases)
	cases = appendQEvalTaskDSemanticEnvelopeCases(cases)
	cases = appendQEvalSupportedExpressionBreadthCases(cases)
	return appendQEvalSemanticCoverageCases(cases)
}

func appendQEvalExpressionCombinationCases(cases []qEvalVectorCase) []qEvalVectorCase {
	combos := []qEvalVectorCase{
		{
			name:   "VectorModuloBucketSum",
			tags:   []string{"numeric-vector", "word-dyadic", "sum"},
			matrix: []string{"numeric-arithmetic:int-vector:hot"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;+/x mod 17", rows)
			},
			goFn: func(rows int) int64 {
				var sum int64
				for i := 0; i < rows; i++ {
					sum += int64(i % 17)
				}
				return sum
			},
		},
		{
			name:   "VectorMinMaxDyadicEnvelope",
			tags:   []string{"numeric-vector", "min-max", "sum"},
			matrix: []string{"numeric-arithmetic:int-vector:hot", "aggregate:running-prd-min-max-avg:vector"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;y:reverse x;(+/x min y)+(+/x max y)", rows)
			},
			goFn: func(rows int) int64 {
				var sum int64
				for i := 0; i < rows; i++ {
					x := int64(i)
					y := int64(rows - 1 - i)
					if x < y {
						sum += x + y
					} else {
						sum += y + x
					}
				}
				return sum
			},
		},
		{
			name:   "ModuloEqMaskCount",
			tags:   []string{"where", "composite-compare", "word-dyadic"},
			matrix: []string{"compare:int-vector:where"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;count where (x mod 4)=2", rows)
			},
			goFn: func(rows int) int64 {
				var count int64
				for i := 0; i < rows; i++ {
					if i%4 == 2 {
						count++
					}
				}
				return count
			},
		},
		{
			name:   "ModuloNeMaskCount",
			tags:   []string{"where", "composite-compare", "word-dyadic"},
			matrix: []string{"compare:int-vector:where"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;count where (x mod 5)<>3", rows)
			},
			goFn: func(rows int) int64 {
				var count int64
				for i := 0; i < rows; i++ {
					if i%5 != 3 {
						count++
					}
				}
				return count
			},
		},
		{
			name:   "ModuloBandWhereIndexSum",
			tags:   []string{"where", "boolean-logical", "composite-compare", "word-dyadic"},
			matrix: []string{"compare:int-vector:where"},
			shapes: []string{"logical:mask-composition:row-scaled"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;m:x mod 10;idx:where (m>=3) and m<8;+/idx", rows)
			},
			goFn: func(rows int) int64 {
				var sum int64
				for i := 0; i < rows; i++ {
					m := i % 10
					if m >= 3 && m < 8 {
						sum += int64(i)
					}
				}
				return sum
			},
		},
		{
			name:   "WhereModuloGatherProjectionSum",
			tags:   []string{"where", "projection", "numeric-vector", "sum", "word-dyadic"},
			matrix: []string{"compare:int-vector:where"},
			shapes: []string{"index:gather-after-where:row-scaled"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;y:(x*2)+1;idx:where (x mod 3)=0;+/y[idx]", rows)
			},
			goFn: func(rows int) int64 {
				var sum int64
				for i := 0; i < rows; i++ {
					if i%3 == 0 {
						sum += int64(i*2 + 1)
					}
				}
				return sum
			},
		},
		{
			name:   "WhereModuloOrMaskCount",
			tags:   []string{"where", "boolean-logical", "word-dyadic"},
			matrix: []string{"compare:int-vector:where"},
			shapes: []string{"logical:mask-composition:row-scaled"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;count where ((x mod 7)=0) or (x mod 11)=0", rows)
			},
			goFn: func(rows int) int64 {
				var count int64
				for i := 0; i < rows; i++ {
					if i%7 == 0 || i%11 == 0 {
						count++
					}
				}
				return count
			},
		},
		{
			name: "TakeCycleBeyondLengthSum",
			tags: []string{"take", "sum"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;a:%d#x;+/a", rows, rows+128)
			},
			goFn: func(rows int) int64 {
				var sum int64
				for i := 0; i < rows+128; i++ {
					sum += int64(i % rows)
				}
				return sum
			},
		},
		{
			name:   "DirectTakeCycleSum",
			tags:   []string{"take", "sum", "numeric-vector"},
			matrix: []string{"numeric-arithmetic:int-vector:hot"},
			shapes: []string{"pipeline:vector-transform-reduce:sum"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;sum %d#x", rows, rows+128)
			},
			goFn: func(rows int) int64 {
				var sum int64
				for i := 0; i < rows+128; i++ {
					sum += int64(i % rows)
				}
				return sum
			},
		},
		{
			name:   "DirectDropPrefixSum",
			tags:   []string{"drop", "sum", "numeric-vector"},
			matrix: []string{"numeric-arithmetic:int-vector:hot"},
			shapes: []string{"pipeline:vector-transform-reduce:sum"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;sum drop 128 x", rows)
			},
			goFn: func(rows int) int64 {
				var sum int64
				for i := 128; i < rows; i++ {
					sum += int64(i)
				}
				return sum
			},
		},
		{
			name:   "DirectReverseSum",
			tags:   []string{"reverse", "sum", "numeric-vector"},
			matrix: []string{"numeric-arithmetic:int-vector:hot"},
			shapes: []string{"pipeline:vector-transform-reduce:sum"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;sum reverse x", rows)
			},
			goFn: func(rows int) int64 {
				var sum int64
				for i := rows - 1; i >= 0; i-- {
					sum += qEvalVectorGoBaselineInput[i]
				}
				return sum
			},
		},
		{
			name:   "DirectRotateSum",
			tags:   []string{"rotate", "sum", "numeric-vector"},
			matrix: []string{"numeric-arithmetic:int-vector:hot"},
			shapes: []string{"pipeline:vector-transform-reduce:sum"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;sum 257 rotate x", rows)
			},
			goFn: func(rows int) int64 {
				shift := 257 % rows
				var sum int64
				for i := 0; i < rows; i++ {
					sum += qEvalVectorGoBaselineInput[(shift+i)%rows]
				}
				return sum
			},
		},
		{
			name:   "DirectTakeCount",
			tags:   []string{"take", "count", "numeric-vector"},
			matrix: []string{"numeric-arithmetic:int-vector:hot"},
			shapes: []string{"pipeline:vector-transform-count"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;count %d#x", rows, rows+128)
			},
			goFn: func(rows int) int64 {
				return qEvalVectorGoBaselineTakeCount(rows, qEvalVectorGoBaselineInput)
			},
		},
		{
			name: "ReverseWhereGatherHeadSum",
			tags: []string{"reverse", "where", "projection", "sum"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;r:reverse x;idx:where r<128;+/r[idx]", rows)
			},
			goFn: func(rows int) int64 {
				var sum int64
				for i := rows - 1; i >= 0; i-- {
					value := qEvalVectorGoBaselineInput[i]
					if value < 128 {
						sum += value
					}
				}
				return sum
			},
		},
		{
			name: "RotateWhereHeadCount",
			tags: []string{"rotate", "where", "composite-compare"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;r:3 rotate x;count where r<10", rows)
			},
			goFn: func(rows int) int64 {
				var count int64
				for i := 0; i < rows; i++ {
					if (3+i)%rows < 10 {
						count++
					}
				}
				return count
			},
		},
		{
			name: "SumsTailChecksum",
			tags: []string{"sums", "adverb-over-scan", "sum"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;s:sums x;last s+count s", rows)
			},
			goFn: func(rows int) int64 {
				return qEvalVectorGoBaselineSumsTailChecksum(rows, qEvalVectorGoBaselineInput)
			},
		},
		{
			name: "ScanPlusTailChecksum",
			tags: []string{"adverb-over-scan", "sum"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;s:+\\x;last s+count s", rows)
			},
			goFn: func(rows int) int64 {
				return qEvalVectorGoBaselineSumsTailChecksum(rows, qEvalVectorGoBaselineInput)
			},
		},
		{
			name:   "RunningMinMaxTailEnvelope",
			tags:   []string{"running-aggregate", "min-max", "pipeline-vector-last-scan"},
			matrix: []string{"aggregate:running-prd-min-max-avg:vector"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;(last mins x)+(last maxs reverse x)+(last maxs x)", rows)
			},
			goFn: func(rows int) int64 {
				return qEvalVectorGoBaselineRunningMinMaxTail(rows, qEvalVectorGoBaselineInput)
			},
		},
		{
			name:   "AvgsTailChecksum",
			tags:   []string{"running-aggregate", "avg-var-dev-med", "pipeline-vector-last-scan"},
			matrix: []string{"aggregate:running-prd-min-max-avg:vector"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;last avgs x", rows)
			},
			goFn: func(rows int) int64 {
				return qEvalVectorGoBaselineAvgsTail(rows, qEvalVectorGoBaselineInput)
			},
		},
		{
			name: "ProductOnesRowScaled",
			tags: []string{"product", "running-aggregate"},
			expr: func(rows int) string {
				return fmt.Sprintf("prd %d#1", rows)
			},
			goFn: func(rows int) int64 {
				return qEvalVectorGoBaselineProductOnes(rows, qEvalVectorGoBaselineInput)
			},
		},
		{
			name: "MovingCount32Sum",
			tags: []string{"moving-window"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;+/32 mcount x", rows)
			},
			goFn: func(rows int) int64 {
				var sum int64
				for i := 0; i < rows; i++ {
					window := i + 1
					if window > 32 {
						window = 32
					}
					sum += int64(window)
				}
				return sum
			},
		},
		{
			name: "MovingMinMax32Envelope",
			tags: []string{"moving-window", "min-max"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:1+til %d;(+/32 mmin x)+(+/32 mmax x)", rows)
			},
			goFn: func(rows int) int64 {
				var sum int64
				for i := 0; i < rows; i++ {
					window := i + 1
					if window > 32 {
						window = 32
					}
					sum += int64(i-window+2) + int64(i+1)
				}
				return sum
			},
		},
		{
			name:   "MovingStdDevEma32Envelope",
			tags:   []string{"moving-window", "avg-var-dev-med", "numeric-vector"},
			matrix: []string{"aggregate:mdev-ema:vector"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:1+til %d;(count 32 mdev x)+(count 0.25 ema x)", rows)
			},
			goFn: func(rows int) int64 {
				return qEvalVectorGoBaselineMovingStdDevEmaCount(rows, 0.25, 32)
			},
		},
		{
			name:   "SampleStatsCorrelationEnvelope",
			tags:   []string{"avg-var-dev-med", "weighted-aggregate", "numeric-vector"},
			matrix: []string{"aggregate:sample-cov-cor-wsum:vector"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:1+til %d;y:x*2;count (svar x;sdev x;x cov y;x scov y;x cor y;x wsum y)", rows)
			},
			goFn: func(rows int) int64 {
				return qEvalVectorGoBaselineSampleStatsCorrelationCount(rows)
			},
		},
		{
			name:   "ListStringBooleanCoverageEnvelope",
			tags:   []string{"sublist", "cut", "cross", "string", "boolean-logical", "projection"},
			matrix: []string{"list:sublist-cross-cut:string-bool"},
			expr: func(rows int) string {
				side := qEvalVectorCrossSide(rows)
				return fmt.Sprintf("x:til %d;(count 100 sublist x)+(count 0 100 200 cut x)+(count (til %d) cross til %d)+(count trim %d#\" AAPL \")+(count where 101001b)", rows, side, side, rows)
			},
			goFn: func(rows int) int64 {
				return qEvalVectorGoBaselineListStringBooleanCount(rows)
			},
		},
		{
			name:   "ListSublistCountEnvelope",
			tags:   []string{"cut", "projection"},
			matrix: []string{"list:sublist-cross-cut:string-bool"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;count 100 sublist x", rows)
			},
			goFn: func(rows int) int64 {
				return qEvalVectorGoBaselineSublistCount(rows)
			},
		},
		{
			name:   "ListCutCountEnvelope",
			tags:   []string{"cut", "projection"},
			matrix: []string{"list:sublist-cross-cut:string-bool"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;count 0 100 200 cut x", rows)
			},
			goFn: func(rows int) int64 {
				return qEvalVectorGoBaselineCutCount(rows)
			},
		},
		{
			name:   "ListCrossCountEnvelope",
			tags:   []string{"cross", "projection"},
			matrix: []string{"list:sublist-cross-cut:string-bool"},
			expr: func(rows int) string {
				side := qEvalVectorCrossSide(rows)
				return fmt.Sprintf("count (til %d) cross til %d", side, side)
			},
			goFn: func(rows int) int64 {
				return qEvalVectorGoBaselineCrossCount(rows)
			},
		},
		{
			name:   "StringTrimCountEnvelope",
			tags:   []string{"string", "projection"},
			matrix: []string{"list:sublist-cross-cut:string-bool"},
			expr: func(rows int) string {
				return fmt.Sprintf("count trim %d#\" AAPL \"", rows)
			},
			goFn: func(rows int) int64 {
				return qEvalVectorGoBaselineStringTrimCount(rows)
			},
		},
		{
			name:   "BooleanWhereLiteralCountEnvelope",
			tags:   []string{"boolean-logical", "where", "projection"},
			matrix: []string{"list:sublist-cross-cut:string-bool"},
			expr: func(rows int) string {
				return "count where 101001b"
			},
			goFn: func(rows int) int64 {
				return qEvalVectorGoBaselineBooleanWhereLiteralCount()
			},
		},
		{
			name:   "SymbolEqualityMaskCount",
			tags:   []string{"symbol", "where", "composite-compare"},
			matrix: []string{"compare:symbol-vector:where"},
			expr: func(rows int) string {
				return fmt.Sprintf("syms:%d#`AAPL`MSFT`NVDA`AAPL`AMD;count where syms=`AAPL", rows)
			},
			goFn: func(rows int) int64 {
				var count int64
				for i := 0; i < rows; i++ {
					if i%5 == 0 || i%5 == 3 {
						count++
					}
				}
				return count
			},
		},
		{
			name:   "SymbolNotEqualMaskCount",
			tags:   []string{"symbol", "where", "composite-compare"},
			matrix: []string{"compare:symbol-vector:where"},
			expr: func(rows int) string {
				return fmt.Sprintf("syms:%d#`AAPL`MSFT`NVDA`AAPL`AMD;count where syms<>`AAPL", rows)
			},
			goFn: func(rows int) int64 {
				var count int64
				for i := 0; i < rows; i++ {
					if i%5 != 0 && i%5 != 3 {
						count++
					}
				}
				return count
			},
		},
		{
			name:   "StringUpperSymbolCount",
			tags:   []string{"string", "symbol"},
			matrix: []string{"compare:string-vector:where"},
			expr: func(rows int) string {
				return fmt.Sprintf("syms:%d#`aapl`msft`nvda;count upper string syms", rows)
			},
			goFn: func(rows int) int64 {
				pattern := []string{"aapl", "msft", "nvda"}
				names := make([]string, rows)
				for i := 0; i < rows; i++ {
					names[i] = strings.ToUpper(pattern[i%len(pattern)])
				}
				qEvalVectorAnyBenchSink = names
				return int64(len(names))
			},
		},
		{
			name:   "TemporalDateCompareMaskCount",
			tags:   []string{"temporal", "where", "composite-compare"},
			matrix: []string{"compare:temporal-vector:where"},
			expr: func(rows int) string {
				return fmt.Sprintf("dates:%d#2026.06.06 2026.06.07 2026.06.08;count where dates>=2026.06.07", rows)
			},
			goFn: func(rows int) int64 {
				var count int64
				for i := 0; i < rows; i++ {
					if i%3 != 0 {
						count++
					}
				}
				return count
			},
		},
		{
			name: "TemporalTimeCompareMaskCount",
			tags: []string{"temporal", "where", "composite-compare"},
			expr: func(rows int) string {
				return fmt.Sprintf("times:%d#09:30 09:31 09:32 09:33;count where times within 09:31 09:32", rows)
			},
			goFn: func(rows int) int64 {
				var count int64
				for i := 0; i < rows; i++ {
					if i%4 == 1 || i%4 == 2 {
						count++
					}
				}
				return count
			},
		},
		{
			name:   "TypedIntCastRoundTripSum",
			tags:   []string{"typed-suffix", "cast", "promotion", "sum"},
			matrix: []string{"cast:typed-numeric:scalar-vector"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;+/`long$`int$x", rows)
			},
			goFn: func(rows int) int64 {
				var sum int64
				for i := 0; i < rows; i++ {
					sum += int64(int32(qEvalVectorGoBaselineInput[i]))
				}
				return sum
			},
		},
		{
			name:   "FloatCastCount",
			tags:   []string{"cast", "promotion", "numeric-vector", "sum"},
			matrix: []string{"cast:typed-numeric:scalar-vector", "numeric-arithmetic:float-vector:hot"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:(til %d)*0.5;count `float$x", rows)
			},
			goFn: func(rows int) int64 {
				floats := make([]float64, rows)
				for i := 0; i < rows; i++ {
					floats[i] = float64(qEvalVectorGoBaselineInput[i]) * 0.5
				}
				qEvalVectorAnyBenchSink = floats
				return int64(len(floats))
			},
		},
		{
			name:   "TypedNullFillSum",
			tags:   []string{"typed-null", "fill", "null-verb", "sum"},
			matrix: []string{"numeric-arithmetic:typed-null:hot", "list:prev-next-deltas-fills:typed-null"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:%d#0N 1 2 0N;y:99^x;+/y", rows)
			},
			goFn: func(rows int) int64 {
				var sum int64
				for i := 0; i < rows; i++ {
					switch i % 4 {
					case 0, 3:
						sum += 99
					case 1:
						sum++
					case 2:
						sum += 2
					}
				}
				return sum
			},
		},
		{
			name:   "PrevNextDeltasCountsRowScaled",
			tags:   []string{"adverb-each-prior", "typed-null"},
			matrix: []string{"list:prev-next-deltas-fills:typed-null"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;(count prev x)+(count next x)+(count deltas x)", rows)
			},
			goFn: func(rows int) int64 {
				prev := make([]int64, rows)
				next := make([]int64, rows)
				deltas := make([]int64, rows)
				for i := 0; i < rows; i++ {
					value := qEvalVectorGoBaselineInput[i]
					if i == 0 {
						deltas[i] = value
					} else {
						prev[i] = qEvalVectorGoBaselineInput[i-1]
						deltas[i] = value - qEvalVectorGoBaselineInput[i-1]
					}
					if i+1 < rows {
						next[i] = qEvalVectorGoBaselineInput[i+1]
					}
				}
				qEvalVectorAnyBenchSink = [][]int64{prev, next, deltas}
				return int64(len(prev) + len(next) + len(deltas))
			},
		},
		{
			name:   "FillsNullPatternSum",
			tags:   []string{"fill", "typed-null", "sum"},
			matrix: []string{"list:prev-next-deltas-fills:typed-null"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:%d#0N 1 0N 2;+/fills x", rows)
			},
			goFn: func(rows int) int64 {
				var sum, last int64
				for i := 0; i < rows; i++ {
					switch i % 4 {
					case 1:
						last = 1
					case 3:
						last = 2
					}
					sum += last
				}
				return sum
			},
		},
		{
			name:   "EachPriorMinusRowScaled",
			tags:   []string{"adverb-each-prior", "sum"},
			matrix: []string{"adverb:each-prior:vector"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;+/ -':x", rows)
			},
			goFn: func(rows int) int64 {
				var sum, prev int64
				for i := 0; i < rows; i++ {
					value := qEvalVectorGoBaselineInput[i]
					if i == 0 {
						sum += value
					} else {
						sum += value - prev
					}
					prev = value
				}
				return sum
			},
		},
		{
			name:   "DistinctModuloCount",
			tags:   []string{"distinct", "word-dyadic"},
			matrix: []string{"set:int-vector:union-inter-except"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;count distinct x mod 257", rows)
			},
			goFn: func(rows int) int64 {
				var seen [257]bool
				var count int64
				for i := 0; i < rows; i++ {
					m := qEvalVectorGoBaselineInput[i] % 257
					if !seen[m] {
						seen[m] = true
						count++
					}
				}
				return count
			},
		},
		{
			name:   "GroupModuloBucketCount",
			tags:   []string{"group", "word-dyadic"},
			matrix: []string{"aggregate:running-prd-min-max-avg:vector"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;count group x mod 64", rows)
			},
			goFn: func(rows int) int64 {
				var seen [64]bool
				var buckets int64
				for i := 0; i < rows; i++ {
					m := qEvalVectorGoBaselineInput[i] % 64
					if !seen[m] {
						seen[m] = true
						buckets++
					}
				}
				return buckets
			},
		},
		{
			name:   "FindModuloPatternSum",
			tags:   []string{"find-bin", "word-dyadic", "sum"},
			matrix: []string{"search:bin-binr-find:vector"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:(til %d) mod 4;+/0 1 2 3?x", rows)
			},
			goFn: func(rows int) int64 {
				var sum int64
				for i := 0; i < rows; i++ {
					sum += int64(i % 4)
				}
				return sum
			},
		},
		{
			name:   "BinAscendingProbeSum",
			tags:   []string{"find-bin", "sum"},
			matrix: []string{"search:bin-binr-find:vector"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:10*til %d;probe:til %d;+/x bin probe", rows, rows)
			},
			goFn: func(rows int) int64 {
				var sum int64
				for probe := 0; probe < rows; probe++ {
					sum += int64(probe / 10)
				}
				return sum
			},
		},
		{
			name:   "XrankModuloBucketsSum",
			tags:   []string{"bin-within-xrank", "word-dyadic", "sum"},
			matrix: []string{"sort:int-vector:index-rank"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:(til %d) mod 100;+/10 xrank x", rows)
			},
			goFn: func(rows int) int64 {
				var sum int64
				for i := 0; i < rows; i++ {
					sum += int64((i % 100) / 10)
				}
				return sum
			},
		},
	}
	return append(cases, combos...)
}

func appendQEvalOrdinaryExpressionCoverageCases(cases []qEvalVectorCase) []qEvalVectorCase {
	for _, p := range []struct {
		name string
		mul  int64
		add  int64
		mod  int64
	}{
		{"DenseA", 2, 1, 3},
		{"DenseB", 3, -7, 5},
		{"DenseC", 5, 11, 7},
		{"DenseD", 9, -13, 11},
		{"DenseE", 17, 19, 13},
		{"DenseF", 31, -29, 17},
	} {
		p := p
		cases = append(cases,
			qEvalVectorCase{
				name:   "OrdinaryArithmeticMapReduce" + p.name,
				tags:   []string{"numeric-vector", "word-dyadic", "sum"},
				matrix: []string{"numeric-arithmetic:int-vector:hot"},
				expr: func(rows int) string {
					return fmt.Sprintf("x:til %d;y:(x*%d)+%d;(+/y)+(+/y mod %d)", rows, p.mul, p.add, p.mod)
				},
				goFn: func(rows int) int64 {
					var sum int64
					for i := int64(0); i < int64(rows); i++ {
						y := i*p.mul + p.add
						sum += y + qPositiveMod(y, p.mod)
					}
					return sum
				},
			},
			qEvalVectorCase{
				name:   "OrdinaryArithmeticWhereProject" + p.name,
				tags:   []string{"numeric-vector", "where", "projection", "word-dyadic", "sum"},
				matrix: []string{"numeric-arithmetic:int-vector:hot", "compare:int-vector:where"},
				shapes: []string{"index:gather-after-where:row-scaled"},
				expr: func(rows int) string {
					return fmt.Sprintf("x:til %d;y:(x*%d)+%d;idx:where (x mod %d)=0;(+/y[idx])+count idx", rows, p.mul, p.add, p.mod)
				},
				goFn: func(rows int) int64 {
					var sum, count int64
					for i := int64(0); i < int64(rows); i++ {
						if i%p.mod == 0 {
							sum += i*p.mul + p.add
							count++
						}
					}
					return sum + count
				},
			},
			qEvalVectorCase{
				name:   "OrdinaryArithmeticScanTail" + p.name,
				tags:   []string{"numeric-vector", "adverb-over-scan", "sums", "sum"},
				matrix: []string{"adverb:over-scan:projection", "aggregate:running-prd-min-max-avg:vector"},
				expr: func(rows int) string {
					return fmt.Sprintf("x:(til %d) mod %d;s:+\\x;last s+count s", rows, p.mod)
				},
				goFn: func(rows int) int64 {
					var last int64
					for i := 0; i < rows; i++ {
						last += int64(i) % p.mod
					}
					return last + int64(rows)
				},
			},
		)
	}

	for _, n := range []int{3, 7, 16, 31, 64, 127, 256, 513, 1024, 2049} {
		n := n
		cases = append(cases,
			qEvalVectorCase{
				name:   fmt.Sprintf("OrdinaryTakeReverseHead%d", n),
				tags:   []string{"take", "reverse", "sum"},
				matrix: []string{"list:cut-raze-enlist:nested"},
				expr: func(rows int) string {
					return fmt.Sprintf("x:til %d;a:%d#reverse x;(+/a)+first a+last a", rows, n)
				},
				goFn: func(rows int) int64 {
					reversed := qEvalVectorGoBaselineMaterializeReverse(rows)
					var sum, first, last int64
					for i := 0; i < n; i++ {
						value := reversed[i%rows]
						if i == 0 {
							first = value
						}
						if i == n-1 {
							last = value
						}
						sum += value
					}
					return sum + first + last
				},
			},
			qEvalVectorCase{
				name:   fmt.Sprintf("OrdinaryDropThenTake%d", n),
				tags:   []string{"drop", "take", "projection", "sum"},
				matrix: []string{"list:cut-raze-enlist:nested"},
				expr: func(rows int) string {
					drop := n % (rows / 2)
					take := n % 257
					if take == 0 {
						take = 1
					}
					return fmt.Sprintf("x:til %d;y:drop %d x;a:%d#y;(+/a)+count a", rows, drop, take)
				},
				goFn: func(rows int) int64 {
					drop := n % (rows / 2)
					take := n % 257
					if take == 0 {
						take = 1
					}
					y := qEvalVectorGoBaselineMaterializeDrop(rows, drop)
					var sum int64
					for i := 0; i < take; i++ {
						sum += y[i%len(y)]
					}
					return sum + int64(take)
				},
			},
			qEvalVectorCase{
				name:   fmt.Sprintf("OrdinaryRotateFilter%d", n),
				tags:   []string{"rotate", "where", "composite-compare"},
				matrix: []string{"compare:int-vector:where"},
				expr: func(rows int) string {
					shift := n % 997
					return fmt.Sprintf("x:til %d;r:%d rotate x;count where (r>=%d) and r<%d", rows, shift, rows/3, rows/3+128)
				},
				goFn: func(rows int) int64 {
					shift := n % 997
					var count int64
					for i := 0; i < rows; i++ {
						value := (shift + i) % rows
						if value >= rows/3 && value < rows/3+128 {
							count++
						}
					}
					return count
				},
			},
		)
	}

	for _, p := range []struct {
		name string
		mod  int
		lo   int
		hi   int
	}{
		{"BandA", 5, 1, 4},
		{"BandB", 7, 2, 6},
		{"BandC", 11, 3, 9},
		{"BandD", 13, 4, 12},
		{"BandE", 17, 5, 15},
		{"BandF", 19, 7, 18},
	} {
		p := p
		cases = append(cases,
			qEvalVectorCase{
				name:   "OrdinaryModuloBandCount" + p.name,
				tags:   []string{"where", "boolean-logical", "composite-compare", "word-dyadic"},
				matrix: []string{"compare:int-vector:where"},
				shapes: []string{"logical:mask-composition:row-scaled"},
				expr: func(rows int) string {
					return fmt.Sprintf("x:til %d;m:x mod %d;count where (m>=%d) and m<%d", rows, p.mod, p.lo, p.hi)
				},
				goFn: func(rows int) int64 {
					var count int64
					for i := 0; i < rows; i++ {
						m := i % p.mod
						if m >= p.lo && m < p.hi {
							count++
						}
					}
					return count
				},
			},
			qEvalVectorCase{
				name:   "OrdinaryModuloBandGatherSum" + p.name,
				tags:   []string{"where", "projection", "numeric-vector", "sum", "word-dyadic"},
				matrix: []string{"compare:int-vector:where"},
				shapes: []string{"where:gather-reduce-selectivity:row-scaled"},
				expr: func(rows int) string {
					return fmt.Sprintf("x:til %d;y:(x*3)+1;m:x mod %d;idx:where (m>=%d) and m<%d;+/y[idx]", rows, p.mod, p.lo, p.hi)
				},
				goFn: func(rows int) int64 {
					var sum int64
					for i := 0; i < rows; i++ {
						m := i % p.mod
						if m >= p.lo && m < p.hi {
							sum += int64(i*3 + 1)
						}
					}
					return sum
				},
			},
		)
	}

	for _, p := range []struct {
		name    string
		pattern string
		needle  string
		hits    map[int]bool
	}{
		{"TechAAPL", "`AAPL`MSFT`NVDA`AAPL`AMD", "`AAPL", map[int]bool{0: true, 3: true}},
		{"TechMSFT", "`AAPL`MSFT`NVDA`AAPL`AMD", "`MSFT", map[int]bool{1: true}},
		{"VenuesXNYS", "`xnys`xnas`bats`xnys`arcx", "`xnys", map[int]bool{0: true, 3: true}},
		{"VenuesBATS", "`xnys`xnas`bats`xnys`arcx", "`bats", map[int]bool{2: true}},
		{"SidesBuy", "`buy`sell`buy`hold", "`buy", map[int]bool{0: true, 2: true}},
		{"SidesSell", "`buy`sell`buy`hold", "`sell", map[int]bool{1: true}},
	} {
		p := p
		cases = append(cases,
			qEvalVectorCase{
				name:   "OrdinarySymbolEqualityWhere" + p.name,
				tags:   []string{"symbol", "where", "composite-compare"},
				matrix: []string{"compare:symbol-vector:where"},
				expr: func(rows int) string {
					return fmt.Sprintf("syms:%d#%s;count where syms=%s", rows, p.pattern, p.needle)
				},
				goFn: func(rows int) int64 {
					width := len(p.hits)
					switch p.name {
					case "SidesBuy", "SidesSell":
						width = 4
					default:
						width = 5
					}
					var count int64
					for i := 0; i < rows; i++ {
						if p.hits[i%width] {
							count++
						}
					}
					return count
				},
			},
			qEvalVectorCase{
				name:   "OrdinarySymbolFilterProjection" + p.name,
				tags:   []string{"symbol", "where", "projection", "membership"},
				matrix: []string{"compare:symbol-vector:where", "membership:in-differ-ratios:vector"},
				shapes: []string{"membership:symbol-filter:row-scaled"},
				expr: func(rows int) string {
					return fmt.Sprintf("syms:%d#%s;px:til %d;idx:where syms=%s;(+/px[idx])+count idx", rows, p.pattern, rows, p.needle)
				},
				goFn: func(rows int) int64 {
					width := len(p.hits)
					switch p.name {
					case "SidesBuy", "SidesSell":
						width = 4
					default:
						width = 5
					}
					var sum, count int64
					for i := 0; i < rows; i++ {
						if p.hits[i%width] {
							sum += int64(i)
							count++
						}
					}
					return sum + count
				},
			},
		)
	}

	for _, p := range []struct {
		name      string
		values    string
		predicate string
		hit       func(i int) bool
	}{
		{"DateGE", "2026.06.06 2026.06.07 2026.06.08", "dates>=2026.06.07", func(i int) bool { return i%3 != 0 }},
		{"DateEQ", "2026.06.06 2026.06.07 2026.06.08", "dates=2026.06.08", func(i int) bool { return i%3 == 2 }},
		{"MonthSuffixGE", "2026.06m 2026.07m 2026.08m", "dates>=2026.07m", func(i int) bool { return i%3 != 0 }},
		{"TimeWithin", "09:30 09:31 09:32 09:33", "dates within 09:31 09:32", func(i int) bool { return i%4 == 1 || i%4 == 2 }},
	} {
		p := p
		cases = append(cases,
			qEvalVectorCase{
				name:   "OrdinaryTemporalCompareCount" + p.name,
				tags:   []string{"temporal", "where", "composite-compare"},
				matrix: []string{"compare:temporal-vector:where"},
				expr: func(rows int) string {
					return fmt.Sprintf("dates:%d#%s;count where %s", rows, p.values, p.predicate)
				},
				goFn: func(rows int) int64 {
					var count int64
					for i := 0; i < rows; i++ {
						if p.hit(i) {
							count++
						}
					}
					return count
				},
			},
			qEvalVectorCase{
				name:   "OrdinaryTemporalFilterProjection" + p.name,
				tags:   []string{"temporal", "where", "projection", "sum"},
				matrix: []string{"compare:temporal-vector:where"},
				expr: func(rows int) string {
					return fmt.Sprintf("dates:%d#%s;v:til %d;idx:where %s;(+/v[idx])+count idx", rows, p.values, rows, p.predicate)
				},
				goFn: func(rows int) int64 {
					var sum, count int64
					for i := 0; i < rows; i++ {
						if p.hit(i) {
							sum += int64(i)
							count++
						}
					}
					return sum + count
				},
			},
		)
	}

	for _, p := range []struct {
		name      string
		values    string
		fill      int64
		sum       func(rows int) int64
		nulls     func(rows int) int64
		castTag   string
		nullSlots map[int]bool
	}{
		{"IntNulls", "0N 1 2 0N", 9, func(rows int) int64 {
			var sum int64
			for i := 0; i < rows; i++ {
				if i%4 == 0 || i%4 == 3 {
					sum += 9
				} else {
					sum += int64(i % 4)
				}
			}
			return sum
		}, func(rows int) int64 { return qPatternCount(rows, 4, map[int]bool{0: true, 3: true}) }, "typed-null", map[int]bool{0: true, 3: true}},
		{"ShortCast", "1 2 3 4", 0, func(rows int) int64 {
			var sum int64
			for i := 0; i < rows; i++ {
				sum += int64(i%4 + 1)
			}
			return sum
		}, func(rows int) int64 { return 0 }, "cast", map[int]bool{}},
		{"FloatNulls", "0Nf 1.5 2.5 0Nf", 7, func(rows int) int64 {
			var sum float64
			for i := 0; i < rows; i++ {
				switch i % 4 {
				case 0, 3:
					sum += 7
				case 1:
					sum += 1.5
				case 2:
					sum += 2.5
				}
			}
			return int64(sum)
		}, func(rows int) int64 { return qPatternCount(rows, 4, map[int]bool{0: true, 3: true}) }, "typed-null", map[int]bool{0: true, 3: true}},
	} {
		p := p
		cases = append(cases,
			qEvalVectorCase{
				name:   "OrdinaryNullFillSum" + p.name,
				tags:   []string{p.castTag, "fill", "null-verb", "sum"},
				matrix: []string{"numeric-arithmetic:typed-null:hot", "list:prev-next-deltas-fills:typed-null"},
				expr: func(rows int) string {
					return fmt.Sprintf("x:%d#%s;y:%d^x;(+/y)+count where null x", rows, p.values, p.fill)
				},
				goFn: func(rows int) int64 {
					return p.sum(rows) + p.nulls(rows)
				},
			},
			qEvalVectorCase{
				name:   "OrdinaryNullFillsCount" + p.name,
				tags:   []string{p.castTag, "fill", "typed-null"},
				matrix: []string{"list:prev-next-deltas-fills:typed-null"},
				expr: func(rows int) string {
					return fmt.Sprintf("x:%d#%s;count fills x", rows, p.values)
				},
				goFn: func(rows int) int64 {
					filled := make([]int64, rows)
					var last int64
					for i := 0; i < rows; i++ {
						if !p.nullSlots[i%4] {
							last = int64(i % 4)
						}
						filled[i] = last
					}
					qEvalVectorAnyBenchSink = filled
					return int64(len(filled))
				},
			},
		)
	}

	return appendQEvalDeepExpressionCombinationCases(cases)
}

func appendQEvalDeepExpressionCombinationCases(cases []qEvalVectorCase) []qEvalVectorCase {
	for _, p := range []struct {
		name  string
		scale int64
		bias  int64
		mod   int64
	}{
		{"A", 2, 7, 5},
		{"B", 3, -11, 7},
		{"C", 5, 13, 11},
		{"D", 9, -17, 13},
	} {
		p := p
		cases = append(cases,
			qEvalVectorCase{
				name:   "DeepNumericAffineModuloWhere" + p.name,
				tags:   []string{"numeric-vector", "where", "word-dyadic", "sum"},
				matrix: []string{"numeric-arithmetic:int-vector:hot", "compare:int-vector:where"},
				shapes: []string{"where:gather-reduce-selectivity:row-scaled"},
				expr: func(rows int) string {
					return fmt.Sprintf("x:til %d;y:(x*%d)+%d;idx:where (y mod %d)>1;(+/y[idx])+count idx", rows, p.scale, p.bias, p.mod)
				},
				goFn: func(rows int) int64 {
					var sum, count int64
					for i := int64(0); i < int64(rows); i++ {
						y := i*p.scale + p.bias
						if qPositiveMod(y, p.mod) > 1 {
							sum += y
							count++
						}
					}
					return sum + count
				},
			},
			qEvalVectorCase{
				name:   "DeepNumericFloorCeilingEnvelope" + p.name,
				tags:   []string{"numeric-vector", "numeric-monad", "sum", "promotion"},
				matrix: []string{"numeric-arithmetic:float-vector:hot"},
				expr: func(rows int) string {
					return fmt.Sprintf("x:(til %d)*0.25;y:x+%d.5;(+/floor y)+(+/ceiling y)+count y", rows, p.scale)
				},
				goFn: func(rows int) int64 {
					var floors, ceilings int64
					for i := 0; i < rows; i++ {
						y := float64(i)*0.25 + float64(p.scale) + 0.5
						floors += int64(math.Floor(y))
						ceilings += int64(math.Ceil(y))
					}
					return floors + ceilings + int64(rows)
				},
			},
			qEvalVectorCase{
				name:   "DeepNumericSignumNotMask" + p.name,
				tags:   []string{"numeric-vector", "numeric-monad", "where", "boolean-logical"},
				matrix: []string{"numeric-arithmetic:int-vector:hot", "compare:int-vector:where"},
				shapes: []string{"logical:mask-composition:row-scaled"},
				expr: func(rows int) string {
					return fmt.Sprintf("x:(til %d) mod %d;m:x mod 2;(+/m)+count where not m", rows, p.mod)
				},
				goFn: func(rows int) int64 {
					var sum, notCount int64
					for i := 0; i < rows; i++ {
						x := int64(i) % p.mod
						sum += x % 2
						if x%2 == 0 {
							notCount++
						}
					}
					return sum + notCount
				},
			},
		)
	}

	for _, p := range []struct {
		name  string
		take  int
		drop  int
		shift int
	}{
		{"Small", 17, 3, 5},
		{"Medium", 257, 128, 31},
		{"Wide", 1025, 513, 257},
		{"Cycle", 9000, 1024, 997},
	} {
		p := p
		cases = append(cases,
			qEvalVectorCase{
				name:   "DeepTakeDropRotateSum" + p.name,
				tags:   []string{"take", "drop", "rotate", "sum"},
				matrix: []string{"list:cut-raze-enlist:nested"},
				shapes: []string{"pipeline:vector-transform-reduce:sum"},
				expr: func(rows int) string {
					return fmt.Sprintf("x:til %d;r:%d rotate x;y:%d#drop %d r;(+/y)+first y+last y", rows, p.shift, p.take, p.drop)
				},
				goFn: func(rows int) int64 {
					rotated := qEvalVectorGoBaselineMaterializeRotate(rows, p.shift)
					dropped := rotated[p.drop:]
					var sum, first, last int64
					for i := 0; i < p.take; i++ {
						value := dropped[i%len(dropped)]
						if i == 0 {
							first = value
						}
						if i == p.take-1 {
							last = value
						}
						sum += value
					}
					return sum + first + last
				},
			},
			qEvalVectorCase{
				name:   "DeepReverseWhereWindowCount" + p.name,
				tags:   []string{"reverse", "where", "composite-compare", "boolean-logical"},
				matrix: []string{"compare:int-vector:where"},
				shapes: []string{"logical:mask-composition:row-scaled"},
				expr: func(rows int) string {
					lo := rows / 4
					hi := lo + p.take%1024
					return fmt.Sprintf("x:reverse til %d;count where (x>=%d) and x<%d", rows, lo, hi)
				},
				goFn: func(rows int) int64 {
					lo := rows / 4
					hi := lo + p.take%1024
					var count int64
					for i := rows - 1; i >= 0; i-- {
						if i >= lo && i < hi {
							count++
						}
					}
					return count
				},
			},
			qEvalVectorCase{
				name:   "DeepRotateModuloGatherSum" + p.name,
				tags:   []string{"rotate", "where", "projection", "word-dyadic", "sum"},
				matrix: []string{"compare:int-vector:where"},
				shapes: []string{"index:gather-after-where:row-scaled"},
				expr: func(rows int) string {
					return fmt.Sprintf("x:%d rotate til %d;idx:where (x mod 9)=4;(+/x[idx])+count idx", p.shift, rows)
				},
				goFn: func(rows int) int64 {
					shift := p.shift % rows
					var sum, count int64
					for i := 0; i < rows; i++ {
						value := int64((shift + i) % rows)
						if value%9 == 4 {
							sum += value
							count++
						}
					}
					return sum + count
				},
			},
		)
	}

	for _, p := range []struct {
		name  string
		width int64
		lo    int64
		hi    int64
	}{
		{"Fine", 5, 100, 250},
		{"Ten", 10, 1000, 2000},
		{"Minute", 60, 1800, 3600},
		{"Kilo", 1000, 2000, 6000},
	} {
		p := p
		cases = append(cases,
			qEvalVectorCase{
				name:   "DeepXbarWithinCount" + p.name,
				tags:   []string{"xbar", "bin-within-xrank", "where"},
				matrix: []string{"temporal:xbar:bucket", "compare:int-vector:where"},
				expr: func(rows int) string {
					return fmt.Sprintf("x:til %d;b:%d xbar x;count where b within %d %d", rows, p.width, p.lo, p.hi)
				},
				goFn: func(rows int) int64 {
					var count int64
					for i := int64(0); i < int64(rows); i++ {
						bucket := floorBucket(i, p.width)
						if bucket >= p.lo && bucket <= p.hi {
							count++
						}
					}
					return count
				},
			},
			qEvalVectorCase{
				name:   "DeepBinBucketProbeSum" + p.name,
				tags:   []string{"find-bin", "xbar", "sum"},
				matrix: []string{"search:bin-binr-find:vector"},
				expr: func(rows int) string {
					return fmt.Sprintf("x:%d*til %d;p:til %d;+/x bin p", p.width, rows, rows)
				},
				goFn: func(rows int) int64 {
					var sum int64
					for probe := int64(0); probe < int64(rows); probe++ {
						sum += probe / p.width
					}
					return sum
				},
			},
			qEvalVectorCase{
				name:   "DeepXrankModuloSum" + p.name,
				tags:   []string{"bin-within-xrank", "word-dyadic", "sum"},
				matrix: []string{"sort:int-vector:index-rank"},
				expr: func(rows int) string {
					return fmt.Sprintf("x:(til %d) mod %d;count 10 xrank x", rows, p.width*10)
				},
				goFn: func(rows int) int64 {
					return qEvalVectorGoBaselineXrankModuloCount(rows, p.width*10)
				},
			},
		)
	}

	for _, p := range []struct {
		name    string
		pattern string
		width   int
		aHits   map[int]bool
		inHits  map[int]bool
	}{
		{"Tech", "`aapl`msft`amd`nvda`amzn", 5, map[int]bool{0: true, 2: true, 4: true}, map[int]bool{0: true, 2: true}},
		{"Venue", "`xnys`xnas`bats`arcx`edgx", 5, map[int]bool{3: true}, map[int]bool{0: true, 2: true}},
		{"Side", "`buy`sell`hold`buy`sell", 5, map[int]bool{}, map[int]bool{0: true, 3: true}},
	} {
		p := p
		cases = append(cases,
			qEvalVectorCase{
				name:   "DeepStringUpperLikeCount" + p.name,
				tags:   []string{"string", "symbol", "match-like", "where"},
				matrix: []string{"compare:string-vector:where"},
				shapes: []string{"string:symbol-string-like:row-scaled"},
				expr: func(rows int) string {
					return fmt.Sprintf("syms:%d#%s;names:upper string syms;count where names like \"A*\"", rows, p.pattern)
				},
				goFn: func(rows int) int64 {
					return qPatternCount(rows, p.width, p.aHits)
				},
			},
			qEvalVectorCase{
				name:   "DeepSymbolMembershipProjection" + p.name,
				tags:   []string{"symbol", "membership", "where", "projection", "sum"},
				matrix: []string{"compare:symbol-vector:where", "membership:in-differ-ratios:vector"},
				shapes: []string{"membership:symbol-filter:row-scaled"},
				expr: func(rows int) string {
					return fmt.Sprintf("syms:%d#%s;v:til %d;idx:where syms in `aapl`amd`xnys`bats`buy;(+/v[idx])+count idx", rows, p.pattern, rows)
				},
				goFn: func(rows int) int64 {
					var sum, count int64
					for i := 0; i < rows; i++ {
						if p.inHits[i%p.width] {
							sum += int64(i)
							count++
						}
					}
					return sum + count
				},
			},
			qEvalVectorCase{
				name:   "DeepSymbolDistinctAfterRotate" + p.name,
				tags:   []string{"symbol", "distinct", "rotate"},
				matrix: []string{"set:symbol-vector:union-inter-except"},
				expr: func(rows int) string {
					return fmt.Sprintf("syms:%d#%s;count distinct 17 rotate syms", rows, p.pattern)
				},
				goFn: func(rows int) int64 {
					symbols := strings.Split(strings.TrimPrefix(p.pattern, "`"), "`")
					seen := make(map[string]struct{}, p.width)
					shift := 17 % rows
					for i := 0; i < rows; i++ {
						seen[symbols[(shift+i)%rows%p.width]] = struct{}{}
					}
					return int64(len(seen))
				},
			},
		)
	}

	for _, p := range []struct {
		name   string
		values string
		pred   string
		hit    func(i int) bool
		width  int
	}{
		{"DateWindow", "2026.06.01 2026.06.02 2026.06.03 2026.06.04", "d within 2026.06.02 2026.06.03", func(i int) bool { return i%4 == 1 || i%4 == 2 }, 4},
		{"MonthWindow", "2026.06m 2026.07m 2026.08m 2026.09m", "d within 2026.07m 2026.08m", func(i int) bool { return i%4 == 1 || i%4 == 2 }, 4},
		{"TimeGE", "09:30 09:31 09:32 09:33 09:34", "d>=09:32", func(i int) bool { return i%5 >= 2 }, 5},
	} {
		p := p
		cases = append(cases,
			qEvalVectorCase{
				name:   "DeepTemporalWhereProjection" + p.name,
				tags:   []string{"temporal", "where", "projection", "sum"},
				matrix: []string{"compare:temporal-vector:where"},
				expr: func(rows int) string {
					return fmt.Sprintf("d:%d#%s;v:til %d;idx:where %s;(+/v[idx])+count idx", rows, p.values, rows, p.pred)
				},
				goFn: func(rows int) int64 {
					var sum, count int64
					for i := 0; i < rows; i++ {
						if p.hit(i) {
							sum += int64(i)
							count++
						}
					}
					return sum + count
				},
			},
			qEvalVectorCase{
				name:   "DeepTemporalReverseCount" + p.name,
				tags:   []string{"temporal", "reverse", "where"},
				matrix: []string{"compare:temporal-vector:where"},
				expr: func(rows int) string {
					return fmt.Sprintf("d:reverse %d#%s;count where %s", rows, p.values, p.pred)
				},
				goFn: func(rows int) int64 {
					var count int64
					for i := rows - 1; i >= 0; i-- {
						if p.hit(i) {
							count++
						}
					}
					return count
				},
			},
		)
	}

	for _, p := range []struct {
		name   string
		values string
		fill   int64
		width  int
		nulls  map[int]bool
		value  func(i int) int64
	}{
		{"Ints", "0Ni 1i 2i 0Ni 4i", 8, 5, map[int]bool{0: true, 3: true}, func(i int) int64 { return int64([]int{8, 1, 2, 8, 4}[i%5]) }},
		{"Longs", "0Nj 10 20 30 0Nj", 6, 5, map[int]bool{0: true, 4: true}, func(i int) int64 { return int64([]int{6, 10, 20, 30, 6}[i%5]) }},
	} {
		p := p
		cases = append(cases,
			qEvalVectorCase{
				name:   "DeepNullFillWhereSum" + p.name,
				tags:   []string{"typed-null", "fill", "where", "sum"},
				matrix: []string{"numeric-arithmetic:typed-null:hot", "list:prev-next-deltas-fills:typed-null"},
				shapes: []string{"null:fill-arithmetic-where:row-scaled"},
				expr: func(rows int) string {
					return fmt.Sprintf("x:%d#%s;y:%d^x;(+/y)+count where null x", rows, p.values, p.fill)
				},
				goFn: func(rows int) int64 {
					var sum, nulls int64
					for i := 0; i < rows; i++ {
						value := p.value(i)
						sum += value
						if p.nulls[i%p.width] {
							nulls++
						}
					}
					return sum + nulls
				},
			},
			qEvalVectorCase{
				name:   "DeepDeltasFillsChecksum" + p.name,
				tags:   []string{"adverb-each-prior", "fill", "typed-null", "sum"},
				matrix: []string{"list:prev-next-deltas-fills:typed-null"},
				expr: func(rows int) string {
					return fmt.Sprintf("x:%d#%s;(+/fills x)+count deltas fills x", rows, p.values)
				},
				goFn: func(rows int) int64 {
					var sum, last int64
					haveLast := false
					raw := make([]int64, p.width)
					isNull := make([]bool, p.width)
					for i := 0; i < p.width; i++ {
						isNull[i] = p.nulls[i]
						if !isNull[i] {
							raw[i] = p.value(i)
						}
					}
					for i := 0; i < rows; i++ {
						pos := i % p.width
						if !isNull[pos] {
							last = raw[pos]
							haveLast = true
						}
						if haveLast {
							sum += last
						}
					}
					return sum + int64(rows)
				},
			},
		)
	}

	cases = append(cases,
		qEvalVectorCase{
			name:   "DeepAdverbEachLeftRightChecksum",
			tags:   []string{"adverb-each", "adverb-each-left-right", "sum"},
			matrix: []string{"adverb:dyadic-each:vector", "adverb:each-left-right:vector"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;(+/100-\\:x)+(+/x-/:100)", rows)
			},
			goFn: func(rows int) int64 {
				var sum int64
				for i := 0; i < rows; i++ {
					sum += 100 - int64(i)
					sum += int64(i) - 100
				}
				return sum
			},
		},
		qEvalVectorCase{
			name:   "DeepAdverbEachPairwiseArithmetic",
			tags:   []string{"adverb-each", "numeric-vector", "sum"},
			matrix: []string{"adverb:dyadic-each:vector"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;y:(x*2)+1;+/x+'y", rows)
			},
			goFn: func(rows int) int64 {
				var sum int64
				for i := 0; i < rows; i++ {
					sum += int64(i) + int64(i*2+1)
				}
				return sum
			},
		},
		qEvalVectorCase{
			name:   "DeepOverScanProjectionChecksum",
			tags:   []string{"adverb-over-scan", "projection", "sum"},
			matrix: []string{"adverb:over-scan:projection"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:1+til %d;s:+\\x;({x+y}/[10;x])+last s+count s", rows)
			},
			goFn: func(rows int) int64 {
				var sum int64 = 10
				var scan int64
				for i := 1; i <= rows; i++ {
					sum += int64(i)
					scan += int64(i)
				}
				return sum + scan + int64(rows)
			},
		},
		qEvalVectorCase{
			name:   "DeepSetUnionInterExceptSymbols",
			tags:   []string{"set-verb", "symbol", "membership"},
			matrix: []string{"set:symbol-vector:union-inter-except"},
			expr: func(rows int) string {
				return "(count `a`b`c`a union `b`d`e)+(count `a`b`c`a inter `c`a`x)+(count `a`b`c`a except `b`x)+(count where `a`b`c`d in `a`d)"
			},
			goFn: func(rows int) int64 {
				return 5 + 2 + 3 + 2
			},
		},
		qEvalVectorCase{
			name:   "DeepSortRankGatherChecksum",
			tags:   []string{"table-sort", "projection", "sum"},
			matrix: []string{"sort:int-vector:index-rank"},
			shapes: []string{"sort:index-gather:row-scaled"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:(%d-1)-til %d;idx:iasc x;y:x[idx];(+/rank y)+first y+last y", rows, rows)
			},
			goFn: func(rows int) int64 {
				var rankSum int64
				for i := 0; i < rows; i++ {
					rankSum += int64(i)
				}
				return rankSum + int64(0) + int64(rows-1)
			},
		},
	)

	return cases
}

func appendQEvalTaskDListMathMatrixApplyIndexCases(cases []qEvalVectorCase) []qEvalVectorCase {
	taskD := []qEvalVectorCase{
		{
			name:   "TaskDMathSqrtLogVectorSum",
			tags:   []string{"math-transcendental", "numeric-monad", "numeric-vector", "sum"},
			matrix: []string{"math:transcendental-vector:hot", "numeric-arithmetic:float-vector:hot"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:1+til %d;(+/sqrt x)+(+/log x)", rows)
			},
			goFn: func(rows int) int64 {
				var total float64
				for i := 1; i <= rows; i++ {
					x := float64(i)
					total += math.Sqrt(x) + math.Log(x)
				}
				return int64(total)
			},
		},
		{
			name:   "TaskDMathTrigVectorEnvelope",
			tags:   []string{"math-transcendental", "numeric-monad", "numeric-vector", "sum"},
			matrix: []string{"math:transcendental-vector:hot", "numeric-arithmetic:float-vector:hot"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:(til %d) mod 8;(+/sin x)+(+/cos x)+(+/tan x)", rows)
			},
			goFn: func(rows int) int64 {
				var total float64
				for i := 0; i < rows; i++ {
					x := float64(i % 8)
					total += math.Sin(x) + math.Cos(x) + math.Tan(x)
				}
				return int64(total)
			},
		},
		{
			name:   "TaskDMathInverseTrigVectorEnvelope",
			tags:   []string{"math-transcendental", "numeric-monad", "numeric-vector", "sum"},
			matrix: []string{"math:transcendental-vector:hot", "numeric-arithmetic:float-vector:hot"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:((til %d) mod 5)%%10;(+/asin x)+(+/acos x)+(+/atan x)", rows)
			},
			goFn: func(rows int) int64 {
				var total float64
				for i := 0; i < rows; i++ {
					x := float64(i%5) / 10
					total += math.Asin(x) + math.Acos(x) + math.Atan(x)
				}
				return int64(total)
			},
		},
		{
			name:   "TaskDMathXexpXlogVectorEnvelope",
			tags:   []string{"math-transcendental", "word-dyadic", "numeric-vector", "sum"},
			matrix: []string{"math:transcendental-vector:hot", "numeric-arithmetic:float-vector:hot"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:(til %d) mod 8;y:2 xexp x;(+/y)+(+/2 xlog y)", rows)
			},
			goFn: func(rows int) int64 {
				var total float64
				for i := 0; i < rows; i++ {
					x := float64(i % 8)
					y := math.Pow(2, x)
					total += y + math.Log(y)/math.Log(2)
				}
				return int64(total)
			},
		},
		{
			name:   "TaskDListRazeNestedRowScaled",
			tags:   []string{"raze", "enlist", "reverse", "sum"},
			matrix: []string{"list:cut-raze-enlist:nested"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;a:(x;reverse x;32#x);(+/raze a)+count a", rows)
			},
			goFn: func(rows int) int64 {
				var total int64
				for i := 0; i < rows; i++ {
					total += int64(i)
				}
				for i := rows - 1; i >= 0; i-- {
					total += int64(i)
				}
				for i := 0; i < 32; i++ {
					total += int64(i % rows)
				}
				return total + 3
			},
		},
		{
			name:   "TaskDListCutRazeVariableBins",
			tags:   []string{"cut", "raze", "sum"},
			matrix: []string{"list:cut-raze-enlist:nested"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;g:0 128 1024 4096 cut x;(+/raze g)+count g", rows)
			},
			goFn: func(rows int) int64 {
				var total int64
				for i := 0; i < rows; i++ {
					total += int64(i)
				}
				return total + 4
			},
		},
		{
			name:   "TaskDListSublistRotateReverse",
			tags:   []string{"cut", "rotate", "reverse", "sum"},
			matrix: []string{"list:sublist-cross-cut:string-bool", "list:cut-raze-enlist:nested"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;r:17 rotate x;y:128 sublist reverse r;(+/y)+first y+last y", rows)
			},
			goFn: func(rows int) int64 {
				rotated := qEvalVectorGoBaselineMaterializeRotate(rows, 17)
				reversed := make([]int64, rows)
				for i := 0; i < rows; i++ {
					reversed[i] = rotated[rows-1-i]
				}
				qEvalVectorAnyBenchSink = reversed
				var total, first, last int64
				for i := 0; i < 128; i++ {
					value := reversed[i]
					if i == 0 {
						first = value
					}
					if i == 127 {
						last = value
					}
					total += value
				}
				return total + first + last
			},
		},
		{
			name:   "TaskDListRatiosCountAndSum",
			tags:   []string{"adverb-each-prior", "numeric-vector", "sum"},
			matrix: []string{"list:prev-next-deltas-fills:typed-null", "membership:in-differ-ratios:vector"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:1+til %d;(count ratios x)+(+/deltas x)", rows)
			},
			goFn: func(rows int) int64 {
				var deltaSum int64
				for i := 1; i <= rows; i++ {
					if i == 1 {
						deltaSum += int64(i)
					} else {
						deltaSum++
					}
				}
				return int64(rows) + deltaSum
			},
		},
		{
			name:   "TaskDListReverseDirectSum",
			tags:   []string{"reverse", "sum", "numeric-vector"},
			matrix: []string{"list:cut-raze-enlist:nested", "numeric-arithmetic:int-vector:hot"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;+/reverse x", rows)
			},
			goFn: func(rows int) int64 {
				var total int64
				for i := rows - 1; i >= 0; i-- {
					total += int64(i)
				}
				return total
			},
		},
		{
			name:   "TaskDListRotateDirectSum",
			tags:   []string{"rotate", "sum", "numeric-vector"},
			matrix: []string{"list:cut-raze-enlist:nested", "numeric-arithmetic:int-vector:hot"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;+/257 rotate x", rows)
			},
			goFn: func(rows int) int64 {
				shift := 257 % rows
				var total int64
				for i := 0; i < rows; i++ {
					total += int64((shift + i) % rows)
				}
				return total
			},
		},
		{
			name:   "TaskDListSublistDirectSum",
			tags:   []string{"cut", "sublist", "sum", "numeric-vector"},
			matrix: []string{"list:sublist-cross-cut:string-bool", "list:cut-raze-enlist:nested"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;+/128 1024 sublist x", rows)
			},
			goFn: func(rows int) int64 {
				var total int64
				for i := 128; i < 128+1024 && i < rows; i++ {
					total += int64(i)
				}
				return total
			},
		},
		{
			name:   "TaskDListRatiosDirectSum",
			tags:   []string{"adverb-each-prior", "numeric-vector", "sum"},
			matrix: []string{"list:prev-next-deltas-fills:typed-null", "membership:in-differ-ratios:vector"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:2 xexp ((til %d) mod 8);+/ratios x", rows)
			},
			goFn: func(rows int) int64 {
				var total float64
				prev := math.Pow(2, float64((rows-1)%8))
				for i := 0; i < rows; i++ {
					v := math.Pow(2, float64(i%8))
					total += v / prev
					prev = v
				}
				return int64(total)
			},
		},
		{
			name:   "TaskDMatrixReshapeRazeSum",
			tags:   []string{"matrix-reshape", "raze", "sum"},
			matrix: []string{"matrix:reshape-flip:vector", "list:cut-raze-enlist:nested"},
			expr: func(rows int) string {
				return fmt.Sprintf("m:2 %d#til %d;(+/raze m)+count m", rows/2, rows)
			},
			goFn: func(rows int) int64 {
				var total int64
				for i := 0; i < rows; i++ {
					total += int64(i)
				}
				return total + 2
			},
		},
		{
			name:   "TaskDMatrixFlipRazeSum",
			tags:   []string{"matrix-reshape", "raze", "sum"},
			matrix: []string{"matrix:reshape-flip:vector", "list:cut-raze-enlist:nested"},
			expr: func(rows int) string {
				return fmt.Sprintf("m:2 %d#til %d;t:flip m;(+/raze t)+count t", rows/2, rows)
			},
			goFn: func(rows int) int64 {
				var total int64
				for i := 0; i < rows; i++ {
					total += int64(i)
				}
				return total + int64(rows/2)
			},
		},
		{
			name:   "TaskDMatrixDotRowIndexSum",
			tags:   []string{"matrix-reshape", "apply-index", "sum"},
			matrix: []string{"matrix:reshape-flip:vector", "apply-index:list-dict-callable:hot"},
			expr: func(rows int) string {
				return fmt.Sprintf("m:2 %d#til %d;row:m . 1;(+/row)+count row", rows/2, rows)
			},
			goFn: func(rows int) int64 {
				var total int64
				for i := rows / 2; i < rows; i++ {
					total += int64(i)
				}
				return total + int64(rows/2)
			},
		},
		{
			name:   "TaskDMatrixDotCellIndex",
			tags:   []string{"matrix-reshape", "apply-index"},
			matrix: []string{"matrix:reshape-flip:vector", "apply-index:list-dict-callable:hot"},
			expr: func(rows int) string {
				return fmt.Sprintf("m:2 %d#til %d;(m . 1 10)+count m", rows/2, rows)
			},
			goFn: func(rows int) int64 {
				return qEvalVectorGoBaselineMatrixCellProbe(2, rows/2, 10, qEvalVectorGoBaselineInput)
			},
		},
		{
			name:   "TaskDApplyAtScalarAndVector",
			tags:   []string{"apply-index", "where", "projection", "sum"},
			matrix: []string{"apply-index:list-dict-callable:hot", "compare:int-vector:where"},
			shapes: []string{"index:gather-after-where:row-scaled"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;idx:where (x mod 257)=0;(x@10)+(x@20)+(x@30)+count idx", rows)
			},
			goFn: func(rows int) int64 {
				var count int64
				for i := 0; i < rows; i++ {
					if i%257 == 0 {
						count++
					}
				}
				return 10 + 20 + 30 + count
			},
		},
		{
			name:   "TaskDApplyDotScalarProbe",
			tags:   []string{"apply-index", "projection"},
			matrix: []string{"apply-index:list-dict-callable:hot"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;(x . 10)+(x . 20)+(x . 30)+count x", rows)
			},
			goFn: func(rows int) int64 {
				values := make([]int64, rows)
				for i := range values {
					values[i] = int64(i)
				}
				qEvalVectorAnyBenchSink = values
				return values[10] + values[20] + values[30] + int64(len(values))
			},
		},
		{
			name:   "TaskDApplyFunctionDotArgs",
			tags:   []string{"apply-index", "projection", "sum"},
			matrix: []string{"apply-index:list-dict-callable:hot"},
			expr: func(rows int) string {
				return fmt.Sprintf("f:{(+/x)+y};.[f;(til %d;10)]", rows)
			},
			goFn: func(rows int) int64 {
				var total int64
				for i := 0; i < rows; i++ {
					total += int64(i)
				}
				return total + 10
			},
		},
		{
			name:   "TaskDApplyAtRepeatedScalarProbe",
			tags:   []string{"apply-index", "projection"},
			matrix: []string{"apply-index:list-dict-callable:hot"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;(x@100)+(x@200)+(x@300)+count x", rows)
			},
			goFn: func(rows int) int64 {
				values := make([]int64, rows)
				for i := range values {
					values[i] = int64(i)
				}
				qEvalVectorAnyBenchSink = values
				return values[100] + values[200] + values[300] + int64(len(values))
			},
		},
		{
			name:   "TaskDStringSearchReplaceJoinRowScaled",
			tags:   []string{"string", "symbol", "match-like"},
			matrix: []string{"compare:string-vector:where", "compare:symbol-vector:where", "list:sublist-cross-cut:string-bool"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:%d#`AAPL`MSFT`AMD`NVDA;s:\",\" sv string x;(count \",\" vs s)+(count s ss \"A\")+(count ssr[s;\"A\";\"Z\"])", rows)
			},
			goFn: func(rows int) int64 {
				values := []string{"AAPL", "MSFT", "AMD", "NVDA"}
				parts := make([]string, rows)
				for i := range parts {
					parts[i] = values[i%len(values)]
				}
				joined := strings.Join(parts, ",")
				split := strings.Split(joined, ",")
				var hits int64
				for offset := strings.Index(joined, "A"); offset >= 0; {
					hits++
					next := strings.Index(joined[offset+1:], "A")
					if next < 0 {
						break
					}
					offset += next + 1
				}
				replaced := strings.ReplaceAll(joined, "A", "Z")
				qEvalVectorAnyBenchSink = []any{split, replaced}
				return int64(len(split)) + hits + int64(len(replaced))
			},
		},
		{
			name:   "TaskDBooleanVectorLiteralWhereProjection",
			tags:   []string{"boolean-logical", "where", "projection", "sum"},
			matrix: []string{"compare:int-vector:where", "list:sublist-cross-cut:string-bool"},
			shapes: []string{"index:gather-after-where:row-scaled", "logical:mask-composition:row-scaled"},
			expr: func(rows int) string {
				return fmt.Sprintf("m:%d#101001b;x:til %d;idx:where m;(+/x[idx])+count idx", rows, rows)
			},
			goFn: func(rows int) int64 {
				hit := [...]bool{true, false, true, false, false, true}
				var sum, count int64
				for i := 0; i < rows; i++ {
					if hit[i%len(hit)] {
						sum += int64(i)
						count++
					}
				}
				return sum + count
			},
		},
		{
			name:   "TaskDMatrixFlipColumnRowChecksum",
			tags:   []string{"matrix-reshape", "apply-index", "sum"},
			matrix: []string{"matrix:reshape-flip:vector", "apply-index:list-dict-callable:hot"},
			expr: func(rows int) string {
				matrixRows := rows / 4
				return fmt.Sprintf("m:%d 4#til %d;t:flip m;(+/(t . 0))+(+/(t . 3))+count t", matrixRows, rows)
			},
			goFn: func(rows int) int64 {
				var firstCol, lastCol int64
				for r := 0; r < rows/4; r++ {
					firstCol += int64(r * 4)
					lastCol += int64(r*4 + 3)
				}
				return firstCol + lastCol + 4
			},
		},
		{
			name:   "TaskDMatrixMmuInvEnvelope",
			tags:   []string{"matrix-reshape", "sum"},
			matrix: []string{"matrix:reshape-flip:vector", "numeric-arithmetic:float-vector:hot"},
			expr: func(rows int) string {
				side := qEvalVectorMatrixMmuSide(rows)
				n := side * side
				return fmt.Sprintf("a:%d %d#1+til %d;b:%d %d#1+((til %d) mod 17);(+/raze mmu[a;b])+(+/raze inv 2 2#1 0 0 1)", side, side, n, side, side, n)
			},
			goFn: func(rows int) int64 {
				return qEvalVectorGoBaselineMatrixMmuInv(rows)
			},
		},
		{
			name:   "TaskDCastSymbolNumericEnvelope",
			tags:   []string{"cast", "symbol", "string"},
			matrix: []string{"cast:symbol-numeric:string"},
			expr: func(rows int) string {
				return "(`long$3.7)+(\"J\"$\"42\")+(\"I\"$\"17\")+(count `$\"AAPL\")+(count string `$\"MSFT\")"
			},
			goFn: func(rows int) int64 {
				// Canonical q integer casts round half-to-even.
				longValue := int64(math.RoundToEven(3.7))
				jValue := int64(42)
				iValue := int64(17)
				symbol := string([]byte{'A', 'A', 'P', 'L'})
				symbolText := string([]byte{'M', 'S', 'F', 'T'})
				qEvalVectorAnyBenchSink = []string{symbol, symbolText}
				return longValue + jValue + iValue + int64(len([]string{symbol})) + int64(len(symbolText))
			},
		},
		{
			name:   "TaskDApplyDotFunctionGatherArgs",
			tags:   []string{"apply-index", "where", "projection", "sum"},
			matrix: []string{"apply-index:list-dict-callable:hot", "compare:int-vector:where"},
			shapes: []string{"index:gather-after-where:row-scaled"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;idx:where (x mod 257)=0;f:{(+/x[y])+count y};.[f;(x;idx)]", rows)
			},
			goFn: func(rows int) int64 {
				var sum, count int64
				for i := 0; i < rows; i++ {
					if i%257 == 0 {
						sum += int64(i)
						count++
					}
				}
				return sum + count
			},
		},
		{
			name:   "TaskDCrossApplyIndexChecksum",
			tags:   []string{"apply-index", "projection", "sum"},
			matrix: []string{"apply-index:list-dict-callable:hot", "list:sublist-cross-cut:string-bool"},
			expr: func(rows int) string {
				side := qEvalVectorCrossSide(rows)
				return fmt.Sprintf("n:%d;p:(til n) cross til n;a:p@%d;b:p . %d;(count p)+(+/a)+(+/b)", side, side+3, side*side-1)
			},
			goFn: func(rows int) int64 {
				return qEvalVectorGoBaselineCrossApplyIndex(rows)
			},
		},
		{
			name:   "TaskDReshapeCycleFillFlipChecksum",
			tags:   []string{"matrix-reshape", "apply-index", "sum"},
			matrix: []string{"matrix:reshape-flip:vector", "apply-index:list-dict-callable:hot"},
			expr: func(rows int) string {
				cols := rows / 4
				return fmt.Sprintf("m:4 %d#1 2 3 4 5;t:flip m;(+/raze t)+(m . 3 %d)+count t", cols, cols-1)
			},
			goFn: func(rows int) int64 {
				source := []int64{1, 2, 3, 4, 5}
				var total int64
				for i := 0; i < rows; i++ {
					total += source[i%len(source)]
				}
				cell := source[(rows-1)%len(source)]
				return total + cell + int64(rows/4)
			},
		},
	}
	return append(cases, taskD...)
}

func appendQEvalTaskDSemanticEnvelopeCases(cases []qEvalVectorCase) []qEvalVectorCase {
	taskD := []qEvalVectorCase{
		{
			name:   "TaskDMathScalarVectorMixedEnvelope",
			tags:   []string{"math-transcendental", "numeric-monad", "numeric-vector", "sum"},
			matrix: []string{"math:transcendental-vector:hot", "numeric-arithmetic:float-vector:hot"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:1+til %d;(+/sqrt x)+log %d", rows, rows)
			},
			goFn: func(rows int) int64 {
				var total float64
				for i := 1; i <= rows; i++ {
					total += math.Sqrt(float64(i))
				}
				total += math.Log(float64(rows))
				qEvalVectorFloatBenchSink = total
				return int64(total)
			},
		},
		{
			name:   "TaskDMathTrigWhereEnvelope",
			tags:   []string{"math-transcendental", "numeric-monad", "where", "sum"},
			matrix: []string{"math:transcendental-vector:hot", "compare:int-vector:where"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:(til %d) mod 12;(+/sin x)+count where x<6", rows)
			},
			goFn: func(rows int) int64 {
				var total float64
				var count int64
				for i := 0; i < rows; i++ {
					x := i % 12
					total += math.Sin(float64(x))
					if x < 6 {
						count++
					}
				}
				qEvalVectorFloatBenchSink = total
				return int64(total) + count
			},
		},
		{
			name:   "TaskDAggregateStatsEnvelope",
			tags:   []string{"sum", "avg-var-dev-med", "min-max", "weighted-aggregate"},
			matrix: []string{"aggregate:running-prd-min-max-avg:vector", "aggregate:wavg-xprev:vector"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:1+til %d;(+/x)+(avg x)+(med x)+(var x)+(dev x)+(wavg[x;x])", rows)
			},
			goFn: func(rows int) int64 {
				var sum, sumSquares float64
				for i := 1; i <= rows; i++ {
					x := float64(i)
					sum += x
					sumSquares += x * x
				}
				avg := sum / float64(rows)
				med := float64(rows+1) / 2
				variance := sumSquares/float64(rows) - avg*avg
				dev := math.Sqrt(variance)
				wavg := sumSquares / sum
				total := sum + avg + med + variance + dev + wavg
				qEvalVectorFloatBenchSink = total
				return int64(total)
			},
		},
		{
			name:   "TaskDWindowScanEnvelope",
			tags:   []string{"sums", "adverb-each-prior", "moving-window", "sum"},
			matrix: []string{"aggregate:running-prd-min-max-avg:vector", "membership:in-differ-ratios:vector"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:1+til %d;(+/sums x)+(+/deltas x)+(+/ratios x)+(+/32 msum x)+(+/32 mavg x)", rows)
			},
			goFn: func(rows int) int64 {
				var running, sumsTotal, deltasTotal, msumTotal int64
				var ratiosTotal, mavgTotal float64
				var prev int64
				for i := 1; i <= rows; i++ {
					x := int64(i)
					running += x
					sumsTotal += running
					if i == 1 {
						deltasTotal += x
					} else {
						deltasTotal++
						ratiosTotal += float64(x) / float64(prev)
					}
					prev = x
					start := i - 31
					if start < 1 {
						start = 1
					}
					var window int64
					for j := start; j <= i; j++ {
						window += int64(j)
					}
					msumTotal += window
					mavgTotal += float64(window) / float64(i-start+1)
				}
				qEvalVectorFloatBenchSink = ratiosTotal + mavgTotal
				return sumsTotal + deltasTotal + int64(ratiosTotal) + 1 + msumTotal + int64(mavgTotal)
			},
		},
		{
			name:   "TaskDApplyAtGatherWindowEnvelope",
			tags:   []string{"apply-index", "where", "projection", "sum"},
			matrix: []string{"apply-index:list-dict-callable:hot", "compare:int-vector:where"},
			shapes: []string{"index:gather-after-where:row-scaled"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;idx:where (x mod 64)<4;y:x@idx;(+/y)+(x@(%d-1))+count y", rows, rows)
			},
			goFn: func(rows int) int64 {
				var total, count int64
				for i := 0; i < rows; i++ {
					if i%64 < 4 {
						total += int64(i)
						count++
					}
				}
				return total + int64(rows-1) + count
			},
		},
		{
			name:   "TaskDListDotDictLookupEnvelope",
			tags:   []string{"apply-index", "dict", "projection", "sum"},
			matrix: []string{"apply-index:list-dict-callable:hot", "dict:lookup-amend-upsert:symbol-key"},
			expr: func(rows int) string {
				return fmt.Sprintf("d:`px`qty!(til %d;1+til %d);px:d`px;qty:d`qty;(+/px)+(+/qty)+(px . 10)", rows, rows)
			},
			goFn: func(rows int) int64 {
				var px, qty int64
				for i := 0; i < rows; i++ {
					px += int64(i)
					qty += int64(i + 1)
				}
				return px + qty + 10
			},
		},
		{
			name:   "TaskDReshapeFlipProbeEnvelope",
			tags:   []string{"matrix-reshape", "apply-index", "sum"},
			matrix: []string{"matrix:reshape-flip:vector", "apply-index:list-dict-callable:hot"},
			expr: func(rows int) string {
				return fmt.Sprintf("m:4 %d#til %d;t:flip m;(m . 1 3)+count t", rows/4, rows)
			},
			goFn: func(rows int) int64 {
				cols := rows / 4
				flipped := make([]int64, rows)
				for c := 0; c < cols; c++ {
					for r := 0; r < 4; r++ {
						flipped[c*4+r] = qEvalVectorGoBaselineInput[r*cols+c]
					}
				}
				qEvalVectorAnyBenchSink = flipped
				return flipped[3*4+1] + int64(cols)
			},
		},
		{
			name:   "TaskDStringLowerUpperLikeEnvelope",
			tags:   []string{"string", "match-like", "symbol", "where"},
			matrix: []string{"compare:string-vector:where", "compare:symbol-vector:where"},
			expr: func(rows int) string {
				return fmt.Sprintf("s:%d#`aapl`MSFT`amd`NVDA;u:upper string s;l:lower string s;(count where u like \"A*\")+(count where l like \"m*\")+(count u)+(count l)", rows)
			},
			goFn: func(rows int) int64 {
				var aHits, mHits int64
				values := []string{"aapl", "MSFT", "amd", "NVDA"}
				for i := 0; i < rows; i++ {
					upper := strings.ToUpper(values[i%len(values)])
					lower := strings.ToLower(values[i%len(values)])
					if strings.HasPrefix(upper, "A") {
						aHits++
					}
					if strings.HasPrefix(lower, "m") {
						mHits++
					}
				}
				return aHits + mHits + int64(rows*2)
			},
		},
		{
			name:   "TaskDSymbolGroupSortEnvelope",
			tags:   []string{"symbol", "group", "table-sort", "sum"},
			matrix: []string{"sort:symbol-vector:index-rank", "compare:symbol-vector:where"},
			expr: func(rows int) string {
				return fmt.Sprintf("s:%d#`MSFT`AAPL`NVDA`AAPL`AMD;(+/iasc s)+(+/rank s)+(count group s)", rows)
			},
			goFn: func(rows int) int64 {
				values := []string{"MSFT", "AAPL", "NVDA", "AAPL", "AMD"}
				repeated := make([]string, rows)
				for i := 0; i < rows; i++ {
					repeated[i] = values[i%len(values)]
				}
				iasc := sortedIndexStringForQEvalVectorGoBaseline(repeated, true)
				rank := rankStringForQEvalVectorGoBaseline(repeated)
				return sumIntSliceForQEvalVectorGoBaseline(iasc) + sumIntSliceForQEvalVectorGoBaseline(rank) + 4
			},
		},
	}
	return append(cases, taskD...)
}

func appendQEvalTaskDColumnarAnalyticsWorkloadCases(cases []qEvalVectorCase) []qEvalVectorCase {
	workloads := []qEvalVectorCase{
		{
			name:   "TaskDColumnarSelectWhereByNotional",
			tags:   []string{"table-literal", "where", "projection", "group", "sum", "symbol"},
			matrix: []string{"table:literal-meta-cols:frame", "compare:int-vector:where", "aggregate:running-prd-min-max-avg:vector"},
			shapes: []string{"where:gather-reduce-selectivity:row-scaled", "group:fby-aggregate:row-scaled"},
			expr: func(rows int) string {
				return fmt.Sprintf("px:100+((til %d) mod 80);sz:1+((til %d) mod 50);sym:%d#`AAPL`MSFT`NVDA`TSLA;idx:where px>=140;vals:(100+(idx mod 80))*(1+(idx mod 50));n:@[%d#0;idx;+;vals];s:sum n fby sym;(+/s)+count idx", rows, rows, rows, rows)
			},
			goFn: func(rows int) int64 {
				var groupSums, groupCounts [4]int64
				var selected int64
				for i := 0; i < rows; i++ {
					groupCounts[i%4]++
					px := int64(100 + i%80)
					if px < 140 {
						continue
					}
					sz := int64(1 + i%50)
					group := i % 4
					groupSums[group] += px * sz
					selected++
				}
				var total int64
				for group := range groupSums {
					total += groupSums[group] * groupCounts[group]
				}
				return total + selected
			},
		},
		{
			name:   "TaskDColumnarComputedProjectionBucket",
			tags:   []string{"where", "projection", "xbar", "sum", "numeric-vector"},
			matrix: []string{"numeric-arithmetic:int-vector:hot", "compare:int-vector:where", "temporal:xbar:bucket"},
			shapes: []string{"where:gather-reduce-selectivity:row-scaled"},
			expr: func(rows int) string {
				return fmt.Sprintf("px:100+((til %d) mod 90);sz:1+((til %d) mod 64);idx:where sz>=32;pxi:100+(idx mod 90);szi:1+(idx mod 64);(+/(pxi*szi))+(+/(count idx)#2)+(+/10 xbar pxi)+count idx", rows, rows)
			},
			goFn: func(rows int) int64 {
				var notional, spread, bucket, count int64
				for i := 0; i < rows; i++ {
					px := int64(100 + i%90)
					sz := int64(1 + i%64)
					if sz < 32 {
						continue
					}
					notional += px * sz
					spread += 2
					bucket += floorBucket(px, 10)
					count++
				}
				return notional + spread + bucket + count
			},
		},
		{
			name:   "TaskDColumnarNullAwareGroupNotional",
			tags:   []string{"typed-null", "null-verb", "fill", "where", "group", "sum", "symbol"},
			matrix: []string{"numeric-arithmetic:typed-null:hot", "list:prev-next-deltas-fills:typed-null", "aggregate:running-prd-min-max-avg:vector"},
			shapes: []string{"null:fill-arithmetic-where:row-scaled", "group:fby-aggregate:row-scaled"},
			expr: func(rows int) string {
				return fmt.Sprintf("px:%d#100 0Ni 105 110 0Ni 115;sz:1+((til %d) mod 20);sym:%d#`AAPL`MSFT`NVDA`TSLA;clean:0^px;idx:where not null px;vals:(clean[idx])*(1+(idx mod 20));n:@[%d#0;idx;+;vals];s:sum n fby sym;(+/s)+count where null px", rows, rows, rows, rows)
			},
			goFn: func(rows int) int64 {
				values := []int64{100, 0, 105, 110, 0, 115}
				nulls := map[int]bool{1: true, 4: true}
				var groupSums, groupCounts [4]int64
				var nullCount int64
				for i := 0; i < rows; i++ {
					groupCounts[i%4]++
					slot := i % len(values)
					if nulls[slot] {
						nullCount++
						continue
					}
					sz := int64(1 + i%20)
					group := i % 4
					groupSums[group] += values[slot] * sz
				}
				var total int64
				for group := range groupSums {
					total += groupSums[group] * groupCounts[group]
				}
				return total + nullCount
			},
		},
		{
			name:   "TaskDColumnarWhereWindowNotional",
			tags:   []string{"where", "projection", "moving-window", "sum", "avg-var-dev-med"},
			matrix: []string{"compare:int-vector:where", "aggregate:running-prd-min-max-avg:vector"},
			shapes: []string{"where:gather-reduce-selectivity:row-scaled", "moving:sum-avg:row-scaled"},
			expr: func(rows int) string {
				return fmt.Sprintf("px:100+((til %d) mod 50);sz:1+((til %d) mod 32);idx:where (px>=120) and (sz>=10);vals:(100+(idx mod 50))*(1+(idx mod 32));v:@[%d#0;idx;+;vals];(+/16 msum v)+(+/16 mavg v)+count idx", rows, rows, rows)
			},
			goFn: func(rows int) int64 {
				values := make([]int64, 0, rows)
				var selected int64
				for i := 0; i < rows; i++ {
					px := int64(100 + i%50)
					sz := int64(1 + i%32)
					value := int64(0)
					if px >= 120 && sz >= 10 {
						value = px * sz
						selected++
					}
					values = append(values, value)
				}
				var msumTotal int64
				var mavgTotal float64
				for i := range values {
					start := i - 15
					if start < 0 {
						start = 0
					}
					var window int64
					for j := start; j <= i; j++ {
						window += values[j]
					}
					msumTotal += window
					mavgTotal += float64(window) / float64(i-start+1)
				}
				qEvalVectorFloatBenchSink = mavgTotal
				return msumTotal + int64(mavgTotal) + selected
			},
		},
		{
			name:   "TaskDColumnarSymbolBucketPipeline",
			tags:   []string{"membership", "where", "projection", "xbar", "group", "sum", "symbol"},
			matrix: []string{"membership:in-differ-ratios:vector", "compare:symbol-vector:where", "temporal:xbar:bucket"},
			shapes: []string{"membership:symbol-filter:row-scaled", "group:fby-aggregate:row-scaled"},
			expr: func(rows int) string {
				return fmt.Sprintf("px:90+((til %d) mod 120);sz:1+((til %d) mod 40);sym:%d#`AAPL`MSFT`NVDA`TSLA;bucket:25 xbar px;idx:where (sym in `AAPL`MSFT) and (px within 120 180);vals:(90+(idx mod 120))*(1+(idx mod 40));n:@[%d#0;idx;+;vals];s:sum n fby bucket;(+/s)+count idx", rows, rows, rows, rows)
			},
			goFn: func(rows int) int64 {
				groupSums := map[int64]int64{}
				groupCounts := map[int64]int64{}
				var selected int64
				for i := 0; i < rows; i++ {
					px := int64(90 + i%120)
					bucket := floorBucket(px, 25)
					groupCounts[bucket]++
					if !(i%4 == 0 || i%4 == 1) || px < 120 || px > 180 {
						continue
					}
					sz := int64(1 + i%40)
					groupSums[bucket] += px * sz
					selected++
				}
				var total int64
				for bucket, sum := range groupSums {
					total += sum * groupCounts[bucket]
				}
				return total + selected
			},
		},
	}
	return append(cases, workloads...)
}

func appendQEvalSupportedExpressionBreadthCases(cases []qEvalVectorCase) []qEvalVectorCase {
	for _, p := range []struct {
		name string
		mod  int64
		bias int64
	}{
		{"Mod3Bias1", 3, 1},
		{"Mod5Bias2", 5, 2},
		{"Mod7Bias4", 7, 4},
		{"Mod11Bias6", 11, 6},
	} {
		p := p
		cases = append(cases,
			qEvalVectorCase{
				name:   "BreadthArithmeticDivModEnvelope" + p.name,
				tags:   []string{"numeric-vector", "word-dyadic", "sum"},
				matrix: []string{"numeric-arithmetic:int-vector:hot"},
				expr: func(rows int) string {
					return fmt.Sprintf("x:til %d;y:x+%d;(+/y div %d)+(+/y mod %d)+count y", rows, p.bias, p.mod, p.mod)
				},
				goFn: func(rows int) int64 {
					var total int64
					for i := int64(0); i < int64(rows); i++ {
						y := i + p.bias
						total += y/p.mod + qPositiveMod(y, p.mod)
					}
					return total + int64(rows)
				},
			},
			qEvalVectorCase{
				name:   "BreadthArithmeticAbsNegSignum" + p.name,
				tags:   []string{"numeric-vector", "numeric-monad", "sum"},
				matrix: []string{"numeric-arithmetic:int-vector:hot"},
				expr: func(rows int) string {
					return fmt.Sprintf("x:(til %d)-%d;(+/abs x)+(+/neg x)+(+/signum x)", rows, rows/2)
				},
				goFn: func(rows int) int64 {
					var total int64
					offset := int64(rows / 2)
					for i := int64(0); i < int64(rows); i++ {
						x := i - offset
						if x < 0 {
							total -= x
						} else {
							total += x
						}
						total -= x
						switch {
						case x < 0:
							total--
						case x > 0:
							total++
						}
					}
					return total
				},
			},
			qEvalVectorCase{
				name:   "BreadthFloatFloorCeilingReciprocal" + p.name,
				tags:   []string{"numeric-vector", "numeric-monad", "promotion", "sum"},
				matrix: []string{"numeric-arithmetic:float-vector:hot"},
				expr: func(rows int) string {
					return fmt.Sprintf("x:1+(til %d)%%%d;(+/floor x)+(+/ceiling x)+(+/reciprocal x)", rows, p.mod)
				},
				goFn: func(rows int) int64 {
					var total float64
					for i := 0; i < rows; i++ {
						x := 1 + float64(i)/float64(p.mod)
						total += math.Floor(x) + math.Ceil(x) + 1/x
					}
					return int64(total)
				},
			},
		)
	}

	for _, p := range []struct {
		name string
		expr string
		goFn func(rows int) int64
	}{
		{
			name: "BreadthAggregateAvgMedRange",
			expr: "x:1+til %d;(avg x)+(med x)+(min x)+(max x)+count x",
			goFn: func(rows int) int64 {
				return qEvalVectorGoBaselineAggregateAvgMedRange(rows, qEvalVectorGoBaselineInput)
			},
		},
		{
			name: "BreadthAggregateProductOnesAndWavg",
			expr: "x:%d#1;(prd x)+(count prds x)+(wavg[1 2 3;10 20 30])",
			goFn: func(rows int) int64 {
				return qEvalVectorGoBaselineAggregateProductWavg(rows, qEvalVectorGoBaselineInput)
			},
		},
		{
			name: "BreadthRunningMinMaxAvgEnvelope",
			expr: "x:1+til %d;(last sums x)+(last mins x)+(last maxs x)+(last avgs x)",
			goFn: func(rows int) int64 {
				return qEvalVectorGoBaselineRunningMinMaxAvgEnvelope(rows, qEvalVectorGoBaselineInput)
			},
		},
		{
			name: "BreadthMovingWindowEnvelope",
			expr: "x:1+til %d;(+/5 msum x)+(+/5 mavg x)+(+/5 mcount x)",
			goFn: func(rows int) int64 {
				var sumWindow int64
				var avgWindow float64
				var countWindow int64
				for i := 0; i < rows; i++ {
					start := i - 4
					if start < 0 {
						start = 0
					}
					var window int64
					for j := start; j <= i; j++ {
						window += int64(j + 1)
					}
					width := int64(i - start + 1)
					sumWindow += window
					avgWindow += float64(window) / float64(width)
					countWindow += width
				}
				return sumWindow + int64(avgWindow) + countWindow
			},
		},
	} {
		p := p
		cases = append(cases, qEvalVectorCase{
			name:   p.name,
			tags:   []string{"sum", "avg-var-dev-med", "running-aggregate", "moving-window", "weighted-aggregate"},
			matrix: []string{"aggregate:running-prd-min-max-avg:vector", "aggregate:wavg-xprev:vector"},
			expr: func(rows int) string {
				return fmt.Sprintf(p.expr, rows)
			},
			goFn: p.goFn,
		})
	}

	for _, p := range []struct {
		name  string
		take  int
		drop  int
		shift int
	}{
		{"Short", 32, 3, 5},
		{"Mid", 511, 17, 97},
		{"Long", 2048, 257, 1021},
		{"Cycle", 9001, 1024, 2049},
	} {
		p := p
		cases = append(cases,
			qEvalVectorCase{
				name:   "BreadthListFirstLastReverseRotate" + p.name,
				tags:   []string{"first", "reverse", "rotate", "sum"},
				matrix: []string{"list:cut-raze-enlist:nested"},
				expr: func(rows int) string {
					return fmt.Sprintf("x:til %d;y:%d rotate reverse x;first y+last y+(+/y)", rows, p.shift)
				},
				goFn: func(rows int) int64 {
					shift := p.shift % rows
					var first, last, sum int64
					for i := 0; i < rows; i++ {
						reverseIndex := (shift + i) % rows
						value := int64(rows - 1 - reverseIndex)
						if i == 0 {
							first = value
						}
						if i == rows-1 {
							last = value
						}
						sum += value
					}
					return first + last + sum
				},
			},
			qEvalVectorCase{
				name:   "BreadthListDropTakeSublist" + p.name,
				tags:   []string{"drop", "take", "cut", "sum"},
				matrix: []string{"list:sublist-cross-cut:string-bool", "list:cut-raze-enlist:nested"},
				expr: func(rows int) string {
					return fmt.Sprintf("x:til %d;y:%d#drop %d x;z:%d sublist y;(+/z)+count z", rows, p.take, p.drop, p.take/2)
				},
				goFn: func(rows int) int64 {
					dropped := qEvalVectorGoBaselineMaterializeDrop(rows, p.drop)
					sub := p.take / 2
					var sum int64
					for i := 0; i < sub; i++ {
						sum += dropped[i%len(dropped)]
					}
					return sum + int64(sub)
				},
			},
			qEvalVectorCase{
				name:   "BreadthListCutRazeChecksum" + p.name,
				tags:   []string{"cut", "raze", "sum"},
				matrix: []string{"list:cut-raze-enlist:nested"},
				expr: func(rows int) string {
					a := p.drop
					b := p.drop + p.take/2
					c := p.drop + p.take
					if c >= rows {
						c = rows - 1
					}
					return fmt.Sprintf("x:til %d;g:%d %d %d cut x;(+/raze g)+count g", rows, a, b, c)
				},
				goFn: func(rows int) int64 {
					var sum int64
					for i := p.drop; i < rows; i++ {
						sum += int64(i)
					}
					return sum + 3
				},
			},
		)
	}

	for _, p := range []struct {
		name  string
		width int
	}{
		{"Tiny", 4},
		{"Small", 8},
		{"Medium", 16},
		{"Wide", 32},
	} {
		p := p
		cases = append(cases,
			qEvalVectorCase{
				name:   "BreadthApplyAtGather" + p.name,
				tags:   []string{"apply-index", "projection", "sum"},
				matrix: []string{"apply-index:list-dict-callable:hot"},
				expr: func(rows int) string {
					return fmt.Sprintf("x:til %d;(x@%d)+(x@%d)+(x@%d)+count x", rows, p.width, p.width*2, p.width*3)
				},
				goFn: func(rows int) int64 {
					return qEvalVectorGoBaselineApplyAtGather(rows, p.width, qEvalVectorGoBaselineInput)
				},
			},
			qEvalVectorCase{
				name:   "BreadthApplyBracketGather" + p.name,
				tags:   []string{"apply-index", "projection", "sum"},
				matrix: []string{"apply-index:list-dict-callable:hot"},
				expr: func(rows int) string {
					return fmt.Sprintf("x:til %d;idx:%d*til %d;(+/x[idx])+count idx", rows, p.width, rows/p.width)
				},
				goFn: func(rows int) int64 {
					x := qEvalVectorGoBaselineMaterializeTil(rows)
					n := rows / p.width
					var sum int64
					for i := 0; i < n; i++ {
						sum += x[p.width*i]
					}
					return sum + int64(n)
				},
			},
			qEvalVectorCase{
				name:   "BreadthCallableDotApply" + p.name,
				tags:   []string{"apply-index", "projection", "sum"},
				matrix: []string{"apply-index:list-dict-callable:hot"},
				expr: func(rows int) string {
					return fmt.Sprintf("f:{(+/x)+count y};.[f;(til %d;%d#1)]", rows, p.width)
				},
				goFn: func(rows int) int64 {
					return qEvalVectorGoBaselineCallableDotApply(rows, p.width, qEvalVectorGoBaselineInput)
				},
			},
		)
	}

	for _, p := range []struct {
		name string
		rows int
		cols int
	}{
		{"TwoByN", 2, 4096},
		{"FourByN", 4, 2048},
		{"EightByN", 8, 1024},
	} {
		p := p
		cellIndex := p.cols / 2
		cases = append(cases,
			qEvalVectorCase{
				name:   "BreadthMatrixReshapeRowSum" + p.name,
				tags:   []string{"matrix-reshape", "apply-index", "sum"},
				matrix: []string{"matrix:reshape-flip:vector", "apply-index:list-dict-callable:hot"},
				expr: func(rows int) string {
					total := p.rows * p.cols
					return fmt.Sprintf("m:%d %d#til %d;r:m . %d;(+/r)+count r", p.rows, p.cols, total, p.rows-1)
				},
				goFn: func(rows int) int64 {
					start := (p.rows - 1) * p.cols
					var sum int64
					for i := 0; i < p.cols; i++ {
						sum += int64(start + i)
					}
					return sum + int64(p.cols)
				},
			},
			qEvalVectorCase{
				name:   "BreadthMatrixReshapeCellProbe" + p.name,
				tags:   []string{"matrix-reshape", "apply-index"},
				matrix: []string{"matrix:reshape-flip:vector", "apply-index:list-dict-callable:hot"},
				expr: func(rows int) string {
					total := p.rows * p.cols
					return fmt.Sprintf("m:%d %d#til %d;(m . %d %d)+count m", p.rows, p.cols, total, p.rows-1, cellIndex)
				},
				goFn: func(rows int) int64 {
					return qEvalVectorGoBaselineMatrixCellProbe(p.rows, p.cols, cellIndex, qEvalVectorGoBaselineInput)
				},
			},
		)
	}

	for _, p := range []struct {
		name    string
		pattern string
		order   []int
		width   int
		hits    map[int]bool
	}{
		{"SymbolsA", "`aapl`msft`amd`nvda", []int{0, 2, 1, 3}, 4, map[int]bool{0: true, 2: true}},
		{"SymbolsN", "`ibm`orcl`nvda`tsla`nflx", []int{0, 4, 2, 1, 3}, 5, map[int]bool{}},
		{"VenuesX", "`xnys`xnas`bats`arcx", []int{3, 2, 1, 0}, 4, map[int]bool{3: true}},
	} {
		p := p
		cases = append(cases,
			qEvalVectorCase{
				name:   "BreadthStringUpperLowerLike" + p.name,
				tags:   []string{"string", "symbol", "match-like", "where"},
				matrix: []string{"compare:string-vector:where", "compare:symbol-vector:where"},
				shapes: []string{"string:symbol-string-like:row-scaled"},
				expr: func(rows int) string {
					return fmt.Sprintf("s:%d#%s;u:upper string s;l:lower u;(count where u like \"A*\")+(count l)", rows, p.pattern)
				},
				goFn: func(rows int) int64 {
					return qPatternCount(rows, p.width, p.hits) + int64(rows)
				},
			},
			qEvalVectorCase{
				name:   "BreadthSymbolDistinctGroupSort" + p.name,
				tags:   []string{"symbol", "distinct", "group", "table-sort"},
				matrix: []string{"set:symbol-vector:union-inter-except", "sort:symbol-vector:index-rank"},
				expr: func(rows int) string {
					return fmt.Sprintf("s:%d#%s;(count distinct s)+(count group s)+(+/iasc s)", rows, p.pattern)
				},
				goFn: func(rows int) int64 {
					indexes := make([]int, rows)
					for i := range indexes {
						indexes[i] = i
					}
					sort.SliceStable(indexes, func(i, j int) bool {
						left := p.order[indexes[i]%p.width]
						right := p.order[indexes[j]%p.width]
						if left == right {
							return indexes[i] < indexes[j]
						}
						return left < right
					})
					var sum int64
					for _, idx := range indexes {
						sum += int64(idx)
					}
					return int64(p.width) + int64(p.width) + sum
				},
			},
		)
	}

	return cases
}

func qPositiveMod(v, mod int64) int64 {
	out := v % mod
	if out < 0 {
		out += mod
	}
	return out
}

func qPatternCount(rows, width int, hits map[int]bool) int64 {
	var count int64
	for i := 0; i < rows; i++ {
		if hits[i%width] {
			count++
		}
	}
	return count
}

func qEvalRepeatedTrimmedStringLen(rows int, pattern string) int64 {
	if rows <= 0 || pattern == "" {
		return 0
	}
	at := func(i int) byte { return pattern[i%len(pattern)] }
	start := 0
	for start < rows && qEvalIsASCIISpace(at(start)) {
		start++
	}
	end := rows - 1
	for end >= start && qEvalIsASCIISpace(at(end)) {
		end--
	}
	return int64(end - start + 1)
}

func qEvalIsASCIISpace(ch byte) bool {
	switch ch {
	case ' ', '\t', '\n', '\r', '\v', '\f':
		return true
	default:
		return false
	}
}

func appendQEvalSemanticCoverageCases(cases []qEvalVectorCase) []qEvalVectorCase {
	coverage := []qEvalVectorCase{
		{
			name: "TypedSuffixNullAndCasts",
			tags: []string{"typed-suffix", "typed-null", "cast", "promotion", "null-verb"},
			matrix: []string{
				"numeric-arithmetic:typed-null:hot",
				"cast:typed-numeric:scalar-vector",
			},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;shorts:`short$x;floats:`float$x;nulls:%d#1 0N 2 0N;(+/shorts)+(type 1i)+(type `float$1)+(count where null nulls)", rows, rows)
			},
			goFn: func(rows int) int64 {
				var sum int64
				for i := 0; i < rows; i++ {
					sum += int64(int16(i))
				}
				var nullCount int64
				for i := 0; i < rows; i++ {
					if i%4 == 1 || i%4 == 3 {
						nullCount++
					}
				}
				return sum + int64(-6) + int64(-9) + nullCount
			},
		},
		{
			name: "NumericMonadsAndWordDyadics",
			tags: []string{"numeric-monad", "word-dyadic"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;(+/abs -1*x)+(+/neg x)+(+/floor x*1.5)+(+/ceiling x*1.5)+((sum x) plus 10)", rows)
			},
			goFn: func(rows int) int64 {
				var absValues, negValues, floors, ceilings, sum int64
				for i := 0; i < rows; i++ {
					value := int64(i)
					absValues += value
					negValues -= value
					sum += value
					scaled := float64(value) * 1.5
					floors += int64(scaled)
					ceiling := int64(scaled)
					if float64(ceiling) != scaled {
						ceiling++
					}
					ceilings += ceiling
				}
				return absValues + negValues + floors + ceilings + sum + 10
			},
		},
		{
			name:   "BooleanLogicalAndCompositeCompare",
			tags:   []string{"boolean-logical", "composite-compare", "where"},
			matrix: []string{"compare:int-vector:where"},
			expr: func(rows int) string {
				return "(count where true false true and true true false)+(count where 10 20 30>=20)+(count where 10 20 30<>20)"
			},
			goFn: func(rows int) int64 {
				return 1 + 2 + 2
			},
		},
		{
			name:   "CutEnlistRazeChecksum",
			tags:   []string{"cut", "enlist", "raze"},
			matrix: []string{"list:cut-raze-enlist:nested"},
			expr: func(rows int) string {
				return "(count 0 2 4_10 20 30 40 50)+(count enlist 10 20 30)+sum raze (1 2;3 4;5)"
			},
			goFn: func(rows int) int64 {
				cut := [][]int64{{10, 20}, {30, 40}, {50}}
				enlisted := [][]int64{{10, 20, 30}}
				nested := [][]int64{{1, 2}, {3, 4}, {5}}
				var sum int64
				for _, group := range nested {
					for _, value := range group {
						sum += value
					}
				}
				return int64(len(cut)) + int64(len(enlisted)) + sum
			},
		},
		{
			name: "MinMaxAvgVarDevMed",
			tags: []string{"min-max", "avg-var-dev-med"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;(min x)+(max x)+(avg x)", rows)
			},
			goFn: func(rows int) int64 {
				var sum int64
				for i := 0; i < rows; i++ {
					sum += int64(i)
				}
				avg := float64(sum) / float64(rows)
				return int64(0) + int64(rows-1) + int64(avg)
			},
		},
		{
			name: "MovingWindowAggregates",
			tags: []string{"moving-window"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:1+til %d;(+/3 mcount x)+(+/3 mmin x)+(+/3 mmax x)", rows)
			},
			goFn: func(rows int) int64 {
				var mcountSum, mminSum, mmaxSum int64
				for i := 0; i < rows; i++ {
					window := i + 1
					if window > 3 {
						window = 3
					}
					mcountSum += int64(window)
					mminSum += int64(i - window + 2)
					mmaxSum += int64(i + 1)
				}
				return mcountSum + mminSum + mmaxSum
			},
		},
		{
			name:   "RunningAggregateProductMatrix",
			tags:   []string{"running-aggregate", "product", "avg-var-dev-med", "min-max", "pipeline-vector-last-scan"},
			matrix: []string{"aggregate:running-prd-min-max-avg:vector"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:1+til %d;p:%d#1 1 1 1;(prd p)+(count prds p)+(last mins x)+(last maxs x)+(last avgs x)", rows, rows)
			},
			goFn: func(rows int) int64 {
				var sum int64
				for i := 1; i <= rows; i++ {
					sum += int64(i)
				}
				avg := float64(sum) / float64(rows)
				return 1 + int64(rows) + 1 + int64(rows) + int64(avg)
			},
		},
		{
			name:   "RunningAggregateCountTerminalProjection",
			tags:   []string{"running-aggregate", "product", "avg-var-dev-med", "min-max"},
			matrix: []string{"aggregate:running-prd-min-max-avg:vector"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:1+til %d;(count sums x)+(count prds x)+(count mins x)+(count maxs x)+(count avgs x)", rows)
			},
			goFn: func(rows int) int64 {
				sums := make([]int64, rows)
				prds := make([]float64, rows)
				mins := make([]int64, rows)
				maxs := make([]int64, rows)
				avgs := make([]float64, rows)
				var running int64
				product := 1.0
				minValue := int64(math.MaxInt64)
				maxValue := int64(math.MinInt64)
				for i := 0; i < rows; i++ {
					value := qEvalVectorGoBaselineInput[i] + 1
					running += value
					sums[i] = running
					product *= float64(value)
					prds[i] = product
					if value < minValue {
						minValue = value
					}
					if value > maxValue {
						maxValue = value
					}
					mins[i] = minValue
					maxs[i] = maxValue
					avgs[i] = float64(running) / float64(i+1)
				}
				qEvalVectorAnyBenchSink = [][]int64{sums, mins, maxs}
				qEvalVectorFloatBenchSink = prds[rows-1] + avgs[rows-1]
				return int64(len(sums) + len(prds) + len(mins) + len(maxs) + len(avgs))
			},
		},
		{
			name:   "MembershipDifferRatiosTruthMatrix",
			tags:   []string{"membership", "boolean-logical", "numeric-vector"},
			matrix: []string{"membership:in-differ-ratios:vector"},
			expr: func(rows int) string {
				return "(count where differ `a`a`b`b`c)+(count ratios 2 4 8 16)+(count where all 1 1 1)+(count where any 0 0 1)+(count where (1 0 1) in 1 2)"
			},
			goFn: func(rows int) int64 {
				syms := []string{"a", "a", "b", "b", "c"}
				differCount := int64(0)
				for i, sym := range syms {
					if i == 0 || sym != syms[i-1] {
						differCount++
					}
				}
				ratios := []float64{2, 4, 8, 16}
				ratioCount := int64(len(ratios))
				allCount := int64(0)
				all := true
				for _, value := range []int64{1, 1, 1} {
					all = all && value != 0
				}
				if all {
					allCount = 1
				}
				anyCount := int64(0)
				for _, value := range []int64{0, 0, 1} {
					if value != 0 {
						anyCount = 1
						break
					}
				}
				inCount := int64(0)
				for _, value := range []int64{1, 0, 1} {
					if value == 1 || value == 2 {
						inCount++
					}
				}
				return differCount + ratioCount + allCount + anyCount + inCount
			},
		},
		{
			name: "AdverbMatrix",
			tags: []string{"adverb-each", "adverb-each-prior", "adverb-each-left-right", "adverb-over-scan", "projection"},
			matrix: []string{
				"adverb:dyadic-each:vector",
				"adverb:each-prior:vector",
				"adverb:each-left-right:vector",
				"adverb:over-scan:projection",
			},
			expr: func(rows int) string {
				return "(+/1 2 3+'10 20 30)+(+/-':10 15 14 20)+(+/10-\\:1 2 3)+(+/10 20 30-/:1)+(+/+\\1 2 3 4)+({x+y}/[10;1 2 3])"
			},
			goFn: func(rows int) int64 {
				return 66 + 20 + 24 + 57 + 20 + 16
			},
		},
		{
			name:   "SetBinWithinXrank",
			tags:   []string{"set-verb", "bin-within-xrank"},
			matrix: []string{"set:int-vector:union-inter-except"},
			expr: func(rows int) string {
				return "(count 1 2 3 except 2)+(count 1 2 1 3 inter 3 1 9)+(count 1 2 1 union 2 3)+(+/10 20 30 bin 5 10 29 30 31)+(count where 10 20 30 within 15 30)+(+/2 xrank 40 10 30 20)"
			},
			goFn: func(rows int) int64 {
				return 2 + 2 + 3 + 4 + 2 + 2
			},
		},
		{
			name:   "SearchBinrWavgXprevMatrix",
			tags:   []string{"find-bin", "weighted-aggregate"},
			matrix: []string{"search:bin-binr-find:vector", "aggregate:wavg-xprev:vector"},
			expr: func(rows int) string {
				return "(+/10 20 30 binr 5 10 29 30 31)+(+/`AAPL`MSFT`NVDA?`MSFT`TSLA)+(wavg[1 2 3;10 20 30])+(+/2 xprev 10 20 30 40)"
			},
			goFn: func(rows int) int64 {
				return 6 + 4 + 23 + 31
			},
		},
		{
			name:   "SymbolSetVerbs",
			tags:   []string{"set-verb", "symbol"},
			matrix: []string{"set:symbol-vector:union-inter-except"},
			expr: func(rows int) string {
				return "(count `AAPL`MSFT`AAPL except `MSFT)+(count `AAPL`MSFT`AAPL inter `AAPL`GOOG)+(count `AAPL`MSFT union `MSFT`NVDA`AAPL)"
			},
			goFn: func(rows int) int64 {
				return 2 + 1 + 3
			},
		},
		{
			name:   "TypedPrevNextDeltasFills",
			tags:   []string{"typed-null", "adverb-each-prior"},
			matrix: []string{"list:prev-next-deltas-fills:typed-null"},
			expr: func(rows int) string {
				return "(count prev 0Ni 1i 0Ni)+(count next 0Ni 1i 0Ni)+(count deltas 1i 0Ni 3i)+(count fills 0Nf 1.5f 0Nf)"
			},
			goFn: func(rows int) int64 {
				return 3 + 3 + 3 + 3
			},
		},
		{
			name: "SortRankTypeMatrix",
			tags: []string{"table-sort", "symbol", "temporal"},
			matrix: []string{
				"sort:int-vector:index-rank",
				"sort:symbol-vector:index-rank",
				"sort:temporal-vector:index-rank",
			},
			expr: func(rows int) string {
				return "(+/iasc 3 1 2 1)+(+/idesc 3 1 2 1)+(+/rank 2 7 3 2 5)+(first asc 3 1 2)+(first desc 3 1 2)+(+/iasc `MSFT`AAPL`AAPL)+(+/rank `x`a`b`z`c)+(+/iasc 2026.06.07 0Nd 2026.06.06)+(+/rank 2026.06.07 0Nd 2026.06.06)"
			},
			goFn: func(rows int) int64 {
				return qEvalVectorGoBaselineSortRankTypeMatrix(qEvalVectorGoBaselineInput)
			},
		},
		{
			name:   "DictAmendUpsertAndLookup",
			tags:   []string{"dict", "dict-amend-upsert"},
			matrix: []string{"dict:lookup-amend-upsert:symbol-key"},
			expr: func(rows int) string {
				return "d:`a`b!10 20;u:@[d;`c;:;30];d[`a]+(lookup d `b)+((@[d;`a;:;99])`a)+u`c"
			},
			goFn: func(rows int) int64 {
				return 10 + 20 + 99 + 30
			},
		},
		{
			name:   "DictKeysValueAttrFillMatch",
			tags:   []string{"dict", "keys-value", "attrs", "fill", "match"},
			matrix: []string{"dict:keys-value-attrs:meta"},
			expr: func(rows int) string {
				return "(count keys `bid`ask!99 101)+(count value `bid`ask!99 101)+(type attr `p#`AAPL`AAPL`MSFT)+(count 0^0N 1 0N)+(count where (1 2 3~1 2 3;1 2~2 1))"
			},
			goFn: func(rows int) int64 {
				return 2 + 2 + -11 + 3 + 1
			},
		},
		{
			// parse is pure (q source -> nested-list parse tree); counting
			// the trees pins the (verb;arg;...) spelling without touching the
			// session-routed value/eval verbs (value_eval_parse.go). The til
			// term keeps the baseline row-scaled.
			name: "ParseTreeCount",
			tags: []string{"parse"},
			expr: func(rows int) string {
				return fmt.Sprintf(`(count parse "2*3+1")+(count parse "neg 7")+count til %d`, rows)
			},
			goFn: func(rows int) int64 {
				return 3 + 2 + int64(rows)
			},
		},
		{
			name:   "EnumSymbolStringMatchLike",
			tags:   []string{"enum", "symbol", "string", "match-like"},
			matrix: []string{"compare:symbol-vector:where", "compare:string-vector:where"},
			expr: func(rows int) string {
				return "(count domain `sym$`AAPL`MSFT`AAPL)+(count codes `sym$`AAPL`MSFT`AAPL)+(count where `AAPL`MSFT`AMD like `A*)+(count upper `aapl`msft)"
			},
			goFn: func(rows int) int64 {
				return 2 + 3 + 2 + 2
			},
		},
		{
			name:   "TemporalTypedXbarAndSort",
			tags:   []string{"temporal", "xbar"},
			matrix: []string{"compare:temporal-vector:where", "temporal:xbar:bucket"},
			expr: func(rows int) string {
				return "(count 2026.06.06 0Nd 2026.06.07)+(count 00:01 xbar 09:30 09:30:59 09:31:00)+(count where 2026.06.06 2026.06.07>=2026.06.07)"
			},
			goFn: func(rows int) int64 {
				return qEvalVectorGoBaselineTemporalTypedXbarAndSort(qEvalVectorGoBaselineInput)
			},
		},
		{
			name:   "TableMetadataReorderSort",
			tags:   []string{"table-literal", "metadata", "table-reorder", "table-sort"},
			matrix: []string{"table:literal-meta-cols:frame", "table:xcols-xasc-xdesc:frame"},
			expr: func(rows int) string {
				return "(count ([] sym:`AAPL`MSFT;price:100 101))+(count cols `price`sym xcols flip `sym`price`size!(`AAPL`MSFT;100 101;10 20))+(count meta `price xasc flip `sym`price!(`MSFT`AAPL;80 101))"
			},
			goFn: func(rows int) int64 {
				return 2 + 3 + 2
			},
		},
		{
			name:   "KeyedTableGroupUngroupFby",
			tags:   []string{"keyed-table", "table-group-ungroup", "fby", "group"},
			matrix: []string{"table:xkey-xgroup-ungroup:keyed-frame"},
			expr: func(rows int) string {
				return "fb:sum 10 20 30 40 fby `a`a`b`b;(count key `sym xkey flip `sym`price!(`AAPL`MSFT;100 101))+(count ungroup (`sym xgroup flip `sym`price!(`AAPL`AAPL`MSFT;100 101 80)))+(+/fb)"
			},
			goFn: func(rows int) int64 {
				// key of a keyed table is its 2-row key TABLE (canonical).
				return 2 + 3 + 200
			},
		},
		{
			name:   "SafeSystemAndLoopbackIPC",
			tags:   []string{"safe-system", "ipc-loopback"},
			matrix: []string{"ipc:loopback:session"},
			expr: func(rows int) string {
				return "h:hopen \"loopback\";v:\\v;h[(\"x+y\";2;3)]+count v"
			},
			goFn: func(rows int) int64 {
				return 5
			},
		},
		{
			name: "ProjectionEachDictNumericTableCombo",
			tags: []string{"projection", "adverb-each", "dict", "table-literal", "typed-null"},
			matrix: []string{
				"adverb:dyadic-each:vector",
				"dict:lookup-amend-upsert:symbol-key",
				"table:literal-meta-cols:frame",
			},
			expr: func(rows int) string {
				return "d:`px`qty!(100 101 102;10 20 0N);t:flip `sym`px`qty!(`AAPL`MSFT`NVDA;d`px;d`qty);(+/(d`px)+10)+(+/fills d`qty)+(count meta t)"
			},
			goFn: func(rows int) int64 {
				return (110 + 111 + 112) + (10 + 20 + 20) + 3
			},
		},
		{
			name: "TemporalStringSymbolProjectionCombo",
			tags: []string{"projection", "composition", "temporal", "string", "symbol", "match-like", "xbar"},
			matrix: []string{
				"compare:symbol-vector:where",
				"compare:string-vector:where",
				"compare:temporal-vector:where",
				"temporal:xbar:bucket",
			},
			expr: func(rows int) string {
				return "syms:`AAPL`MSFT`AMD`AAPL;names:upper string syms;(count where syms=`AAPL)+(count where names like \"A*\")+(count xbar[00:01;09:30 09:30:59 09:31:01])+(count where 2026.06.06 2026.06.07>=2026.06.07)"
			},
			goFn: func(rows int) int64 {
				return 2 + 3 + 3 + 1
			},
		},
		{
			name: "NestedDictProjectionAmendCombo",
			tags: []string{"projection", "dict", "dict-amend-upsert", "adverb-over-scan", "typed-null"},
			matrix: []string{
				"adverb:over-scan:projection",
				"dict:lookup-amend-upsert:symbol-key",
			},
			expr: func(rows int) string {
				return "sum2:{x+y}/;d:`a`b!(1 2 0Ni;10 20 30);u:@[d;`a;:;fills d`a];(+/u`a)+sum2[d`b]+((@[d;`c;:;40])`c)"
			},
			goFn: func(rows int) int64 {
				return (1 + 2 + 2) + (10 + 20 + 30) + 40
			},
		},
		{
			name: "KeyedTableTemporalStringMetaCombo",
			tags: []string{"keyed-table", "table-reorder", "table-sort", "metadata", "temporal", "string", "symbol"},
			matrix: []string{
				"table:xcols-xasc-xdesc:frame",
				"table:xkey-xgroup-ungroup:keyed-frame",
				"sort:temporal-vector:index-rank",
			},
			expr: func(rows int) string {
				return "t:`time xasc flip `sym`time`venue!(`MSFT`AAPL`AAPL;09:31 09:30 09:32;`xnys`xnas`bats);k:`sym xkey (`venue`time`sym xcols t);(count key k)+(count meta k)+(count cols k)+(+/iasc t`time)+(count upper string t`sym)"
			},
			goFn: func(rows int) int64 {
				// key of a keyed table is its 3-row key TABLE (canonical).
				return 3 + 3 + 3 + 3 + 3
			},
		},
	}
	cases = append(cases, coverage...)
	cases = append(cases, qEvalVectorVerbFormBacklogCases()...)
	cases = append(cases, qEvalVectorJoinCases()...)
	cases = append(cases, qEvalVectorComboCases()...)
	cases = append(cases, qEvalVectorTypeNullMatrixCases()...)
	return cases
}

func TestQEvalVectorBenchmarkExpressions(t *testing.T) {
	if len(qEvalVectorCases) < 480 {
		t.Fatalf("q.eval benchmark coverage too small: got %d cases, want at least 480", len(qEvalVectorCases))
	}
	eval := qEvalVectorEval(t)
	for _, tc := range qEvalVectorCases {
		t.Run(tc.name, func(t *testing.T) {
			got := qEvalVectorRun(t, eval, tc.expr(qEvalVectorRows))
			if want := tc.goFn(qEvalVectorRows); got != want {
				t.Fatalf("q.eval checksum = %d, want Go baseline %d", got, want)
			}
		})
	}
}

func TestQEvalVectorListStringBooleanFallbackCharacterization(t *testing.T) {
	eval := qEvalVectorEval(t)
	rows := qEvalVectorRows
	cases := []struct {
		name string
		src  string
		want int64
	}{
		{
			name: "combined",
			src:  fmt.Sprintf("x:til %d;(count 100 sublist x)+(count 0 100 200 cut x)+(count (til %d) cross til %d)+(count trim %d#\" AAPL \")+(count where 101001b)", rows, qEvalVectorCrossSide(rows), qEvalVectorCrossSide(rows), rows),
			want: 100 + 3 + qEvalVectorGoBaselineCrossCount(rows) + qEvalRepeatedTrimmedStringLen(rows, " AAPL ") + 3,
		},
		{name: "sublist", src: fmt.Sprintf("x:til %d;count 100 sublist x", rows), want: 100},
		{name: "cut", src: fmt.Sprintf("x:til %d;count 0 100 200 cut x", rows), want: 3},
		{name: "cross", src: fmt.Sprintf("count (til %d) cross til %d", qEvalVectorCrossSide(rows), qEvalVectorCrossSide(rows)), want: qEvalVectorGoBaselineCrossCount(rows)},
		{name: "trim", src: fmt.Sprintf("count trim %d#\" AAPL \"", rows), want: qEvalRepeatedTrimmedStringLen(rows, " AAPL ")},
		{name: "where", src: "count where 101001b", want: 3},
	}

	fallbacks := make(map[string]uint64)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stdq.ClearRuntimeKernelExecutionStats()
			got := qEvalVectorRun(t, eval, tc.src)
			if got != tc.want {
				t.Fatalf("q.eval checksum = %d, want %d", got, tc.want)
			}
			fallbacks[tc.name] = qEvalVectorRuntimeFallbackCount()
			t.Logf("runtime stats: %#v", stdq.RuntimeKernelExecutionStats())
		})
	}
	stdq.ClearRuntimeKernelExecutionStats()

	if fallbacks["sublist"] != 0 || fallbacks["cut"] != 0 || fallbacks["cross"] != 0 || fallbacks["where"] != 0 {
		t.Fatalf("unexpected split primitive fallback counts: %#v", fallbacks)
	}
	if fallbacks["combined"] > fallbacks["trim"] {
		t.Fatalf("combined fallback count = %d exceeds trim fallback count = %d; split fallback counts=%#v", fallbacks["combined"], fallbacks["trim"], fallbacks)
	}
}

func TestQEvalVectorBenchmarkCoverageTags(t *testing.T) {
	covered := make(map[string][]string)
	matrixCovered := make(map[string][]string)
	shapeCovered := make(map[string][]string)
	categoryCovered := make(map[string][]string)
	for _, tc := range qEvalVectorCases {
		for _, tag := range tc.tags {
			covered[tag] = append(covered[tag], tc.name)
		}
		for _, item := range tc.matrix {
			matrixCovered[item] = append(matrixCovered[item], tc.name)
		}
		for _, shape := range tc.shapes {
			shapeCovered[shape] = append(shapeCovered[shape], tc.name)
		}
		for _, category := range qEvalVectorPipelineCategories(tc) {
			categoryCovered[category] = append(categoryCovered[category], tc.name)
		}
	}
	var missing []string
	for _, tag := range qEvalRequiredCoverageTags {
		if len(covered[tag]) == 0 {
			missing = append(missing, tag)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("q.eval performance coverage missing tags: %s", strings.Join(missing, ", "))
	}
	var missingMatrix []string
	for _, item := range qEvalRequiredMatrixCoverage {
		if len(matrixCovered[item]) == 0 {
			missingMatrix = append(missingMatrix, item)
		}
	}
	if len(missingMatrix) > 0 {
		t.Fatalf("q.eval performance coverage missing matrix entries: %s", strings.Join(missingMatrix, ", "))
	}
	var missingShapes []string
	for _, shape := range qEvalRequiredSemanticShapes {
		if len(shapeCovered[shape]) == 0 {
			missingShapes = append(missingShapes, shape)
		}
	}
	if len(missingShapes) > 0 {
		t.Fatalf("q.eval performance coverage missing semantic shapes: %s", strings.Join(missingShapes, ", "))
	}
	var missingCategories []string
	for _, category := range qEvalRequiredPipelineCategories {
		if len(categoryCovered[category]) == 0 {
			missingCategories = append(missingCategories, category)
		}
	}
	if len(missingCategories) > 0 {
		t.Fatalf("q.eval performance coverage missing pipeline categories: %s", strings.Join(missingCategories, ", "))
	}
}

func TestQEvalVectorRuntimeFallbackReport(t *testing.T) {
	eval := qSessionEvalVectorEval(t)
	report := qEvalVectorRuntimeFallbackReport(t, eval, qEvalVectorRows)
	if report.caseCount != len(qEvalVectorCases) {
		t.Fatalf("fallback report case count = %d, want %d", report.caseCount, len(qEvalVectorCases))
	}
	if len(report.categories) < len(qEvalRequiredPipelineCategories) {
		t.Fatalf("fallback report categories = %d, want at least %d", len(report.categories), len(qEvalRequiredPipelineCategories))
	}
	for _, line := range report.LogLines(10) {
		t.Log(line)
	}
}

func TestQEvalVectorTargetGoBaselinesDoRealWork(t *testing.T) {
	targets := map[string]struct{}{
		"MovingStdDevEma32Envelope":         {},
		"SampleStatsCorrelationEnvelope":    {},
		"ListStringBooleanCoverageEnvelope": {},
		"ListSublistCountEnvelope":          {},
		"ListCutCountEnvelope":              {},
		"ListCrossCountEnvelope":            {},
		"StringTrimCountEnvelope":           {},
		"BooleanWhereLiteralCountEnvelope":  {},
		"TaskDMatrixFlipColumnRowChecksum":  {},
		"TaskDMatrixMmuInvEnvelope":         {},
		"TaskDCrossApplyIndexChecksum":      {},
		"TaskDReshapeCycleFillFlipChecksum": {},
	}
	rowScaledTargets := map[string]struct{}{
		"ListStringBooleanCoverageEnvelope": {},
		"ListCrossCountEnvelope":            {},
		"TaskDMatrixFlipColumnRowChecksum":  {},
		"TaskDMatrixMmuInvEnvelope":         {},
		"TaskDCrossApplyIndexChecksum":      {},
		"TaskDReshapeCycleFillFlipChecksum": {},
	}
	allocationTargets := map[string]struct{}{
		"MovingStdDevEma32Envelope":         {},
		"SampleStatsCorrelationEnvelope":    {},
		"ListStringBooleanCoverageEnvelope": {},
		"ListSublistCountEnvelope":          {},
		"ListCutCountEnvelope":              {},
		"StringTrimCountEnvelope":           {},
		"BooleanWhereLiteralCountEnvelope":  {},
		"TaskDMatrixMmuInvEnvelope":         {},
	}
	for _, tc := range qEvalVectorCases {
		if _, ok := targets[tc.name]; !ok {
			continue
		}
		if _, ok := rowScaledTargets[tc.name]; ok {
			if got, smaller := tc.goFn(qEvalVectorRows), tc.goFn(qEvalVectorRows/2); got == smaller {
				t.Fatalf("%s Go baseline returned the same result for different row counts: %d", tc.name, got)
			}
		}
		if _, ok := allocationTargets[tc.name]; ok {
			allocs := testing.AllocsPerRun(1, func() {
				qEvalVectorBenchSink = tc.goFn(qEvalVectorRows)
			})
			if allocs == 0 {
				t.Fatalf("%s Go baseline performed no allocations; it likely regressed to a constant-folded correctness oracle", tc.name)
			}
		}
		delete(targets, tc.name)
	}
	if len(targets) > 0 {
		missing := make([]string, 0, len(targets))
		for name := range targets {
			missing = append(missing, name)
		}
		sort.Strings(missing)
		t.Fatalf("missing target q.eval Go baseline cases: %s", strings.Join(missing, ", "))
	}
}

// qEvalRowInvariantBaselineAllowlist lists the only cases whose Go baseline is
// allowed to return the same value for qEvalVectorRows and qEvalVectorRows/2.
// Every entry needs a justification: either the q expression itself does
// constant-size literal work, or the result is legitimately row-invariant even
// though the baseline work scales with rows. Do not add entries to hide a
// constant-folded baseline; fix the baseline instead.
var qEvalRowInvariantBaselineAllowlist = map[string]string{
	// Literal-envelope cases: the q expression operates on small literal
	// vectors/dicts/tables, so constant work and a constant result are correct.
	"TemporalXbarCount":                   "q expr buckets a 4-element literal time vector; constant work on both sides",
	"TimestampXbarCount":                  "q expr buckets a 3-element literal timestamp vector; constant work on both sides",
	"DictEachCountDistinct":               "q expr scans a literal 3-key dict of tiny vectors; constant work on both sides",
	"BooleanWhereLiteralCountEnvelope":    "q expr scans the 6-bit literal mask 101001b; constant work on both sides",
	"CutEnlistRazeChecksum":               "q expr cuts/razes literal 5-element lists; constant work on both sides",
	"MembershipDifferRatiosTruthMatrix":   "q expr probes literal 3-5 element vectors; constant work on both sides",
	"AdverbMatrix":                        "q expr exercises adverbs over literal 3-4 element vectors; constant work on both sides",
	"SetBinWithinXrank":                   "q expr runs set/bin/within/xrank over literal 3-5 element vectors; constant work on both sides",
	"SearchBinrWavgXprevMatrix":           "q expr runs binr/find/wavg/xprev over literal vectors; constant work on both sides",
	"SymbolSetVerbs":                      "q expr runs set verbs over literal 2-3 symbol vectors; constant work on both sides",
	"TypedPrevNextDeltasFills":            "q expr runs prev/next/deltas/fills over literal 3-element vectors; constant work on both sides",
	"SortRankTypeMatrix":                  "q expr sorts/ranks literal 3-5 element vectors; constant work on both sides",
	"DictAmendUpsertAndLookup":            "q expr amends a literal 2-key dict; constant work on both sides",
	"DictKeysValueAttrFillMatch":          "q expr probes literal 2-3 element dicts and vectors; constant work on both sides",
	"EnumSymbolStringMatchLike":           "q expr enumerates literal 2-3 symbol vectors; constant work on both sides",
	"TemporalTypedXbarAndSort":            "q expr buckets/compares literal 2-3 element temporal vectors; constant work on both sides",
	"TableMetadataReorderSort":            "q expr reorders literal 2-row tables; constant work on both sides",
	"KeyedTableGroupUngroupFby":           "q expr keys/groups literal 2-3 row tables; constant work on both sides",
	"SafeSystemAndLoopbackIPC":            "q expr measures loopback IPC plus a literal call; constant work on both sides",
	"ProjectionEachDictNumericTableCombo": "q expr combines literal 3-element dicts and tables; constant work on both sides",
	"TemporalStringSymbolProjectionCombo": "q expr combines literal 4-element symbol/temporal vectors; constant work on both sides",
	"NestedDictProjectionAmendCombo":      "q expr amends literal 3-element dicts; constant work on both sides",
	"KeyedTableTemporalStringMetaCombo":   "q expr keys/sorts literal 3-row tables; constant work on both sides",
	"BooleanLogicalAndCompositeCompare":   "q expr compares literal 3-element vectors; constant work on both sides",
	"DeepSetUnionInterExceptSymbols":      "q expr runs set verbs over literal 2-4 symbol vectors; constant work on both sides",
	"TaskDCastSymbolNumericEnvelope":      "q expr casts scalar literals; constant work on both sides",

	// Row-invariant results with row-scaled baseline work.
	"SampleStatsCorrelationEnvelope":        "result counts 6 statistics; baseline computes full sample stats over rows",
	"ListSublistCountEnvelope":              "100 sublist always counts 100; baseline materializes the rows-long source vector",
	"ListCutCountEnvelope":                  "0 100 200 cut always yields 3 bins; baseline materializes and cuts the rows-long vector",
	"SymbolDistinctCount":                   "distinct of a 6-symbol cycle is 6 at any benchmarked size; seen-set scan scales with rows",
	"SymbolRotateDistinctCount":             "distinct of a rotated 5-symbol cycle is 4 at any benchmarked size; seen-set scan scales with rows",
	"ProductOnesRowScaled":                  "prd rows#1 is always 1; baseline multiplies across all rows",
	"FunctionalAmendWhereVector":            "amend targets a fixed 128-index prefix; result 256 at any benchmarked size; mask scan scales with rows",
	"RotateWhereHeadCount":                  "a permutation always has exactly 10 values below 10; full mask scan scales with rows",
	"WhereValueGatherReduceSelectivityPct0": "0% selectivity gathers nothing; result 0; mask scan scales with rows",
	"ReverseWhereGatherHeadSum":             "values below 128 sum identically in any permutation; reverse scan scales with rows",
	"DistinctModuloCount":                   "x mod 257 covers all 257 residues at benchmarked sizes; distinct scan scales with rows",
	"GroupModuloBucketCount":                "x mod 64 covers all 64 buckets at benchmarked sizes; group scan scales with rows",
	"DeepStringUpperLikeCountSide":          "side symbols never match A*; result 0; pattern scan scales with rows",
	"DeepAdverbEachLeftRightChecksum":       "the each-left and each-right sums cancel to 0 for any rows; per-row adverb work scales",
	"TaskDListSublistDirectSum":             "128 1024 sublist is a fixed window; summing the 1024-element window is the honest work",
	"DeepSymbolDistinctAfterRotateTech":     "rotation preserves the 5-symbol distinct set; seen-set scan scales with rows",
	"DeepSymbolDistinctAfterRotateVenue":    "rotation preserves the 5-symbol distinct set; seen-set scan scales with rows",
	"DeepSymbolDistinctAfterRotateSide":     "rotation preserves the 3-symbol distinct set; seen-set scan scales with rows",
	"TakeHead1":                             "take-prefix sum is independent of rows; baseline materializes til rows before taking",
	"TakeHead8":                             "take-prefix sum is independent of rows; baseline materializes til rows before taking",
	"TakeHead64":                            "take-prefix sum is independent of rows; baseline materializes til rows before taking",
	"TakeHead128":                           "take-prefix sum is independent of rows; baseline materializes til rows before taking",
	"TakeHead1024":                          "take-prefix sum is independent of rows; baseline materializes til rows before taking",
	"TakeHead4096":                          "take-prefix sum is independent of rows; baseline materializes til rows before taking",
	"OrdinaryDropThenTake3":                 "drop+take window clears wraparound at benchmarked sizes; baseline materializes the dropped vector",
	"OrdinaryDropThenTake7":                 "drop+take window clears wraparound at benchmarked sizes; baseline materializes the dropped vector",
	"OrdinaryDropThenTake16":                "drop+take window clears wraparound at benchmarked sizes; baseline materializes the dropped vector",
	"OrdinaryDropThenTake31":                "drop+take window clears wraparound at benchmarked sizes; baseline materializes the dropped vector",
	"OrdinaryDropThenTake64":                "drop+take window clears wraparound at benchmarked sizes; baseline materializes the dropped vector",
	"OrdinaryDropThenTake127":               "drop+take window clears wraparound at benchmarked sizes; baseline materializes the dropped vector",
	"OrdinaryDropThenTake256":               "drop+take window clears wraparound at benchmarked sizes; baseline materializes the dropped vector",
	"OrdinaryDropThenTake513":               "drop+take window clears wraparound at benchmarked sizes; baseline materializes the dropped vector",
	"OrdinaryDropThenTake1024":              "drop+take window clears wraparound at benchmarked sizes; baseline materializes the dropped vector",
	"OrdinaryRotateFilter3":                 "a 128-wide band of a permutation always counts 128; full mask scan scales with rows",
	"OrdinaryRotateFilter7":                 "a 128-wide band of a permutation always counts 128; full mask scan scales with rows",
	"OrdinaryRotateFilter16":                "a 128-wide band of a permutation always counts 128; full mask scan scales with rows",
	"OrdinaryRotateFilter31":                "a 128-wide band of a permutation always counts 128; full mask scan scales with rows",
	"OrdinaryRotateFilter64":                "a 128-wide band of a permutation always counts 128; full mask scan scales with rows",
	"OrdinaryRotateFilter127":               "a 128-wide band of a permutation always counts 128; full mask scan scales with rows",
	"OrdinaryRotateFilter256":               "a 128-wide band of a permutation always counts 128; full mask scan scales with rows",
	"OrdinaryRotateFilter513":               "a 128-wide band of a permutation always counts 128; full mask scan scales with rows",
	"OrdinaryRotateFilter1024":              "a 128-wide band of a permutation always counts 128; full mask scan scales with rows",
	"OrdinaryRotateFilter2049":              "a 128-wide band of a permutation always counts 128; full mask scan scales with rows",
	"DeepReverseWhereWindowCountSmall":      "fixed-width band over a permutation counts the band width; full scan scales with rows",
	"DeepReverseWhereWindowCountMedium":     "fixed-width band over a permutation counts the band width; full scan scales with rows",
	"DeepReverseWhereWindowCountWide":       "fixed-width band over a permutation counts the band width; full scan scales with rows",
	"DeepReverseWhereWindowCountCycle":      "fixed-width band over a permutation counts the band width; full scan scales with rows",
	"DeepXbarWithinCountFine":               "xbar band is fully inside/outside both benchmarked row counts; full bucket scan scales with rows",
	"DeepXbarWithinCountTen":                "xbar band is fully inside/outside both benchmarked row counts; full bucket scan scales with rows",
	"DeepXbarWithinCountMinute":             "xbar band is fully inside/outside both benchmarked row counts; full bucket scan scales with rows",
	"DeepXbarWithinCountKilo":               "xbar band is fully inside/outside both benchmarked row counts; full bucket scan scales with rows",
	"DeepTakeDropRotateSumSmall":            "take window clears wraparound; values independent of rows; rotate materialization scales with rows",
	"DeepTakeDropRotateSumMedium":           "take window clears wraparound; values independent of rows; rotate materialization scales with rows",
	"DeepTakeDropRotateSumWide":             "take window clears wraparound; values independent of rows; rotate materialization scales with rows",
	"BreadthListDropTakeSublistShort":       "sublist window clears wraparound at benchmarked sizes; baseline materializes the dropped vector",
	"BreadthListDropTakeSublistMid":         "sublist window clears wraparound at benchmarked sizes; baseline materializes the dropped vector",
	"BreadthListDropTakeSublistLong":        "sublist window clears wraparound at benchmarked sizes; baseline materializes the dropped vector",
	"BreadthMatrixReshapeRowSumTwoByN":      "matrix is fixed at 8192 cells regardless of rows; row sum work matches the fixed shape",
	"BreadthMatrixReshapeRowSumFourByN":     "matrix is fixed at 8192 cells regardless of rows; row sum work matches the fixed shape",
	"BreadthMatrixReshapeRowSumEightByN":    "matrix is fixed at 8192 cells regardless of rows; row sum work matches the fixed shape",
	"BreadthMatrixReshapeCellProbeTwoByN":   "matrix is fixed at 8192 cells regardless of rows; baseline materializes the reshaped backing",
	"BreadthMatrixReshapeCellProbeFourByN":  "matrix is fixed at 8192 cells regardless of rows; baseline materializes the reshaped backing",
	"BreadthMatrixReshapeCellProbeEightByN": "matrix is fixed at 8192 cells regardless of rows; baseline materializes the reshaped backing",
}

// qEvalRowInvariantBaselineAllowlistMax pins the allowlist size: it may only
// shrink. Fix new baselines to scale with rows instead of growing this list.
const qEvalRowInvariantBaselineAllowlistMax = 89

func TestQEvalVectorAllGoBaselinesDoRealWork(t *testing.T) {
	if len(qEvalRowInvariantBaselineAllowlist) > qEvalRowInvariantBaselineAllowlistMax {
		t.Fatalf("row-invariant baseline allowlist has %d entries, max %d; fix new baselines to scale with rows instead of allowlisting them",
			len(qEvalRowInvariantBaselineAllowlist), qEvalRowInvariantBaselineAllowlistMax)
	}
	caseNames := make(map[string]struct{}, len(qEvalVectorCases))
	for _, tc := range qEvalVectorCases {
		caseNames[tc.name] = struct{}{}
	}
	staleNames := make([]string, 0)
	for name, justification := range qEvalRowInvariantBaselineAllowlist {
		if justification == "" {
			t.Errorf("allowlist entry %q has no justification", name)
		}
		if _, ok := caseNames[name]; !ok {
			staleNames = append(staleNames, name)
		}
	}
	if len(staleNames) > 0 {
		sort.Strings(staleNames)
		t.Fatalf("stale row-invariant baseline allowlist entries (no matching case): %s", strings.Join(staleNames, ", "))
	}
	for _, tc := range qEvalVectorCases {
		if _, ok := qEvalRowInvariantBaselineAllowlist[tc.name]; ok {
			continue
		}
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			full := tc.goFn(qEvalVectorRows)
			half := tc.goFn(qEvalVectorRows / 2)
			if full == half {
				t.Fatalf("Go baseline result %d is identical for %d and %d rows; make the baseline row-scaled or allowlist it with a justification",
					full, qEvalVectorRows, qEvalVectorRows/2)
			}
		})
	}
}

func BenchmarkQEvalVectorResultCacheWarm(b *testing.B) {
	for _, tc := range qEvalVectorCases {
		b.Run(tc.name, func(b *testing.B) {
			eval := qEvalVectorEval(b)
			src := tc.expr(qEvalVectorRows)
			qEvalVectorBenchSink = qEvalVectorRun(b, eval, src)

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				qEvalVectorBenchSink = qEvalVectorRun(b, eval, src)
			}
		})
	}
}

func BenchmarkQSessionEvalVectorWarmExecution(b *testing.B) {
	for _, tc := range qEvalVectorCases {
		b.Run(tc.name, func(b *testing.B) {
			eval := qSessionEvalVectorEval(b)
			src := tc.expr(qEvalVectorRows)
			qEvalVectorBenchSink = qEvalVectorRun(b, eval, src)
			stdq.ClearRuntimeKernelExecutionStats()

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				qEvalVectorBenchSink = qEvalVectorRun(b, eval, src)
			}
			b.StopTimer()
			qEvalVectorReportRuntimeKernelStats(b)
			qEvalVectorReportPipelineCategories(b, tc)
		})
	}
}

func BenchmarkQEvalVectorCold(b *testing.B) {
	for _, tc := range qEvalVectorCases {
		b.Run(tc.name, func(b *testing.B) {
			eval := qEvalVectorEval(b)
			variants := qEvalVectorColdVariants(tc.expr(qEvalVectorRows), 512)

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				qEvalVectorBenchSink = qEvalVectorRun(b, eval, variants[i%len(variants)])
			}
		})
	}
}

func BenchmarkQEvalVectorGoBaseline(b *testing.B) {
	for _, tc := range qEvalVectorCases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				qEvalVectorBenchSink = tc.goFn(qEvalVectorRows)
			}
		})
	}
}

func qEvalVectorEval(tb testing.TB) *bind.GoFunction {
	tb.Helper()
	q := bind.BuildQ()
	eval := q.RawGetString("eval").GoFunction()
	if eval == nil {
		tb.Fatalf("q.eval is not a Go function")
	}
	return eval
}

func qSessionEvalVectorEval(tb testing.TB) *bind.GoFunction {
	tb.Helper()
	q := bind.BuildQ()
	sessionFn := q.RawGetString("session").GoFunction()
	if sessionFn == nil {
		tb.Fatalf("q.session is not a Go function")
	}
	out, err := sessionFn.Fn(nil)
	if err != nil {
		tb.Fatalf("q.session: %v", err)
	}
	if len(out) != 1 || !out[0].IsTable() {
		tb.Fatalf("q.session returned %d values, want one table", len(out))
	}
	eval := out[0].Table().RawGetString("eval").GoFunction()
	if eval == nil {
		tb.Fatalf("q.session.eval is not a Go function")
	}
	return eval
}

func qEvalVectorRun(tb testing.TB, eval *bind.GoFunction, src string) int64 {
	tb.Helper()
	// Embedder lifetime discipline: the result value is fully consumed into
	// an int64 checksum inside this scope, so all of its NaN-box roots are
	// released on return instead of accumulating in the global root log.
	scope := bind.PushValueScope()
	defer scope.Release()
	if eval.FastArg1 != nil {
		out, err := eval.FastArg1(bind.StringValue(src))
		if err != nil {
			tb.Fatalf("q.eval(%q): %v", src, err)
		}
		return qEvalVectorInt64(tb, out)
	}
	out, err := eval.Fn([]bind.Value{bind.StringValue(src)})
	if err != nil {
		tb.Fatalf("q.eval(%q): %v", src, err)
	}
	if len(out) != 1 {
		tb.Fatalf("q.eval(%q) returned %d values, want 1", src, len(out))
	}
	return qEvalVectorInt64(tb, out[0])
}

func qEvalVectorReportRuntimeKernelStats(b *testing.B) {
	b.Helper()
	var attempts, hits, fallbacks, errors uint64
	pipelineShapes := map[string]struct{}{}
	fallbackPipelineShapes := map[string]struct{}{}
	for _, stat := range stdq.RuntimeKernelExecutionStats() {
		switch stat.Outcome {
		case "attempt":
			attempts += stat.Count
			if stat.PipelineShape != "" {
				pipelineShapes[stat.PipelineShape] = struct{}{}
			}
		case "hit", "success":
			hits += stat.Count
		case "fallback":
			fallbacks += stat.Count
			if stat.PipelineShape != "" {
				fallbackPipelineShapes[stat.PipelineShape] = struct{}{}
			}
		case "error":
			errors += stat.Count
			if stat.PipelineShape != "" {
				fallbackPipelineShapes[stat.PipelineShape] = struct{}{}
			}
		}
	}
	if attempts > 0 {
		b.ReportMetric(100*float64(hits)/float64(attempts), "typed_kernel_hit_pct")
	}
	if b.N > 0 {
		b.ReportMetric(float64(attempts)/float64(b.N), "typed_kernel_attempts/op")
		b.ReportMetric(float64(hits)/float64(b.N), "typed_kernel_hits/op")
		b.ReportMetric(float64(fallbacks)/float64(b.N), "typed_kernel_fallbacks/op")
		b.ReportMetric(float64(errors)/float64(b.N), "typed_kernel_errors/op")
		b.ReportMetric(float64(len(pipelineShapes)), "typed_pipeline_shapes")
		b.ReportMetric(float64(len(fallbackPipelineShapes)), "typed_pipeline_fallback_shapes")
	}
}

func qEvalVectorRuntimeFallbackCount() uint64 {
	var fallbacks uint64
	for _, stat := range stdq.RuntimeKernelExecutionStats() {
		if stat.Outcome == "fallback" || stat.Outcome == "error" {
			fallbacks += stat.Count
		}
	}
	return fallbacks
}

func qEvalVectorReportPipelineCategories(b *testing.B, tc qEvalVectorCase) {
	b.Helper()
	for _, category := range qEvalVectorPipelineCategories(tc) {
		b.ReportMetric(1, "q_pipeline_category_"+category)
	}
}

type qEvalVectorRuntimeFallbackSummary struct {
	caseCount  int
	categories map[string]struct{}
	rows       map[qEvalVectorRuntimeFallbackKey]uint64
}

type qEvalVectorRuntimeFallbackKey struct {
	category      string
	pipelineShape string
	kernel        string
	reason        string
	outcome       string
}

type qEvalVectorRuntimeFallbackRow struct {
	key   qEvalVectorRuntimeFallbackKey
	count uint64
}

func qEvalVectorRuntimeFallbackReportForCases(tb testing.TB, eval *bind.GoFunction, rows int, cases []qEvalVectorCase) qEvalVectorRuntimeFallbackSummary {
	tb.Helper()
	report := qEvalVectorRuntimeFallbackSummary{
		categories: make(map[string]struct{}),
		rows:       make(map[qEvalVectorRuntimeFallbackKey]uint64),
	}
	for _, tc := range cases {
		categories := qEvalVectorPipelineCategories(tc)
		if len(categories) == 0 {
			categories = []string{"uncategorized"}
		}
		for _, category := range categories {
			report.categories[category] = struct{}{}
		}
		stdq.ClearRuntimeKernelExecutionStats()
		qEvalVectorBenchSink = qEvalVectorRun(tb, eval, tc.expr(rows))
		report.caseCount++
		for _, stat := range stdq.RuntimeKernelExecutionStats() {
			if stat.Outcome != "fallback" && stat.Outcome != "error" {
				continue
			}
			statCategories := qEvalVectorRuntimeFallbackCategories(categories, stat)
			for _, category := range statCategories {
				key := qEvalVectorRuntimeFallbackKey{
					category:      category,
					pipelineShape: qEvalRuntimeStatField(stat.PipelineShape),
					kernel:        qEvalRuntimeStatField(stat.Kernel),
					reason:        qEvalRuntimeStatField(stat.ReasonCode),
					outcome:       qEvalRuntimeStatField(stat.Outcome),
				}
				report.rows[key] += stat.Count
			}
		}
	}
	stdq.ClearRuntimeKernelExecutionStats()
	return report
}

func qEvalVectorRuntimeFallbackCategories(categories []string, stat stdq.RuntimeKernelExecutionStat) []string {
	if len(categories) != 1 || categories[0] != "uncategorized" {
		return categories
	}
	shape := qEvalRuntimeStatField(stat.PipelineShape)
	kernel := qEvalRuntimeStatField(stat.Kernel)
	if strings.HasPrefix(shape, "vector_reduce") || strings.HasPrefix(shape, "vector_unary") ||
		strings.HasPrefix(shape, "vector_map") || kernel == "ArraySum" ||
		strings.HasPrefix(kernel, "ArrayNumeric") {
		return []string{"vector_numeric"}
	}
	return categories
}

func qEvalVectorRuntimeFallbackReport(tb testing.TB, eval *bind.GoFunction, rows int) qEvalVectorRuntimeFallbackSummary {
	tb.Helper()
	return qEvalVectorRuntimeFallbackReportForCases(tb, eval, rows, qEvalVectorCases)
}

func (r qEvalVectorRuntimeFallbackSummary) Rows() []qEvalVectorRuntimeFallbackRow {
	rows := make([]qEvalVectorRuntimeFallbackRow, 0, len(r.rows))
	for key, count := range r.rows {
		rows = append(rows, qEvalVectorRuntimeFallbackRow{key: key, count: count})
	}
	sort.Slice(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.count != b.count {
			return a.count > b.count
		}
		if a.key.category != b.key.category {
			return a.key.category < b.key.category
		}
		if a.key.pipelineShape != b.key.pipelineShape {
			return a.key.pipelineShape < b.key.pipelineShape
		}
		if a.key.kernel != b.key.kernel {
			return a.key.kernel < b.key.kernel
		}
		if a.key.reason != b.key.reason {
			return a.key.reason < b.key.reason
		}
		return a.key.outcome < b.key.outcome
	})
	return rows
}

func (r qEvalVectorRuntimeFallbackSummary) LogLines(limit int) []string {
	rows := r.Rows()
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	lines := make([]string, 0, len(rows)+1)
	lines = append(lines, fmt.Sprintf("q_pipeline_fallback_report cases=%d categories=%d rows=%d", r.caseCount, len(r.categories), len(r.rows)))
	if len(rows) == 0 {
		lines = append(lines, "q_pipeline_fallback_report rank=none category=none pipeline_shape=none kernel=none reason=none outcome=none count=0")
		return lines
	}
	for i, row := range rows {
		lines = append(lines, fmt.Sprintf(
			"q_pipeline_fallback_report rank=%d category=%s pipeline_shape=%s kernel=%s reason=%s outcome=%s count=%d",
			i+1,
			row.key.category,
			row.key.pipelineShape,
			row.key.kernel,
			row.key.reason,
			row.key.outcome,
			row.count,
		))
	}
	return lines
}

func qEvalRuntimeStatField(value string) string {
	if value == "" {
		return "unknown"
	}
	return strings.ReplaceAll(value, " ", "_")
}

func qEvalVectorPipelineCategories(tc qEvalVectorCase) []string {
	seen := make(map[string]struct{}, 4)
	add := func(category string) {
		seen[category] = struct{}{}
	}
	hasTag := func(tag string) bool {
		for _, item := range tc.tags {
			if item == tag {
				return true
			}
		}
		return false
	}
	hasShapePrefix := func(prefix string) bool {
		for _, item := range tc.shapes {
			if strings.HasPrefix(item, prefix) {
				return true
			}
		}
		return false
	}
	hasMatrixPrefix := func(prefix string) bool {
		for _, item := range tc.matrix {
			if strings.HasPrefix(item, prefix) {
				return true
			}
		}
		return false
	}
	name := strings.ToLower(tc.name)
	if strings.Contains(name, "xbarwithin") || (hasTag("xbar") && (hasTag("bin-within-xrank") || strings.Contains(name, "within"))) {
		add("xbar_within")
	}
	if strings.Contains(name, "where") && (strings.Contains(name, "project") || strings.Contains(name, "gather") || strings.Contains(name, "sum") || strings.Contains(name, "count")) {
		add("where_project_reduce")
	}
	if hasShapePrefix("where:") || hasShapePrefix("index:gather-after-where") || hasTag("where") {
		add("where_project_reduce")
	}
	if hasTag("fby") || hasTag("group") || hasShapePrefix("group:") || hasMatrixPrefix("table:xkey-xgroup") {
		add("group_fby")
	}
	if hasTag("table-sort") || hasShapePrefix("sort:") || hasMatrixPrefix("sort:") || strings.Contains(name, "sort") || strings.Contains(name, "gather") {
		add("sort_gather")
	}
	if hasTag("adverb-over-scan") || hasTag("adverb-each") || hasTag("adverb-each-prior") || hasTag("adverb-each-left-right") ||
		hasTag("cut") || hasTag("enlist") || hasTag("raze") || hasShapePrefix("adverb:") || hasMatrixPrefix("list:") ||
		strings.Contains(name, "adverb") || strings.Contains(name, "list") || strings.Contains(name, "deltas") {
		add("ordinary_list_adverb")
	}
	if hasTag("numeric-vector") || hasTag("weighted-aggregate") || hasShapePrefix("verb:") ||
		hasMatrixPrefix("numeric:") || hasMatrixPrefix("aggregate:") || strings.Contains(name, "numeric") ||
		strings.Contains(name, "dyadic") || strings.Contains(name, "minmax") || strings.Contains(name, "signum") ||
		strings.Contains(name, "reciprocal") {
		add("vector_numeric")
	}
	if hasTag("math-transcendental") || hasMatrixPrefix("math:") || strings.Contains(name, "math") {
		add("math_transcendental")
	}
	if hasTag("matrix-reshape") || hasMatrixPrefix("matrix:") || strings.Contains(name, "matrix") {
		add("matrix_reshape")
	}
	if hasTag("apply-index") || hasMatrixPrefix("apply-index:") || strings.Contains(name, "apply") {
		add("apply_index")
	}
	out := make([]string, 0, len(seen))
	for category := range seen {
		out = append(out, category)
	}
	sort.Strings(out)
	return out
}

func qEvalVectorInt64(tb testing.TB, v bind.Value) int64 {
	tb.Helper()
	if v.IsInt() {
		return v.Int()
	}
	if v.IsFloat() {
		return int64(v.Float())
	}
	tb.Fatalf("q.eval returned %s, want numeric scalar", v.TypeName())
	return 0
}

func qEvalVectorColdVariants(src string, count int) []string {
	variants := make([]string, count)
	for i := range variants {
		variants[i] = src + strings.Repeat(" ", i+1)
	}
	return variants
}

func qEvalVectorNameInt(n int) string {
	if n < 0 {
		return fmt.Sprintf("Neg%d", -n)
	}
	return fmt.Sprintf("Pos%d", n)
}

func floorBucket(v, width int64) int64 {
	if v >= 0 {
		return (v / width) * width
	}
	return -(((-v + width - 1) / width) * width)
}

func floorBucketFloat(v, width float64) float64 {
	bucket := float64(int64(v / width))
	if v < 0 && bucket*width != v {
		bucket--
	}
	return bucket * width
}
