package leia_test

import (
	"fmt"
	"strings"
	"testing"
)

func TestFinRobotLiveSECCompanyTickersDataIntegration(t *testing.T) {
	vm := newFinRobotLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
sec_company_tickers_request_error := nil
sec_company_tickers_json_error := nil
sec_company_tickers_status := 0
sec_company_tickers_ok := false
sec_company_tickers_entry_key := ""
sec_company_tickers_cik := 0
sec_company_tickers_ticker := ""
sec_company_tickers_title := ""

headers := {}
headers["User-Agent"] = os.getenv("LEIA_SEC_USER_AGENT")
headers["Accept"] = "application/json"

resp, err := net.get("https://www.sec.gov/files/company_tickers.json", {
    headers: headers
    timeout: 30
})
if err != nil {
    sec_company_tickers_request_error = err
} else {
    sec_company_tickers_status = resp.status
    sec_company_tickers_ok = resp.ok
    if resp.ok {
        data, json_err := resp.json()
        if json_err != nil {
            sec_company_tickers_json_error = json_err
        } else {
            for key, company := range pairs(data) {
                if sec_company_tickers_ticker == "" && company.ticker == "AAPL" {
                    sec_company_tickers_entry_key = tostring(key)
                    sec_company_tickers_cik = company.cik_str
                    sec_company_tickers_ticker = company.ticker
                    sec_company_tickers_title = company.title
                }
            }
        }
    } else {
        sec_company_tickers_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	status := mustGetInt(t, vm, "sec_company_tickers_status")
	skipUnavailableFinRobotLiveData(t, "SEC company tickers", status, getOrNil(t, vm, "sec_company_tickers_request_error"))
	if status != 200 {
		t.Fatalf("SEC company tickers status = %d, want 200", status)
	}
	if got := getOrNil(t, vm, "sec_company_tickers_json_error"); got != nil {
		t.Fatalf("SEC company tickers JSON decode failed: %v", got)
	}
	if ok := mustGetBool(t, vm, "sec_company_tickers_ok"); !ok {
		t.Fatalf("SEC company tickers ok = false")
	}
	key := mustGetString(t, vm, "sec_company_tickers_entry_key")
	cik := mustGetInt(t, vm, "sec_company_tickers_cik")
	ticker := mustGetString(t, vm, "sec_company_tickers_ticker")
	title := mustGetString(t, vm, "sec_company_tickers_title")
	fmt.Printf("sec_company_tickers_key=%q cik=%d ticker=%q title=%q\n", key, cik, ticker, title)
	if key == "" || ticker != "AAPL" || !strings.Contains(title, "Apple") || cik != 320193 {
		t.Fatalf("unexpected SEC company tickers payload: key=%q cik=%d ticker=%q title=%q", key, cik, ticker, title)
	}
}

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

func TestFinRobotLiveSECFilingDocumentDataIntegration(t *testing.T) {
	vm := newFinRobotLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
sec_filing_doc_submissions_error := nil
sec_filing_doc_submissions_json_error := nil
sec_filing_doc_submissions_status := 0
sec_filing_doc_submissions_ok := false
sec_filing_doc_company := ""
sec_filing_doc_ticker := ""
sec_filing_doc_recent_count := 0
sec_filing_doc_form := ""
sec_filing_doc_accession := ""
sec_filing_doc_accession_path := ""
sec_filing_doc_primary_document := ""
sec_filing_doc_filing_date := ""
sec_filing_doc_url := ""
sec_filing_doc_request_error := nil
sec_filing_doc_status := 0
sec_filing_doc_ok := false
sec_filing_doc_content_type := ""
sec_filing_doc_body := ""

headers := {}
headers["User-Agent"] = os.getenv("LEIA_SEC_USER_AGENT")
headers["Accept"] = "application/json"

resp, err := net.get("https://data.sec.gov/submissions/CIK0000320193.json", {
    headers: headers
    timeout: 30
})
if err != nil {
    sec_filing_doc_submissions_error = err
} else {
    sec_filing_doc_submissions_status = resp.status
    sec_filing_doc_submissions_ok = resp.ok
    if resp.ok {
        data, json_err := resp.json()
        if json_err != nil {
            sec_filing_doc_submissions_json_error = json_err
        } else {
            sec_filing_doc_company = data.name
            sec_filing_doc_ticker = data.tickers[1]
            sec_filing_doc_recent_count = #data.filings.recent.accessionNumber
            for i := 1; i <= sec_filing_doc_recent_count; i++ {
                form := data.filings.recent.form[i]
                accession := data.filings.recent.accessionNumber[i]
                primary := data.filings.recent.primaryDocument[i]
                filing_date := data.filings.recent.filingDate[i]
                if sec_filing_doc_accession == "" && (form == "10-K" || form == "10-Q") && accession != "" && primary != "" && filing_date != "" && (string.hasSuffix(primary, ".htm") || string.hasSuffix(primary, ".html")) {
                    sec_filing_doc_form = form
                    sec_filing_doc_accession = accession
                    sec_filing_doc_accession_path = string.replaceAll(accession, "-", "")
                    sec_filing_doc_primary_document = primary
                    sec_filing_doc_filing_date = filing_date
                    sec_filing_doc_url = "https://www.sec.gov/Archives/edgar/data/320193/" .. sec_filing_doc_accession_path .. "/" .. primary
                }
            }
        }
    } else {
        sec_filing_doc_submissions_error = resp.statusText
    }
}

if sec_filing_doc_url != "" {
    headers["Accept"] = "text/html,application/xhtml+xml"
    doc_resp, doc_err := net.get(sec_filing_doc_url, {
        headers: headers
        timeout: 30
    })
    if doc_err != nil {
        sec_filing_doc_request_error = doc_err
    } else {
        sec_filing_doc_status = doc_resp.status
        sec_filing_doc_ok = doc_resp.ok
        if doc_resp.ok {
            sec_filing_doc_body = doc_resp.body
            sec_filing_doc_content_type = doc_resp.headers["Content-Type"]
        } else {
            sec_filing_doc_request_error = doc_resp.statusText
        }
    }
}
`); err != nil {
		t.Fatal(err)
	}

	submissionsStatus := mustGetInt(t, vm, "sec_filing_doc_submissions_status")
	skipUnavailableFinRobotLiveData(t, "SEC filing submissions", submissionsStatus, getOrNil(t, vm, "sec_filing_doc_submissions_error"))
	if submissionsStatus != 200 {
		t.Fatalf("SEC filing submissions status = %d, want 200", submissionsStatus)
	}
	if got := getOrNil(t, vm, "sec_filing_doc_submissions_json_error"); got != nil {
		t.Fatalf("SEC filing submissions JSON decode failed: %v", got)
	}
	if ok := mustGetBool(t, vm, "sec_filing_doc_submissions_ok"); !ok {
		t.Fatalf("SEC filing submissions ok = false")
	}
	company := mustGetString(t, vm, "sec_filing_doc_company")
	ticker := mustGetString(t, vm, "sec_filing_doc_ticker")
	recentCount := mustGetInt(t, vm, "sec_filing_doc_recent_count")
	form := mustGetString(t, vm, "sec_filing_doc_form")
	accession := mustGetString(t, vm, "sec_filing_doc_accession")
	accessionPath := mustGetString(t, vm, "sec_filing_doc_accession_path")
	primaryDocument := mustGetString(t, vm, "sec_filing_doc_primary_document")
	filingDate := mustGetString(t, vm, "sec_filing_doc_filing_date")
	url := mustGetString(t, vm, "sec_filing_doc_url")
	if !strings.Contains(company, "Apple") || ticker != "AAPL" || recentCount <= 0 || (form != "10-K" && form != "10-Q") || accession == "" || accessionPath == "" || primaryDocument == "" || filingDate == "" || url == "" {
		t.Fatalf("unexpected SEC filing selection: company=%q ticker=%q recent_count=%d form=%q accession=%q accession_path=%q primary=%q filing_date=%q url=%q", company, ticker, recentCount, form, accession, accessionPath, primaryDocument, filingDate, url)
	}

	status := mustGetInt(t, vm, "sec_filing_doc_status")
	skipUnavailableFinRobotLiveData(t, "SEC filing document", status, getOrNil(t, vm, "sec_filing_doc_request_error"))
	if status != 200 {
		t.Fatalf("SEC filing document status = %d, want 200", status)
	}
	if ok := mustGetBool(t, vm, "sec_filing_doc_ok"); !ok {
		t.Fatalf("SEC filing document ok = false")
	}
	contentType := mustGetString(t, vm, "sec_filing_doc_content_type")
	body := mustGetString(t, vm, "sec_filing_doc_body")
	fmt.Printf("sec_filing_doc_form=%q accession=%q primary=%q filing_date=%q content_type=%q body_bytes=%d\n", form, accession, primaryDocument, filingDate, contentType, len(body))
	if !strings.Contains(contentType, "text/html") {
		t.Fatalf("SEC filing document content type = %q, want text/html", contentType)
	}
	if len(body) < 1024 || !strings.Contains(body, "Apple Inc.") || !strings.Contains(body, form) {
		t.Fatalf("unexpected SEC filing document payload: content_type=%q body_bytes=%d", contentType, len(body))
	}
}
