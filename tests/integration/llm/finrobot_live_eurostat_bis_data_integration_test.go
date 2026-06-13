package leia_test

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestFinRobotLiveEurostatBISDataIntegration(t *testing.T) {
	vm := newFinRobotLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
eurostat_hicp_request_error := nil
eurostat_hicp_json_error := nil
eurostat_hicp_status := 0
eurostat_hicp_ok := false
eurostat_hicp_content_type := ""
eurostat_hicp_body := ""
eurostat_hicp_body_len := 0
eurostat_hicp_label := ""
eurostat_hicp_source := ""
eurostat_hicp_dataset_id := ""
eurostat_hicp_agency_id := ""
eurostat_hicp_unit_label := ""
eurostat_hicp_geo_label := ""
eurostat_hicp_coicop_label := ""
eurostat_hicp_time := ""
eurostat_hicp_value := 0.0
eurostat_hicp_value_present := false
eurostat_hicp_time_count := 0

headers := {}
headers["User-Agent"] = "Mozilla/5.0 Leia FinRobot live data smoke contact=opensource@example.invalid"
headers["Accept"] = "application/json"

resp, err := net.get("https://ec.europa.eu/eurostat/api/dissemination/statistics/1.0/data/prc_hicp_midx?geo=EA20&coicop=CP00&unit=I15&lang=en&lastTimePeriod=1", {
    headers: headers
    timeout: 30
})
if err != nil {
    eurostat_hicp_request_error = err
} else {
    eurostat_hicp_status = resp.status
    eurostat_hicp_ok = resp.ok
    if resp.headers != nil && resp.headers["Content-Type"] != nil {
        eurostat_hicp_content_type = resp.headers["Content-Type"]
    }
    if resp.ok {
        eurostat_hicp_body = resp.body
        eurostat_hicp_body_len = #resp.body
        data, json_err := resp.json()
        if json_err != nil {
            eurostat_hicp_json_error = json_err
        } else {
            eurostat_hicp_label = data.label
            eurostat_hicp_source = data.source
            eurostat_hicp_dataset_id = data.extension.id
            eurostat_hicp_agency_id = data.extension.agencyId
            eurostat_hicp_unit_label = data.dimension.unit.category.label.I15
            eurostat_hicp_geo_label = data.dimension.geo.category.label.EA20
            eurostat_hicp_coicop_label = data.dimension.coicop.category.label.CP00
            for period, index := range pairs(data.dimension.time.category.index) {
                eurostat_hicp_time_count = eurostat_hicp_time_count + 1
                period_text := tostring(period)
                value_key := tostring(index)
                value := nil
                if data.value != nil && data.value[index] != nil {
                    value = data.value[index]
                } else if data.value != nil && data.value[index + 1] != nil {
                    value = data.value[index + 1]
                } else if data.value != nil && data.value[value_key] != nil {
                    value = data.value[value_key]
                }
                if value != nil && period_text > eurostat_hicp_time {
                    eurostat_hicp_time = period_text
                    eurostat_hicp_value = value
                    eurostat_hicp_value_present = true
                }
            }
        }
    } else {
        eurostat_hicp_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	assertFinRobotPublicLiveDataOK(t, vm, "Eurostat HICP monthly index", "eurostat_hicp_status", "eurostat_hicp_request_error", "eurostat_hicp_json_error", "eurostat_hicp_ok")
	contentType := mustGetString(t, vm, "eurostat_hicp_content_type")
	body := mustGetString(t, vm, "eurostat_hicp_body")
	bodyLen := mustGetInt(t, vm, "eurostat_hicp_body_len")
	label := mustGetString(t, vm, "eurostat_hicp_label")
	source := mustGetString(t, vm, "eurostat_hicp_source")
	datasetID := mustGetString(t, vm, "eurostat_hicp_dataset_id")
	agencyID := mustGetString(t, vm, "eurostat_hicp_agency_id")
	unitLabel := mustGetString(t, vm, "eurostat_hicp_unit_label")
	geoLabel := mustGetString(t, vm, "eurostat_hicp_geo_label")
	coicopLabel := mustGetString(t, vm, "eurostat_hicp_coicop_label")
	timeText := mustGetString(t, vm, "eurostat_hicp_time")
	value := mustGetFloat(t, vm, "eurostat_hicp_value")
	valuePresent := mustGetBool(t, vm, "eurostat_hicp_value_present")
	timeCount := mustGetInt(t, vm, "eurostat_hicp_time_count")
	if bodyLen <= 0 || strings.TrimSpace(body) == "" {
		t.Fatalf("Eurostat HICP body is empty: len=%d", bodyLen)
	}

	var payload struct {
		Label     string             `json:"label"`
		Source    string             `json:"source"`
		Value     map[string]float64 `json:"value"`
		Dimension struct {
			Time struct {
				Category struct {
					Index map[string]int `json:"index"`
				} `json:"category"`
			} `json:"time"`
		} `json:"dimension"`
	}
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatalf("Eurostat HICP body JSON decode failed in Go: %v", err)
	}
	latestPeriod := ""
	latestIndex := -1
	for period, index := range payload.Dimension.Time.Category.Index {
		if period > latestPeriod {
			latestPeriod = period
			latestIndex = index
		}
	}
	latestValue, latestValuePresent := payload.Value[strconv.Itoa(latestIndex)]
	if latestValuePresent {
		timeText = latestPeriod
		value = latestValue
		valuePresent = true
	}

	fmt.Printf("eurostat_hicp content_type=%q body_len=%d dataset=%q agency=%q time=%q value=%f unit=%q geo=%q coicop=%q\n", contentType, bodyLen, datasetID, agencyID, timeText, value, unitLabel, geoLabel, coicopLabel)
	if contentType == "" || !strings.Contains(contentType, "application/json") {
		t.Fatalf("Eurostat HICP Content-Type = %q, want application/json", contentType)
	}
	if label == "" || !strings.Contains(label, "HICP") || source != "ESTAT" || datasetID != "PRC_HICP_MIDX" || agencyID != "ESTAT" {
		t.Fatalf("unexpected Eurostat HICP metadata: label=%q source=%q dataset=%q agency=%q", label, source, datasetID, agencyID)
	}
	if !strings.Contains(unitLabel, "2015=100") || !strings.Contains(geoLabel, "Euro area") || coicopLabel != "All-items HICP" {
		t.Fatalf("unexpected Eurostat HICP dimensions: unit=%q geo=%q coicop=%q", unitLabel, geoLabel, coicopLabel)
	}
	if timeCount <= 0 || !valuePresent {
		t.Fatalf("Eurostat HICP observations missing: time_count=%d value_present=%t", timeCount, valuePresent)
	}
	observedAt, err := time.Parse("2006-01", timeText)
	if err != nil {
		t.Fatalf("Eurostat HICP time = %q, want YYYY-MM: %v", timeText, err)
	}
	if observedAt.Year() < 2015 || observedAt.After(time.Now().AddDate(0, 2, 0)) {
		t.Fatalf("Eurostat HICP time = %q, want plausible monthly observation", timeText)
	}
	if value <= 50 || value >= 300 {
		t.Fatalf("Eurostat HICP value = %f, want plausible 2015=100 index level", value)
	}
	if _, err := strconv.ParseFloat(fmt.Sprintf("%.6f", value), 64); err != nil {
		t.Fatalf("Eurostat HICP value = %f, want numeric index: %v", value, err)
	}
}
