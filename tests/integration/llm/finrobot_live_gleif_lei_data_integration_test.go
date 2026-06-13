package leia_test

import (
	"fmt"
	"strings"
	"testing"
)

func TestFinRobotLiveGLEIFLEIDataIntegration(t *testing.T) {
	vm := newFinRobotLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
gleif_lei_request_error := nil
gleif_lei_json_error := nil
gleif_lei_status := 0
gleif_lei_ok := false
gleif_lei_content_type := ""
gleif_lei_publish_date := ""
gleif_lei_record_type := ""
gleif_lei_id := ""
gleif_lei_value := ""
gleif_lei_legal_name := ""
gleif_lei_entity_status := ""
gleif_lei_jurisdiction := ""
gleif_lei_registered_as := ""
gleif_lei_registered_at := ""
gleif_lei_registration_status := ""
gleif_lei_corroboration_level := ""
gleif_lei_headquarters_city := ""
gleif_lei_headquarters_country := ""
gleif_lei_next_renewal_date := ""

headers := {}
headers["User-Agent"] = "Mozilla/5.0 Leia FinRobot live data smoke contact=opensource@example.invalid"
headers["Accept"] = "application/vnd.api+json,application/json"

resp, err := net.get("https://api.gleif.org/api/v1/lei-records/5493001KJTIIGC8Y1R12", {
    headers: headers
    timeout: 30
})
if err != nil {
    gleif_lei_request_error = err
} else {
    gleif_lei_status = resp.status
    gleif_lei_ok = resp.ok
    if resp.headers != nil && resp.headers["Content-Type"] != nil {
        gleif_lei_content_type = resp.headers["Content-Type"]
    }
    if resp.ok {
        data, json_err := resp.json()
        if json_err != nil {
            gleif_lei_json_error = json_err
        } else {
            gleif_lei_publish_date = data.meta.goldenCopy.publishDate
            gleif_lei_record_type = data.data.type
            gleif_lei_id = data.data.id
            gleif_lei_value = data.data.attributes.lei
            gleif_lei_legal_name = data.data.attributes.entity.legalName.name
            gleif_lei_entity_status = data.data.attributes.entity.status
            gleif_lei_jurisdiction = data.data.attributes.entity.jurisdiction
            gleif_lei_registered_as = data.data.attributes.entity.registeredAs
            gleif_lei_registered_at = data.data.attributes.entity.registeredAt.id
            gleif_lei_registration_status = data.data.attributes.registration.status
            gleif_lei_corroboration_level = data.data.attributes.registration.corroborationLevel
            gleif_lei_headquarters_city = data.data.attributes.entity.headquartersAddress.city
            gleif_lei_headquarters_country = data.data.attributes.entity.headquartersAddress.country
            gleif_lei_next_renewal_date = data.data.attributes.registration.nextRenewalDate
        }
    } else {
        gleif_lei_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	assertFinRobotPublicLiveDataOK(t, vm, "GLEIF LEI legal entity registry", "gleif_lei_status", "gleif_lei_request_error", "gleif_lei_json_error", "gleif_lei_ok")
	contentType := mustGetString(t, vm, "gleif_lei_content_type")
	publishDate := mustGetString(t, vm, "gleif_lei_publish_date")
	recordType := mustGetString(t, vm, "gleif_lei_record_type")
	id := mustGetString(t, vm, "gleif_lei_id")
	lei := mustGetString(t, vm, "gleif_lei_value")
	legalName := mustGetString(t, vm, "gleif_lei_legal_name")
	entityStatus := mustGetString(t, vm, "gleif_lei_entity_status")
	jurisdiction := mustGetString(t, vm, "gleif_lei_jurisdiction")
	registeredAs := mustGetString(t, vm, "gleif_lei_registered_as")
	registeredAt := mustGetString(t, vm, "gleif_lei_registered_at")
	registrationStatus := mustGetString(t, vm, "gleif_lei_registration_status")
	corroborationLevel := mustGetString(t, vm, "gleif_lei_corroboration_level")
	headquartersCity := mustGetString(t, vm, "gleif_lei_headquarters_city")
	headquartersCountry := mustGetString(t, vm, "gleif_lei_headquarters_country")
	nextRenewalDate := mustGetString(t, vm, "gleif_lei_next_renewal_date")

	fmt.Printf("gleif_lei id=%q name=%q status=%q registration=%q jurisdiction=%q registered_as=%q headquarters=%q/%q next_renewal=%q publish_date=%q content_type=%q\n", id, legalName, entityStatus, registrationStatus, jurisdiction, registeredAs, headquartersCity, headquartersCountry, nextRenewalDate, publishDate, contentType)
	if !strings.Contains(contentType, "application/vnd.api+json") {
		t.Fatalf("GLEIF LEI Content-Type = %q, want JSON:API", contentType)
	}
	if publishDate == "" || recordType != "lei-records" {
		t.Fatalf("unexpected GLEIF LEI envelope: publish_date=%q record_type=%q", publishDate, recordType)
	}
	if id != "5493001KJTIIGC8Y1R12" || lei != id || legalName != "Bloomberg Finance L.P." {
		t.Fatalf("unexpected GLEIF LEI identity: id=%q lei=%q legal_name=%q", id, lei, legalName)
	}
	if entityStatus != "ACTIVE" || registrationStatus != "ISSUED" || corroborationLevel != "FULLY_CORROBORATED" {
		t.Fatalf("unexpected GLEIF LEI status: entity=%q registration=%q corroboration=%q", entityStatus, registrationStatus, corroborationLevel)
	}
	if jurisdiction != "US-DE" || registeredAs != "4348344" || registeredAt != "RA000602" {
		t.Fatalf("unexpected GLEIF LEI registry identifiers: jurisdiction=%q registered_as=%q registered_at=%q", jurisdiction, registeredAs, registeredAt)
	}
	if headquartersCity != "New York" || headquartersCountry != "US" || nextRenewalDate == "" {
		t.Fatalf("unexpected GLEIF LEI operational metadata: city=%q country=%q next_renewal=%q", headquartersCity, headquartersCountry, nextRenewalDate)
	}
}
