package leia_test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestFinRobotLiveCFTCCOTMarketPositionsDataIntegration(t *testing.T) {
	vm := newFinRobotLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
cftc_cot_request_error := nil
cftc_cot_json_error := nil
cftc_cot_status := 0
cftc_cot_ok := false
cftc_cot_count := 0
cftc_cot_market := ""
cftc_cot_report_date := ""
cftc_cot_commodity := ""
cftc_cot_open_interest := ""
cftc_cot_noncomm_long := ""
cftc_cot_noncomm_short := ""
cftc_cot_comm_long := ""
cftc_cot_comm_short := ""
cftc_cot_report_type := ""

headers := {}
headers["User-Agent"] = "Mozilla/5.0 Leia FinRobot live data smoke"
headers["Accept"] = "application/json"

resp, err := net.get("https://publicreporting.cftc.gov/resource/6dca-aqww.json?%24select=market_and_exchange_names,report_date_as_yyyy_mm_dd,commodity_name,open_interest_all,noncomm_positions_long_all,noncomm_positions_short_all,comm_positions_long_all,comm_positions_short_all,futonly_or_combined&%24order=report_date_as_yyyy_mm_dd%20DESC&%24limit=1", {
    headers: headers
    timeout: 30
})
if err != nil {
    cftc_cot_request_error = err
} else {
    cftc_cot_status = resp.status
    cftc_cot_ok = resp.ok
    if resp.ok {
        data, json_err := resp.json()
        if json_err != nil {
            cftc_cot_json_error = json_err
        } else {
            cftc_cot_count = #data
            if cftc_cot_count > 0 {
                row := data[1]
                cftc_cot_market = row.market_and_exchange_names
                cftc_cot_report_date = row.report_date_as_yyyy_mm_dd
                cftc_cot_commodity = row.commodity_name
                cftc_cot_open_interest = row.open_interest_all
                cftc_cot_noncomm_long = row.noncomm_positions_long_all
                cftc_cot_noncomm_short = row.noncomm_positions_short_all
                cftc_cot_comm_long = row.comm_positions_long_all
                cftc_cot_comm_short = row.comm_positions_short_all
                cftc_cot_report_type = row.futonly_or_combined
            }
        }
    } else {
        cftc_cot_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	assertFinRobotPublicLiveDataOK(t, vm, "CFTC COT market positions", "cftc_cot_status", "cftc_cot_request_error", "cftc_cot_json_error", "cftc_cot_ok")
	count := mustGetInt(t, vm, "cftc_cot_count")
	market := mustGetString(t, vm, "cftc_cot_market")
	reportDate := mustGetString(t, vm, "cftc_cot_report_date")
	commodity := mustGetString(t, vm, "cftc_cot_commodity")
	openInterestText := mustGetString(t, vm, "cftc_cot_open_interest")
	nonCommLongText := mustGetString(t, vm, "cftc_cot_noncomm_long")
	nonCommShortText := mustGetString(t, vm, "cftc_cot_noncomm_short")
	commLongText := mustGetString(t, vm, "cftc_cot_comm_long")
	commShortText := mustGetString(t, vm, "cftc_cot_comm_short")
	reportType := mustGetString(t, vm, "cftc_cot_report_type")
	fmt.Printf("cftc_cot count=%d market=%q date=%q commodity=%q open_interest=%q noncomm=%q/%q comm=%q/%q type=%q\n", count, market, reportDate, commodity, openInterestText, nonCommLongText, nonCommShortText, commLongText, commShortText, reportType)
	if count <= 0 || market == "" || reportDate == "" || commodity == "" || reportType == "" {
		t.Fatalf("unexpected CFTC COT identity payload: count=%d market=%q date=%q commodity=%q type=%q", count, market, reportDate, commodity, reportType)
	}
	reportTime, err := time.Parse("2006-01-02T15:04:05.000", reportDate)
	if err != nil {
		t.Fatalf("CFTC COT report date = %q, want Socrata timestamp: %v", reportDate, err)
	}
	if reportTime.Year() < 2020 || reportTime.After(time.Now().AddDate(0, 1, 0)) {
		t.Fatalf("CFTC COT report date = %s, want plausible recent report date", reportDate)
	}
	openInterest := mustParseNonNegativeCFTCInt(t, "open_interest_all", openInterestText)
	nonCommLong := mustParseNonNegativeCFTCInt(t, "noncomm_positions_long_all", nonCommLongText)
	nonCommShort := mustParseNonNegativeCFTCInt(t, "noncomm_positions_short_all", nonCommShortText)
	commLong := mustParseNonNegativeCFTCInt(t, "comm_positions_long_all", commLongText)
	commShort := mustParseNonNegativeCFTCInt(t, "comm_positions_short_all", commShortText)
	if openInterest <= 0 || nonCommLong+nonCommShort+commLong+commShort <= 0 {
		t.Fatalf("unexpected CFTC COT position values: open_interest=%d noncomm=%d/%d comm=%d/%d", openInterest, nonCommLong, nonCommShort, commLong, commShort)
	}
	if !strings.EqualFold(reportType, "FutOnly") && !strings.EqualFold(reportType, "Combined") {
		t.Fatalf("CFTC COT report type = %q, want FutOnly or Combined", reportType)
	}
}

func mustParseNonNegativeCFTCInt(t *testing.T, name, text string) int64 {
	t.Helper()
	value, err := strconv.ParseInt(text, 10, 64)
	if err != nil || value < 0 {
		t.Fatalf("CFTC COT %s = %q, want non-negative integer: %v", name, text, err)
	}
	return value
}
