package benchmarks

import (
	"fmt"
	"sort"
	"strings"
	"testing"
	"unicode"
)

// qEvalVectorVerbFormBacklogCases covers the verb/form coverage backlog found
// by the source-derived gate: raw `&`/`|` logical operators, dyadic word
// aliases (divide, equal, equals, fill, greater, more, less, minus, match,
// left, right), sibling verbs (intersect, ltrim, rtrim, prior, xdesc), and
// the control-flow special forms if[...], do[...], while[...], $[c;t;f].
func qEvalVectorVerbFormBacklogCases() []qEvalVectorCase {
	return []qEvalVectorCase{
		{
			name:   "VerbFormAmpLogicalMaskCount",
			tags:   []string{"boolean-logical", "where", "numeric-vector"},
			matrix: []string{"compare:int-vector:where"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;a:x mod 2;b:x mod 3;m:a & b;count where m", rows)
			},
			goFn: func(rows int) int64 {
				return qEvalVectorVerbFormGoAmpMaskCount(rows)
			},
		},
		{
			name:   "VerbFormPipeLogicalMaskCount",
			tags:   []string{"boolean-logical", "where", "numeric-vector"},
			matrix: []string{"compare:int-vector:where"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;a:x mod 2;b:x mod 5;m:a | b;count where m", rows)
			},
			goFn: func(rows int) int64 {
				return qEvalVectorVerbFormGoPipeMaskCount(rows)
			},
		},
		{
			name:   "VerbFormWordAliasDivideMinus",
			tags:   []string{"word-dyadic", "numeric-vector", "sum"},
			matrix: []string{"numeric-arithmetic:float-vector:hot"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;y:x minus %d;z:y divide 2;+/z", rows, rows/2)
			},
			goFn: func(rows int) int64 {
				return qEvalVectorVerbFormGoDivideMinusSum(rows)
			},
		},
		{
			name:   "VerbFormWordAliasEqualEqualsCount",
			tags:   []string{"word-dyadic", "composite-compare", "where"},
			matrix: []string{"compare:int-vector:where"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;a:count where (x mod 7) equal 3;b:count where (x mod 5) equals 2;a+b", rows)
			},
			goFn: func(rows int) int64 {
				return qEvalVectorVerbFormGoEqualEqualsCount(rows)
			},
		},
		{
			name:   "VerbFormWordAliasLessGreaterMore",
			tags:   []string{"word-dyadic", "composite-compare", "where"},
			matrix: []string{"compare:int-vector:where"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;(count where x less %d)+(count where x greater %d)+(count where x more %d)", rows, rows/4, rows/2, rows*3/4)
			},
			goFn: func(rows int) int64 {
				return qEvalVectorVerbFormGoLessGreaterMoreCount(rows)
			},
		},
		{
			name: "VerbFormWordAliasLeftRightGather",
			tags: []string{"word-dyadic", "numeric-vector", "reverse", "sum"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;y:reverse x;a:x left y;b:x right y;(+/a)+first b", rows)
			},
			goFn: func(rows int) int64 {
				return qEvalVectorVerbFormGoLeftRightGather(rows)
			},
		},
		{
			name:   "VerbFormFillPriorShiftSum",
			tags:   []string{"fill", "typed-null", "sum"},
			matrix: []string{"list:prev-next-deltas-fills:typed-null"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:1+til %d;p:0 fill prior x;(+/p)+count p", rows)
			},
			goFn: func(rows int) int64 {
				return qEvalVectorVerbFormGoFillPriorShiftSum(rows)
			},
		},
		{
			name:   "VerbFormIntersectSentinelSum",
			tags:   []string{"set-verb", "sum"},
			matrix: []string{"set:int-vector:union-inter-except"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;y:x intersect 100 5000 9000 20000;(+/y)+count y", rows)
			},
			goFn: func(rows int) int64 {
				return qEvalVectorVerbFormGoIntersectSentinelSum(rows)
			},
		},
		{
			name: "VerbFormLtrimRtrimMatchCond",
			tags: []string{"string", "match"},
			expr: func(rows int) string {
				return fmt.Sprintf("s:%d#\" AAPL \";a:ltrim s;b:rtrim s;$[a match b;0;(count a)+count b]", rows)
			},
			goFn: func(rows int) int64 {
				return qEvalVectorVerbFormGoLtrimRtrimMatch(rows)
			},
		},
		{
			name: "VerbFormCondBranchReduce",
			tags: []string{"numeric-vector", "sum"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;$[(sum x)>10000000;sum x*2;sum x*3]", rows)
			},
			goFn: func(rows int) int64 {
				return qEvalVectorVerbFormGoCondBranchReduce(rows)
			},
		},
		{
			name: "VerbFormIfGuardedReduce",
			tags: []string{"numeric-vector", "sum"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;s:0;if[(sum x)>0;s:(sum x)+count x];s", rows)
			},
			goFn: func(rows int) int64 {
				return qEvalVectorVerbFormGoIfGuardedReduce(rows)
			},
		},
		{
			name: "VerbFormDoAccumulateVectorSum",
			tags: []string{"numeric-vector", "sum"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:1+til %d;acc:0;do[6;acc:acc+sum x];acc", rows)
			},
			goFn: func(rows int) int64 {
				return qEvalVectorVerbFormGoDoAccumulate(rows, 6)
			},
		},
		{
			name: "VerbFormWhileRowBoundSum",
			tags: []string{"numeric-vector", "sum"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:1+til %d;i:0;s:0;while[i<%d;s:s+sum x;i:i+1];s", rows, rows/1024)
			},
			goFn: func(rows int) int64 {
				return qEvalVectorVerbFormGoWhileRowBound(rows)
			},
		},
		{
			name:   "VerbFormXdescColumnProbe",
			tags:   []string{"table-sort", "table-literal", "projection"},
			matrix: []string{"table:xcols-xasc-xdesc:frame"},
			expr: func(rows int) string {
				return fmt.Sprintf("px:til %d;syms:%d#`AAPL`MSFT`NVDA;t:flip `sym`px!(syms;px);d:`px xdesc t;c:d[`px];(first c)+count c", rows, rows)
			},
			goFn: func(rows int) int64 {
				return qEvalVectorVerbFormGoXdescColumnProbe(rows)
			},
		},
	}
}

// Canonical `&` is elementwise min: over the 0/1 and 0..2 mod masks the min
// is 1 exactly when both are nonzero, and `where` over the 0/1 vector
// replicates indexes by count, so the count matches the boolean-and mask.
//
//go:noinline
func qEvalVectorVerbFormGoAmpMaskCount(rows int) int64 {
	var count int64
	for i := 0; i < rows; i++ {
		x := qEvalVectorGoBaselineInput[i]
		if x%2 != 0 && x%3 != 0 {
			count++
		}
	}
	return count
}

// Canonical `|` is elementwise max: `count where m` over the integer max
// mask follows where's replication semantics (indexes repeat per count), so
// the expected value is the sum of max(x mod 2, x mod 5).
//
//go:noinline
func qEvalVectorVerbFormGoPipeMaskCount(rows int) int64 {
	var count int64
	for i := 0; i < rows; i++ {
		x := qEvalVectorGoBaselineInput[i]
		a := x % 2
		b := x % 5
		if b > a {
			a = b
		}
		count += a
	}
	return count
}

//go:noinline
func qEvalVectorVerbFormGoDivideMinusSum(rows int) int64 {
	offset := int64(rows / 2)
	var sum float64
	for i := 0; i < rows; i++ {
		y := qEvalVectorGoBaselineInput[i] - offset
		sum += float64(y) / 2
	}
	return int64(sum)
}

//go:noinline
func qEvalVectorVerbFormGoEqualEqualsCount(rows int) int64 {
	var a, b int64
	for i := 0; i < rows; i++ {
		x := qEvalVectorGoBaselineInput[i]
		if x%7 == 3 {
			a++
		}
		if x%5 == 2 {
			b++
		}
	}
	return a + b
}

//go:noinline
func qEvalVectorVerbFormGoLessGreaterMoreCount(rows int) int64 {
	lessBound := int64(rows / 4)
	greaterBound := int64(rows / 2)
	moreBound := int64(rows * 3 / 4)
	var less, greater, more int64
	for i := 0; i < rows; i++ {
		x := qEvalVectorGoBaselineInput[i]
		if x < lessBound {
			less++
		}
		if x > greaterBound {
			greater++
		}
		if x > moreBound {
			more++
		}
	}
	return less + greater + more
}

//go:noinline
func qEvalVectorVerbFormGoLeftRightGather(rows int) int64 {
	x := qEvalVectorGoBaselineMaterializeTil(rows)
	y := qEvalVectorGoBaselineMaterializeReverse(rows)
	a := make([]int64, rows)
	b := make([]int64, rows)
	for i := 0; i < rows; i++ {
		a[i] = x[i]
		b[i] = y[i]
	}
	qEvalVectorAnyBenchSink = []any{a, b}
	var sum int64
	for i := 0; i < rows; i++ {
		sum += a[i]
	}
	return sum + b[0]
}

//go:noinline
func qEvalVectorVerbFormGoFillPriorShiftSum(rows int) int64 {
	x := make([]int64, rows)
	for i := 0; i < rows; i++ {
		x[i] = qEvalVectorGoBaselineInput[i] + 1
	}
	p := make([]int64, rows)
	for i := 0; i < rows; i++ {
		if i == 0 {
			// prior shifts in a null head; 0 fill replaces it with 0.
			p[i] = 0
			continue
		}
		p[i] = x[i-1]
	}
	qEvalVectorAnyBenchSink = p
	var sum int64
	for i := 0; i < rows; i++ {
		sum += p[i]
	}
	return sum + int64(len(p))
}

//go:noinline
func qEvalVectorVerbFormGoIntersectSentinelSum(rows int) int64 {
	sentinels := [...]int64{100, 5000, 9000, 20000}
	matched := make([]int64, 0, len(sentinels))
	for i := 0; i < rows; i++ {
		x := qEvalVectorGoBaselineInput[i]
		inRight := false
		for _, s := range sentinels {
			if x == s {
				inRight = true
				break
			}
		}
		if !inRight {
			continue
		}
		seen := false
		for _, m := range matched {
			if m == x {
				seen = true
				break
			}
		}
		if !seen {
			matched = append(matched, x)
		}
	}
	qEvalVectorAnyBenchSink = matched
	var sum int64
	for _, m := range matched {
		sum += m
	}
	return sum + int64(len(matched))
}

//go:noinline
func qEvalVectorVerbFormGoLtrimRtrimMatch(rows int) int64 {
	pattern := " AAPL "
	cycled := make([]byte, rows)
	for i := 0; i < rows; i++ {
		cycled[i] = pattern[i%len(pattern)]
	}
	s := string(cycled)
	a := strings.TrimLeftFunc(s, unicode.IsSpace)
	b := strings.TrimRightFunc(s, unicode.IsSpace)
	if a == b {
		return 0
	}
	return int64(len([]rune(a))) + int64(len([]rune(b)))
}

//go:noinline
func qEvalVectorVerbFormGoCondBranchReduce(rows int) int64 {
	var sum int64
	for i := 0; i < rows; i++ {
		sum += qEvalVectorGoBaselineInput[i]
	}
	var out int64
	if sum > 10000000 {
		for i := 0; i < rows; i++ {
			out += qEvalVectorGoBaselineInput[i] * 2
		}
		return out
	}
	for i := 0; i < rows; i++ {
		out += qEvalVectorGoBaselineInput[i] * 3
	}
	return out
}

//go:noinline
func qEvalVectorVerbFormGoIfGuardedReduce(rows int) int64 {
	var cond int64
	for i := 0; i < rows; i++ {
		cond += qEvalVectorGoBaselineInput[i]
	}
	var s int64
	if cond > 0 {
		var sum int64
		for i := 0; i < rows; i++ {
			sum += qEvalVectorGoBaselineInput[i]
		}
		s = sum + int64(rows)
	}
	return s
}

//go:noinline
func qEvalVectorVerbFormGoDoAccumulate(rows, iterations int) int64 {
	var acc int64
	for iter := 0; iter < iterations; iter++ {
		var sum int64
		for i := 0; i < rows; i++ {
			sum += qEvalVectorGoBaselineInput[i] + 1
		}
		acc += sum
	}
	return acc
}

//go:noinline
func qEvalVectorVerbFormGoWhileRowBound(rows int) int64 {
	bound := rows / 1024
	var s int64
	for i := 0; i < bound; i++ {
		var sum int64
		for j := 0; j < rows; j++ {
			sum += qEvalVectorGoBaselineInput[j] + 1
		}
		s += sum
	}
	return s
}

//go:noinline
func qEvalVectorVerbFormGoXdescColumnProbe(rows int) int64 {
	pattern := []string{"AAPL", "MSFT", "NVDA"}
	syms := make([]string, rows)
	px := make([]int64, rows)
	for i := 0; i < rows; i++ {
		syms[i] = pattern[i%len(pattern)]
		px[i] = qEvalVectorGoBaselineInput[i]
	}
	idx := make([]int, rows)
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(i, j int) bool { return px[idx[i]] > px[idx[j]] })
	sortedPx := make([]int64, rows)
	sortedSyms := make([]string, rows)
	for to, from := range idx {
		sortedPx[to] = px[from]
		sortedSyms[to] = syms[from]
	}
	qEvalVectorAnyBenchSink = []any{sortedPx, sortedSyms}
	return sortedPx[0] + int64(len(sortedPx))
}

// TestQEvalVectorVerbFormBacklogCaseValues is the oracle for this case
// family: every case's q.eval result must exactly equal its Go baseline at
// full and half row counts, and the Go baseline must be row-scaled.
func TestQEvalVectorVerbFormBacklogCaseValues(t *testing.T) {
	eval := qEvalVectorEval(t)
	seen := map[string]struct{}{}
	for _, tc := range qEvalVectorVerbFormBacklogCases() {
		tc := tc
		if _, dup := seen[tc.name]; dup {
			t.Fatalf("duplicate verb-form case name %q", tc.name)
		}
		seen[tc.name] = struct{}{}
		t.Run(tc.name, func(t *testing.T) {
			for _, rows := range []int{qEvalVectorRows, qEvalVectorRows / 2} {
				src := tc.expr(rows)
				got := qEvalVectorRun(t, eval, src)
				want := tc.goFn(rows)
				if got != want {
					t.Fatalf("rows=%d q.eval(%q) = %d, want Go baseline %d", rows, src, got, want)
				}
			}
			if tc.goFn(qEvalVectorRows) == tc.goFn(qEvalVectorRows/2) {
				t.Fatalf("case %q is row-invariant: goFn(%d) == goFn(%d)", tc.name, qEvalVectorRows, qEvalVectorRows/2)
			}
		})
	}
}
