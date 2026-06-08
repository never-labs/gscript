package bind

import (
	"testing"
	"time"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

var qSQLBindBenchSink Value

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
}

func qSQLBindBenchTradesFrame(b *testing.B, rows int) data.Frame {
	b.Helper()
	symbols := []string{"AAPL", "MSFT", "NVDA", "TSLA", "AMZN", "META", "GOOG", "IBM"}
	sym := make([]string, rows)
	price := make([]float64, rows)
	size := make([]int64, rows)
	active := make([]bool, rows)
	for i := 0; i < rows; i++ {
		sym[i] = symbols[i%len(symbols)]
		price[i] = 75 + float64((i*17)%220) + float64(i%10)/10
		size[i] = int64(1 + (i*13)%500)
		active[i] = i%5 != 0
	}
	frame, err := data.NewFrame(
		data.Column{Name: "sym", Data: data.NewSymbols(sym)},
		data.Column{Name: "price", Data: data.NewF64(price)},
		data.Column{Name: "size", Data: data.NewI64(size)},
		data.Column{Name: "active", Data: data.NewBool(active)},
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
