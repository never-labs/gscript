package leia_test

import (
	"fmt"
	"testing"
)

func TestFinRobotLiveYahooTrendingDataIntegration(t *testing.T) {
	vm := newFinRobotLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
yahoo_trending_request_error := nil
yahoo_trending_json_error := nil
yahoo_trending_status := 0
yahoo_trending_ok := false
yahoo_trending_result_count := 0
yahoo_trending_quote_count := 0
yahoo_trending_first_symbol := ""
yahoo_trending_second_symbol := ""
yahoo_trending_job_timestamp := 0
yahoo_trending_start_interval := 0
yahoo_trending_has_error := false

headers := {}
headers["User-Agent"] = "Mozilla/5.0 Leia FinRobot live data smoke"
headers["Accept"] = "application/json"

resp, err := net.get("https://query1.finance.yahoo.com/v1/finance/trending/US?count=5", {
    headers: headers
    timeout: 30
})
if err != nil {
    yahoo_trending_request_error = err
} else {
    yahoo_trending_status = resp.status
    yahoo_trending_ok = resp.ok
    if resp.ok {
        data, json_err := resp.json()
        if json_err != nil {
            yahoo_trending_json_error = json_err
        } else {
            yahoo_trending_has_error = data.finance.error != nil
            result := data.finance.result[1]
            yahoo_trending_result_count = result.count
            yahoo_trending_quote_count = #result.quotes
            yahoo_trending_job_timestamp = result.jobTimestamp
            yahoo_trending_start_interval = result.startInterval
            if yahoo_trending_quote_count > 0 {
                yahoo_trending_first_symbol = result.quotes[1].symbol
            }
            if yahoo_trending_quote_count > 1 {
                yahoo_trending_second_symbol = result.quotes[2].symbol
            }
        }
    } else {
        yahoo_trending_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	assertFinRobotPublicLiveDataOK(t, vm, "Yahoo trending", "yahoo_trending_status", "yahoo_trending_request_error", "yahoo_trending_json_error", "yahoo_trending_ok")
	resultCount := mustGetInt(t, vm, "yahoo_trending_result_count")
	quoteCount := mustGetInt(t, vm, "yahoo_trending_quote_count")
	firstSymbol := mustGetString(t, vm, "yahoo_trending_first_symbol")
	secondSymbol := mustGetString(t, vm, "yahoo_trending_second_symbol")
	jobTimestamp := mustGetInt(t, vm, "yahoo_trending_job_timestamp")
	startInterval := mustGetInt(t, vm, "yahoo_trending_start_interval")
	hasError := mustGetBool(t, vm, "yahoo_trending_has_error")
	fmt.Printf("yahoo_trending result_count=%d quote_count=%d first_symbol=%q second_symbol=%q job_timestamp=%d start_interval=%d has_error=%v\n", resultCount, quoteCount, firstSymbol, secondSymbol, jobTimestamp, startInterval, hasError)
	if hasError || resultCount != 5 || quoteCount != 5 || firstSymbol == "" || secondSymbol == "" || jobTimestamp <= 0 || startInterval <= 0 {
		t.Fatalf("unexpected Yahoo trending payload: result_count=%d quote_count=%d first_symbol=%q second_symbol=%q job_timestamp=%d start_interval=%d has_error=%v", resultCount, quoteCount, firstSymbol, secondSymbol, jobTimestamp, startInterval, hasError)
	}
}
