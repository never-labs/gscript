package leia_test

import (
	"fmt"
	"strings"
	"testing"
)

func TestFinRobotLiveWikidataSPARQLCompanyEntityDataIntegration(t *testing.T) {
	vm := newFinRobotLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
wikidata_sparql_request_error := nil
wikidata_sparql_json_error := nil
wikidata_sparql_status := 0
wikidata_sparql_ok := false
wikidata_sparql_content_type := ""
wikidata_sparql_var_count := 0
wikidata_sparql_binding_count := 0
wikidata_sparql_entity_uri := ""
wikidata_sparql_company_label := ""
wikidata_sparql_country_label := ""
wikidata_sparql_inception := ""
wikidata_sparql_found_nasdaq := false
wikidata_sparql_exchange_label := ""

headers := {}
headers["User-Agent"] = "Mozilla/5.0 Leia FinRobot live data smoke contact=opensource@example.invalid"
headers["Accept"] = "application/sparql-results+json,application/json"

resp, err := net.get("https://query.wikidata.org/sparql?format=json&query=SELECT%20%3Fcompany%20%3FcompanyLabel%20%3FexchangeLabel%20%3FcountryLabel%20%3Finception%20WHERE%20%7B%20VALUES%20%3Fcompany%20%7B%20wd%3AQ312%20%7D%20OPTIONAL%20%7B%20%3Fcompany%20wdt%3AP414%20%3Fexchange.%20%7D%20OPTIONAL%20%7B%20%3Fcompany%20wdt%3AP17%20%3Fcountry.%20%7D%20OPTIONAL%20%7B%20%3Fcompany%20wdt%3AP571%20%3Finception.%20%7D%20SERVICE%20wikibase%3Alabel%20%7B%20bd%3AserviceParam%20wikibase%3Alanguage%20%22en%22.%20%7D%20%7D%20LIMIT%205", {
    headers: headers
    timeout: 30
})
if err != nil {
    wikidata_sparql_request_error = err
} else {
    wikidata_sparql_status = resp.status
    wikidata_sparql_ok = resp.ok
    if resp.headers != nil && resp.headers["Content-Type"] != nil {
        wikidata_sparql_content_type = resp.headers["Content-Type"]
    }
    if resp.ok {
        data, json_err := resp.json()
        if json_err != nil {
            wikidata_sparql_json_error = json_err
        } else {
            wikidata_sparql_var_count = #data.head.vars
            wikidata_sparql_binding_count = #data.results.bindings
            for _, row := range pairs(data.results.bindings) {
                if wikidata_sparql_entity_uri == "" {
                    wikidata_sparql_entity_uri = row.company.value
                    wikidata_sparql_company_label = row.companyLabel.value
                    wikidata_sparql_country_label = row.countryLabel.value
                    wikidata_sparql_inception = row.inception.value
                }
                if row.exchangeLabel != nil && string.contains(row.exchangeLabel.value, "Nasdaq") {
                    wikidata_sparql_found_nasdaq = true
                    wikidata_sparql_exchange_label = row.exchangeLabel.value
                }
            }
        }
    } else {
        wikidata_sparql_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	assertFinRobotPublicLiveDataOK(t, vm, "Wikidata SPARQL company entity", "wikidata_sparql_status", "wikidata_sparql_request_error", "wikidata_sparql_json_error", "wikidata_sparql_ok")
	contentType := mustGetString(t, vm, "wikidata_sparql_content_type")
	varCount := mustGetInt(t, vm, "wikidata_sparql_var_count")
	bindingCount := mustGetInt(t, vm, "wikidata_sparql_binding_count")
	entityURI := mustGetString(t, vm, "wikidata_sparql_entity_uri")
	companyLabel := mustGetString(t, vm, "wikidata_sparql_company_label")
	countryLabel := mustGetString(t, vm, "wikidata_sparql_country_label")
	inception := mustGetString(t, vm, "wikidata_sparql_inception")
	foundNasdaq := mustGetBool(t, vm, "wikidata_sparql_found_nasdaq")
	exchangeLabel := mustGetString(t, vm, "wikidata_sparql_exchange_label")

	fmt.Printf("wikidata_sparql vars=%d bindings=%d entity=%q label=%q country=%q exchange=%q inception=%q content_type=%q\n", varCount, bindingCount, entityURI, companyLabel, countryLabel, exchangeLabel, inception, contentType)
	if contentType == "" || (!strings.Contains(contentType, "sparql-results+json") && !strings.Contains(contentType, "application/json")) {
		t.Fatalf("Wikidata SPARQL Content-Type = %q, want SPARQL JSON results", contentType)
	}
	if varCount < 5 || bindingCount <= 0 {
		t.Fatalf("unexpected Wikidata SPARQL result shape: vars=%d bindings=%d", varCount, bindingCount)
	}
	if entityURI != "http://www.wikidata.org/entity/Q312" || companyLabel != "Apple Inc." || countryLabel != "United States" {
		t.Fatalf("unexpected Wikidata company identity: entity=%q label=%q country=%q", entityURI, companyLabel, countryLabel)
	}
	if !strings.HasPrefix(inception, "1976-04-01") || !foundNasdaq {
		t.Fatalf("unexpected Wikidata company metadata: inception=%q found_nasdaq=%v exchange=%q", inception, foundNasdaq, exchangeLabel)
	}
}
