package leia_test

import (
	"fmt"
	"testing"
)

func TestFinRobotLiveTreasuryFiscalDataAverageInterestRatesIntegration(t *testing.T) {
	vm := newFinRobotLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
treasury_avg_rates_request_error := nil
treasury_avg_rates_json_error := nil
treasury_avg_rates_status := 0
treasury_avg_rates_ok := false
treasury_avg_rates_count := 0
treasury_avg_rates_meta_present := false
treasury_avg_rates_record_date := ""
treasury_avg_rates_security_type_desc := ""
treasury_avg_rates_security_desc := ""
treasury_avg_rates_amt := ""

headers := {}
headers["User-Agent"] = "Mozilla/5.0 Leia FinRobot live data smoke"
headers["Accept"] = "application/json"

resp, err := net.get("https://api.fiscaldata.treasury.gov/services/api/fiscal_service/v2/accounting/od/avg_interest_rates?sort=-record_date&page[size]=5&format=json", {
    headers: headers
    timeout: 30
})
if err != nil {
    treasury_avg_rates_request_error = err
} else {
    treasury_avg_rates_status = resp.status
    treasury_avg_rates_ok = resp.ok
    if resp.ok {
        data, json_err := resp.json()
        if json_err != nil {
            treasury_avg_rates_json_error = json_err
        } else {
            treasury_avg_rates_meta_present = data.meta != nil
            treasury_avg_rates_count = #data.data
            if treasury_avg_rates_count > 0 {
                row := data.data[1]
                treasury_avg_rates_record_date = row.record_date
                treasury_avg_rates_security_type_desc = row.security_type_desc
                treasury_avg_rates_security_desc = row.security_desc
                treasury_avg_rates_amt = row.avg_interest_rate_amt
            }
        }
    } else {
        treasury_avg_rates_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	assertFinRobotPublicLiveDataOK(t, vm, "Treasury FiscalData average interest rates", "treasury_avg_rates_status", "treasury_avg_rates_request_error", "treasury_avg_rates_json_error", "treasury_avg_rates_ok")
	count := mustGetInt(t, vm, "treasury_avg_rates_count")
	metaPresent := mustGetBool(t, vm, "treasury_avg_rates_meta_present")
	recordDate := mustGetString(t, vm, "treasury_avg_rates_record_date")
	securityTypeDesc := mustGetString(t, vm, "treasury_avg_rates_security_type_desc")
	securityDesc := mustGetString(t, vm, "treasury_avg_rates_security_desc")
	avgInterestRateAmt := mustGetString(t, vm, "treasury_avg_rates_amt")
	fmt.Printf("treasury_avg_rates count=%d meta_present=%v record_date=%q security_type_desc=%q security_desc=%q avg_interest_rate_amt=%q\n", count, metaPresent, recordDate, securityTypeDesc, securityDesc, avgInterestRateAmt)
	if count <= 0 || !metaPresent || recordDate == "" || securityTypeDesc == "" || securityDesc == "" || avgInterestRateAmt == "" {
		t.Fatalf("unexpected Treasury FiscalData average interest rates payload: count=%d meta_present=%v record_date=%q security_type_desc=%q security_desc=%q avg_interest_rate_amt=%q", count, metaPresent, recordDate, securityTypeDesc, securityDesc, avgInterestRateAmt)
	}
}
