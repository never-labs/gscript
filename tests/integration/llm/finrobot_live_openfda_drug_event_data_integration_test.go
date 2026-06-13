package leia_test

import (
	"fmt"
	"testing"
)

func TestFinRobotLiveOpenFDADrugEventDataIntegration(t *testing.T) {
	vm := newFinRobotLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
openfda_drug_event_request_error := nil
openfda_drug_event_json_error := nil
openfda_drug_event_status := 0
openfda_drug_event_ok := false
openfda_drug_event_total := 0
openfda_drug_event_count := 0
openfda_drug_event_last_updated := ""
openfda_drug_event_report_id := ""
openfda_drug_event_receivedate := ""
openfda_drug_event_serious := ""
openfda_drug_event_country := ""
openfda_drug_event_reaction := ""
openfda_drug_event_drug_name := ""

headers := {}
headers["User-Agent"] = "Mozilla/5.0 Leia FinRobot live data smoke"
headers["Accept"] = "application/json"

resp, err := net.get("https://api.fda.gov/drug/event.json?search=receivedate:%5B20250101+TO+20260613%5D&limit=1", {
    headers: headers
    timeout: 30
})
if err != nil {
    openfda_drug_event_request_error = err
} else {
    openfda_drug_event_status = resp.status
    openfda_drug_event_ok = resp.ok
    if resp.ok {
        data, json_err := resp.json()
        if json_err != nil {
            openfda_drug_event_json_error = json_err
        } else {
            openfda_drug_event_total = data.meta.results.total
            openfda_drug_event_last_updated = data.meta.last_updated
            openfda_drug_event_count = #data.results
            if openfda_drug_event_count > 0 {
                row := data.results[1]
                openfda_drug_event_report_id = row.safetyreportid
                openfda_drug_event_receivedate = row.receivedate
                openfda_drug_event_serious = row.serious
                openfda_drug_event_country = row.occurcountry
                if row.patient != nil && row.patient.reaction != nil && #row.patient.reaction > 0 {
                    openfda_drug_event_reaction = row.patient.reaction[1].reactionmeddrapt
                }
                if row.patient != nil && row.patient.drug != nil && #row.patient.drug > 0 {
                    drug := row.patient.drug[1]
                    if drug.medicinalproduct != nil {
                        openfda_drug_event_drug_name = drug.medicinalproduct
                    }
                }
            }
        }
    } else {
        openfda_drug_event_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	assertFinRobotPublicLiveDataOK(t, vm, "openFDA drug adverse events", "openfda_drug_event_status", "openfda_drug_event_request_error", "openfda_drug_event_json_error", "openfda_drug_event_ok")
	total := mustGetInt(t, vm, "openfda_drug_event_total")
	count := mustGetInt(t, vm, "openfda_drug_event_count")
	lastUpdated := mustGetString(t, vm, "openfda_drug_event_last_updated")
	reportID := mustGetString(t, vm, "openfda_drug_event_report_id")
	receivedDate := mustGetString(t, vm, "openfda_drug_event_receivedate")
	serious := mustGetString(t, vm, "openfda_drug_event_serious")
	country := mustGetString(t, vm, "openfda_drug_event_country")
	reaction := mustGetString(t, vm, "openfda_drug_event_reaction")
	drugName := mustGetString(t, vm, "openfda_drug_event_drug_name")
	fmt.Printf("openfda_drug_event total=%d count=%d last_updated=%q report_id=%q receivedate=%q serious=%q country=%q reaction=%q drug=%q\n", total, count, lastUpdated, reportID, receivedDate, serious, country, reaction, drugName)
	if total <= 0 || count <= 0 || lastUpdated == "" || reportID == "" || receivedDate == "" || serious == "" {
		t.Fatalf("unexpected openFDA drug event metadata: total=%d count=%d last_updated=%q report_id=%q receivedate=%q serious=%q", total, count, lastUpdated, reportID, receivedDate, serious)
	}
	if country == "" && reaction == "" && drugName == "" {
		t.Fatalf("unexpected openFDA drug event payload: country, reaction, and drug name are all empty")
	}
}
