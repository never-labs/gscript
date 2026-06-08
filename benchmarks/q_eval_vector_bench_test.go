package benchmarks

import (
	"fmt"
	"strings"
	"testing"

	"github.com/never-labs/leia/internal/stdlib/bind"
)

const qEvalVectorRows = 8192

var qEvalVectorBenchSink int64

type qEvalVectorCase struct {
	name string
	expr func(rows int) string
	goFn func(rows int) int64
}

var qEvalVectorCases = []qEvalVectorCase{
	{
		name: "VectorArithmetic",
		expr: func(rows int) string {
			return fmt.Sprintf("x:til %d;y:(x*3)+7;+/y*y", rows)
		},
		goFn: func(rows int) int64 {
			var sum int64
			for i := int64(0); i < int64(rows); i++ {
				y := i*3 + 7
				sum += y * y
			}
			return sum
		},
	},
	{
		name: "MaskWhere",
		expr: func(rows int) string {
			return fmt.Sprintf("x:til %d;idx:where x>=1000;+/idx", rows)
		},
		goFn: func(rows int) int64 {
			var sum int64
			for i := int64(0); i < int64(rows); i++ {
				if i >= 1000 {
					sum += i
				}
			}
			return sum
		},
	},
	{
		name: "TakeDropReverseRotate",
		expr: func(rows int) string {
			return fmt.Sprintf("x:til %d;a:128#x;b:drop 64 x;c:reverse x;d:257 rotate x;(+/a)+(+/b)+(+/c)+(+/d)", rows)
		},
		goFn: func(rows int) int64 {
			var take, drop, reverse, rotate int64
			for i := 0; i < 128; i++ {
				take += int64(i % rows)
			}
			for i := 64; i < rows; i++ {
				drop += int64(i)
			}
			for i := rows - 1; i >= 0; i-- {
				reverse += int64(i)
			}
			for i := 0; i < rows; i++ {
				rotate += int64((i + 257) % rows)
			}
			return take + drop + reverse + rotate
		},
	},
	{
		name: "AdverbReductions",
		expr: func(rows int) string {
			return fmt.Sprintf("x:1+til %d;s:+/x;scan:+\\x;named:sums x;s+last scan+last named", rows)
		},
		goFn: func(rows int) int64 {
			var sum, scanLast, namedLast int64
			for i := int64(1); i <= int64(rows); i++ {
				sum += i
				scanLast += i
				namedLast += i
			}
			return sum + scanLast + namedLast
		},
	},
	{
		name: "ListReductions",
		expr: func(rows int) string {
			return fmt.Sprintf("x:til %d;s:+/x;named:sum x;mx:max x;mn:min x;cnt:count x;s+named+mx+mn+cnt", rows)
		},
		goFn: func(rows int) int64 {
			var sum, named, max, min, count int64
			for i := int64(0); i < int64(rows); i++ {
				sum += i
				named += i
				if i == 0 || i > max {
					max = i
				}
				if i == 0 || i < min {
					min = i
				}
				count++
			}
			return sum + named + max + min + count
		},
	},
}

func TestQEvalVectorBenchmarkExpressions(t *testing.T) {
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

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				qEvalVectorBenchSink = qEvalVectorRun(b, eval, src)
			}
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
