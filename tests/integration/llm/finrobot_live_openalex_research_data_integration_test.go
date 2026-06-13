package leia_test

import (
	"fmt"
	"strings"
	"testing"
)

func TestFinRobotLiveOpenAlexFinanceResearchDataIntegration(t *testing.T) {
	vm := newFinRobotLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
openalex_research_request_error := nil
openalex_research_json_error := nil
openalex_research_status := 0
openalex_research_ok := false
openalex_research_meta_count := 0
openalex_research_results_count := 0
openalex_research_first_id := ""
openalex_research_first_display_name := ""
openalex_research_first_publication_year := 0
openalex_research_first_ids_openalex := ""

headers := {}
headers["User-Agent"] = "Mozilla/5.0 Leia FinRobot live data smoke contact=opensource@example.invalid"
headers["Accept"] = "application/json"

resp, err := net.get("https://api.openalex.org/works?search=large%20language%20models%20finance&filter=from_publication_date:2024-01-01&per-page=2&mailto=opensource@example.invalid", {
    headers: headers
    timeout: 30
})
if err != nil {
    openalex_research_request_error = err
} else {
    openalex_research_status = resp.status
    openalex_research_ok = resp.ok
    if resp.ok {
        data, json_err := resp.json()
        if json_err != nil {
            openalex_research_json_error = json_err
        } else {
            openalex_research_meta_count = data.meta.count
            openalex_research_results_count = #data.results
            if openalex_research_results_count > 0 {
                first := data.results[1]
                openalex_research_first_id = first.id
                openalex_research_first_display_name = first.display_name
                openalex_research_first_publication_year = first.publication_year
                openalex_research_first_ids_openalex = first.ids.openalex
            }
        }
    } else {
        openalex_research_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	assertFinRobotPublicLiveDataOK(t, vm, "OpenAlex finance research", "openalex_research_status", "openalex_research_request_error", "openalex_research_json_error", "openalex_research_ok")
	metaCount := mustGetInt(t, vm, "openalex_research_meta_count")
	resultsCount := mustGetInt(t, vm, "openalex_research_results_count")
	firstID := mustGetString(t, vm, "openalex_research_first_id")
	firstDisplayName := mustGetString(t, vm, "openalex_research_first_display_name")
	firstPublicationYear := mustGetInt(t, vm, "openalex_research_first_publication_year")
	firstIDsOpenAlex := mustGetString(t, vm, "openalex_research_first_ids_openalex")
	fmt.Printf("openalex_research meta_count=%d results_count=%d first_id=%q first_display_name=%q first_publication_year=%d first_ids_openalex=%q\n", metaCount, resultsCount, firstID, firstDisplayName, firstPublicationYear, firstIDsOpenAlex)
	if metaCount <= 0 || resultsCount <= 0 || !strings.HasPrefix(firstID, "https://openalex.org/W") || firstDisplayName == "" || firstPublicationYear < 2024 || firstIDsOpenAlex == "" {
		t.Fatalf("unexpected OpenAlex finance research payload: meta_count=%d results_count=%d first_id=%q first_display_name=%q first_publication_year=%d first_ids_openalex=%q", metaCount, resultsCount, firstID, firstDisplayName, firstPublicationYear, firstIDsOpenAlex)
	}
}
