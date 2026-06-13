package leia_test

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestFinRobotLiveCFPBConsumerComplaintsDataIntegration(t *testing.T) {
	vm := newFinRobotLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
cfpb_complaints_request_error := nil
cfpb_complaints_json_error := nil
cfpb_complaints_status := 0
cfpb_complaints_ok := false
cfpb_complaints_total := 0
cfpb_complaints_count := 0
cfpb_complaints_id := ""
cfpb_complaints_product := ""
cfpb_complaints_issue := ""
cfpb_complaints_company := ""
cfpb_complaints_state := ""
cfpb_complaints_received := ""
cfpb_complaints_sent_to_company := ""
cfpb_complaints_submitted_via := ""
cfpb_complaints_timely := ""
cfpb_complaints_company_response := ""

headers := {}
headers["User-Agent"] = "Mozilla/5.0 Leia FinRobot live data smoke"
headers["Accept"] = "application/json"

resp, err := net.get("https://www.consumerfinance.gov/data-research/consumer-complaints/search/api/v1/?size=1&sort=created_date_desc", {
    headers: headers
    timeout: 30
})
if err != nil {
    cfpb_complaints_request_error = err
} else {
    cfpb_complaints_status = resp.status
    cfpb_complaints_ok = resp.ok
    if resp.ok {
        data, json_err := resp.json()
        if json_err != nil {
            cfpb_complaints_json_error = json_err
        } else {
            cfpb_complaints_total = data.hits.total.value
            cfpb_complaints_count = #data.hits.hits
            if cfpb_complaints_count > 0 {
                hit := data.hits.hits[1]
                row := hit["_source"]
                cfpb_complaints_id = row.complaint_id
                cfpb_complaints_product = row.product
                cfpb_complaints_issue = row.issue
                cfpb_complaints_company = row.company
                cfpb_complaints_state = row.state
                cfpb_complaints_received = row.date_received
                cfpb_complaints_sent_to_company = row.date_sent_to_company
                cfpb_complaints_submitted_via = row.submitted_via
                cfpb_complaints_timely = row.timely
                cfpb_complaints_company_response = row.company_response
            }
        }
    } else {
        cfpb_complaints_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	assertFinRobotPublicLiveDataOK(t, vm, "CFPB consumer complaints", "cfpb_complaints_status", "cfpb_complaints_request_error", "cfpb_complaints_json_error", "cfpb_complaints_ok")
	total := mustGetInt(t, vm, "cfpb_complaints_total")
	count := mustGetInt(t, vm, "cfpb_complaints_count")
	id := mustGetString(t, vm, "cfpb_complaints_id")
	product := mustGetString(t, vm, "cfpb_complaints_product")
	issue := mustGetString(t, vm, "cfpb_complaints_issue")
	company := mustGetString(t, vm, "cfpb_complaints_company")
	state := mustGetString(t, vm, "cfpb_complaints_state")
	received := mustGetString(t, vm, "cfpb_complaints_received")
	sentToCompany := mustGetString(t, vm, "cfpb_complaints_sent_to_company")
	submittedVia := mustGetString(t, vm, "cfpb_complaints_submitted_via")
	timely := mustGetString(t, vm, "cfpb_complaints_timely")
	companyResponse := mustGetString(t, vm, "cfpb_complaints_company_response")
	fmt.Printf("cfpb_complaints total=%d count=%d id=%q product=%q issue=%q company=%q state=%q received=%q submitted_via=%q timely=%q response=%q\n", total, count, id, product, issue, company, state, received, submittedVia, timely, companyResponse)
	if total <= 0 || count <= 0 || id == "" || product == "" || issue == "" || company == "" || received == "" || sentToCompany == "" || submittedVia == "" || timely == "" || companyResponse == "" {
		t.Fatalf("unexpected CFPB complaints payload: total=%d count=%d id=%q product=%q issue=%q company=%q received=%q sent=%q submitted=%q timely=%q response=%q", total, count, id, product, issue, company, received, sentToCompany, submittedVia, timely, companyResponse)
	}
	if state != "" && len(state) != 2 {
		t.Fatalf("CFPB complaint state = %q, want 2-letter code or empty", state)
	}
	receivedAt, err := time.Parse(time.RFC3339, received)
	if err != nil {
		t.Fatalf("CFPB complaint date_received = %q, want RFC3339 timestamp: %v", received, err)
	}
	if receivedAt.Year() < 2011 || receivedAt.After(time.Now().AddDate(0, 0, 2)) {
		t.Fatalf("CFPB complaint date_received = %s, want plausible received date", received)
	}
	if !strings.EqualFold(timely, "Yes") && !strings.EqualFold(timely, "No") {
		t.Fatalf("CFPB complaint timely = %q, want Yes or No", timely)
	}
}
