package leia_test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestFinRobotLiveCDCPublicHealthDataIntegration(t *testing.T) {
	vm := newFinRobotLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
cdc_public_health_request_error := nil
cdc_public_health_json_error := nil
cdc_public_health_status := 0
cdc_public_health_ok := false
cdc_public_health_content_type := ""
cdc_public_health_count := 0
cdc_public_health_year_start := ""
cdc_public_health_year_end := ""
cdc_public_health_location_abbr := ""
cdc_public_health_location_desc := ""
cdc_public_health_topic := ""
cdc_public_health_question := ""
cdc_public_health_data_value_type := ""
cdc_public_health_data_value := ""
cdc_public_health_data_value_alt := ""
cdc_public_health_data_source := ""

headers := {}
headers["User-Agent"] = "Mozilla/5.0 Leia FinRobot live data smoke contact=opensource@example.invalid"
headers["Accept"] = "application/json"

resp, err := net.get("https://data.cdc.gov/resource/hksd-2xuw.json?%24select=yearstart%2Cyearend%2Clocationabbr%2Clocationdesc%2Ctopic%2Cquestion%2Cdatavaluetype%2Cdatavalue%2Cdatavaluealt%2Cdatasource&%24where=datavalue%20IS%20NOT%20NULL%20AND%20topic%3D%27Tobacco%27&%24order=yearstart%20DESC&%24limit=5", {
    headers: headers
    timeout: 30
})
if err != nil {
    cdc_public_health_request_error = err
} else {
    cdc_public_health_status = resp.status
    cdc_public_health_ok = resp.ok
    if resp.headers != nil && resp.headers["Content-Type"] != nil {
        cdc_public_health_content_type = resp.headers["Content-Type"]
    }
    if resp.ok {
        data, json_err := resp.json()
        if json_err != nil {
            cdc_public_health_json_error = json_err
        } else {
            cdc_public_health_count = #data
            if cdc_public_health_count > 0 {
                row := data[1]
                cdc_public_health_year_start = row.yearstart
                cdc_public_health_year_end = row.yearend
                cdc_public_health_location_abbr = row.locationabbr
                cdc_public_health_location_desc = row.locationdesc
                cdc_public_health_topic = row.topic
                cdc_public_health_question = row.question
                cdc_public_health_data_value_type = row.datavaluetype
                cdc_public_health_data_value = row.datavalue
                cdc_public_health_data_value_alt = row.datavaluealt
                cdc_public_health_data_source = row.datasource
            }
        }
    } else {
        cdc_public_health_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	assertFinRobotPublicLiveDataOK(t, vm, "CDC Data.CDC.gov chronic disease indicators", "cdc_public_health_status", "cdc_public_health_request_error", "cdc_public_health_json_error", "cdc_public_health_ok")
	contentType := mustGetString(t, vm, "cdc_public_health_content_type")
	count := mustGetInt(t, vm, "cdc_public_health_count")
	yearStart := mustGetString(t, vm, "cdc_public_health_year_start")
	yearEnd := mustGetString(t, vm, "cdc_public_health_year_end")
	locationAbbr := mustGetString(t, vm, "cdc_public_health_location_abbr")
	locationDesc := mustGetString(t, vm, "cdc_public_health_location_desc")
	topic := mustGetString(t, vm, "cdc_public_health_topic")
	question := mustGetString(t, vm, "cdc_public_health_question")
	dataValueType := mustGetString(t, vm, "cdc_public_health_data_value_type")
	dataValueText := mustGetString(t, vm, "cdc_public_health_data_value")
	dataValueAltText := mustGetString(t, vm, "cdc_public_health_data_value_alt")
	dataSource := mustGetString(t, vm, "cdc_public_health_data_source")

	fmt.Printf("cdc_public_health content_type=%q count=%d year=%q-%q location=%q/%q topic=%q question=%q value_type=%q value=%q value_alt=%q source=%q\n", contentType, count, yearStart, yearEnd, locationAbbr, locationDesc, topic, question, dataValueType, dataValueText, dataValueAltText, dataSource)
	if contentType == "" || !strings.Contains(contentType, "application/json") {
		t.Fatalf("CDC Data.CDC.gov Content-Type = %q, want application/json", contentType)
	}
	if count <= 0 {
		t.Fatalf("CDC Data.CDC.gov row count = %d, want > 0", count)
	}
	if locationAbbr == "" || locationDesc == "" || topic != "Tobacco" || question == "" || dataValueType == "" || dataSource == "" {
		t.Fatalf("unexpected CDC Data.CDC.gov key fields: location_abbr=%q location_desc=%q topic=%q question=%q value_type=%q source=%q", locationAbbr, locationDesc, topic, question, dataValueType, dataSource)
	}

	startYear, err := strconv.Atoi(yearStart)
	if err != nil {
		t.Fatalf("CDC Data.CDC.gov yearstart = %q, want numeric year: %v", yearStart, err)
	}
	endYear, err := strconv.Atoi(yearEnd)
	if err != nil {
		t.Fatalf("CDC Data.CDC.gov yearend = %q, want numeric year: %v", yearEnd, err)
	}
	currentYear := time.Now().Year()
	if startYear < 2000 || endYear < startYear || endYear > currentYear+1 {
		t.Fatalf("CDC Data.CDC.gov year range = %d-%d, want plausible public health observation years", startYear, endYear)
	}

	dataValue, err := strconv.ParseFloat(dataValueAltText, 64)
	if err != nil {
		t.Fatalf("CDC Data.CDC.gov datavaluealt = %q, want numeric value: %v", dataValueAltText, err)
	}
	if dataValue <= 0 || dataValue > 1000000000 {
		t.Fatalf("CDC Data.CDC.gov datavaluealt = %f, want plausible positive public health value", dataValue)
	}
}
