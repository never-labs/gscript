package leia_test

import (
	"fmt"
	"testing"
)

func TestFinRobotLiveYahooChartDataIntegration(t *testing.T) {
	vm := newFinRobotLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
yahoo_chart_request_error := nil
yahoo_chart_json_error := nil
yahoo_chart_status := 0
yahoo_chart_ok := false
yahoo_chart_symbol := ""
yahoo_chart_currency := ""
yahoo_chart_exchange := ""
yahoo_chart_regular_price := 0.0
yahoo_chart_previous_close := 0.0
yahoo_chart_timestamp_count := 0
yahoo_chart_close_count := 0

headers := {}
headers["User-Agent"] = "Mozilla/5.0 Leia FinRobot live data smoke"
headers["Accept"] = "application/json"

resp, err := net.get("https://query2.finance.yahoo.com/v8/finance/chart/AAPL?range=5d&interval=1d&events=div%2Csplits", {
    headers: headers
    timeout: 30
})
if err != nil {
    yahoo_chart_request_error = err
} else {
    yahoo_chart_status = resp.status
    yahoo_chart_ok = resp.ok
    if resp.ok {
        data, json_err := resp.json()
        if json_err != nil {
            yahoo_chart_json_error = json_err
        } else {
            result := data.chart.result[1]
            meta := result.meta
            quote := result.indicators.quote[1]
            yahoo_chart_symbol = meta.symbol
            yahoo_chart_currency = meta.currency
            yahoo_chart_exchange = meta.exchangeName
            yahoo_chart_regular_price = meta.regularMarketPrice
            yahoo_chart_previous_close = meta.chartPreviousClose
            yahoo_chart_timestamp_count = #result.timestamp
            yahoo_chart_close_count = #quote.close
        }
    } else {
        yahoo_chart_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	assertFinRobotPublicLiveDataOK(t, vm, "Yahoo chart", "yahoo_chart_status", "yahoo_chart_request_error", "yahoo_chart_json_error", "yahoo_chart_ok")
	symbol := mustGetString(t, vm, "yahoo_chart_symbol")
	currency := mustGetString(t, vm, "yahoo_chart_currency")
	exchange := mustGetString(t, vm, "yahoo_chart_exchange")
	regularPrice := mustGetFloat(t, vm, "yahoo_chart_regular_price")
	previousClose := mustGetFloat(t, vm, "yahoo_chart_previous_close")
	timestampCount := mustGetInt(t, vm, "yahoo_chart_timestamp_count")
	closeCount := mustGetInt(t, vm, "yahoo_chart_close_count")
	fmt.Printf("yahoo_chart symbol=%q currency=%q exchange=%q regular_price=%f previous_close=%f timestamps=%d closes=%d\n", symbol, currency, exchange, regularPrice, previousClose, timestampCount, closeCount)
	if symbol != "AAPL" || currency == "" || exchange == "" || regularPrice <= 0 || previousClose <= 0 || timestampCount <= 0 || closeCount <= 0 {
		t.Fatalf("unexpected Yahoo chart payload: symbol=%q currency=%q exchange=%q regular_price=%f previous_close=%f timestamps=%d closes=%d", symbol, currency, exchange, regularPrice, previousClose, timestampCount, closeCount)
	}
}

func TestFinRobotLiveYahooSearchDataIntegration(t *testing.T) {
	vm := newFinRobotLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
yahoo_search_request_error := nil
yahoo_search_json_error := nil
yahoo_search_status := 0
yahoo_search_ok := false
yahoo_search_symbol := ""
yahoo_search_name := ""
yahoo_search_quote_type := ""
yahoo_search_exchange := ""
yahoo_search_sector := ""
yahoo_search_is_yahoo := false

headers := {}
headers["User-Agent"] = "Mozilla/5.0 Leia FinRobot live data smoke"
headers["Accept"] = "application/json"

resp, err := net.get("https://query2.finance.yahoo.com/v1/finance/search?q=AAPL&quotesCount=1&newsCount=0", {
    headers: headers
    timeout: 30
})
if err != nil {
    yahoo_search_request_error = err
} else {
    yahoo_search_status = resp.status
    yahoo_search_ok = resp.ok
    if resp.ok {
        data, json_err := resp.json()
        if json_err != nil {
            yahoo_search_json_error = json_err
        } else {
            row := data.quotes[1]
            yahoo_search_symbol = row.symbol
            yahoo_search_name = row.longname
            yahoo_search_quote_type = row.quoteType
            yahoo_search_exchange = row.exchange
            yahoo_search_sector = row.sector
            yahoo_search_is_yahoo = row.isYahooFinance
        }
    } else {
        yahoo_search_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	assertFinRobotPublicLiveDataOK(t, vm, "Yahoo search", "yahoo_search_status", "yahoo_search_request_error", "yahoo_search_json_error", "yahoo_search_ok")
	symbol := mustGetString(t, vm, "yahoo_search_symbol")
	name := mustGetString(t, vm, "yahoo_search_name")
	quoteType := mustGetString(t, vm, "yahoo_search_quote_type")
	exchange := mustGetString(t, vm, "yahoo_search_exchange")
	sector := mustGetString(t, vm, "yahoo_search_sector")
	isYahoo := mustGetBool(t, vm, "yahoo_search_is_yahoo")
	fmt.Printf("yahoo_search symbol=%q name=%q quote_type=%q exchange=%q sector=%q is_yahoo=%v\n", symbol, name, quoteType, exchange, sector, isYahoo)
	if symbol != "AAPL" || name == "" || quoteType != "EQUITY" || exchange == "" || sector == "" || !isYahoo {
		t.Fatalf("unexpected Yahoo search payload: symbol=%q name=%q quote_type=%q exchange=%q sector=%q is_yahoo=%v", symbol, name, quoteType, exchange, sector, isYahoo)
	}
}
