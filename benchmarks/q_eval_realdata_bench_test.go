//go:build leia_q

package benchmarks

// Realistic-data annex for the q.eval performance suite.
//
// Every case in the synthetic 483-case suite builds its data in-q (til-affine
// or N#pattern-periodic), so lazy carriers and closed-form kernels can absorb
// the work. This annex injects data through the EvalWithEnv environment as
// dense Go-built columns, so const-memo and lazy-carrier closed forms cannot
// fire: the measured cost is the real dense-kernel cost an embedder sees on
// external data. Each case carries a hand-written, //go:noinline, row-scaled
// Go baseline over the SAME dense slices, and a Test* verifies exact value
// equality between the q result and the baseline at two row counts.
//
// Naming: BenchmarkQEvalRealDataWarm/<case> vs
// BenchmarkQEvalRealDataGoBaseline/<case>. The ratio family in
// leia bench q-report is realdata_go_ratio, reported separately from the
// synthetic families.

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"sync"
	"testing"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
	stdq "github.com/never-labs/leia/internal/stdlib/lib/q"
)

const (
	qRealDataRows          = 8192
	qRealDataRowsLarge     = 1 << 20
	qRealDataTileRows      = 100000
	qRealDataTilePeriod    = 5000 // > predAnalyticMaxPeriod (4096): pins the cheap bail
	qRealDataSymCount      = 256
	qRealDataAjSymCount    = 64
	qRealDataFbySymCount   = 1024
	qRealDataThrashSources = 600
	qRealDataValueDomain   = 1_000_000
	qRealDataSeed          = 20260612
)

var qRealDataBenchSink uint64

// qRealDataSet holds the deterministic master datasets. Everything is drawn
// from one fixed-seed PRNG in a fixed order so runs are reproducible.
type qRealDataSet struct {
	a, b, c, d []int64 // where-chain leaves
	v          []int64 // generic random value column
	xLarge     []int64 // 1M random values
	idxLarge   []int64 // 1M random gather indexes (reduced mod rows per size)
	nullVals   []int64 // 40%-null column values (valid where nullMask false)
	nullMask   []bool  // true = null
	zipf       []int64 // Zipf-duplicate values (1024 distinct)
	px         []int64 // price cents
	pxSym      []int64 // symbol index per px row (256 syms)
	tradeTs    []int64 // sorted-random trade timestamps (nanos)
	tradePx    []int64
	tradeSym   []int64 // 64 partitions
	quoteTs    []int64 // sorted-random quote timestamps; first 64 seed every sym at ts=0
	quoteBid   []int64
	quoteSym   []int64
	f64        []float64 // NaN/Inf-laden float column
	tileP      []int64   // 5000-row periodic pattern source
	w8         []int64   // 8-row tiny vector
	symNames   []string  // "s0".."s1023"
}

var qRealData = buildQRealDataSet()

func buildQRealDataSet() *qRealDataSet {
	r := rand.New(rand.NewSource(qRealDataSeed))
	randI64s := func(n int, domain int64) []int64 {
		out := make([]int64, n)
		for i := range out {
			out[i] = r.Int63n(domain)
		}
		return out
	}
	s := &qRealDataSet{}
	s.a = randI64s(qRealDataRows, qRealDataValueDomain)
	s.b = randI64s(qRealDataRows, qRealDataValueDomain)
	s.c = randI64s(qRealDataRows, qRealDataValueDomain)
	s.d = randI64s(qRealDataRows, qRealDataValueDomain)
	s.v = randI64s(qRealDataRows, qRealDataValueDomain)
	s.xLarge = randI64s(qRealDataRowsLarge, qRealDataValueDomain)
	s.idxLarge = randI64s(qRealDataRowsLarge, qRealDataRowsLarge)

	s.nullVals = randI64s(qRealDataRows, qRealDataValueDomain)
	s.nullMask = make([]bool, qRealDataRows)
	for i := range s.nullMask {
		s.nullMask[i] = r.Intn(100) < 40
	}

	zipf := rand.NewZipf(r, 1.4, 1.0, qRealDataFbySymCount-1)
	s.zipf = make([]int64, qRealDataRows)
	for i := range s.zipf {
		s.zipf[i] = int64(zipf.Uint64())
	}

	s.px = randI64s(qRealDataRows, qRealDataValueDomain)
	s.pxSym = randI64s(qRealDataRows, qRealDataSymCount)

	s.tradeTs = make([]int64, qRealDataRows)
	ts := int64(1)
	for i := range s.tradeTs {
		ts += 1 + r.Int63n(1_000_000)
		s.tradeTs[i] = ts
	}
	s.tradePx = randI64s(qRealDataRows, qRealDataValueDomain)
	s.tradeSym = randI64s(qRealDataRows, qRealDataAjSymCount)

	quoteRows := qRealDataAjSymCount + 2*qRealDataRows
	s.quoteTs = make([]int64, quoteRows)
	s.quoteSym = make([]int64, quoteRows)
	s.quoteBid = randI64s(quoteRows, qRealDataValueDomain)
	for i := 0; i < qRealDataAjSymCount; i++ {
		s.quoteTs[i] = 0 // seed every partition so no trade joins null
		s.quoteSym[i] = int64(i)
	}
	ts = 0
	for i := qRealDataAjSymCount; i < quoteRows; i++ {
		ts += 1 + r.Int63n(500_000)
		s.quoteTs[i] = ts
		s.quoteSym[i] = r.Int63n(qRealDataAjSymCount)
	}

	s.f64 = make([]float64, qRealDataRows)
	for i := range s.f64 {
		switch {
		case i%50 == 17:
			s.f64[i] = math.NaN()
		case i%97 == 31:
			s.f64[i] = math.Inf(1)
		case i%101 == 47:
			s.f64[i] = math.Inf(-1)
		default:
			s.f64[i] = r.Float64() * 1000
		}
	}

	s.tileP = randI64s(qRealDataTilePeriod, 1000)
	s.w8 = randI64s(8, qRealDataValueDomain)

	s.symNames = make([]string, qRealDataFbySymCount)
	for i := range s.symNames {
		s.symNames[i] = fmt.Sprintf("s%d", i)
	}
	return s
}

// qRealDataEnvCache memoizes per-(case,rows) env maps so the timed loop never
// re-copies master slices into fresh Arrays. The env map values are shared,
// immutable Arrays; EvalWithEnv clones only the map per call.
var (
	qRealDataEnvMu    sync.Mutex
	qRealDataEnvCache = map[string]map[string]any{}
)

func qRealDataEnvFor(key string, build func() map[string]any) map[string]any {
	qRealDataEnvMu.Lock()
	defer qRealDataEnvMu.Unlock()
	if env, ok := qRealDataEnvCache[key]; ok {
		return env
	}
	env := build()
	qRealDataEnvCache[key] = env
	return env
}

func qRealDataGatherIdx(rows int) []int64 {
	out := make([]int64, rows)
	for i := 0; i < rows; i++ {
		out[i] = qRealData.idxLarge[i] % int64(rows)
	}
	return out
}

func qRealDataNullColumn(rows int) data.Array {
	vals := make([]any, rows)
	for i := 0; i < rows; i++ {
		if qRealData.nullMask[i] {
			vals[i] = data.NullValue
		} else {
			vals[i] = qRealData.nullVals[i]
		}
	}
	return data.InferArray(vals)
}

func qRealDataSyms(idx []int64, rows int) data.Array {
	names := make([]string, rows)
	for i := 0; i < rows; i++ {
		names[i] = qRealData.symNames[idx[i]]
	}
	return data.NewSymbols(names)
}

func qRealDataTimestamps(ts []int64, rows int) data.Array {
	out := make([]data.Timestamp, rows)
	for i := 0; i < rows; i++ {
		out[i] = data.TimestampFromUnixNanos(ts[i])
	}
	return data.NewTimestamp(out)
}

func qRealDataTradesFrame(rows int) data.Frame {
	frame, err := data.NewFrame(
		data.Column{Name: "sym", Data: qRealDataSyms(qRealData.tradeSym, rows)},
		data.Column{Name: "ts", Data: qRealDataTimestamps(qRealData.tradeTs, rows)},
		data.Column{Name: "price", Data: data.NewI64(qRealData.tradePx[:rows])},
	)
	if err != nil {
		panic(err)
	}
	return frame
}

func qRealDataQuotesFrame(rows int) data.Frame {
	quoteRows := qRealDataAjSymCount + 2*rows
	frame, err := data.NewFrame(
		data.Column{Name: "sym", Data: qRealDataSyms(qRealData.quoteSym, quoteRows)},
		data.Column{Name: "ts", Data: qRealDataTimestamps(qRealData.quoteTs, quoteRows)},
		data.Column{Name: "bid", Data: data.NewI64(qRealData.quoteBid[:quoteRows])},
	)
	if err != nil {
		panic(err)
	}
	return frame
}

func qRealDataPxFrame(rows int) data.Frame {
	frame, err := data.NewFrame(
		data.Column{Name: "sym", Data: qRealDataSyms(qRealData.pxSym, rows)},
		data.Column{Name: "px", Data: data.NewI64(qRealData.px[:rows])},
	)
	if err != nil {
		panic(err)
	}
	return frame
}

var (
	qRealDataFrameMu    sync.Mutex
	qRealDataFrameCache = map[string]data.Frame{}
)

func qRealDataFrameFor(key string, build func() data.Frame) data.Frame {
	qRealDataFrameMu.Lock()
	defer qRealDataFrameMu.Unlock()
	if frame, ok := qRealDataFrameCache[key]; ok {
		return frame
	}
	frame := build()
	qRealDataFrameCache[key] = frame
	return frame
}

var (
	qRealDataGroupByOnce     sync.Once
	qRealDataGroupByLowered_ *stdq.Lowered
)

func qRealDataGroupByLowered() *stdq.Lowered {
	qRealDataGroupByOnce.Do(func() {
		query, err := stdq.Parse("select s:sum px by sym from t")
		if err != nil {
			panic(err)
		}
		lowered, err := stdq.Lower(query)
		if err != nil {
			panic(err)
		}
		qRealDataGroupByLowered_ = lowered
	})
	return qRealDataGroupByLowered_
}

// --- q-side evaluation helpers ---

func qRealDataEval(tb testing.TB, src string, env map[string]any) any {
	tb.Helper()
	out, err := stdq.EvalWithEnv(src, env)
	if err != nil {
		tb.Fatalf("EvalWithEnv(%q): %v", src, err)
	}
	return out
}

func qRealDataI64(tb testing.TB, v any) uint64 {
	tb.Helper()
	n, ok := v.(int64)
	if !ok {
		tb.Fatalf("q result is %T (%v), want int64", v, v)
	}
	return uint64(n)
}

func qRealDataF64(tb testing.TB, v any) uint64 {
	tb.Helper()
	f, ok := v.(float64)
	if !ok {
		tb.Fatalf("q result is %T (%v), want float64", v, v)
	}
	return math.Float64bits(f)
}

func qRealDataColumnI64s(tb testing.TB, v any, name string) []int64 {
	tb.Helper()
	frame, ok := v.(data.Frame)
	if !ok {
		tb.Fatalf("q result is %T, want data.Frame", v)
	}
	col, ok := frame.Column(data.Symbol(name))
	if !ok {
		tb.Fatalf("q result frame has no column %q", name)
	}
	out := make([]int64, col.Len())
	if handled, err := data.TryExportI64Copy(col, out); err != nil {
		tb.Fatalf("column %q typed i64 export failed: %v", name, err)
	} else if handled {
		return out
	}
	for i := 0; i < col.Len(); i++ {
		item, ok := col.At(i)
		if !ok {
			tb.Fatalf("column %q row %d out of range", name, i)
		}
		switch n := item.(type) {
		case int64:
			out[i] = n
		case float64:
			// Aggregates may promote to f64; the annex keeps magnitudes
			// far below 2^53 so the integral round-trip is exact.
			if n != math.Trunc(n) {
				tb.Fatalf("column %q row %d is non-integral float %v", name, i, n)
			}
			out[i] = int64(n)
		default:
			tb.Fatalf("column %q row %d is %T, want int64 or integral float64", name, i, item)
		}
	}
	return out
}

const qRealDataChecksumMod = 1_000_003

// qRealDataGroupChecksum folds per-group sums order-independently so the
// checksum pins per-group values, not just the (permutation-invariant) total.
func qRealDataGroupChecksum(sums []int64) uint64 {
	var total int64
	for _, s := range sums {
		m := s % qRealDataChecksumMod
		total += m * m
	}
	return uint64(total)
}

// --- Go baselines: hand-written, row-scaled, over the same dense slices ---

//go:noinline
func qRealDataGoWhereChainSum(rows int, thr int64) uint64 {
	a, b, c, d, v := qRealData.a, qRealData.b, qRealData.c, qRealData.d, qRealData.v
	var total int64
	for i := 0; i < rows; i++ {
		if a[i] > thr && b[i] > thr && c[i] > thr && d[i] > thr {
			total += v[i]
		}
	}
	return uint64(total)
}

//go:noinline
func qRealDataGoGatherSum(rows int) uint64 {
	x := qRealData.xLarge
	idx := qRealDataGatherIdxFor(rows)
	var total int64
	for i := 0; i < rows; i++ {
		total += x[idx[i]]
	}
	return uint64(total)
}

//go:noinline
func qRealDataGoFillsMsumSum(rows, width int) uint64 {
	vals, nulls := qRealData.nullVals, qRealData.nullMask
	ring := make([]int64, width)
	var window, total int64
	last := int64(0) // 0^ fills leading nulls with zero
	for i := 0; i < rows; i++ {
		if !nulls[i] {
			last = vals[i]
		}
		window += last
		if i >= width {
			window -= ring[i%width]
		}
		ring[i%width] = last
		total += window
	}
	return uint64(total)
}

//go:noinline
func qRealDataGoMavgSum(rows, width int) uint64 {
	v := qRealData.v
	// Bit-exact mirror of the q moving-average reduction: int64 sliding
	// window, divided by the prefix-clamped count per row.
	var window int64
	var total float64
	for i := 0; i < rows; i++ {
		window += v[i]
		if i >= width {
			window -= v[i-width]
		}
		count := i + 1
		if count > width {
			count = width
		}
		total += float64(window) / float64(count)
	}
	return math.Float64bits(total)
}

//go:noinline
func qRealDataGoIascDropGatherSum(values []int64, rows, drop int) uint64 {
	sorted := make([]int64, rows)
	copy(sorted, values[:rows])
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	var total int64
	for _, value := range sorted[drop:] {
		total += value
	}
	return uint64(total)
}

//go:noinline
func qRealDataGoFbySum(rows int) uint64 {
	v, g := qRealData.v, qRealData.zipf
	var sums [qRealDataFbySymCount]int64
	for i := 0; i < rows; i++ {
		sums[g[i]] += v[i]
	}
	var total int64
	for i := 0; i < rows; i++ {
		total += sums[g[i]]
	}
	return uint64(total)
}

//go:noinline
func qRealDataGoGroupBySum(rows int) uint64 {
	px, sym := qRealData.px, qRealData.pxSym
	var sums [qRealDataSymCount]int64
	var seen [qRealDataSymCount]bool
	for i := 0; i < rows; i++ {
		sums[sym[i]] += px[i]
		seen[sym[i]] = true
	}
	groupSums := make([]int64, 0, qRealDataSymCount)
	for s := 0; s < qRealDataSymCount; s++ {
		if seen[s] {
			groupSums = append(groupSums, sums[s])
		}
	}
	return qRealDataGroupChecksum(groupSums)
}

//go:noinline
func qRealDataGoAsofJoinBidSum(rows int) uint64 {
	quoteRows := qRealDataAjSymCount + 2*rows
	type quote struct {
		ts  int64
		bid int64
	}
	perSym := make([][]quote, qRealDataAjSymCount)
	for i := 0; i < quoteRows; i++ {
		s := qRealData.quoteSym[i]
		perSym[s] = append(perSym[s], quote{ts: qRealData.quoteTs[i], bid: qRealData.quoteBid[i]})
	}
	var total int64
	for i := 0; i < rows; i++ {
		quotes := perSym[qRealData.tradeSym[i]]
		ts := qRealData.tradeTs[i]
		// Last quote with quote.ts <= trade.ts (quotes are ts-sorted).
		lo, hi := 0, len(quotes)
		for lo < hi {
			mid := (lo + hi) / 2
			if quotes[mid].ts <= ts {
				lo = mid + 1
			} else {
				hi = mid
			}
		}
		if lo > 0 {
			total += quotes[lo-1].bid
		}
	}
	return uint64(total)
}

//go:noinline
func qRealDataGoXrankWeightedSum(rows int, buckets int64) uint64 {
	v := qRealData.v
	indexes := make([]int, rows)
	for i := range indexes {
		indexes[i] = i
	}
	sort.Slice(indexes, func(i, j int) bool { return v[indexes[i]] < v[indexes[j]] })
	out := make([]int64, rows)
	rank := 0
	for rank < rows {
		next := rank + 1
		for next < rows && v[indexes[next]] == v[indexes[rank]] {
			next++
		}
		bucket := int64(rank) * buckets / int64(rows)
		if bucket >= buckets {
			bucket = buckets - 1
		}
		for _, index := range indexes[rank:next] {
			out[index] = bucket
		}
		rank = next
	}
	var total int64
	for i := 0; i < rows; i++ {
		total += v[i] * out[i]
	}
	return uint64(total)
}

//go:noinline
func qRealDataGoXbarBucketCount(rows int, width, want float64) uint64 {
	f := qRealData.f64
	var count int64
	for i := 0; i < rows; i++ {
		if math.Floor(f[i]/width)*width == want {
			count++
		}
	}
	return uint64(count)
}

//go:noinline
func qRealDataGoTileCount(tileRows int, thr int64) uint64 {
	p := qRealData.tileP
	var count int64
	for i := 0; i < tileRows; i++ {
		if p[i%qRealDataTilePeriod] > thr {
			count++
		}
	}
	return uint64(count)
}

//go:noinline
func qRealDataGoTinySum(rows int, thr int64) uint64 {
	w := qRealData.w8
	var total int64
	for i := 0; i < rows; i++ {
		if w[i] > thr {
			total += w[i]
		}
	}
	return uint64(total)
}

func qRealDataThrashThreshold(iter int) int64 {
	return int64(iter%qRealDataThrashSources) * 1663 % qRealDataValueDomain
}

//go:noinline
func qRealDataGoThrashSum(rows, iter int) uint64 {
	v := qRealData.v
	thr := qRealDataThrashThreshold(iter)
	var total int64
	for i := 0; i < rows; i++ {
		if v[i] > thr {
			total += v[i]
		}
	}
	return uint64(total)
}

var (
	qRealDataGatherIdxMu    sync.Mutex
	qRealDataGatherIdxCache = map[int][]int64{}
)

func qRealDataGatherIdxFor(rows int) []int64 {
	qRealDataGatherIdxMu.Lock()
	defer qRealDataGatherIdxMu.Unlock()
	if idx, ok := qRealDataGatherIdxCache[rows]; ok {
		return idx
	}
	idx := qRealDataGatherIdx(rows)
	qRealDataGatherIdxCache[rows] = idx
	return idx
}

// --- case table ---

type qRealDataCase struct {
	name string
	rows int
	// checkIters lists the iteration values exercised by the equality test
	// (round-robin cases vary the source per iteration).
	checkIters []int
	eval       func(tb testing.TB, rows, iter int) uint64
	goFn       func(rows, iter int) uint64
	// prewarm warms every distinct source before the timed loop.
	prewarm func(tb testing.TB, rows int)
}

func qRealDataWhereChainEnv(rows int) map[string]any {
	return qRealDataEnvFor(fmt.Sprintf("wherechain/%d", rows), func() map[string]any {
		return map[string]any{
			"a": data.NewI64(qRealData.a[:rows]),
			"b": data.NewI64(qRealData.b[:rows]),
			"c": data.NewI64(qRealData.c[:rows]),
			"d": data.NewI64(qRealData.d[:rows]),
			"v": data.NewI64(qRealData.v[:rows]),
		}
	})
}

func qRealDataVEnv(rows int) map[string]any {
	return qRealDataEnvFor(fmt.Sprintf("v/%d", rows), func() map[string]any {
		return map[string]any{"v": data.NewI64(qRealData.v[:rows])}
	})
}

func qRealDataWhereChainCase(name string, thr int64) qRealDataCase {
	src := fmt.Sprintf("+/v where (a>%d)&(b>%d)&(c>%d)&(d>%d)", thr, thr, thr, thr)
	return qRealDataCase{
		name: name,
		rows: qRealDataRows,
		eval: func(tb testing.TB, rows, _ int) uint64 {
			return qRealDataI64(tb, qRealDataEval(tb, src, qRealDataWhereChainEnv(rows)))
		},
		goFn: func(rows, _ int) uint64 { return qRealDataGoWhereChainSum(rows, thr) },
	}
}

var qRealDataCases = []qRealDataCase{
	qRealDataWhereChainCase("WhereChainAnd4Mid", 500_000),
	qRealDataWhereChainCase("WhereChainAnd4Sel0p01", 900_000),
	qRealDataWhereChainCase("WhereChainAnd4Sel99", 2_500),
	{
		name: "GatherSum1M",
		rows: qRealDataRowsLarge,
		eval: func(tb testing.TB, rows, _ int) uint64 {
			env := qRealDataEnvFor(fmt.Sprintf("gather/%d", rows), func() map[string]any {
				return map[string]any{
					"x":   data.NewI64(qRealData.xLarge[:rows]),
					"idx": data.NewI64(qRealDataGatherIdxFor(rows)),
				}
			})
			return qRealDataI64(tb, qRealDataEval(tb, "+/x[idx]", env))
		},
		goFn: func(rows, _ int) uint64 { return qRealDataGoGatherSum(rows) },
	},
	{
		name: "FillsMsumNull40",
		rows: qRealDataRows,
		eval: func(tb testing.TB, rows, _ int) uint64 {
			env := qRealDataEnvFor(fmt.Sprintf("nullcol/%d", rows), func() map[string]any {
				return map[string]any{"c": qRealDataNullColumn(rows)}
			})
			return qRealDataI64(tb, qRealDataEval(tb, "+/20 msum 0^fills c", env))
		},
		goFn: func(rows, _ int) uint64 { return qRealDataGoFillsMsumSum(rows, 20) },
	},
	{
		name: "MavgSumDenseRandom",
		rows: qRealDataRows,
		eval: func(tb testing.TB, rows, _ int) uint64 {
			return qRealDataF64(tb, qRealDataEval(tb, "+/20 mavg v", qRealDataVEnv(rows)))
		},
		goFn: func(rows, _ int) uint64 { return qRealDataGoMavgSum(rows, 20) },
	},
	{
		name: "IascDropGatherRandom8K",
		rows: qRealDataRows,
		eval: func(tb testing.TB, rows, _ int) uint64 {
			return qRealDataI64(tb, qRealDataEval(tb, "+/v[1000 _ iasc v]", qRealDataVEnv(rows)))
		},
		goFn: func(rows, _ int) uint64 { return qRealDataGoIascDropGatherSum(qRealData.v, rows, 1000) },
	},
	{
		name: "IascDropGatherRandom1M",
		rows: qRealDataRowsLarge,
		eval: func(tb testing.TB, rows, _ int) uint64 {
			env := qRealDataEnvFor(fmt.Sprintf("xlarge/%d", rows), func() map[string]any {
				return map[string]any{"x": data.NewI64(qRealData.xLarge[:rows])}
			})
			return qRealDataI64(tb, qRealDataEval(tb, "+/x[1000 _ iasc x]", env))
		},
		goFn: func(rows, _ int) uint64 { return qRealDataGoIascDropGatherSum(qRealData.xLarge, rows, 1000) },
	},
	{
		name: "IascZipfDuplicates",
		rows: qRealDataRows,
		eval: func(tb testing.TB, rows, _ int) uint64 {
			env := qRealDataEnvFor(fmt.Sprintf("zipf/%d", rows), func() map[string]any {
				return map[string]any{"z": data.NewI64(qRealData.zipf[:rows])}
			})
			return qRealDataI64(tb, qRealDataEval(tb, "+/z[1000 _ iasc z]", env))
		},
		goFn: func(rows, _ int) uint64 { return qRealDataGoIascDropGatherSum(qRealData.zipf, rows, 1000) },
	},
	{
		name: "FbyZipfSym1024",
		rows: qRealDataRows,
		eval: func(tb testing.TB, rows, _ int) uint64 {
			env := qRealDataEnvFor(fmt.Sprintf("fby/%d", rows), func() map[string]any {
				return map[string]any{
					"v": data.NewI64(qRealData.v[:rows]),
					"g": qRealDataSyms(qRealData.zipf, rows),
				}
			})
			return qRealDataI64(tb, qRealDataEval(tb, "+/sum v fby g", env))
		},
		goFn: func(rows, _ int) uint64 { return qRealDataGoFbySum(rows) },
	},
	{
		// qSQL group-by aggregate through the production query runtime
		// (Parse -> Lower -> ExecRuntime, the same path q.sql executes after
		// its plan-cache lookup) over a random px column with 256 symbols.
		name: "QSQLSumBySym256",
		rows: qRealDataRows,
		eval: func(tb testing.TB, rows, _ int) uint64 {
			frame := qRealDataFrameFor(fmt.Sprintf("pxframe/%d", rows), func() data.Frame {
				return qRealDataPxFrame(rows)
			})
			out, err := qRealDataGroupByLowered().ExecRuntime(frame)
			if err != nil {
				tb.Fatalf("qSQL sum-by-sym ExecRuntime: %v", err)
			}
			return qRealDataGroupChecksum(qRealDataColumnI64s(tb, out, "s"))
		},
		goFn: func(rows, _ int) uint64 { return qRealDataGoGroupBySum(rows) },
	},
	{
		// qSQL asof join through data.AsofJoinOn, the production kernel
		// q.sql's aj lowering executes, on sorted-random timestamps with 64
		// symbol partitions.
		name: "QSQLAsofJoin64Sym",
		rows: qRealDataRows,
		eval: func(tb testing.TB, rows, _ int) uint64 {
			trades := qRealDataFrameFor(fmt.Sprintf("trades/%d", rows), func() data.Frame {
				return qRealDataTradesFrame(rows)
			})
			quotes := qRealDataFrameFor(fmt.Sprintf("quotes/%d", rows), func() data.Frame {
				return qRealDataQuotesFrame(rows)
			})
			out, err := data.AsofJoin(trades, quotes, "ts", "sym")
			if err != nil {
				tb.Fatalf("AsofJoin: %v", err)
			}
			bids := qRealDataColumnI64s(tb, out, "bid")
			var total int64
			for _, bid := range bids {
				total += bid
			}
			return uint64(total)
		},
		goFn: func(rows, _ int) uint64 { return qRealDataGoAsofJoinBidSum(rows) },
	},
	{
		name: "Xrank10Weighted",
		rows: qRealDataRows,
		eval: func(tb testing.TB, rows, _ int) uint64 {
			return qRealDataI64(tb, qRealDataEval(tb, "+/v*(10 xrank v)", qRealDataVEnv(rows)))
		},
		goFn: func(rows, _ int) uint64 { return qRealDataGoXrankWeightedSum(rows, 10) },
	},
	{
		name: "XbarNaNInfCountWhere",
		rows: qRealDataRows,
		eval: func(tb testing.TB, rows, _ int) uint64 {
			env := qRealDataEnvFor(fmt.Sprintf("f64/%d", rows), func() map[string]any {
				return map[string]any{"f": data.NewF64(qRealData.f64[:rows])}
			})
			return qRealDataI64(tb, qRealDataEval(tb, "count where (100 xbar f)=200", env))
		},
		goFn: func(rows, _ int) uint64 { return qRealDataGoXbarBucketCount(rows, 100, 200) },
	},
	{
		name: "TiledPeriod5000Count",
		rows: qRealDataTileRows,
		eval: func(tb testing.TB, rows, _ int) uint64 {
			env := qRealDataEnvFor("tile", func() map[string]any {
				return map[string]any{"p": data.NewI64(qRealData.tileP)}
			})
			src := fmt.Sprintf("count where (%d#p)>500", rows)
			return qRealDataI64(tb, qRealDataEval(tb, src, env))
		},
		goFn: func(rows, _ int) uint64 { return qRealDataGoTileCount(rows, 500) },
	},
	{
		name: "TinyVector8Rows",
		rows: 8,
		eval: func(tb testing.TB, rows, _ int) uint64 {
			env := qRealDataEnvFor(fmt.Sprintf("w8/%d", rows), func() map[string]any {
				return map[string]any{"w": data.NewI64(qRealData.w8[:rows])}
			})
			return qRealDataI64(tb, qRealDataEval(tb, "+/w where w>500000", env))
		},
		goFn: func(rows, _ int) uint64 { return qRealDataGoTinySum(rows, 500_000) },
	},
	{
		name:       "PlanThrash600Sources",
		rows:       qRealDataRows,
		checkIters: []int{0, 1, 299, 599, 600},
		eval: func(tb testing.TB, rows, iter int) uint64 {
			src := fmt.Sprintf("+/v where v>%d", qRealDataThrashThreshold(iter))
			return qRealDataI64(tb, qRealDataEval(tb, src, qRealDataVEnv(rows)))
		},
		goFn: func(rows, iter int) uint64 { return qRealDataGoThrashSum(rows, iter) },
		prewarm: func(tb testing.TB, rows int) {
			for iter := 0; iter < qRealDataThrashSources; iter++ {
				src := fmt.Sprintf("+/v where v>%d", qRealDataThrashThreshold(iter))
				qRealDataEval(tb, src, qRealDataVEnv(rows))
			}
		},
	},
}

// TestQEvalRealDataMatchesGoBaseline checksum-verifies every annex case: the
// q result over env-injected dense columns must equal the hand-written Go
// baseline over the same slices, at full and half row counts (which also
// proves the baselines are row-scaled).
func TestQEvalRealDataMatchesGoBaseline(t *testing.T) {
	for _, tc := range qRealDataCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			sizes := []int{tc.rows}
			if half := tc.rows / 2; half > 0 {
				sizes = append(sizes, half)
			}
			iters := tc.checkIters
			if len(iters) == 0 {
				iters = []int{0}
			}
			for _, rows := range sizes {
				for _, iter := range iters {
					got := tc.eval(t, rows, iter)
					want := tc.goFn(rows, iter)
					if got != want {
						t.Fatalf("rows=%d iter=%d: q checksum %d != Go baseline checksum %d", rows, iter, got, want)
					}
				}
			}
		})
	}
}

func TestQEvalRealDataCaseNamesUnique(t *testing.T) {
	seen := map[string]struct{}{}
	for _, tc := range qRealDataCases {
		if _, dup := seen[tc.name]; dup {
			t.Fatalf("duplicate realdata case name %q", tc.name)
		}
		seen[tc.name] = struct{}{}
	}
}

func BenchmarkQEvalRealDataWarm(b *testing.B) {
	for _, tc := range qRealDataCases {
		tc := tc
		b.Run(tc.name, func(b *testing.B) {
			sink := tc.eval(b, tc.rows, 0)
			if tc.prewarm != nil {
				tc.prewarm(b, tc.rows)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				sink ^= tc.eval(b, tc.rows, i)
			}
			b.StopTimer()
			qRealDataBenchSink = sink
		})
	}
}

func BenchmarkQEvalRealDataGoBaseline(b *testing.B) {
	for _, tc := range qRealDataCases {
		tc := tc
		b.Run(tc.name, func(b *testing.B) {
			sink := tc.goFn(tc.rows, 0)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				sink ^= tc.goFn(tc.rows, i)
			}
			b.StopTimer()
			qRealDataBenchSink = sink
		})
	}
}
