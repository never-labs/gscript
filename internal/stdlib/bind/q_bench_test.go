package bind

import (
	"container/heap"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

var qSQLBindBenchSink Value
var qSQLNativeBenchSink int64
var qSQLDataFrameBenchSink data.Frame
var qSQLNativeJoinRowsBenchSink qSQLNativeJoinRows

type qSQLBenchmarkCase struct {
	name string
	tags []string
}

var qSQLBenchmarkCases = []qSQLBenchmarkCase{
	{
		name: "BenchmarkQSQLBindRunSQLWarmCacheSelectWhereProject",
		tags: []string{"qsql", "bind", "warm-cache", "select", "where", "project", "order", "take", "typed-filter", "kernel-stats"},
	},
	{
		name: "BenchmarkQSQLBindRunSQLColdCacheSelectWhereProject",
		tags: []string{"qsql", "bind", "cold-cache", "select", "where", "project", "order", "take", "typed-filter", "kernel-stats"},
	},
	{
		name: "BenchmarkQSQLBindFastArg2WarmCacheSelectWhereProject",
		tags: []string{"qsql", "bind-fastarg", "warm-cache", "select", "where", "project", "order", "take", "typed-filter", "kernel-stats"},
	},
	{
		name: "BenchmarkQSQLBindRunSQLWarmCacheGroupByAggregate",
		tags: []string{"qsql", "bind", "warm-cache", "group", "aggregate", "computed-aggregate", "symbol-key", "typed-filter", "kernel-stats"},
	},
	{
		name: "BenchmarkQSQLBindRunSQLWarmCacheJoin",
		tags: []string{"qsql", "bind", "warm-cache", "join", "inner-join", "symbol-key", "order", "take", "kernel-stats"},
	},
	{
		name: "BenchmarkQSQLNativeGoSelectWhereProject",
		tags: []string{"native-go", "select", "where", "project", "order", "take"},
	},
	{
		name: "BenchmarkQSQLNativeGoGroupByAggregate",
		tags: []string{"native-go", "group", "aggregate", "computed-aggregate", "symbol-key"},
	},
	{
		name: "BenchmarkQSQLNativeGoJoin",
		tags: []string{"native-go", "join", "inner-join", "symbol-key", "order", "take"},
	},
	{
		name: "BenchmarkQSQLNativeGoJoinTopK",
		tags: []string{"native-go", "join", "inner-join", "topk", "symbol-key", "order", "take"},
	},
	{
		name: "BenchmarkQSQLNativeGoJoinTopKMaterialized",
		tags: []string{"native-go", "join", "inner-join", "topk", "materialized", "symbol-key", "order", "take"},
	},
	{
		name: "BenchmarkQSQLDataRuntimeJoinTopK",
		tags: []string{"data-runtime", "join", "inner-join", "topk", "frame-primitive", "symbol-key", "order", "take"},
	},
}

var qSQLRequiredBenchmarkTags = []string{
	"qsql",
	"bind",
	"warm-cache",
	"cold-cache",
	"bind-fastarg",
	"select",
	"where",
	"project",
	"order",
	"take",
	"group",
	"aggregate",
	"computed-aggregate",
	"join",
	"inner-join",
	"symbol-key",
	"typed-filter",
	"kernel-stats",
	"native-go",
	"data-runtime",
	"topk",
	"frame-primitive",
}

func TestQSQLBenchmarkCoverageTags(t *testing.T) {
	covered := make(map[string][]string)
	for _, tc := range qSQLBenchmarkCases {
		for _, tag := range tc.tags {
			covered[tag] = append(covered[tag], tc.name)
		}
	}
	var missing []string
	for _, tag := range qSQLRequiredBenchmarkTags {
		if len(covered[tag]) == 0 {
			missing = append(missing, tag)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("qSQL benchmark coverage missing tags: %s", strings.Join(missing, ", "))
	}
}

func BenchmarkQSQLBindRunSQLWarmCacheSelectWhereProject(b *testing.B) {
	qClearCaches()
	defer qClearCaches()

	const rows = 8192
	frame := qSQLBindBenchTradesFrame(b, rows)
	frameValue := qSQLBindBenchFrameValue(b, frame)
	query := "select sym,price,size from trades where active=true,price>=100 order by price desc take 128"
	args := qSQLArgsResult{frameValue: frameValue, source: query}

	if _, err := qRunSQL("q.sql", args); err != nil {
		b.Fatalf("warm q.sql: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	start := time.Now()
	for i := 0; i < b.N; i++ {
		out, err := qRunSQL("q.sql", args)
		if err != nil {
			b.Fatalf("q.sql: %v", err)
		}
		qSQLBindBenchSink = out
	}
	b.StopTimer()
	qSQLBindBenchReportRows(b, rows, start)
	qSQLBindBenchReportQStats(b)
}

func BenchmarkQSQLBindRunSQLColdCacheSelectWhereProject(b *testing.B) {
	qClearCaches()
	defer qClearCaches()

	const rows = 8192
	frame := qSQLBindBenchTradesFrame(b, rows)
	frameValue := qSQLBindBenchFrameValue(b, frame)
	query := "select sym,price,size from trades where active=true,price>=100 order by price desc take 128"
	args := qSQLArgsResult{frameValue: frameValue, source: query}

	b.ReportAllocs()
	b.ResetTimer()
	start := time.Now()
	for i := 0; i < b.N; i++ {
		qClearCaches()
		out, err := qRunSQL("q.sql", args)
		if err != nil {
			b.Fatalf("q.sql: %v", err)
		}
		qSQLBindBenchSink = out
	}
	b.StopTimer()
	qSQLBindBenchReportRows(b, rows, start)
	qSQLBindBenchReportQStats(b)
}

func BenchmarkQSQLBindFastArg2WarmCacheSelectWhereProject(b *testing.B) {
	qClearCaches()
	defer qClearCaches()

	const rows = 8192
	frame := qSQLBindBenchTradesFrame(b, rows)
	frameValue := qSQLBindBenchFrameValue(b, frame)
	sourceValue := StringValue("select sym,price,size from trades where active=true,price>=100 order by price desc take 128")

	sql := BuildQ().RawGetString("sql").GoFunction()
	if sql == nil || sql.FastArg2 == nil {
		b.Fatalf("q.sql FastArg2 missing: %#v", sql)
	}
	if _, err := sql.FastArg2(frameValue, sourceValue); err != nil {
		b.Fatalf("warm q.sql FastArg2: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	start := time.Now()
	for i := 0; i < b.N; i++ {
		out, err := sql.FastArg2(frameValue, sourceValue)
		if err != nil {
			b.Fatalf("q.sql FastArg2: %v", err)
		}
		qSQLBindBenchSink = out
	}
	b.StopTimer()
	qSQLBindBenchReportRows(b, rows, start)
	qSQLBindBenchReportQStats(b)
}

func BenchmarkQSQLBindRunSQLWarmCacheGroupByAggregate(b *testing.B) {
	qClearCaches()
	defer qClearCaches()

	const rows = 8192
	frame := qSQLBindBenchTradesFrame(b, rows)
	frameValue := qSQLBindBenchFrameValue(b, frame)
	query := "select notional:sum price*size,fills:count i by sym from trades where active=true,price>=100"
	args := qSQLArgsResult{frameValue: frameValue, source: query}

	if _, err := qRunSQL("q.sql", args); err != nil {
		b.Fatalf("warm q.sql: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	start := time.Now()
	for i := 0; i < b.N; i++ {
		out, err := qRunSQL("q.sql", args)
		if err != nil {
			b.Fatalf("q.sql: %v", err)
		}
		qSQLBindBenchSink = out
	}
	b.StopTimer()
	qSQLBindBenchReportRows(b, rows, start)
	qSQLBindBenchReportQStats(b)
}

func BenchmarkQSQLBindRunSQLWarmCacheJoin(b *testing.B) {
	qClearCaches()
	defer qClearCaches()

	const rows = 8192
	tradesValue := qSQLBindBenchFrameValue(b, qSQLBindBenchTradesFrame(b, rows))
	quotesValue := qSQLBindBenchFrameValue(b, qSQLBindBenchQuotesFrame(b))
	env := NewTable()
	env.RawSetString("trades", tradesValue)
	env.RawSetString("quotes", quotesValue)
	envValue := TableValue(env)
	query := "select sym,price,bid,ask from trades join quotes on sym order by price desc take 128"
	args := qSQLArgsResult{
		frameValue:    envValue,
		source:        query,
		resolveSource: true,
		envValue:      envValue,
	}

	if _, err := qRunSQL("q.sql", args); err != nil {
		b.Fatalf("warm q.sql: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	start := time.Now()
	for i := 0; i < b.N; i++ {
		out, err := qRunSQL("q.sql", args)
		if err != nil {
			b.Fatalf("q.sql: %v", err)
		}
		qSQLBindBenchSink = out
	}
	b.StopTimer()
	qSQLBindBenchReportRows(b, rows, start)
	qSQLBindBenchReportQStats(b)
}

func BenchmarkQSQLNativeGoSelectWhereProject(b *testing.B) {
	const rows = 8192
	trades := qSQLNativeBenchTrades(rows)

	b.ReportAllocs()
	b.ResetTimer()
	start := time.Now()
	for i := 0; i < b.N; i++ {
		qSQLNativeBenchSink = qSQLNativeSelectWhereProject(trades)
	}
	b.StopTimer()
	qSQLBindBenchReportRows(b, rows, start)
}

func BenchmarkQSQLNativeGoGroupByAggregate(b *testing.B) {
	const rows = 8192
	trades := qSQLNativeBenchTrades(rows)

	b.ReportAllocs()
	b.ResetTimer()
	start := time.Now()
	for i := 0; i < b.N; i++ {
		qSQLNativeBenchSink = qSQLNativeGroupByAggregate(trades)
	}
	b.StopTimer()
	qSQLBindBenchReportRows(b, rows, start)
}

func BenchmarkQSQLNativeGoJoin(b *testing.B) {
	const rows = 8192
	trades := qSQLNativeBenchTrades(rows)
	quotes := qSQLNativeBenchQuotes()

	b.ReportAllocs()
	b.ResetTimer()
	start := time.Now()
	for i := 0; i < b.N; i++ {
		qSQLNativeBenchSink = qSQLNativeJoin(trades, quotes)
	}
	b.StopTimer()
	qSQLBindBenchReportRows(b, rows, start)
}

func BenchmarkQSQLNativeGoJoinTopK(b *testing.B) {
	const rows = 8192
	trades := qSQLNativeBenchTrades(rows)
	quotes := qSQLNativeBenchQuotes()

	b.ReportAllocs()
	b.ResetTimer()
	start := time.Now()
	for i := 0; i < b.N; i++ {
		qSQLNativeBenchSink = qSQLNativeJoinTopK(trades, quotes)
	}
	b.StopTimer()
	qSQLBindBenchReportRows(b, rows, start)
}

func BenchmarkQSQLNativeGoJoinTopKMaterialized(b *testing.B) {
	const rows = 8192
	trades := qSQLNativeBenchTrades(rows)
	quotes := qSQLNativeBenchQuotes()

	b.ReportAllocs()
	b.ResetTimer()
	start := time.Now()
	for i := 0; i < b.N; i++ {
		qSQLNativeJoinRowsBenchSink = qSQLNativeJoinTopKMaterialized(trades, quotes)
	}
	b.StopTimer()
	qSQLBindBenchReportRows(b, rows, start)
}

func BenchmarkQSQLDataRuntimeJoinTopK(b *testing.B) {
	const rows = 8192
	trades := qSQLBindBenchTradesFrame(b, rows)
	quotes := qSQLBindBenchQuotesFrame(b)
	opts := data.JoinOptions{
		LeftColumns:  []data.Symbol{"sym", "price"},
		RightColumns: []data.Symbol{"sym", "bid", "ask"},
		OrderBy:      []data.JoinOrderSpec{{Column: "price", Desc: true}},
		LimitN:       128,
	}
	keys := []data.JoinKey{{Left: "sym", Right: "sym"}}
	if _, err := data.InnerJoinOnWithOptions(trades, quotes, opts, keys...); err != nil {
		b.Fatalf("warm data join: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	start := time.Now()
	for i := 0; i < b.N; i++ {
		out, err := data.InnerJoinOnWithOptions(trades, quotes, opts, keys...)
		if err != nil {
			b.Fatalf("data join: %v", err)
		}
		qSQLDataFrameBenchSink = out
	}
	b.StopTimer()
	qSQLBindBenchReportRows(b, rows, start)
}

func qSQLBindBenchTradesFrame(b *testing.B, rows int) data.Frame {
	b.Helper()
	trades := qSQLNativeBenchTrades(rows)
	frame, err := data.NewFrame(
		data.Column{Name: "sym", Data: data.NewSymbols(trades.sym)},
		data.Column{Name: "price", Data: data.NewF64(trades.price)},
		data.Column{Name: "size", Data: data.NewI64(trades.size)},
		data.Column{Name: "active", Data: data.NewBool(trades.active)},
	)
	if err != nil {
		b.Fatalf("NewFrame trades: %v", err)
	}
	return frame
}

func qSQLBindBenchQuotesFrame(b *testing.B) data.Frame {
	b.Helper()
	frame, err := data.NewFrame(
		data.Column{Name: "sym", Data: data.NewSymbols([]string{"AAPL", "MSFT", "NVDA", "TSLA", "AMZN", "META", "GOOG", "IBM"})},
		data.Column{Name: "bid", Data: data.NewF64([]float64{100.10, 80.10, 210.10, 190.10, 150.10, 120.10, 130.10, 99.10})},
		data.Column{Name: "ask", Data: data.NewF64([]float64{100.20, 80.20, 210.20, 190.20, 150.20, 120.20, 130.20, 99.20})},
	)
	if err != nil {
		b.Fatalf("NewFrame quotes: %v", err)
	}
	return frame
}

func qSQLBindBenchFrameValue(b *testing.B, frame data.Frame) Value {
	b.Helper()
	value, err := qDataFrameValue(frame)
	if err != nil {
		b.Fatalf("qDataFrameValue: %v", err)
	}
	return value
}

func qSQLBindBenchReportRows(b *testing.B, rows int, start time.Time) {
	b.Helper()
	if elapsed := time.Since(start); elapsed > 0 {
		b.ReportMetric(float64(rows)*float64(b.N)/elapsed.Seconds(), "input_rows/s")
	}
}

func qSQLBindBenchReportQStats(b *testing.B) {
	b.Helper()
	stats := qSQLPlanCacheStatsSnapshot()
	b.ReportMetric(qSQLBindBenchHitPct(stats.TemplateHits, stats.TemplateMisses), "template_hit_pct")
	b.ReportMetric(qSQLBindBenchHitPct(stats.AlignedHits, stats.AlignedMisses), "aligned_hit_pct")
	b.ReportMetric(qSQLBindBenchHitPct(stats.KernelHits, stats.KernelMisses), "kernel_hit_pct")

	qFallbackStatsMu.Lock()
	fallbacks := qFallbackCounters.KernelUnsupported +
		qFallbackCounters.KernelCompileErr +
		qFallbackCounters.SourceErr +
		qFallbackCounters.JoinErr +
		qFallbackCounters.Mutation +
		qFallbackCounters.QueryKernel
	queryKernelHits := qFallbackCounters.QueryKernelHit
	qFallbackStatsMu.Unlock()

	if b.N > 0 {
		b.ReportMetric(float64(fallbacks)/float64(b.N), "fallbacks/op")
		b.ReportMetric(float64(queryKernelHits)/float64(b.N), "query_kernel_supported/op")
	}
}

func qSQLBindBenchHitPct(hits, misses int) float64 {
	total := hits + misses
	if total == 0 {
		return 0
	}
	return 100 * float64(hits) / float64(total)
}

type qSQLNativeTrades struct {
	sym    []string
	price  []float64
	size   []int64
	active []bool
}

type qSQLNativeQuotes struct {
	bid map[string]float64
	ask map[string]float64
}

func qSQLNativeBenchTrades(rows int) qSQLNativeTrades {
	symbols := []string{"AAPL", "MSFT", "NVDA", "TSLA", "AMZN", "META", "GOOG", "IBM"}
	trades := qSQLNativeTrades{
		sym:    make([]string, rows),
		price:  make([]float64, rows),
		size:   make([]int64, rows),
		active: make([]bool, rows),
	}
	for i := 0; i < rows; i++ {
		trades.sym[i] = symbols[i%len(symbols)]
		trades.price[i] = 75 + float64((i*17)%220) + float64(i%10)/10
		trades.size[i] = int64(1 + (i*13)%500)
		trades.active[i] = i%5 != 0
	}
	return trades
}

func qSQLNativeBenchQuotes() qSQLNativeQuotes {
	return qSQLNativeQuotes{
		bid: map[string]float64{
			"AAPL": 100.10,
			"MSFT": 80.10,
			"NVDA": 210.10,
			"TSLA": 190.10,
			"AMZN": 150.10,
			"META": 120.10,
			"GOOG": 130.10,
			"IBM":  99.10,
		},
		ask: map[string]float64{
			"AAPL": 100.20,
			"MSFT": 80.20,
			"NVDA": 210.20,
			"TSLA": 190.20,
			"AMZN": 150.20,
			"META": 120.20,
			"GOOG": 130.20,
			"IBM":  99.20,
		},
	}
}

func qSQLNativeSelectWhereProject(trades qSQLNativeTrades) int64 {
	indexes := make([]int, 0, 128)
	for i := range trades.price {
		if trades.active[i] && trades.price[i] >= 100 {
			indexes = append(indexes, i)
		}
	}
	sort.Slice(indexes, func(i, j int) bool {
		return trades.price[indexes[i]] > trades.price[indexes[j]]
	})
	if len(indexes) > 128 {
		indexes = indexes[:128]
	}
	var checksum int64
	for _, row := range indexes {
		checksum += int64(trades.price[row]*100) + trades.size[row] + int64(len(trades.sym[row]))
	}
	return checksum
}

func qSQLNativeGroupByAggregate(trades qSQLNativeTrades) int64 {
	type group struct {
		notional float64
		fills    int64
	}
	groups := make(map[string]group, 8)
	for i := range trades.price {
		if !trades.active[i] || trades.price[i] < 100 {
			continue
		}
		g := groups[trades.sym[i]]
		g.notional += trades.price[i] * float64(trades.size[i])
		g.fills++
		groups[trades.sym[i]] = g
	}
	var checksum int64
	for sym, g := range groups {
		checksum += int64(g.notional) + g.fills + int64(len(sym))
	}
	return checksum
}

func qSQLNativeJoin(trades qSQLNativeTrades, quotes qSQLNativeQuotes) int64 {
	indexes := make([]int, 0, len(trades.price))
	for i, sym := range trades.sym {
		if _, ok := quotes.bid[sym]; ok {
			indexes = append(indexes, i)
		}
	}
	sort.Slice(indexes, func(i, j int) bool {
		return trades.price[indexes[i]] > trades.price[indexes[j]]
	})
	if len(indexes) > 128 {
		indexes = indexes[:128]
	}
	var checksum int64
	for _, row := range indexes {
		sym := trades.sym[row]
		checksum += int64(trades.price[row]*100) + int64(quotes.bid[sym]*100) + int64(quotes.ask[sym]*100)
	}
	return checksum
}

type qSQLNativeTopKRow struct {
	row int
	seq int
}

type qSQLNativeJoinTopKHeap struct {
	items  []qSQLNativeTopKRow
	trades qSQLNativeTrades
}

func (h qSQLNativeJoinTopKHeap) Len() int { return len(h.items) }

func (h qSQLNativeJoinTopKHeap) Less(i, j int) bool {
	return qSQLNativeJoinTopKBefore(h.trades, h.items[j], h.items[i])
}

func (h qSQLNativeJoinTopKHeap) Swap(i, j int) { h.items[i], h.items[j] = h.items[j], h.items[i] }

func (h *qSQLNativeJoinTopKHeap) Push(x any) {
	h.items = append(h.items, x.(qSQLNativeTopKRow))
}

func (h *qSQLNativeJoinTopKHeap) Pop() any {
	old := h.items
	n := len(old)
	item := old[n-1]
	h.items = old[:n-1]
	return item
}

func qSQLNativeJoinTopKBefore(trades qSQLNativeTrades, left, right qSQLNativeTopKRow) bool {
	if trades.price[left.row] != trades.price[right.row] {
		return trades.price[left.row] > trades.price[right.row]
	}
	return left.seq < right.seq
}

func qSQLNativeJoinTopK(trades qSQLNativeTrades, quotes qSQLNativeQuotes) int64 {
	const limit = 128
	h := &qSQLNativeJoinTopKHeap{items: make([]qSQLNativeTopKRow, 0, limit), trades: trades}
	seq := 0
	for row, sym := range trades.sym {
		if _, ok := quotes.bid[sym]; !ok {
			continue
		}
		item := qSQLNativeTopKRow{row: row, seq: seq}
		seq++
		if h.Len() < limit {
			heap.Push(h, item)
			continue
		}
		if qSQLNativeJoinTopKBefore(trades, item, h.items[0]) {
			h.items[0] = item
			heap.Fix(h, 0)
		}
	}
	out := append([]qSQLNativeTopKRow(nil), h.items...)
	sort.SliceStable(out, func(i, j int) bool {
		return qSQLNativeJoinTopKBefore(trades, out[i], out[j])
	})
	var checksum int64
	for _, item := range out {
		sym := trades.sym[item.row]
		checksum += int64(trades.price[item.row]*100) + int64(quotes.bid[sym]*100) + int64(quotes.ask[sym]*100)
	}
	return checksum
}

type qSQLNativeJoinRows struct {
	sym   []string
	price []float64
	bid   []float64
	ask   []float64
}

func qSQLNativeJoinTopKMaterialized(trades qSQLNativeTrades, quotes qSQLNativeQuotes) qSQLNativeJoinRows {
	const limit = 128
	h := &qSQLNativeJoinTopKHeap{items: make([]qSQLNativeTopKRow, 0, limit), trades: trades}
	seq := 0
	for row, sym := range trades.sym {
		if _, ok := quotes.bid[sym]; !ok {
			continue
		}
		item := qSQLNativeTopKRow{row: row, seq: seq}
		seq++
		if h.Len() < limit {
			heap.Push(h, item)
			continue
		}
		if qSQLNativeJoinTopKBefore(trades, item, h.items[0]) {
			h.items[0] = item
			heap.Fix(h, 0)
		}
	}
	out := append([]qSQLNativeTopKRow(nil), h.items...)
	sort.SliceStable(out, func(i, j int) bool {
		return qSQLNativeJoinTopKBefore(trades, out[i], out[j])
	})
	rows := qSQLNativeJoinRows{
		sym:   make([]string, len(out)),
		price: make([]float64, len(out)),
		bid:   make([]float64, len(out)),
		ask:   make([]float64, len(out)),
	}
	for i, item := range out {
		sym := trades.sym[item.row]
		rows.sym[i] = sym
		rows.price[i] = trades.price[item.row]
		rows.bid[i] = quotes.bid[sym]
		rows.ask[i] = quotes.ask[sym]
	}
	return rows
}
