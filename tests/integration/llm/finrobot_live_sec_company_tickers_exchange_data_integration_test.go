package leia_test

import (
	"fmt"
	"strings"
	"testing"
)

func TestFinRobotLiveSECCompanyTickersExchangeDataIntegration(t *testing.T) {
	vm := newFinRobotLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
sec_exchange_request_error := nil
sec_exchange_json_error := nil
sec_exchange_status := 0
sec_exchange_ok := false
sec_exchange_field_count := 0
sec_exchange_row_count := 0
sec_exchange_field_cik := ""
sec_exchange_field_name := ""
sec_exchange_field_ticker := ""
sec_exchange_field_exchange := ""
sec_exchange_aapl_cik := 0
sec_exchange_aapl_name := ""
sec_exchange_aapl_ticker := ""
sec_exchange_aapl_exchange := ""

headers := {}
headers["User-Agent"] = os.getenv("LEIA_SEC_USER_AGENT")
headers["Accept"] = "application/json"

resp, err := net.get("https://www.sec.gov/files/company_tickers_exchange.json", {
    headers: headers
    timeout: 30
})
if err != nil {
    sec_exchange_request_error = err
} else {
    sec_exchange_status = resp.status
    sec_exchange_ok = resp.ok
    if resp.ok {
        data, json_err := resp.json()
        if json_err != nil {
            sec_exchange_json_error = json_err
        } else {
            sec_exchange_field_count = #data.fields
            sec_exchange_row_count = #data.data
            if sec_exchange_field_count >= 4 {
                sec_exchange_field_cik = data.fields[1]
                sec_exchange_field_name = data.fields[2]
                sec_exchange_field_ticker = data.fields[3]
                sec_exchange_field_exchange = data.fields[4]
            }
            for _, row := range pairs(data.data) {
                if #row >= 4 && row[3] == "AAPL" {
                    sec_exchange_aapl_cik = row[1]
                    sec_exchange_aapl_name = row[2]
                    sec_exchange_aapl_ticker = row[3]
                    sec_exchange_aapl_exchange = row[4]
                }
            }
        }
    } else {
        sec_exchange_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	status := mustGetInt(t, vm, "sec_exchange_status")
	skipUnavailableFinRobotLiveData(t, "SEC company tickers exchange", status, getOrNil(t, vm, "sec_exchange_request_error"))
	if status != 200 {
		t.Fatalf("SEC company tickers exchange status = %d, want 200", status)
	}
	if got := getOrNil(t, vm, "sec_exchange_json_error"); got != nil {
		t.Fatalf("SEC company tickers exchange JSON decode failed: %v", got)
	}
	if ok := mustGetBool(t, vm, "sec_exchange_ok"); !ok {
		t.Fatalf("SEC company tickers exchange ok = false")
	}
	fieldCount := mustGetInt(t, vm, "sec_exchange_field_count")
	rowCount := mustGetInt(t, vm, "sec_exchange_row_count")
	fieldCIK := mustGetString(t, vm, "sec_exchange_field_cik")
	fieldName := mustGetString(t, vm, "sec_exchange_field_name")
	fieldTicker := mustGetString(t, vm, "sec_exchange_field_ticker")
	fieldExchange := mustGetString(t, vm, "sec_exchange_field_exchange")
	aaplCIK := mustGetInt(t, vm, "sec_exchange_aapl_cik")
	aaplName := mustGetString(t, vm, "sec_exchange_aapl_name")
	aaplTicker := mustGetString(t, vm, "sec_exchange_aapl_ticker")
	aaplExchange := mustGetString(t, vm, "sec_exchange_aapl_exchange")
	fmt.Printf("sec_exchange fields=%d rows=%d field_names=%q/%q/%q/%q aapl_cik=%d aapl_name=%q aapl_exchange=%q\n", fieldCount, rowCount, fieldCIK, fieldName, fieldTicker, fieldExchange, aaplCIK, aaplName, aaplExchange)
	if fieldCount < 4 || rowCount <= 0 || fieldCIK != "cik" || fieldName != "name" || fieldTicker != "ticker" || fieldExchange != "exchange" {
		t.Fatalf("unexpected SEC company tickers exchange fields: count=%d rows=%d fields=%q/%q/%q/%q", fieldCount, rowCount, fieldCIK, fieldName, fieldTicker, fieldExchange)
	}
	if aaplCIK != 320193 || !strings.Contains(aaplName, "Apple") || aaplTicker != "AAPL" || aaplExchange == "" {
		t.Fatalf("unexpected SEC company tickers exchange AAPL row: cik=%d name=%q ticker=%q exchange=%q", aaplCIK, aaplName, aaplTicker, aaplExchange)
	}
}
