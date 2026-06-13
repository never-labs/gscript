package leia_test

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestFinRobotLiveUKCarbonIntensityDataIntegration(t *testing.T) {
	vm := newFinRobotLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
carbon_intensity_request_error := nil
carbon_intensity_json_error := nil
carbon_intensity_status := 0
carbon_intensity_ok := false
carbon_intensity_count := 0
carbon_intensity_from := ""
carbon_intensity_to := ""
carbon_intensity_forecast := 0
carbon_intensity_actual := 0
carbon_intensity_index := ""

headers := {}
headers["User-Agent"] = "Mozilla/5.0 Leia FinRobot live data smoke"
headers["Accept"] = "application/json"

resp, err := net.get("https://api.carbonintensity.org.uk/intensity", {
    headers: headers
    timeout: 30
})
if err != nil {
    carbon_intensity_request_error = err
} else {
    carbon_intensity_status = resp.status
    carbon_intensity_ok = resp.ok
    if resp.ok {
        data, json_err := resp.json()
        if json_err != nil {
            carbon_intensity_json_error = json_err
        } else {
            if data.data != nil {
                carbon_intensity_count = #data.data
                if carbon_intensity_count > 0 {
                    row := data.data[1]
                    carbon_intensity_from = row.from
                    carbon_intensity_to = row.to
                    if row.intensity != nil {
                        carbon_intensity_forecast = row.intensity.forecast
                        if row.intensity.actual != nil {
                            carbon_intensity_actual = row.intensity.actual
                        }
                        carbon_intensity_index = row.intensity.index
                    }
                }
            }
        }
    } else {
        carbon_intensity_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	assertFinRobotPublicLiveDataOK(t, vm, "UK Carbon Intensity", "carbon_intensity_status", "carbon_intensity_request_error", "carbon_intensity_json_error", "carbon_intensity_ok")
	count := mustGetInt(t, vm, "carbon_intensity_count")
	from := mustGetString(t, vm, "carbon_intensity_from")
	to := mustGetString(t, vm, "carbon_intensity_to")
	forecast := mustGetInt(t, vm, "carbon_intensity_forecast")
	actual := mustGetInt(t, vm, "carbon_intensity_actual")
	index := mustGetString(t, vm, "carbon_intensity_index")

	fmt.Printf("carbon_intensity count=%d from=%q to=%q forecast=%d actual=%d index=%q\n", count, from, to, forecast, actual, index)
	if count <= 0 || from == "" || to == "" || index == "" {
		t.Fatalf("unexpected UK Carbon Intensity payload: count=%d from=%q to=%q index=%q", count, from, to, index)
	}
	if forecast < 0 || forecast > 1000 || actual < 0 || actual > 1000 {
		t.Fatalf("unexpected UK Carbon Intensity values: forecast=%d actual=%d", forecast, actual)
	}
	if !strings.Contains(from, "T") || !strings.Contains(to, "T") {
		t.Fatalf("UK Carbon Intensity time range is not ISO-like: from=%q to=%q", from, to)
	}
	parsedFrom, err := time.Parse("2006-01-02T15:04Z", from)
	if err != nil {
		t.Fatalf("UK Carbon Intensity from=%q, want YYYY-MM-DDTHH:MMZ: %v", from, err)
	}
	if parsedFrom.Before(time.Now().AddDate(0, 0, -2)) || parsedFrom.After(time.Now().AddDate(0, 0, 2)) {
		t.Fatalf("UK Carbon Intensity from=%s, want current live interval", from)
	}
}
