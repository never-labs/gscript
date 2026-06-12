package bind

// qSQL benchmark matrix: table-driven qSQL cases that pair every query with a
// hand-written Go baseline, mirroring the q.eval vector-suite methodology
// (benchmarks/q_eval_vector_bench_test.go).
//
// Phase-0 honesty rules:
//   - every Go baseline is //go:noinline and does row-scaled work over the
//     shared non-constant qSQLMatrixInput dataset (no closed forms where the
//     query does row work);
//   - every baseline returns an int64 checksum that must equal the checksum of
//     the actual qSQL result (TestQSQLMatrixBenchmarkCasesMatchGoBaseline);
//   - TestQSQLMatrixGoBaselinesDoRealWork fails any baseline whose result is
//     row-invariant unless it is allowlisted with a written justification.
//
// Float determinism: all float inputs sit on an exact binary grid (multiples
// of 0.25), so sums, sums of squares, and weighted sums are exact in float64
// regardless of accumulation order; derived averages/variances are then a
// fixed sequence of deterministic roundings. Checksums fold floats through
// math.Float64bits, which is therefore stable across kernel implementations.
//
// Benchmark families (all names flow into benchmarks/q_perf_report.py):
//   - BenchmarkQSQLBindMatrixWarm/<Case>  warm plan-cache execution
//   - BenchmarkQSQLBindMatrixCold/<Case>  cold parse/lower/bind execution
//   - BenchmarkQSQLNativeGoMatrix/<Case>  hand-written Go baseline

import (
	"math"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

const qSQLMatrixRows = 8192

// ---------------------------------------------------------------------------
// Shared dataset (single source for q frames and Go baselines).
// ---------------------------------------------------------------------------

var (
	qSQLMatrixSymbols = []string{"AAPL", "MSFT", "NVDA", "TSLA", "AMZN", "META", "GOOG", "IBM"}
	qSQLMatrixVenues  = []string{"XNAS", "XNYS", "BATS", "IEX"}
	qSQLMatrixRegions = []string{"US", "EU", "US", "APAC"}

	// quotes: aliased join key qsym covering the first six symbols.
	qSQLMatrixQuoteSyms = qSQLMatrixSymbols[:6]
	qSQLMatrixQuoteBid  = []float64{90.25, 102.75, 115.25, 127.75, 140.25, 152.75}
	qSQLMatrixQuoteAsk  = []float64{90.75, 103.25, 115.75, 128.25, 140.75, 153.25}

	// usyms: union-join right table; includes one symbol absent from trades.
	qSQLMatrixUnionSyms = []string{"AAPL", "MSFT", "NVDA", "TSLA", "AMZN", "META", "ZZZT"}
	qSQLMatrixUnionBid  = []float64{10.25, 20.5, 30.75, 41.0, 51.25, 61.5, 71.75}

	// psizes: plus-join right table sharing the numeric "size" column.
	qSQLMatrixPlusSyms  = qSQLMatrixSymbols[:6]
	qSQLMatrixPlusSizes = []int64{10, 20, 30, 40, 50, 60}
)

type qSQLMatrixTrades struct {
	sym    []string
	venue  []string
	ts     []data.Timestamp
	price  []float64
	size   []int64
	active []bool
}

type qSQLMatrixTQuotes struct {
	sym []string
	ts  []data.Timestamp
	bid []float64
}

var qSQLMatrixInput = buildQSQLMatrixTrades(qSQLMatrixRows)
var qSQLMatrixTQuotesInput = buildQSQLMatrixTQuotes(qSQLMatrixRows / 2)

func buildQSQLMatrixTrades(rows int) qSQLMatrixTrades {
	in := qSQLMatrixTrades{
		sym:    make([]string, rows),
		venue:  make([]string, rows),
		ts:     make([]data.Timestamp, rows),
		price:  make([]float64, rows),
		size:   make([]int64, rows),
		active: make([]bool, rows),
	}
	for i := 0; i < rows; i++ {
		in.sym[i] = qSQLMatrixSymbols[i%len(qSQLMatrixSymbols)]
		in.venue[i] = qSQLMatrixVenues[(i/8)%len(qSQLMatrixVenues)]
		in.ts[i] = data.TimestampFromUnixNanos(int64(i) * 10_000_000_000)
		// 0.25 grid with a monotone component so ordered/take windows are
		// row-scaled.
		in.price[i] = 75 + 0.25*float64((i*17)%220) + 0.25*float64(i/16)
		in.size[i] = int64(1 + (i*13)%500)
		in.active[i] = i%5 != 0
	}
	return in
}

func buildQSQLMatrixTQuotes(rows int) qSQLMatrixTQuotes {
	in := qSQLMatrixTQuotes{
		sym: make([]string, rows),
		ts:  make([]data.Timestamp, rows),
		bid: make([]float64, rows),
	}
	for j := 0; j < rows; j++ {
		in.sym[j] = qSQLMatrixSymbols[j%len(qSQLMatrixSymbols)]
		in.ts[j] = data.TimestampFromUnixNanos(int64(j) * 20_000_000_000)
		in.bid[j] = 50 + 0.25*float64((j*7)%400)
	}
	return in
}

// qSQLMatrixQuoteIndex maps a quoted symbol to its quotes-table row.
var qSQLMatrixQuoteIndex = func() map[string]int {
	m := make(map[string]int, len(qSQLMatrixQuoteSyms))
	for k, s := range qSQLMatrixQuoteSyms {
		m[s] = k
	}
	return m
}()

var qSQLMatrixUnionIndex = func() map[string]int {
	m := make(map[string]int, len(qSQLMatrixUnionSyms))
	for k, s := range qSQLMatrixUnionSyms {
		m[s] = k
	}
	return m
}()

var qSQLMatrixPlusIndex = func() map[string]int {
	m := make(map[string]int, len(qSQLMatrixPlusSyms))
	for k, s := range qSQLMatrixPlusSyms {
		m[s] = k
	}
	return m
}()

// ---------------------------------------------------------------------------
// q-side environment construction.
// ---------------------------------------------------------------------------

func qSQLMatrixTradesFrame(tb testing.TB, rows int) data.Frame {
	tb.Helper()
	in := qSQLMatrixInput
	if rows > len(in.sym) {
		tb.Fatalf("rows %d exceeds qSQLMatrixInput size %d", rows, len(in.sym))
	}
	frame, err := data.NewFrame(
		data.Column{Name: "sym", Data: data.NewSymbols(in.sym[:rows])},
		data.Column{Name: "venue", Data: data.NewString(in.venue[:rows])},
		data.Column{Name: "ts", Data: data.NewTimestamp(in.ts[:rows])},
		data.Column{Name: "price", Data: data.NewF64(in.price[:rows])},
		data.Column{Name: "size", Data: data.NewI64(in.size[:rows])},
		data.Column{Name: "active", Data: data.NewBool(in.active[:rows])},
	)
	if err != nil {
		tb.Fatalf("NewFrame matrix trades: %v", err)
	}
	return frame
}

func qSQLMatrixEnvArgs(tb testing.TB, rows int, query string) qSQLArgsResult {
	tb.Helper()
	trades := qSQLMatrixTradesFrame(tb, rows)

	tq := qSQLMatrixTQuotesInput
	tqRows := rows / 2
	if tqRows > len(tq.sym) {
		tb.Fatalf("tquotes rows %d exceeds input size %d", tqRows, len(tq.sym))
	}
	tquotes, err := data.NewFrame(
		data.Column{Name: "sym", Data: data.NewSymbols(tq.sym[:tqRows])},
		data.Column{Name: "ts", Data: data.NewTimestamp(tq.ts[:tqRows])},
		data.Column{Name: "bid", Data: data.NewF64(tq.bid[:tqRows])},
	)
	if err != nil {
		tb.Fatalf("NewFrame matrix tquotes: %v", err)
	}

	quotes, err := data.NewFrame(
		data.Column{Name: "qsym", Data: data.NewSymbols(qSQLMatrixQuoteSyms)},
		data.Column{Name: "bid", Data: data.NewF64(qSQLMatrixQuoteBid)},
		data.Column{Name: "ask", Data: data.NewF64(qSQLMatrixQuoteAsk)},
	)
	if err != nil {
		tb.Fatalf("NewFrame matrix quotes: %v", err)
	}
	usyms, err := data.NewFrame(
		data.Column{Name: "sym", Data: data.NewSymbols(qSQLMatrixUnionSyms)},
		data.Column{Name: "bid", Data: data.NewF64(qSQLMatrixUnionBid)},
	)
	if err != nil {
		tb.Fatalf("NewFrame matrix usyms: %v", err)
	}
	psizes, err := data.NewFrame(
		data.Column{Name: "sym", Data: data.NewSymbols(qSQLMatrixPlusSyms)},
		data.Column{Name: "size", Data: data.NewI64(qSQLMatrixPlusSizes)},
	)
	if err != nil {
		tb.Fatalf("NewFrame matrix psizes: %v", err)
	}
	venues, err := data.NewFrame(
		data.Column{Name: "venue", Data: data.NewString(qSQLMatrixVenues)},
		data.Column{Name: "region", Data: data.NewSymbols(qSQLMatrixRegions)},
	)
	if err != nil {
		tb.Fatalf("NewFrame matrix venues: %v", err)
	}

	in := qSQLMatrixInput
	ids := make([]int64, rows)
	for i := range ids {
		ids[i] = int64(i)
	}
	book, err := data.NewFrame(
		data.Column{Name: "id", Data: data.NewI64(ids)},
		data.Column{Name: "sym", Data: data.NewSymbols(in.sym[:rows])},
		data.Column{Name: "price", Data: data.NewF64(in.price[:rows])},
		data.Column{Name: "qty", Data: data.NewI64(in.size[:rows])},
	)
	if err != nil {
		tb.Fatalf("NewFrame matrix book: %v", err)
	}
	kbook, err := data.KeyBy(book, "id")
	if err != nil {
		tb.Fatalf("KeyBy matrix book: %v", err)
	}
	ktrades, err := data.KeyBy(trades, "sym")
	if err != nil {
		tb.Fatalf("KeyBy matrix trades: %v", err)
	}

	frameValue := func(frame data.Frame) Value {
		value, err := qDataFrameValue(frame)
		if err != nil {
			tb.Fatalf("qDataFrameValue: %v", err)
		}
		return value
	}

	env := NewTable()
	env.RawSetString("trades", frameValue(trades))
	env.RawSetString("tquotes", frameValue(tquotes))
	env.RawSetString("quotes", frameValue(quotes))
	env.RawSetString("usyms", frameValue(usyms))
	env.RawSetString("psizes", frameValue(psizes))
	env.RawSetString("venues", frameValue(venues))
	env.RawSetString("ktrades", qKeyedFrameToValue(ktrades))
	env.RawSetString("kbook", qKeyedFrameToValue(kbook))
	envValue := TableValue(env)
	return qSQLArgsResult{
		frameValue:    envValue,
		source:        query,
		resolveSource: true,
		envValue:      envValue,
	}
}

// ---------------------------------------------------------------------------
// Result checksums (q side).
// ---------------------------------------------------------------------------

func qSQLMatrixBitsF64(v float64) int64 {
	return int64(math.Float64bits(v))
}

func qSQLMatrixAnyChecksum(tb testing.TB, v any) int64 {
	tb.Helper()
	if v == nil || data.IsNull(v) {
		return 0
	}
	switch x := v.(type) {
	case float64:
		return qSQLMatrixBitsF64(x)
	case float32:
		return qSQLMatrixBitsF64(float64(x))
	case int64:
		return x
	case int:
		return int64(x)
	case int32:
		return int64(x)
	case int16:
		return int64(x)
	case int8:
		return int64(x)
	case bool:
		if x {
			return 1
		}
		return 0
	case string:
		return int64(len(x))
	case data.Symbol:
		return int64(len(string(x)))
	case data.Timestamp:
		return x.UnixNanos()
	case data.Array:
		var sum int64
		for i := 0; i < x.Len(); i++ {
			item, ok := x.At(i)
			if !ok {
				tb.Fatalf("array checksum row %d out of range", i)
			}
			sum += qSQLMatrixAnyChecksum(tb, item)
		}
		return sum
	case []any:
		var sum int64
		for _, item := range x {
			sum += qSQLMatrixAnyChecksum(tb, item)
		}
		return sum
	default:
		tb.Fatalf("qSQL matrix checksum: unsupported value type %T (%v)", v, v)
		return 0
	}
}

func qSQLMatrixFrameChecksum(tb testing.TB, frame data.Frame) int64 {
	tb.Helper()
	var sum int64
	for _, name := range frame.Schema().Names() {
		col, ok := frame.Column(name)
		if !ok {
			tb.Fatalf("checksum column %q missing", name)
		}
		for row := 0; row < frame.Len(); row++ {
			v, ok := col.At(row)
			if !ok {
				tb.Fatalf("checksum column %q row %d out of range", name, row)
			}
			sum += qSQLMatrixAnyChecksum(tb, v)
		}
	}
	return sum
}

// qSQLMatrixFrameColumnChecksums is a debugging aid: per-column sums for
// mismatch triage in TestQSQLMatrixBenchmarkCasesMatchGoBaseline.
func qSQLMatrixFrameColumnChecksums(tb testing.TB, frame data.Frame) map[string]int64 {
	tb.Helper()
	out := make(map[string]int64, len(frame.Schema().Names()))
	for _, name := range frame.Schema().Names() {
		col, _ := frame.Column(name)
		var sum int64
		for row := 0; row < frame.Len(); row++ {
			v, _ := col.At(row)
			sum += qSQLMatrixAnyChecksum(tb, v)
		}
		out[string(name)] = sum
	}
	return out
}

func qSQLMatrixResultChecksum(tb testing.TB, out Value) int64 {
	tb.Helper()
	if out.IsDenseArray() {
		arr := out.DenseArray()
		if xs, ok := arr.I64(); ok {
			var sum int64
			for _, v := range xs {
				sum += v
			}
			return sum
		}
		if xs, ok := arr.F64(); ok {
			var sum int64
			for _, v := range xs {
				sum += qSQLMatrixBitsF64(v)
			}
			return sum
		}
		tb.Fatalf("qSQL matrix checksum: unsupported dense array dtype %v", arr.DType())
	}
	carrier, err := qSQLSourceCarrierFromValue(out, "")
	if err != nil {
		tb.Fatalf("qSQL matrix checksum: result is not a frame: %v", err)
	}
	return qSQLMatrixFrameChecksum(tb, carrier.frame)
}

// ---------------------------------------------------------------------------
// Case table.
// ---------------------------------------------------------------------------

type qSQLMatrixBenchCase struct {
	name  string
	tags  []string
	query string
	goFn  func(rows int) int64
}

var qSQLMatrixBenchCases = []qSQLMatrixBenchCase{
	// --- joins -----------------------------------------------------------
	{
		name:  "JoinInnerAliasedKeyTopK",
		tags:  []string{"qsql", "matrix", "query:select", "join", "join:inner", "aliased-key", "order", "take"},
		query: "select sym,price,bid,ask from trades ij quotes on sym=qsym order by price desc take 128",
		goFn:  qSQLMatrixGoJoinInnerAliasedTopK,
	},
	{
		name:  "JoinLeftNullFill",
		tags:  []string{"qsql", "matrix", "query:select", "join", "join:left", "aliased-key", "null-fill"},
		query: "select sym,price,bid from trades lj quotes on sym=qsym",
		goFn:  qSQLMatrixGoJoinLeftNullFill,
	},
	{
		name:  "JoinChainedInnerLeft",
		tags:  []string{"qsql", "matrix", "query:select", "join", "join:inner", "join:left", "chained-join", "aliased-key"},
		query: "select sym,venue,price,bid,region from trades ij quotes on sym=qsym lj venues on venue",
		goFn:  qSQLMatrixGoJoinChainedInnerLeft,
	},
	{
		name:  "JoinUnionAppend",
		tags:  []string{"qsql", "matrix", "query:select", "join", "join:union"},
		query: "select sym,size,bid from trades uj usyms on sym",
		goFn:  qSQLMatrixGoJoinUnionAppend,
	},
	{
		name:  "JoinPlusAddShared",
		tags:  []string{"qsql", "matrix", "query:select", "join", "join:plus"},
		query: "select sym,size from trades pj psizes on sym",
		goFn:  qSQLMatrixGoJoinPlusAddShared,
	},
	{
		name:  "JoinAsofTemporal",
		tags:  []string{"qsql", "matrix", "query:select", "join", "join:asof", "temporal-key"},
		query: "select sym,ts,price,bid from trades aj tquotes on sym,ts",
		goFn:  qSQLMatrixGoJoinAsof,
	},
	{
		name:  "JoinAsof0PreserveRightTime",
		tags:  []string{"qsql", "matrix", "query:select", "join", "join:asof0", "temporal-key"},
		query: "select sym,ts,price,bid from trades aj0 tquotes on sym,ts",
		goFn:  qSQLMatrixGoJoinAsof0,
	},
	{
		name:  "JoinAsofFill",
		tags:  []string{"qsql", "matrix", "query:select", "join", "join:asof_fill", "temporal-key"},
		query: "select sym,ts,price,bid from trades ajf tquotes on sym,ts",
		goFn:  qSQLMatrixGoJoinAsof,
	},
	{
		name:  "JoinAsofFill0",
		tags:  []string{"qsql", "matrix", "query:select", "join", "join:asof_fill0", "temporal-key"},
		query: "select sym,ts,price,bid from trades ajf0 tquotes on sym,ts",
		goFn:  qSQLMatrixGoJoinAsof0,
	},
	{
		name:  "JoinWindowAggSum",
		tags:  []string{"qsql", "matrix", "query:select", "join", "join:window", "temporal-key", "agg:sum"},
		query: "select sym,ts,wbid:sum bid from trades wj[-300000000000 0] tquotes on sym,ts",
		goFn:  qSQLMatrixGoJoinWindowSum,
	},
	{
		name:  "JoinWindow1Last",
		tags:  []string{"qsql", "matrix", "query:select", "join", "join:window1", "temporal-key"},
		query: "select sym,ts,bid from trades wj1[-300000000000 0] tquotes on sym,ts",
		goFn:  qSQLMatrixGoJoinWindow1Last,
	},
	// --- mutations: update ------------------------------------------------
	{
		name:  "UpdateComputedWhere",
		tags:  []string{"qsql", "matrix", "query:update", "mutation", "where", "computed-column"},
		query: "update notional:price*size from trades where active=true,price>=100",
		goFn:  qSQLMatrixGoUpdateComputedWhere,
	},
	{
		name:  "UpdateGroupedBroadcast",
		tags:  []string{"qsql", "matrix", "query:update", "mutation", "grouped-update", "where", "agg:avg"},
		query: "update avg_px:avg price by sym from trades where active=true",
		goFn:  qSQLMatrixGoUpdateGroupedBroadcast,
	},
	{
		name:  "UpdateEmptyMatchBoundary",
		tags:  []string{"qsql", "matrix", "query:update", "mutation", "empty-match", "where"},
		query: "update price:price+0.25 from trades where price<0",
		goFn:  qSQLMatrixGoTradesBaseChecksumNoinline,
	},
	{
		name:  "UpdateKeyedWhere",
		tags:  []string{"qsql", "matrix", "query:update", "mutation", "keyed-table", "where"},
		query: "update size:size+7 from ktrades where sym=`AAPL",
		goFn:  qSQLMatrixGoUpdateKeyedWhere,
	},
	// --- mutations: delete ------------------------------------------------
	{
		name:  "DeleteWhereInactive",
		tags:  []string{"qsql", "matrix", "query:delete", "mutation", "where"},
		query: "delete from trades where active=false",
		goFn:  qSQLMatrixGoDeleteWhereInactive,
	},
	{
		name:  "DeleteColumns",
		tags:  []string{"qsql", "matrix", "query:delete", "mutation", "column-delete"},
		query: "delete venue,active from trades",
		goFn:  qSQLMatrixGoDeleteColumns,
	},
	{
		name:  "DeleteEmptyMatchBoundary",
		tags:  []string{"qsql", "matrix", "query:delete", "mutation", "empty-match", "where"},
		query: "delete from trades where price<0",
		goFn:  qSQLMatrixGoTradesBaseChecksumNoinline,
	},
	{
		name:  "DeleteKeyedWhere",
		tags:  []string{"qsql", "matrix", "query:delete", "mutation", "keyed-table", "where"},
		query: "delete from ktrades where sym=`IBM",
		goFn:  qSQLMatrixGoDeleteKeyedWhere,
	},
	// --- mutations: insert/upsert -----------------------------------------
	{
		name:  "InsertPlainRow",
		tags:  []string{"qsql", "matrix", "query:insert", "mutation"},
		query: "insert into trades (sym,venue,price,size,active) values (`ZZZT,\"XNAS\",101.25,7,true)",
		goFn:  qSQLMatrixGoInsertPlainRow,
	},
	{
		name:  "InsertKeyedNewKey",
		tags:  []string{"qsql", "matrix", "query:insert", "mutation", "keyed-table"},
		query: "insert into kbook (id,sym,price,qty) values (9000000,`ZZZT,101.25,7)",
		goFn:  qSQLMatrixGoInsertKeyedNewKey,
	},
	{
		name:  "UpsertPlainAppend",
		tags:  []string{"qsql", "matrix", "query:upsert", "mutation"},
		query: "upsert into trades (sym,price,size) values (`ZZZT,103.5,11)",
		goFn:  qSQLMatrixGoUpsertPlainAppend,
	},
	{
		name:  "UpsertKeyedExistingKey",
		tags:  []string{"qsql", "matrix", "query:upsert", "mutation", "keyed-table"},
		query: "upsert into kbook (id,sym,price,qty) values (1024,`ZZZT,90.25,33)",
		goFn:  qSQLMatrixGoUpsertKeyedExistingKey,
	},
	// --- grouped aggregates -----------------------------------------------
	{
		name:  "GroupMultiKeySumCountAvg",
		tags:  []string{"qsql", "matrix", "query:select", "group", "multi-key", "where", "agg:sum", "agg:count", "agg:avg"},
		query: "select notional:sum price*size,fills:count i,avg_px:avg price by sym,venue from trades where active=true",
		goFn:  qSQLMatrixGoGroupMultiKey,
	},
	{
		name:  "GroupXbarBucketSum",
		tags:  []string{"qsql", "matrix", "query:select", "group", "temporal-key", "xbar", "agg:sum", "agg:count"},
		query: "select shares:sum size,fills:count i by sym,bucket:xbar 3600000000000 ts from trades",
		goFn:  qSQLMatrixGoGroupXbarBucket,
	},
	{
		name:  "GroupVarDev",
		tags:  []string{"qsql", "matrix", "query:select", "group", "agg:var", "agg:dev"},
		query: "select pvar:var price,pdev:dev price by sym from trades",
		goFn:  qSQLMatrixGoGroupVarDev,
	},
	{
		name:  "GroupMedMinMax",
		tags:  []string{"qsql", "matrix", "query:select", "group", "agg:med", "agg:min", "agg:max"},
		query: "select pmed:med price,lo:min price,hi:max price by sym from trades",
		goFn:  qSQLMatrixGoGroupMedMinMax,
	},
	{
		name:  "GroupFirstLastCount",
		tags:  []string{"qsql", "matrix", "query:select", "group", "agg:first", "agg:last", "agg:count"},
		query: "select first_px:first price,last_px:last price,fills:count i by sym from trades",
		goFn:  qSQLMatrixGoGroupFirstLastCount,
	},
	// --- exec ---------------------------------------------------------------
	{
		name:  "ExecVectorWhere",
		tags:  []string{"qsql", "matrix", "query:exec", "where", "typed-filter"},
		query: "exec size from trades where active=true,price>=100",
		goFn:  qSQLMatrixGoExecVectorWhere,
	},
	// --- ungrouped (whole-table) aggregates --------------------------------
	{
		name:  "UngroupedAggSumCountMax",
		tags:  []string{"qsql", "matrix", "query:select", "ungrouped-agg", "where", "agg:sum", "agg:count", "agg:max"},
		query: "select shares:sum size,fills:count i,hi:max price from trades where active=true",
		goFn:  qSQLMatrixGoUngroupedAggSumCountMax,
	},
	// --- subquery sources ---------------------------------------------------
	{
		name:  "SubquerySelectFromSelect",
		tags:  []string{"qsql", "matrix", "query:select", "subquery", "where"},
		query: "select sym,price from (select sym,price,size from trades where active=true) where price>=100",
		goFn:  qSQLMatrixGoSubquerySelectFromSelect,
	},
	// --- canonical select[n;>c] ---------------------------------------------
	{
		name:  "SelectNTopByPrice",
		tags:  []string{"qsql", "matrix", "query:select", "select-n", "order", "take"},
		query: "select[128;>price] sym,price from trades",
		goFn:  qSQLMatrixGoSelectNTopByPrice,
	},
	// --- weighted average ----------------------------------------------------
	{
		name:  "GroupWavgBySym",
		tags:  []string{"qsql", "matrix", "query:select", "group", "agg:wavg"},
		query: "select vw:wavg[size;price] by sym from trades",
		goFn:  qSQLMatrixGoGroupWavgBySym,
	},
}

// ---------------------------------------------------------------------------
// Go baselines.
// ---------------------------------------------------------------------------

//go:noinline
func qSQLMatrixGoJoinInnerAliasedTopK(rows int) int64 {
	in := qSQLMatrixInput
	matched := make([]int, 0, rows)
	for i := 0; i < rows; i++ {
		if _, ok := qSQLMatrixQuoteIndex[in.sym[i]]; ok {
			matched = append(matched, i)
		}
	}
	sort.SliceStable(matched, func(a, b int) bool {
		return in.price[matched[a]] > in.price[matched[b]]
	})
	if len(matched) > 128 {
		matched = matched[:128]
	}
	var sum int64
	for _, i := range matched {
		k := qSQLMatrixQuoteIndex[in.sym[i]]
		sum += int64(len(in.sym[i])) +
			qSQLMatrixBitsF64(in.price[i]) +
			qSQLMatrixBitsF64(qSQLMatrixQuoteBid[k]) +
			qSQLMatrixBitsF64(qSQLMatrixQuoteAsk[k])
	}
	return sum
}

//go:noinline
func qSQLMatrixGoJoinLeftNullFill(rows int) int64 {
	in := qSQLMatrixInput
	var sum int64
	for i := 0; i < rows; i++ {
		sum += int64(len(in.sym[i])) + qSQLMatrixBitsF64(in.price[i])
		if k, ok := qSQLMatrixQuoteIndex[in.sym[i]]; ok {
			sum += qSQLMatrixBitsF64(qSQLMatrixQuoteBid[k])
		}
	}
	return sum
}

//go:noinline
func qSQLMatrixGoJoinChainedInnerLeft(rows int) int64 {
	in := qSQLMatrixInput
	regionBySymVenue := make(map[string]string, len(qSQLMatrixVenues))
	for v, venue := range qSQLMatrixVenues {
		regionBySymVenue[venue] = qSQLMatrixRegions[v]
	}
	var sum int64
	for i := 0; i < rows; i++ {
		k, ok := qSQLMatrixQuoteIndex[in.sym[i]]
		if !ok {
			continue
		}
		sum += int64(len(in.sym[i])) +
			int64(len(in.venue[i])) +
			qSQLMatrixBitsF64(in.price[i]) +
			qSQLMatrixBitsF64(qSQLMatrixQuoteBid[k]) +
			int64(len(regionBySymVenue[in.venue[i]]))
	}
	return sum
}

//go:noinline
func qSQLMatrixGoJoinUnionAppend(rows int) int64 {
	in := qSQLMatrixInput
	matchedRight := make([]bool, len(qSQLMatrixUnionSyms))
	var sum int64
	for i := 0; i < rows; i++ {
		sum += int64(len(in.sym[i])) + in.size[i]
		if k, ok := qSQLMatrixUnionIndex[in.sym[i]]; ok {
			sum += qSQLMatrixBitsF64(qSQLMatrixUnionBid[k])
			matchedRight[k] = true
		}
	}
	for k := range qSQLMatrixUnionSyms {
		if matchedRight[k] {
			continue
		}
		// Appended unmatched right row: key column from the right, other left
		// columns null.
		sum += int64(len(qSQLMatrixUnionSyms[k])) + qSQLMatrixBitsF64(qSQLMatrixUnionBid[k])
	}
	return sum
}

//go:noinline
func qSQLMatrixGoJoinPlusAddShared(rows int) int64 {
	in := qSQLMatrixInput
	var sum int64
	for i := 0; i < rows; i++ {
		// Plus join adds the shared numeric column through ApplyBinary, which
		// promotes the output column to float64.
		size := float64(in.size[i])
		if k, ok := qSQLMatrixPlusIndex[in.sym[i]]; ok {
			size += float64(qSQLMatrixPlusSizes[k])
		}
		sum += int64(len(in.sym[i])) + qSQLMatrixBitsF64(size)
	}
	return sum
}

// qSQLMatrixAsofMatch returns the latest tquotes row for trade i (same symbol,
// quote ts <= trade ts), or -1.
func qSQLMatrixAsofMatch(i, tqRows int) int64 {
	s := i % len(qSQLMatrixSymbols)
	m := i / 2
	if m > tqRows-1 {
		m = tqRows - 1
	}
	if m < s {
		return -1
	}
	return int64(m - ((m - s) % len(qSQLMatrixSymbols)))
}

//go:noinline
func qSQLMatrixGoJoinAsof(rows int) int64 {
	in := qSQLMatrixInput
	tq := qSQLMatrixTQuotesInput
	tqRows := rows / 2
	var sum int64
	for i := 0; i < rows; i++ {
		sum += int64(len(in.sym[i])) + in.ts[i].UnixNanos() + qSQLMatrixBitsF64(in.price[i])
		if j := qSQLMatrixAsofMatch(i, tqRows); j >= 0 {
			sum += qSQLMatrixBitsF64(tq.bid[j])
		}
	}
	return sum
}

//go:noinline
func qSQLMatrixGoJoinAsof0(rows int) int64 {
	in := qSQLMatrixInput
	tq := qSQLMatrixTQuotesInput
	tqRows := rows / 2
	var sum int64
	for i := 0; i < rows; i++ {
		sum += int64(len(in.sym[i])) + qSQLMatrixBitsF64(in.price[i])
		if j := qSQLMatrixAsofMatch(i, tqRows); j >= 0 {
			// asof0 preserves the right (quote) timestamp.
			sum += tq.ts[j].UnixNanos() + qSQLMatrixBitsF64(tq.bid[j])
		}
	}
	return sum
}

// qSQLMatrixWindowMatches collects tquotes rows for trade i with quote ts in
// [trade ts - 300s, trade ts], same symbol, ascending.
func qSQLMatrixWindowMatches(i, tqRows int) []int {
	s := i % len(qSQLMatrixSymbols)
	loNanos := int64(i)*10_000_000_000 - 300_000_000_000
	hiNanos := int64(i) * 10_000_000_000
	first := (i - 30) / 2
	if first < 0 {
		first = 0
	}
	matches := make([]int, 0, 4)
	for j := first; j <= i/2 && j < tqRows; j++ {
		if j%len(qSQLMatrixSymbols) != s {
			continue
		}
		ts := int64(j) * 20_000_000_000
		if ts < loNanos || ts > hiNanos {
			continue
		}
		matches = append(matches, j)
	}
	return matches
}

//go:noinline
func qSQLMatrixGoJoinWindowSum(rows int) int64 {
	in := qSQLMatrixInput
	tq := qSQLMatrixTQuotesInput
	tqRows := rows / 2
	var sum int64
	for i := 0; i < rows; i++ {
		sum += int64(len(in.sym[i])) + in.ts[i].UnixNanos()
		matches := qSQLMatrixWindowMatches(i, tqRows)
		if len(matches) == 0 {
			continue
		}
		var wbid float64
		for _, j := range matches {
			wbid += tq.bid[j]
		}
		sum += qSQLMatrixBitsF64(wbid)
	}
	return sum
}

//go:noinline
func qSQLMatrixGoJoinWindow1Last(rows int) int64 {
	in := qSQLMatrixInput
	tq := qSQLMatrixTQuotesInput
	tqRows := rows / 2
	var sum int64
	for i := 0; i < rows; i++ {
		sum += int64(len(in.sym[i])) + in.ts[i].UnixNanos()
		matches := qSQLMatrixWindowMatches(i, tqRows)
		if len(matches) > 0 {
			sum += qSQLMatrixBitsF64(tq.bid[matches[len(matches)-1]])
		}
	}
	return sum
}

// qSQLMatrixGoTradesBaseChecksum is the full-table checksum of the trades
// frame (all six columns); empty-match mutations must return it unchanged.
func qSQLMatrixGoTradesBaseChecksum(rows int) int64 {
	in := qSQLMatrixInput
	var sum int64
	for i := 0; i < rows; i++ {
		sum += int64(len(in.sym[i])) +
			int64(len(in.venue[i])) +
			in.ts[i].UnixNanos() +
			qSQLMatrixBitsF64(in.price[i]) +
			in.size[i]
		if in.active[i] {
			sum++
		}
	}
	return sum
}

//go:noinline
func qSQLMatrixGoTradesBaseChecksumNoinline(rows int) int64 {
	return qSQLMatrixGoTradesBaseChecksum(rows)
}

//go:noinline
func qSQLMatrixGoUpdateComputedWhere(rows int) int64 {
	in := qSQLMatrixInput
	sum := qSQLMatrixGoTradesBaseChecksum(rows)
	for i := 0; i < rows; i++ {
		if in.active[i] && in.price[i] >= 100 {
			sum += qSQLMatrixBitsF64(in.price[i] * float64(in.size[i]))
		}
	}
	return sum
}

//go:noinline
func qSQLMatrixGoUpdateGroupedBroadcast(rows int) int64 {
	in := qSQLMatrixInput
	nsym := len(qSQLMatrixSymbols)
	sums := make([]float64, nsym)
	counts := make([]int64, nsym)
	for i := 0; i < rows; i++ {
		if !in.active[i] {
			continue
		}
		s := i % nsym
		sums[s] += in.price[i]
		counts[s]++
	}
	sum := qSQLMatrixGoTradesBaseChecksum(rows)
	for i := 0; i < rows; i++ {
		if !in.active[i] {
			continue
		}
		s := i % nsym
		sum += qSQLMatrixBitsF64(sums[s] / float64(counts[s]))
	}
	return sum
}

//go:noinline
func qSQLMatrixGoUpdateKeyedWhere(rows int) int64 {
	in := qSQLMatrixInput
	var sum int64
	for i := 0; i < rows; i++ {
		size := in.size[i]
		if in.sym[i] == "AAPL" {
			size += 7
		}
		sum += int64(len(in.sym[i])) +
			int64(len(in.venue[i])) +
			in.ts[i].UnixNanos() +
			qSQLMatrixBitsF64(in.price[i]) +
			size
		if in.active[i] {
			sum++
		}
	}
	return sum
}

//go:noinline
func qSQLMatrixGoDeleteWhereInactive(rows int) int64 {
	in := qSQLMatrixInput
	var sum int64
	for i := 0; i < rows; i++ {
		if !in.active[i] {
			continue
		}
		sum += int64(len(in.sym[i])) +
			int64(len(in.venue[i])) +
			in.ts[i].UnixNanos() +
			qSQLMatrixBitsF64(in.price[i]) +
			in.size[i] + 1
	}
	return sum
}

//go:noinline
func qSQLMatrixGoDeleteColumns(rows int) int64 {
	in := qSQLMatrixInput
	var sum int64
	for i := 0; i < rows; i++ {
		sum += int64(len(in.sym[i])) +
			in.ts[i].UnixNanos() +
			qSQLMatrixBitsF64(in.price[i]) +
			in.size[i]
	}
	return sum
}

//go:noinline
func qSQLMatrixGoDeleteKeyedWhere(rows int) int64 {
	in := qSQLMatrixInput
	var sum int64
	for i := 0; i < rows; i++ {
		if in.sym[i] == "IBM" {
			continue
		}
		sum += int64(len(in.sym[i])) +
			int64(len(in.venue[i])) +
			in.ts[i].UnixNanos() +
			qSQLMatrixBitsF64(in.price[i]) +
			in.size[i]
		if in.active[i] {
			sum++
		}
	}
	return sum
}

//go:noinline
func qSQLMatrixGoInsertPlainRow(rows int) int64 {
	sum := qSQLMatrixGoTradesBaseChecksum(rows)
	// Appended row: sym ZZZT, venue XNAS, ts null, price 101.25, size 7,
	// active true.
	return sum + 4 + 4 + qSQLMatrixBitsF64(101.25) + 7 + 1
}

func qSQLMatrixGoBookBaseChecksum(rows int) int64 {
	in := qSQLMatrixInput
	var sum int64
	for i := 0; i < rows; i++ {
		sum += int64(i) +
			int64(len(in.sym[i])) +
			qSQLMatrixBitsF64(in.price[i]) +
			in.size[i]
	}
	return sum
}

//go:noinline
func qSQLMatrixGoInsertKeyedNewKey(rows int) int64 {
	sum := qSQLMatrixGoBookBaseChecksum(rows)
	return sum + 9000000 + 4 + qSQLMatrixBitsF64(101.25) + 7
}

//go:noinline
func qSQLMatrixGoUpsertPlainAppend(rows int) int64 {
	sum := qSQLMatrixGoTradesBaseChecksum(rows)
	// Appended row: sym ZZZT, price 103.5, size 11; venue/ts/active null.
	return sum + 4 + qSQLMatrixBitsF64(103.5) + 11
}

//go:noinline
func qSQLMatrixGoUpsertKeyedExistingKey(rows int) int64 {
	in := qSQLMatrixInput
	var sum int64
	for i := 0; i < rows; i++ {
		if i == 1024 {
			sum += 1024 + 4 + qSQLMatrixBitsF64(90.25) + 33
			continue
		}
		sum += int64(i) +
			int64(len(in.sym[i])) +
			qSQLMatrixBitsF64(in.price[i]) +
			in.size[i]
	}
	return sum
}

//go:noinline
func qSQLMatrixGoGroupMultiKey(rows int) int64 {
	in := qSQLMatrixInput
	nsym := len(qSQLMatrixSymbols)
	nven := len(qSQLMatrixVenues)
	type group struct {
		notional float64
		priceSum float64
		fills    int64
	}
	groups := make([]group, nsym*nven)
	seen := make([]bool, nsym*nven)
	for i := 0; i < rows; i++ {
		if !in.active[i] {
			continue
		}
		g := (i%nsym)*nven + (i/8)%nven
		groups[g].notional += in.price[i] * float64(in.size[i])
		groups[g].priceSum += in.price[i]
		groups[g].fills++
		seen[g] = true
	}
	var sum int64
	for g := range groups {
		if !seen[g] {
			continue
		}
		s := g / nven
		v := g % nven
		sum += int64(len(qSQLMatrixSymbols[s])) +
			int64(len(qSQLMatrixVenues[v])) +
			qSQLMatrixBitsF64(groups[g].notional) +
			groups[g].fills +
			qSQLMatrixBitsF64(groups[g].priceSum/float64(groups[g].fills))
	}
	return sum
}

//go:noinline
func qSQLMatrixGoGroupXbarBucket(rows int) int64 {
	in := qSQLMatrixInput
	nsym := len(qSQLMatrixSymbols)
	type group struct {
		shares int64
		fills  int64
	}
	groups := make(map[int64]group, rows/256)
	for i := 0; i < rows; i++ {
		bucket := int64(i) * 10_000_000_000 / 3_600_000_000_000
		key := bucket*int64(nsym) + int64(i%nsym)
		g := groups[key]
		// The qSQL sum aggregate is kind-preserving: sums over integer
		// columns stay i64.
		g.shares += in.size[i]
		g.fills++
		groups[key] = g
	}
	var sum int64
	for key, g := range groups {
		bucketNanos := (key / int64(nsym)) * 3_600_000_000_000
		s := int(key % int64(nsym))
		sum += int64(len(qSQLMatrixSymbols[s])) + bucketNanos + g.shares + g.fills
	}
	return sum
}

//go:noinline
func qSQLMatrixGoGroupVarDev(rows int) int64 {
	in := qSQLMatrixInput
	nsym := len(qSQLMatrixSymbols)
	sums := make([]float64, nsym)
	sumsq := make([]float64, nsym)
	counts := make([]int64, nsym)
	for i := 0; i < rows; i++ {
		s := i % nsym
		sums[s] += in.price[i]
		sumsq[s] += in.price[i] * in.price[i]
		counts[s]++
	}
	var sum int64
	for s := 0; s < nsym; s++ {
		if counts[s] == 0 {
			continue
		}
		mean := sums[s] / float64(counts[s])
		variance := sumsq[s]/float64(counts[s]) - mean*mean
		sum += int64(len(qSQLMatrixSymbols[s])) +
			qSQLMatrixBitsF64(variance) +
			qSQLMatrixBitsF64(math.Sqrt(variance))
	}
	return sum
}

//go:noinline
func qSQLMatrixGoGroupMedMinMax(rows int) int64 {
	in := qSQLMatrixInput
	nsym := len(qSQLMatrixSymbols)
	values := make([][]float64, nsym)
	for i := 0; i < rows; i++ {
		s := i % nsym
		values[s] = append(values[s], in.price[i])
	}
	var sum int64
	for s := 0; s < nsym; s++ {
		group := values[s]
		if len(group) == 0 {
			continue
		}
		sorted := append([]float64(nil), group...)
		sort.Float64s(sorted)
		mid := len(sorted) / 2
		med := sorted[mid]
		if len(sorted)%2 == 0 {
			med = (sorted[mid-1] + sorted[mid]) / 2
		}
		sum += int64(len(qSQLMatrixSymbols[s])) +
			qSQLMatrixBitsF64(med) +
			qSQLMatrixBitsF64(sorted[0]) +
			qSQLMatrixBitsF64(sorted[len(sorted)-1])
	}
	return sum
}

//go:noinline
func qSQLMatrixGoGroupFirstLastCount(rows int) int64 {
	in := qSQLMatrixInput
	nsym := len(qSQLMatrixSymbols)
	first := make([]float64, nsym)
	last := make([]float64, nsym)
	counts := make([]int64, nsym)
	for i := 0; i < rows; i++ {
		s := i % nsym
		if counts[s] == 0 {
			first[s] = in.price[i]
		}
		last[s] = in.price[i]
		counts[s]++
	}
	var sum int64
	for s := 0; s < nsym; s++ {
		if counts[s] == 0 {
			continue
		}
		sum += int64(len(qSQLMatrixSymbols[s])) +
			qSQLMatrixBitsF64(first[s]) +
			qSQLMatrixBitsF64(last[s]) +
			counts[s]
	}
	return sum
}

//go:noinline
func qSQLMatrixGoExecVectorWhere(rows int) int64 {
	in := qSQLMatrixInput
	var sum int64
	for i := 0; i < rows; i++ {
		if in.active[i] && in.price[i] >= 100 {
			sum += in.size[i]
		}
	}
	return sum
}

//go:noinline
func qSQLMatrixGoUngroupedAggSumCountMax(rows int) int64 {
	in := qSQLMatrixInput
	var shares, fills int64
	hi := math.Inf(-1)
	for i := 0; i < rows; i++ {
		if !in.active[i] {
			continue
		}
		// Kind-preserving columnar sums: integer columns stay i64.
		shares += in.size[i]
		fills++
		if in.price[i] > hi {
			hi = in.price[i]
		}
	}
	return shares + fills + qSQLMatrixBitsF64(hi)
}

//go:noinline
func qSQLMatrixGoSubquerySelectFromSelect(rows int) int64 {
	in := qSQLMatrixInput
	var sum int64
	for i := 0; i < rows; i++ {
		if !in.active[i] || in.price[i] < 100 {
			continue
		}
		sum += int64(len(in.sym[i])) + qSQLMatrixBitsF64(in.price[i])
	}
	return sum
}

//go:noinline
func qSQLMatrixGoSelectNTopByPrice(rows int) int64 {
	in := qSQLMatrixInput
	matched := make([]int, rows)
	for i := 0; i < rows; i++ {
		matched[i] = i
	}
	sort.SliceStable(matched, func(a, b int) bool {
		return in.price[matched[a]] > in.price[matched[b]]
	})
	if len(matched) > 128 {
		matched = matched[:128]
	}
	var sum int64
	for _, i := range matched {
		sum += int64(len(in.sym[i])) + qSQLMatrixBitsF64(in.price[i])
	}
	return sum
}

//go:noinline
func qSQLMatrixGoGroupWavgBySym(rows int) int64 {
	in := qSQLMatrixInput
	nsym := len(qSQLMatrixSymbols)
	weighted := make([]float64, nsym)
	weights := make([]float64, nsym)
	for i := 0; i < rows; i++ {
		s := i % nsym
		w := float64(in.size[i])
		weighted[s] += w * in.price[i]
		weights[s] += w
	}
	var sum int64
	for s := 0; s < nsym && s < rows; s++ {
		sum += int64(len(qSQLMatrixSymbols[s])) + qSQLMatrixBitsF64(weighted[s]/weights[s])
	}
	return sum
}

// ---------------------------------------------------------------------------
// Verification gates.
// ---------------------------------------------------------------------------

// qSQLMatrixCaseFloor pins the matrix size: it may only ratchet upward.
const qSQLMatrixCaseFloor = 29

func TestQSQLMatrixBenchmarkCasesMatchGoBaseline(t *testing.T) {
	if len(qSQLMatrixBenchCases) < qSQLMatrixCaseFloor {
		t.Fatalf("qSQL matrix has %d cases, floor is %d", len(qSQLMatrixBenchCases), qSQLMatrixCaseFloor)
	}
	seen := make(map[string]bool, len(qSQLMatrixBenchCases))
	for _, tc := range qSQLMatrixBenchCases {
		if seen[tc.name] {
			t.Fatalf("duplicate qSQL matrix case name %q", tc.name)
		}
		seen[tc.name] = true
	}
	qClearCaches()
	defer qClearCaches()
	for _, tc := range qSQLMatrixBenchCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// Exercise the embedder lifetime discipline: result roots are
			// scope-captured and released once the checksum is consumed.
			scope := PushValueScope()
			defer scope.Release()
			args := qSQLMatrixEnvArgs(t, qSQLMatrixRows, tc.query)
			out, err := qRunSQL("q.sql", args)
			if err != nil {
				t.Fatalf("q.sql(%q): %v", tc.query, err)
			}
			got := qSQLMatrixResultChecksum(t, out)
			want := tc.goFn(qSQLMatrixRows)
			if got != want {
				if carrier, cerr := qSQLSourceCarrierFromValue(out, ""); cerr == nil {
					t.Logf("q result rows=%d per-column checksums: %v", carrier.frame.Len(), qSQLMatrixFrameColumnChecksums(t, carrier.frame))
				}
				t.Fatalf("qSQL checksum = %d, want Go baseline %d", got, want)
			}
		})
	}
}

// qSQLMatrixRowInvariantAllowlist names the only cases whose Go baseline may
// return the same value at qSQLMatrixRows and qSQLMatrixRows/2. Entries need a
// written justification; the list is shrink-only.
var qSQLMatrixRowInvariantAllowlist = map[string]string{}

const qSQLMatrixRowInvariantAllowlistMax = 0

func TestQSQLMatrixGoBaselinesDoRealWork(t *testing.T) {
	if len(qSQLMatrixRowInvariantAllowlist) > qSQLMatrixRowInvariantAllowlistMax {
		t.Fatalf("row-invariant allowlist has %d entries, max %d; make baselines row-scaled instead",
			len(qSQLMatrixRowInvariantAllowlist), qSQLMatrixRowInvariantAllowlistMax)
	}
	caseNames := make(map[string]bool, len(qSQLMatrixBenchCases))
	for _, tc := range qSQLMatrixBenchCases {
		caseNames[tc.name] = true
	}
	for name, justification := range qSQLMatrixRowInvariantAllowlist {
		if justification == "" {
			t.Errorf("allowlist entry %q has no justification", name)
		}
		if !caseNames[name] {
			t.Errorf("stale allowlist entry %q: no matching case", name)
		}
	}
	for _, tc := range qSQLMatrixBenchCases {
		if _, ok := qSQLMatrixRowInvariantAllowlist[tc.name]; ok {
			continue
		}
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			full := tc.goFn(qSQLMatrixRows)
			half := tc.goFn(qSQLMatrixRows / 2)
			if full == half {
				t.Fatalf("Go baseline result %d is identical for %d and %d rows; make it row-scaled or allowlist it with a justification",
					full, qSQLMatrixRows, qSQLMatrixRows/2)
			}
		})
	}
}

func TestQSQLMatrixCaseTagsWellFormed(t *testing.T) {
	for _, tc := range qSQLMatrixBenchCases {
		hasKind := false
		for _, tag := range tc.tags {
			if strings.HasPrefix(tag, "query:") {
				hasKind = true
			}
		}
		if !hasKind {
			t.Errorf("case %q has no query:<kind> tag", tc.name)
		}
		if tc.goFn == nil {
			t.Errorf("case %q has no Go baseline", tc.name)
		}
		if tc.query == "" {
			t.Errorf("case %q has no query", tc.name)
		}
	}
}

// ---------------------------------------------------------------------------
// Benchmarks.
// ---------------------------------------------------------------------------

func BenchmarkQSQLBindMatrixWarm(b *testing.B) {
	for _, tc := range qSQLMatrixBenchCases {
		tc := tc
		b.Run(tc.name, func(b *testing.B) {
			qClearCaches()
			defer qClearCaches()
			args := qSQLMatrixEnvArgs(b, qSQLMatrixRows, tc.query)
			if _, err := qRunSQL("q.sql", args); err != nil {
				b.Fatalf("warm q.sql(%q): %v", tc.query, err)
			}

			b.ReportAllocs()
			b.ResetTimer()
			start := time.Now()
			for i := 0; i < b.N; i++ {
				// Per-op value scope: the result is dead once the sink is
				// overwritten, so its roots are released instead of pinning
				// every result vector for the life of the process.
				scope := PushValueScope()
				out, err := qRunSQL("q.sql", args)
				if err != nil {
					b.Fatalf("q.sql(%q): %v", tc.query, err)
				}
				qSQLBindBenchSink = out
				scope.Release()
			}
			b.StopTimer()
			qSQLBindBenchReportRows(b, qSQLMatrixRows, start)
			qSQLBindBenchReportQStats(b)
		})
	}
}

func BenchmarkQSQLBindMatrixCold(b *testing.B) {
	for _, tc := range qSQLMatrixBenchCases {
		tc := tc
		b.Run(tc.name, func(b *testing.B) {
			qClearCaches()
			defer qClearCaches()
			args := qSQLMatrixEnvArgs(b, qSQLMatrixRows, tc.query)

			b.ReportAllocs()
			b.ResetTimer()
			start := time.Now()
			for i := 0; i < b.N; i++ {
				qClearCaches()
				scope := PushValueScope()
				out, err := qRunSQL("q.sql", args)
				if err != nil {
					b.Fatalf("q.sql(%q): %v", tc.query, err)
				}
				qSQLBindBenchSink = out
				scope.Release()
			}
			b.StopTimer()
			qSQLBindBenchReportRows(b, qSQLMatrixRows, start)
			qSQLBindBenchReportQStats(b)
		})
	}
}

func BenchmarkQSQLNativeGoMatrix(b *testing.B) {
	for _, tc := range qSQLMatrixBenchCases {
		tc := tc
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			start := time.Now()
			for i := 0; i < b.N; i++ {
				qSQLNativeBenchSink = tc.goFn(qSQLMatrixRows)
			}
			b.StopTimer()
			qSQLBindBenchReportRows(b, qSQLMatrixRows, start)
		})
	}
}
