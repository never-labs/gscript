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

func TestFinRobotLiveTreasuryFiscalDataDebtToPennyIntegration(t *testing.T) {
	vm := newFinRobotLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
treasury_debt_request_error := nil
treasury_debt_json_error := nil
treasury_debt_status := 0
treasury_debt_ok := false
treasury_debt_count := 0
treasury_debt_meta_present := false
treasury_debt_record_date := ""
treasury_debt_public_amt := ""
treasury_debt_intragov_amt := ""
treasury_debt_total_amt := ""
treasury_debt_fiscal_year := ""

headers := {}
headers["User-Agent"] = "Mozilla/5.0 Leia FinRobot live data smoke"
headers["Accept"] = "application/json"

resp, err := net.get("https://api.fiscaldata.treasury.gov/services/api/fiscal_service/v2/accounting/od/debt_to_penny?sort=-record_date&page%5Bsize%5D=5&format=json", {
    headers: headers
    timeout: 30
})
if err != nil {
    treasury_debt_request_error = err
} else {
    treasury_debt_status = resp.status
    treasury_debt_ok = resp.ok
    if resp.ok {
        data, json_err := resp.json()
        if json_err != nil {
            treasury_debt_json_error = json_err
        } else {
            treasury_debt_meta_present = data.meta != nil
            treasury_debt_count = #data.data
            if treasury_debt_count > 0 {
                row := data.data[1]
                treasury_debt_record_date = row.record_date
                treasury_debt_public_amt = row.debt_held_public_amt
                treasury_debt_intragov_amt = row.intragov_hold_amt
                treasury_debt_total_amt = row.tot_pub_debt_out_amt
                treasury_debt_fiscal_year = row.record_fiscal_year
            }
        }
    } else {
        treasury_debt_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	assertFinRobotPublicLiveDataOK(t, vm, "Treasury FiscalData debt to penny", "treasury_debt_status", "treasury_debt_request_error", "treasury_debt_json_error", "treasury_debt_ok")
	count := mustGetInt(t, vm, "treasury_debt_count")
	metaPresent := mustGetBool(t, vm, "treasury_debt_meta_present")
	recordDate := mustGetString(t, vm, "treasury_debt_record_date")
	publicAmt := mustGetString(t, vm, "treasury_debt_public_amt")
	intragovAmt := mustGetString(t, vm, "treasury_debt_intragov_amt")
	totalAmt := mustGetString(t, vm, "treasury_debt_total_amt")
	fiscalYear := mustGetString(t, vm, "treasury_debt_fiscal_year")
	fmt.Printf("treasury_debt count=%d meta_present=%v record_date=%q public_amt=%q intragov_amt=%q total_amt=%q fiscal_year=%q\n", count, metaPresent, recordDate, publicAmt, intragovAmt, totalAmt, fiscalYear)
	if count <= 0 || !metaPresent || recordDate == "" || publicAmt == "" || intragovAmt == "" || totalAmt == "" || fiscalYear == "" {
		t.Fatalf("unexpected Treasury FiscalData debt to penny payload: count=%d meta_present=%v record_date=%q public_amt=%q intragov_amt=%q total_amt=%q fiscal_year=%q", count, metaPresent, recordDate, publicAmt, intragovAmt, totalAmt, fiscalYear)
	}
}
