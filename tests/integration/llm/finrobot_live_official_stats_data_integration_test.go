package leia_test

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestFinRobotLiveOfficialStatsDataIntegration(t *testing.T) {
	vm := newFinRobotLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
ons_cpih_request_error := nil
ons_cpih_json_error := nil
ons_cpih_status := 0
ons_cpih_ok := false
ons_cpih_content_type := ""
ons_cpih_count := 0
ons_cpih_total_observations := 0
ons_cpih_unit := ""
ons_cpih_dataset_version := ""
ons_cpih_geography := ""
ons_cpih_aggregate := ""
ons_cpih_time_id := ""
ons_cpih_time_label := ""
ons_cpih_value := ""

headers := {}
headers["User-Agent"] = "Mozilla/5.0 Leia FinRobot live data smoke contact=opensource@example.invalid"
headers["Accept"] = "application/json"

resp, err := net.get("https://api.beta.ons.gov.uk/v1/datasets/cpih01/editions/time-series/versions/6/observations?time=*&geography=K02000001&aggregate=cpih1dim1A0", {
    headers: headers
    timeout: 30
})
if err != nil {
    ons_cpih_request_error = err
} else {
    ons_cpih_status = resp.status
    ons_cpih_ok = resp.ok
    if resp.headers != nil && resp.headers["Content-Type"] != nil {
        ons_cpih_content_type = resp.headers["Content-Type"]
    }
    if resp.ok {
        data, json_err := resp.json()
        if json_err != nil {
            ons_cpih_json_error = json_err
        } else {
            ons_cpih_count = #data.observations
            ons_cpih_total_observations = data.total_observations
            ons_cpih_unit = data.unit_of_measure
            ons_cpih_dataset_version = data.links.version.id
            ons_cpih_geography = data.dimensions.geography.option.id
            ons_cpih_aggregate = data.dimensions.aggregate.option.id
            if ons_cpih_count > 0 {
                row := data.observations[1]
                ons_cpih_time_id = row.dimensions.Time.id
                ons_cpih_time_label = row.dimensions.Time.label
                ons_cpih_value = row.observation
            }
        }
    } else {
        ons_cpih_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	assertFinRobotPublicLiveDataOK(t, vm, "UK ONS CPIH official statistics", "ons_cpih_status", "ons_cpih_request_error", "ons_cpih_json_error", "ons_cpih_ok")
	contentType := mustGetString(t, vm, "ons_cpih_content_type")
	count := mustGetInt(t, vm, "ons_cpih_count")
	totalObservations := mustGetInt(t, vm, "ons_cpih_total_observations")
	unit := mustGetString(t, vm, "ons_cpih_unit")
	version := mustGetString(t, vm, "ons_cpih_dataset_version")
	geography := mustGetString(t, vm, "ons_cpih_geography")
	aggregate := mustGetString(t, vm, "ons_cpih_aggregate")
	timeID := mustGetString(t, vm, "ons_cpih_time_id")
	timeLabel := mustGetString(t, vm, "ons_cpih_time_label")
	valueText := mustGetString(t, vm, "ons_cpih_value")

	fmt.Printf("ons_cpih content_type=%q count=%d total=%d unit=%q version=%q geography=%q aggregate=%q time=%q value=%q\n", contentType, count, totalObservations, unit, version, geography, aggregate, timeLabel, valueText)
	if contentType == "" || !strings.Contains(contentType, "application/json") {
		t.Fatalf("ONS CPIH Content-Type = %q, want application/json", contentType)
	}
	if count <= 0 || totalObservations <= 0 {
		t.Fatalf("ONS CPIH observations count=%d total=%d, want non-empty observations", count, totalObservations)
	}
	if totalObservations < count {
		t.Fatalf("ONS CPIH total_observations=%d, want >= returned count=%d", totalObservations, count)
	}
	if unit == "" || !strings.Contains(unit, "Index") {
		t.Fatalf("ONS CPIH unit_of_measure = %q, want index unit", unit)
	}
	if version == "" || geography != "K02000001" || aggregate != "cpih1dim1A0" {
		t.Fatalf("unexpected ONS CPIH metadata: version=%q geography=%q aggregate=%q", version, geography, aggregate)
	}
	if timeID == "" || timeLabel == "" || timeID != timeLabel {
		t.Fatalf("unexpected ONS CPIH time fields: id=%q label=%q", timeID, timeLabel)
	}
	if !regexp.MustCompile(`^[A-Z][a-z]{2}-[0-9]{2}$`).MatchString(timeLabel) {
		t.Fatalf("ONS CPIH time label = %q, want Mon-YY format", timeLabel)
	}
	period, err := time.Parse("Jan-06", timeLabel)
	if err != nil {
		t.Fatalf("ONS CPIH time label = %q, want parseable Mon-YY period: %v", timeLabel, err)
	}
	if period.Year() < 1988 || period.Year() > time.Now().Year()+1 {
		t.Fatalf("ONS CPIH time label = %q, want plausible published period", timeLabel)
	}
	value, err := strconv.ParseFloat(valueText, 64)
	if err != nil {
		t.Fatalf("ONS CPIH observation = %q, want numeric index: %v", valueText, err)
	}
	if value <= 0 || value >= 300 {
		t.Fatalf("ONS CPIH observation = %f, want plausible index level", value)
	}
}
