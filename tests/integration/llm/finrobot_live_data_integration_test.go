package leia_test

import (
	"fmt"
	"strings"
	"testing"
)

func TestFinRobotLiveSECCompanyConceptDataIntegration(t *testing.T) {
	vm := newFinRobotLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
sec_live_data_request_error := nil
sec_live_data_json_error := nil
sec_live_data_status := 0
sec_live_data_ok := false
sec_live_data_entity := ""
sec_live_data_tag := ""
sec_live_data_taxonomy := ""
sec_live_data_unit_count := 0

headers := {}
headers["User-Agent"] = os.getenv("LEIA_SEC_USER_AGENT")
headers["Accept"] = "application/json"

resp, err := net.get("https://data.sec.gov/api/xbrl/companyconcept/CIK0000320193/us-gaap/Assets.json", {
    headers: headers
    timeout: 30
})
if err != nil {
    sec_live_data_request_error = err
} else {
    sec_live_data_status = resp.status
    sec_live_data_ok = resp.ok
    if resp.ok {
        data, json_err := resp.json()
        if json_err != nil {
            sec_live_data_json_error = json_err
        } else {
            sec_live_data_entity = data.entityName
            sec_live_data_tag = data.tag
            sec_live_data_taxonomy = data.taxonomy
            sec_live_data_unit_count = #data.units.USD
        }
    } else {
        sec_live_data_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	status := mustGetInt(t, vm, "sec_live_data_status")
	skipUnavailableFinRobotLiveData(t, "SEC companyconcept", status, getOrNil(t, vm, "sec_live_data_request_error"))
	if status != 200 {
		t.Fatalf("SEC live data status = %d, want 200", status)
	}
	if got := getOrNil(t, vm, "sec_live_data_json_error"); got != nil {
		t.Fatalf("SEC companyconcept JSON decode failed: %v", got)
	}
	if ok := mustGetBool(t, vm, "sec_live_data_ok"); !ok {
		t.Fatalf("SEC live data ok = false")
	}
	entity := mustGetString(t, vm, "sec_live_data_entity")
	tag := mustGetString(t, vm, "sec_live_data_tag")
	taxonomy := mustGetString(t, vm, "sec_live_data_taxonomy")
	unitCount := mustGetInt(t, vm, "sec_live_data_unit_count")
	fmt.Printf("sec_entity=%q tag=%q taxonomy=%q units=%d\n", entity, tag, taxonomy, unitCount)
	if !strings.Contains(entity, "Apple") || tag != "Assets" || taxonomy != "us-gaap" || unitCount <= 0 {
		t.Fatalf("unexpected SEC companyconcept payload: entity=%q tag=%q taxonomy=%q units=%d", entity, tag, taxonomy, unitCount)
	}
}

func TestFinRobotLiveSECCompanyFactsDataIntegration(t *testing.T) {
	vm := newFinRobotLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
sec_companyfacts_request_error := nil
sec_companyfacts_json_error := nil
sec_companyfacts_status := 0
sec_companyfacts_ok := false
sec_companyfacts_entity := ""
sec_companyfacts_cik := 0
sec_companyfacts_assets_unit_count := 0
sec_companyfacts_revenue_unit_count := 0
sec_companyfacts_asset_form := ""

headers := {}
headers["User-Agent"] = os.getenv("LEIA_SEC_USER_AGENT")
headers["Accept"] = "application/json"

resp, err := net.get("https://data.sec.gov/api/xbrl/companyfacts/CIK0000320193.json", {
    headers: headers
    timeout: 30
})
if err != nil {
    sec_companyfacts_request_error = err
} else {
    sec_companyfacts_status = resp.status
    sec_companyfacts_ok = resp.ok
    if resp.ok {
        data, json_err := resp.json()
        if json_err != nil {
            sec_companyfacts_json_error = json_err
        } else {
            us_gaap := data.facts["us-gaap"]
            assets := us_gaap.Assets.units.USD
            revenues := us_gaap.Revenues.units.USD
            sec_companyfacts_entity = data.entityName
            sec_companyfacts_cik = data.cik
            sec_companyfacts_assets_unit_count = #assets
            sec_companyfacts_revenue_unit_count = #revenues
            sec_companyfacts_asset_form = assets[1].form
        }
    } else {
        sec_companyfacts_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	status := mustGetInt(t, vm, "sec_companyfacts_status")
	skipUnavailableFinRobotLiveData(t, "SEC companyfacts", status, getOrNil(t, vm, "sec_companyfacts_request_error"))
	if status != 200 {
		t.Fatalf("SEC companyfacts status = %d, want 200", status)
	}
	if got := getOrNil(t, vm, "sec_companyfacts_json_error"); got != nil {
		t.Fatalf("SEC companyfacts JSON decode failed: %v", got)
	}
	if ok := mustGetBool(t, vm, "sec_companyfacts_ok"); !ok {
		t.Fatalf("SEC companyfacts ok = false")
	}
	entity := mustGetString(t, vm, "sec_companyfacts_entity")
	cik := mustGetInt(t, vm, "sec_companyfacts_cik")
	assetCount := mustGetInt(t, vm, "sec_companyfacts_assets_unit_count")
	revenueCount := mustGetInt(t, vm, "sec_companyfacts_revenue_unit_count")
	assetForm := mustGetString(t, vm, "sec_companyfacts_asset_form")
	fmt.Printf("sec_companyfacts_entity=%q cik=%d assets=%d revenues=%d first_asset_form=%q\n", entity, cik, assetCount, revenueCount, assetForm)
	if !strings.Contains(entity, "Apple") || cik != 320193 || assetCount <= 0 || revenueCount <= 0 || assetForm == "" {
		t.Fatalf("unexpected SEC companyfacts payload: entity=%q cik=%d assets=%d revenues=%d first_asset_form=%q", entity, cik, assetCount, revenueCount, assetForm)
	}
}

func TestFinRobotLiveSECSubmissionsDataIntegration(t *testing.T) {
	vm := newFinRobotLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
sec_submissions_request_error := nil
sec_submissions_json_error := nil
sec_submissions_status := 0
sec_submissions_ok := false
sec_submissions_name := ""
sec_submissions_ticker := ""
sec_submissions_exchange := ""
sec_submissions_recent_count := 0
sec_submissions_recent_form := ""

headers := {}
headers["User-Agent"] = os.getenv("LEIA_SEC_USER_AGENT")
headers["Accept"] = "application/json"

resp, err := net.get("https://data.sec.gov/submissions/CIK0000320193.json", {
    headers: headers
    timeout: 30
})
if err != nil {
    sec_submissions_request_error = err
} else {
    sec_submissions_status = resp.status
    sec_submissions_ok = resp.ok
    if resp.ok {
        data, json_err := resp.json()
        if json_err != nil {
            sec_submissions_json_error = json_err
        } else {
            sec_submissions_name = data.name
            sec_submissions_ticker = data.tickers[1]
            sec_submissions_exchange = data.exchanges[1]
            sec_submissions_recent_count = #data.filings.recent.accessionNumber
            sec_submissions_recent_form = data.filings.recent.form[1]
        }
    } else {
        sec_submissions_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	status := mustGetInt(t, vm, "sec_submissions_status")
	skipUnavailableFinRobotLiveData(t, "SEC submissions", status, getOrNil(t, vm, "sec_submissions_request_error"))
	if status != 200 {
		t.Fatalf("SEC submissions status = %d, want 200", status)
	}
	if got := getOrNil(t, vm, "sec_submissions_json_error"); got != nil {
		t.Fatalf("SEC submissions JSON decode failed: %v", got)
	}
	if ok := mustGetBool(t, vm, "sec_submissions_ok"); !ok {
		t.Fatalf("SEC submissions ok = false")
	}
	name := mustGetString(t, vm, "sec_submissions_name")
	ticker := mustGetString(t, vm, "sec_submissions_ticker")
	exchange := mustGetString(t, vm, "sec_submissions_exchange")
	recentCount := mustGetInt(t, vm, "sec_submissions_recent_count")
	recentForm := mustGetString(t, vm, "sec_submissions_recent_form")
	fmt.Printf("sec_submissions_name=%q ticker=%q exchange=%q recent_count=%d recent_form=%q\n", name, ticker, exchange, recentCount, recentForm)
	if !strings.Contains(name, "Apple") || ticker != "AAPL" || exchange == "" || recentCount <= 0 || recentForm == "" {
		t.Fatalf("unexpected SEC submissions payload: name=%q ticker=%q exchange=%q recent_count=%d recent_form=%q", name, ticker, exchange, recentCount, recentForm)
	}
}
