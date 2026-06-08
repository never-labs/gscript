package benchmarks

import (
	"fmt"
	"strings"
	"testing"

	"github.com/never-labs/leia/internal/stdlib/bind"
	stdq "github.com/never-labs/leia/internal/stdlib/lib/q"
)

const qEvalVectorRows = 8192

var qEvalVectorBenchSink int64

type qEvalVectorCase struct {
	name   string
	tags   []string
	matrix []string
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
				if threshold <= 0 {
					return int64(rows)
				}
				if threshold >= int64(rows) {
					return 0
				}
				return int64(rows) - threshold
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
			return int64(rows-1) + sumTil(rows)
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
				first, last := rotatedEndpoints(rows, n)
				return sumTil(rows) + first + last
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
				sum := sumOneTo(rows)
				return sum + sum + sum
			},
		},
		qEvalVectorCase{
			name: "AdverbDeltasSum",
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;+/deltas x", rows)
			},
			goFn: func(rows int) int64 {
				return int64(rows - 1)
			},
		},
		qEvalVectorCase{
			name: "ListReductionsInt",
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;s:+/x;named:sum x;mx:max x;mn:min x;cnt:count x;s+named+mx+mn+cnt", rows)
			},
			goFn: func(rows int) int64 {
				return sumTil(rows)*2 + int64(rows-1) + int64(rows)
			},
		},
		qEvalVectorCase{
			name: "CompositionCountDistinctAndReverse",
			tags: []string{"composition", "distinct"},
			expr: func(rows int) string {
				return "(count distinct)[10 20 10 30]+(first reverse)[1 2 3]"
			},
			goFn: func(rows int) int64 {
				return 3 + 3
			},
		},
		qEvalVectorCase{
			name: "DictEachCountDistinct",
			expr: func(rows int) string {
				return "d:(count distinct)'`a`b`c!(1 1 2;3 3 3;9 8 9 7);a:d`a;b:d`b;c:d`c;a+b+c"
			},
			goFn: func(rows int) int64 {
				return 2 + 1 + 3
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
				return "count 0.5 xbar 0.1 0.5 0.9 1.0 -1.25 -1.0 -0.75"
			},
			goFn: func(rows int) int64 {
				return 7
			},
		},
		qEvalVectorCase{
			name: "SymbolDistinctCount",
			expr: func(rows int) string {
				return "count distinct `AAPL`MSFT`AAPL`NVDA`MSFT`TSLA"
			},
			goFn: func(rows int) int64 {
				return 4
			},
		},
		qEvalVectorCase{
			name: "SymbolRotateDistinctCount",
			expr: func(rows int) string {
				return "count distinct -1 rotate `AAPL`MSFT`NVDA`AAPL`TSLA"
			},
			goFn: func(rows int) int64 {
				return 4
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
				return "count ([] sym:`AAPL`MSFT`NVDA;price:100 101 102;size:10 20 30)"
			},
			goFn: func(rows int) int64 {
				return 3
			},
		},
		qEvalVectorCase{
			name: "XbarStaticMixedSignCount",
			expr: func(rows int) string {
				return "count xbar[10;-21 -20 -19 -10 -1 0 1]"
			},
			goFn: func(rows int) int64 {
				return 7
			},
		},
	)

	return appendQEvalSemanticCoverageCases(cases)
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
				return "(+/1h 2h 3h)+(type 1i)+(type `float$1)+(count where null 1 0N 2 0N)"
			},
			goFn: func(rows int) int64 {
				return 6 - 6 - 9 + 2
			},
		},
		{
			name: "NumericMonadsAndWordDyadics",
			tags: []string{"numeric-monad", "word-dyadic"},
			expr: func(rows int) string {
				return "(abs -2)+(neg 5)+(signum -7)+(floor 2.9)+(ceiling 2.1)+sum 1 2 3 plus 10"
			},
			goFn: func(rows int) int64 {
				return 2 - 5 - 1 + 2 + 3 + 16
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
				return 3 + 1 + 15
			},
		},
		{
			name: "MinMaxAvgVarDevMed",
			tags: []string{"min-max", "avg-var-dev-med"},
			expr: func(rows int) string {
				return "(min 10 20 30)+(max 10 20 30)+(avg 10 20 30)+(var 10 20 30)+(dev 10 20 30)+(med 10 20 30)"
			},
			goFn: func(rows int) int64 {
				return 10 + 30 + 20 + 66 + 8 + 20
			},
		},
		{
			name: "MovingWindowAggregates",
			tags: []string{"moving-window"},
			expr: func(rows int) string {
				return "(+/3 mcount 10 0N 30 40)+(+/3 mmin 30 0N 10 20)+(+/3 mmax 30 0N 10 20)"
			},
			goFn: func(rows int) int64 {
				return 6 + 80 + 110
			},
		},
		{
			name:   "RunningAggregateProductMatrix",
			tags:   []string{"running-aggregate", "product", "avg-var-dev-med", "min-max"},
			matrix: []string{"aggregate:running-prd-min-max-avg:vector"},
			expr: func(rows int) string {
				return "(prd 1 2 3 4)+(last prds 1 2 3 4)+(last mins 4 3 5 2)+(last maxs 4 3 5 2)+(last avgs 2 4 6 8)"
			},
			goFn: func(rows int) int64 {
				return 24 + 24 + 2 + 5 + 5
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
				return 3 + 4 + 1 + 1 + 2
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
	if len(qEvalVectorCases) < 50 {
		t.Fatalf("q.eval benchmark coverage too small: got %d cases, want at least 50", len(qEvalVectorCases))
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
	for _, tc := range qEvalVectorCases {
		for _, tag := range tc.tags {
			covered[tag] = append(covered[tag], tc.name)
		}
		for _, item := range tc.matrix {
			matrixCovered[item] = append(matrixCovered[item], tc.name)
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
	for _, stat := range stdq.RuntimeKernelExecutionStats() {
		switch stat.Outcome {
		case "attempt":
			attempts += stat.Count
		case "hit", "success":
			hits += stat.Count
		case "fallback":
			fallbacks += stat.Count
		case "error":
			errors += stat.Count
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

func sumTil(rows int) int64 {
	n := int64(rows)
	return n * (n - 1) / 2
}

func sumOneTo(rows int) int64 {
	n := int64(rows)
	return n * (n + 1) / 2
}

func floorBucket(v, width int64) int64 {
	if v >= 0 {
		return (v / width) * width
	}
	return -(((-v + width - 1) / width) * width)
}

func rotatedEndpoints(rows, n int) (int64, int64) {
	shift := n % rows
	if shift < 0 {
		shift += rows
	}
	return int64(shift), int64((shift + rows - 1) % rows)
}
