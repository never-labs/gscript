package leia_test

import (
	"fmt"
	"strings"
	"testing"
)

func TestFinRobotLiveBLSCPIDataIntegration(t *testing.T) {
	vm := newFinRobotLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
bls_cpi_request_error := nil
bls_cpi_json_error := nil
bls_cpi_status_code := 0
bls_cpi_ok := false
bls_cpi_status := ""
bls_cpi_series_count := 0
bls_cpi_series_id := ""
bls_cpi_observation_count := 0
bls_cpi_year := ""
bls_cpi_period := ""
bls_cpi_period_name := ""
bls_cpi_value := ""
bls_cpi_latest := ""

headers := {}
headers["User-Agent"] = "Mozilla/5.0 Leia FinRobot live data smoke"
headers["Accept"] = "application/json"

resp, err := net.get("https://api.bls.gov/publicAPI/v2/timeseries/data/CUUR0000SA0?latest=true", {
    headers: headers
    timeout: 30
})
if err != nil {
    bls_cpi_request_error = err
} else {
    bls_cpi_status_code = resp.status
    bls_cpi_ok = resp.ok
    if resp.ok {
        data, json_err := resp.json()
        if json_err != nil {
            bls_cpi_json_error = json_err
        } else {
            bls_cpi_status = data.status
            series := data.Results.series
            bls_cpi_series_count = #series
            if bls_cpi_series_count > 0 {
                row := series[1]
                bls_cpi_series_id = row.seriesID
                observations := row.data
                bls_cpi_observation_count = #observations
                if bls_cpi_observation_count > 0 {
                    obs := observations[1]
                    bls_cpi_year = obs.year
                    bls_cpi_period = obs.period
                    bls_cpi_period_name = obs.periodName
                    bls_cpi_value = obs.value
                    bls_cpi_latest = obs.latest
                }
            }
        }
    } else {
        bls_cpi_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	assertFinRobotPublicLiveDataOK(t, vm, "BLS CPI", "bls_cpi_status_code", "bls_cpi_request_error", "bls_cpi_json_error", "bls_cpi_ok")
	status := mustGetString(t, vm, "bls_cpi_status")
	seriesCount := mustGetInt(t, vm, "bls_cpi_series_count")
	seriesID := mustGetString(t, vm, "bls_cpi_series_id")
	observationCount := mustGetInt(t, vm, "bls_cpi_observation_count")
	year := mustGetString(t, vm, "bls_cpi_year")
	period := mustGetString(t, vm, "bls_cpi_period")
	periodName := mustGetString(t, vm, "bls_cpi_period_name")
	value := mustGetString(t, vm, "bls_cpi_value")
	latest := mustGetString(t, vm, "bls_cpi_latest")
	fmt.Printf("bls_cpi status=%q series=%d series_id=%q observations=%d year=%q period=%q period_name=%q value=%q latest=%q\n", status, seriesCount, seriesID, observationCount, year, period, periodName, value, latest)
	if status != "REQUEST_SUCCEEDED" || seriesCount <= 0 || seriesID != "CUUR0000SA0" || observationCount <= 0 || year < "2020" || !strings.HasPrefix(period, "M") || periodName == "" || value == "" {
		t.Fatalf("unexpected BLS CPI payload: status=%q series=%d series_id=%q observations=%d year=%q period=%q period_name=%q value=%q latest=%q", status, seriesCount, seriesID, observationCount, year, period, periodName, value, latest)
	}
}
