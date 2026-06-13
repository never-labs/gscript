package leia_test

import (
	"fmt"
	"strconv"
	"testing"
)

func TestFinRobotLiveTreasuryFiscalDataAuctionsIntegration(t *testing.T) {
	vm := newFinRobotLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
treasury_auctions_request_error := nil
treasury_auctions_json_error := nil
treasury_auctions_status := 0
treasury_auctions_ok := false
treasury_auctions_count := 0
treasury_auctions_meta_present := false
treasury_auctions_record_date := ""
treasury_auctions_cusip := ""
treasury_auctions_security_type := ""
treasury_auctions_security_term := ""
treasury_auctions_auction_date := ""
treasury_auctions_issue_date := ""
treasury_auctions_maturity_date := ""
treasury_auctions_offering_amt := ""
treasury_auctions_total_accepted := ""

headers := {}
headers["User-Agent"] = "Mozilla/5.0 Leia FinRobot live data smoke"
headers["Accept"] = "application/json"

resp, err := net.get("https://api.fiscaldata.treasury.gov/services/api/fiscal_service/v1/accounting/od/auctions_query?fields=record_date,cusip,security_type,security_term,auction_date,issue_date,maturity_date,offering_amt,total_accepted&sort=-record_date&page%5Bsize%5D=5&format=json", {
    headers: headers
    timeout: 30
})
if err != nil {
    treasury_auctions_request_error = err
} else {
    treasury_auctions_status = resp.status
    treasury_auctions_ok = resp.ok
    if resp.ok {
        data, json_err := resp.json()
        if json_err != nil {
            treasury_auctions_json_error = json_err
        } else {
            treasury_auctions_meta_present = data.meta != nil
            treasury_auctions_count = #data.data
            if treasury_auctions_count > 0 {
                row := data.data[1]
                treasury_auctions_record_date = row.record_date
                treasury_auctions_cusip = row.cusip
                treasury_auctions_security_type = row.security_type
                treasury_auctions_security_term = row.security_term
                treasury_auctions_auction_date = row.auction_date
                treasury_auctions_issue_date = row.issue_date
                treasury_auctions_maturity_date = row.maturity_date
                treasury_auctions_offering_amt = row.offering_amt
                treasury_auctions_total_accepted = row.total_accepted
            }
        }
    } else {
        treasury_auctions_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	assertFinRobotPublicLiveDataOK(t, vm, "Treasury FiscalData auctions", "treasury_auctions_status", "treasury_auctions_request_error", "treasury_auctions_json_error", "treasury_auctions_ok")
	count := mustGetInt(t, vm, "treasury_auctions_count")
	metaPresent := mustGetBool(t, vm, "treasury_auctions_meta_present")
	recordDate := mustGetString(t, vm, "treasury_auctions_record_date")
	cusip := mustGetString(t, vm, "treasury_auctions_cusip")
	securityType := mustGetString(t, vm, "treasury_auctions_security_type")
	securityTerm := mustGetString(t, vm, "treasury_auctions_security_term")
	auctionDate := mustGetString(t, vm, "treasury_auctions_auction_date")
	issueDate := mustGetString(t, vm, "treasury_auctions_issue_date")
	maturityDate := mustGetString(t, vm, "treasury_auctions_maturity_date")
	offeringAmt := mustGetString(t, vm, "treasury_auctions_offering_amt")
	totalAccepted := mustGetString(t, vm, "treasury_auctions_total_accepted")
	fmt.Printf("treasury_auctions count=%d meta_present=%v record_date=%q cusip=%q type=%q term=%q auction_date=%q issue_date=%q maturity_date=%q offering_amt=%q total_accepted=%q\n", count, metaPresent, recordDate, cusip, securityType, securityTerm, auctionDate, issueDate, maturityDate, offeringAmt, totalAccepted)
	if count <= 0 || !metaPresent || recordDate == "" || cusip == "" || securityType == "" || securityTerm == "" || auctionDate == "" || issueDate == "" || maturityDate == "" {
		t.Fatalf("unexpected Treasury FiscalData auctions identity payload: count=%d meta_present=%v record_date=%q cusip=%q type=%q term=%q auction_date=%q issue_date=%q maturity_date=%q", count, metaPresent, recordDate, cusip, securityType, securityTerm, auctionDate, issueDate, maturityDate)
	}
	if _, ok := parseOptionalPositiveTreasuryAmount(offeringAmt); offeringAmt != "" && offeringAmt != "null" && !ok {
		t.Fatalf("unexpected Treasury FiscalData auctions offering amount %q", offeringAmt)
	}
	if _, ok := parseOptionalPositiveTreasuryAmount(totalAccepted); totalAccepted != "" && totalAccepted != "null" && !ok {
		t.Fatalf("unexpected Treasury FiscalData auctions total accepted %q", totalAccepted)
	}
}

func parseOptionalPositiveTreasuryAmount(text string) (float64, bool) {
	if text == "" || text == "null" {
		return 0, false
	}
	value, err := strconv.ParseFloat(text, 64)
	return value, err == nil && value > 0
}
