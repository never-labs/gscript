package leia_test

import (
	"fmt"
	"testing"
)

func TestFinRobotLiveUSASpendingAgenciesDataIntegration(t *testing.T) {
	vm := newFinRobotLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
usaspending_request_error := nil
usaspending_json_error := nil
usaspending_status := 0
usaspending_ok := false
usaspending_count := 0
usaspending_agency_id := 0
usaspending_toptier_code := ""
usaspending_abbreviation := ""
usaspending_agency_name := ""
usaspending_active_fy := ""
usaspending_active_fq := ""
usaspending_outlay_amount := 0.0
usaspending_obligated_amount := 0.0
usaspending_budget_authority_amount := 0.0
usaspending_total_budget_authority_amount := 0.0
usaspending_agency_slug := ""

headers := {}
headers["User-Agent"] = "Mozilla/5.0 Leia FinRobot live data smoke"
headers["Accept"] = "application/json"

resp, err := net.get("https://api.usaspending.gov/api/v2/references/toptier_agencies/", {
    headers: headers
    timeout: 30
})
if err != nil {
    usaspending_request_error = err
} else {
    usaspending_status = resp.status
    usaspending_ok = resp.ok
    if resp.ok {
        data, json_err := resp.json()
        if json_err != nil {
            usaspending_json_error = json_err
        } else {
            usaspending_count = #data.results
            if usaspending_count > 0 {
                row := data.results[1]
                usaspending_agency_id = row.agency_id
                usaspending_toptier_code = row.toptier_code
                usaspending_abbreviation = row.abbreviation
                usaspending_agency_name = row.agency_name
                usaspending_active_fy = row.active_fy
                usaspending_active_fq = row.active_fq
                usaspending_outlay_amount = row.outlay_amount
                usaspending_obligated_amount = row.obligated_amount
                usaspending_budget_authority_amount = row.budget_authority_amount
                usaspending_total_budget_authority_amount = row.current_total_budget_authority_amount
                usaspending_agency_slug = row.agency_slug
            }
        }
    } else {
        usaspending_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	assertFinRobotPublicLiveDataOK(t, vm, "USAspending top-tier agencies", "usaspending_status", "usaspending_request_error", "usaspending_json_error", "usaspending_ok")
	count := mustGetInt(t, vm, "usaspending_count")
	agencyID := mustGetInt(t, vm, "usaspending_agency_id")
	toptierCode := mustGetString(t, vm, "usaspending_toptier_code")
	abbreviation := mustGetString(t, vm, "usaspending_abbreviation")
	agencyName := mustGetString(t, vm, "usaspending_agency_name")
	activeFY := mustGetString(t, vm, "usaspending_active_fy")
	activeFQ := mustGetString(t, vm, "usaspending_active_fq")
	outlay := mustGetFloat(t, vm, "usaspending_outlay_amount")
	obligated := mustGetFloat(t, vm, "usaspending_obligated_amount")
	budgetAuthority := mustGetFloat(t, vm, "usaspending_budget_authority_amount")
	totalBudgetAuthority := mustGetFloat(t, vm, "usaspending_total_budget_authority_amount")
	agencySlug := mustGetString(t, vm, "usaspending_agency_slug")
	fmt.Printf("usaspending count=%d agency_id=%d code=%q abbr=%q name=%q fy=%q fq=%q outlay=%f obligated=%f budget_authority=%f total_budget_authority=%f slug=%q\n", count, agencyID, toptierCode, abbreviation, agencyName, activeFY, activeFQ, outlay, obligated, budgetAuthority, totalBudgetAuthority, agencySlug)
	if count <= 0 || agencyID <= 0 || toptierCode == "" || abbreviation == "" || agencyName == "" || activeFY == "" || activeFQ == "" || agencySlug == "" {
		t.Fatalf("unexpected USAspending agency identity payload: count=%d agency_id=%d code=%q abbr=%q name=%q fy=%q fq=%q slug=%q", count, agencyID, toptierCode, abbreviation, agencyName, activeFY, activeFQ, agencySlug)
	}
	if totalBudgetAuthority <= 0 {
		t.Fatalf("USAspending total budget authority = %f, want positive", totalBudgetAuthority)
	}
	if outlay < 0 || obligated < 0 || budgetAuthority < 0 {
		t.Fatalf("USAspending agency amounts should be non-negative: outlay=%f obligated=%f budget_authority=%f", outlay, obligated, budgetAuthority)
	}
}
