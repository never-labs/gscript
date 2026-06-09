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
func qEvalVectorGoBaselineTakeCount(rows int, values []int64) int64 {
	if rows < 0 || rows > len(values) {
		return 0
	}
	return int64(rows + qEvalVectorGoBaselineTakeExtra)
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
				if selected == 0 {
					return 0
				}
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
				var sum int64
				for i := 0; i < n; i++ {
					sum += int64(i % rows)
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
			tags:   []string{"adverb-over-scan", "sum", "sums", "product"},
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
				return int64(rows)
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
				for value := 0; value < 128 && value < rows; value++ {
					sum += int64(value)
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
				return int64(rows-1)*int64(rows)/2 + int64(rows)
			},
		},
		{
			name: "ScanPlusTailChecksum",
			tags: []string{"adverb-over-scan", "sum"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;s:+\\x;last s+count s", rows)
			},
			goFn: func(rows int) int64 {
				return int64(rows-1)*int64(rows)/2 + int64(rows)
			},
		},
		{
			name:   "RunningMinMaxTailEnvelope",
			tags:   []string{"running-aggregate", "min-max"},
			matrix: []string{"aggregate:running-prd-min-max-avg:vector"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;(last mins x)+(last maxs reverse x)+(last maxs x)", rows)
			},
			goFn: func(rows int) int64 {
				return int64(2 * (rows - 1))
			},
		},
		{
			name:   "AvgsTailChecksum",
			tags:   []string{"running-aggregate", "avg-var-dev-med"},
			matrix: []string{"aggregate:running-prd-min-max-avg:vector"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;last avgs x", rows)
			},
			goFn: func(rows int) int64 {
				return int64(float64(rows-1) / 2)
			},
		},
		{
			name: "ProductOnesRowScaled",
			tags: []string{"product", "running-aggregate"},
			expr: func(rows int) string {
				return fmt.Sprintf("prd %d#1", rows)
			},
			goFn: func(rows int) int64 {
				return 1
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
				return int64(rows)
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
				return int64(rows-1) * int64(rows) / 2
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
				return int64(rows)
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
				return int64(rows * 3)
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
				if rows <= 0 {
					return 0
				}
				return int64(rows - 1)
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
				if rows < 257 {
					return int64(rows)
				}
				return 257
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
				if rows < 64 {
					return int64(rows)
				}
				return 64
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
					var sum, first, last int64
					for i := 0; i < n; i++ {
						value := int64(rows - 1 - (i % rows))
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
					length := rows - drop
					var sum int64
					for i := 0; i < take; i++ {
						sum += int64(drop + (i % length))
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
		name    string
		values  string
		fill    int64
		sum     func(rows int) int64
		nulls   func(rows int) int64
		castTag string
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
		}, func(rows int) int64 { return qPatternCount(rows, 4, map[int]bool{0: true, 3: true}) }, "typed-null"},
		{"ShortCast", "1 2 3 4", 0, func(rows int) int64 {
			var sum int64
			for i := 0; i < rows; i++ {
				sum += int64(i%4 + 1)
			}
			return sum
		}, func(rows int) int64 { return 0 }, "cast"},
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
		}, func(rows int) int64 { return qPatternCount(rows, 4, map[int]bool{0: true, 3: true}) }, "typed-null"},
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
					return int64(rows)
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
					shift := p.shift % rows
					rotatedLen := rows - p.drop
					var sum, first, last int64
					for i := 0; i < p.take; i++ {
						value := int64((shift + p.drop + (i % rotatedLen)) % rows)
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
					return int64(rows)
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
					seen := make(map[int]struct{}, p.width)
					for i := 0; i < p.width; i++ {
						seen[i] = struct{}{}
					}
					if p.name == "Side" {
						return 3
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
			tags:   []string{"running-aggregate", "product", "avg-var-dev-med", "min-max"},
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
				return int64(rows * 5)
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
				return 6 + 6 + 10 + 1 + 3 + 3 + 10 + 3 + 3
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
				return 3 + 3 + 1
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
				return 1 + 3 + 200
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
				return 1 + 3 + 3 + 3 + 3
			},
		},
	}
	return append(cases, coverage...)
}

func TestQEvalVectorBenchmarkExpressions(t *testing.T) {
	if len(qEvalVectorCases) < 220 {
		t.Fatalf("q.eval benchmark coverage too small: got %d cases, want at least 220", len(qEvalVectorCases))
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

func TestQEvalVectorBenchmarkCoverageTags(t *testing.T) {
	covered := make(map[string][]string)
	matrixCovered := make(map[string][]string)
	shapeCovered := make(map[string][]string)
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
