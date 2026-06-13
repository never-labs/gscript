package leia_test

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestFinRobotLiveFederalRegisterDataIntegration(t *testing.T) {
	vm := newFinRobotLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
federal_register_request_error := nil
federal_register_json_error := nil
federal_register_status := 0
federal_register_ok := false
federal_register_count := 0
federal_register_total_pages := 0
federal_register_results_count := 0
federal_register_document_number := ""
federal_register_title := ""
federal_register_type := ""
federal_register_publication_date := ""
federal_register_html_url := ""
federal_register_agencies_count := 0
federal_register_agency_name := ""
federal_register_agency_raw_name := ""

headers := {}
headers["User-Agent"] = "Mozilla/5.0 Leia FinRobot live data smoke contact=opensource@example.invalid"
headers["Accept"] = "application/json"

resp, err := net.get("https://www.federalregister.gov/api/v1/documents.json?per_page=1&order=newest", {
    headers: headers
    timeout: 30
})
if err != nil {
    federal_register_request_error = err
} else {
    federal_register_status = resp.status
    federal_register_ok = resp.ok
    if resp.ok {
        data, json_err := resp.json()
        if json_err != nil {
            federal_register_json_error = json_err
        } else {
            federal_register_count = data.count
            federal_register_total_pages = data.total_pages
            federal_register_results_count = #data.results
            if federal_register_results_count > 0 {
                doc := data.results[1]
                federal_register_document_number = doc.document_number
                federal_register_title = doc.title
                federal_register_type = doc.type
                federal_register_publication_date = doc.publication_date
                federal_register_html_url = doc.html_url
                if doc.agencies != nil {
                    federal_register_agencies_count = #doc.agencies
                    if federal_register_agencies_count > 0 {
                        agency := doc.agencies[1]
                        if agency.name != nil {
                            federal_register_agency_name = agency.name
                        }
                        if agency.raw_name != nil {
                            federal_register_agency_raw_name = agency.raw_name
                        }
                    }
                }
            }
        }
    } else {
        federal_register_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	assertFinRobotPublicLiveDataOK(t, vm, "Federal Register documents", "federal_register_status", "federal_register_request_error", "federal_register_json_error", "federal_register_ok")
	count := mustGetInt(t, vm, "federal_register_count")
	totalPages := mustGetInt(t, vm, "federal_register_total_pages")
	resultsCount := mustGetInt(t, vm, "federal_register_results_count")
	documentNumber := mustGetString(t, vm, "federal_register_document_number")
	title := mustGetString(t, vm, "federal_register_title")
	documentType := mustGetString(t, vm, "federal_register_type")
	publicationDate := mustGetString(t, vm, "federal_register_publication_date")
	htmlURL := mustGetString(t, vm, "federal_register_html_url")
	agenciesCount := mustGetInt(t, vm, "federal_register_agencies_count")
	agencyName := mustGetString(t, vm, "federal_register_agency_name")
	agencyRawName := mustGetString(t, vm, "federal_register_agency_raw_name")

	fmt.Printf("federal_register count=%d total_pages=%d results=%d document_number=%q type=%q publication_date=%q agency=%q raw_agency=%q title=%q\n", count, totalPages, resultsCount, documentNumber, documentType, publicationDate, agencyName, agencyRawName, title)
	if count <= 0 || totalPages <= 0 || resultsCount <= 0 {
		t.Fatalf("unexpected Federal Register result counts: count=%d total_pages=%d results=%d", count, totalPages, resultsCount)
	}
	if documentNumber == "" || title == "" || documentType == "" || publicationDate == "" || htmlURL == "" {
		t.Fatalf("unexpected Federal Register document payload: document_number=%q title=%q type=%q publication_date=%q html_url=%q", documentNumber, title, documentType, publicationDate, htmlURL)
	}
	if !strings.HasPrefix(htmlURL, "https://www.federalregister.gov/documents/") {
		t.Fatalf("Federal Register html_url = %q, want official document URL", htmlURL)
	}
	if agenciesCount <= 0 || (agencyName == "" && agencyRawName == "") {
		t.Fatalf("unexpected Federal Register agencies payload: count=%d agency=%q raw_agency=%q", agenciesCount, agencyName, agencyRawName)
	}
	publishedAt, err := time.Parse(time.DateOnly, publicationDate)
	if err != nil {
		t.Fatalf("Federal Register publication_date = %q, want YYYY-MM-DD: %v", publicationDate, err)
	}
	if publishedAt.Year() < 1994 || publishedAt.After(time.Now().AddDate(0, 0, 14)) {
		t.Fatalf("Federal Register publication_date = %s, want plausible regulatory notice date", publicationDate)
	}
}
