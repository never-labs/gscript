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
            if data.Results != nil && data.Results.series != nil {
                series := data.Results.series
                bls_cpi_series_count = #series
                if bls_cpi_series_count > 0 {
                    row := series[1]
                    bls_cpi_series_id = row.seriesID
                    observations := row.data
                    if observations != nil {
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
	if status != "REQUEST_SUCCEEDED" && seriesCount == 0 {
		t.Skipf("BLS CPI business response unavailable: status=%q", status)
	}
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

func TestFinRobotLiveBLSUnemploymentRateDataIntegration(t *testing.T) {
	vm := newFinRobotLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
bls_unemployment_request_error := nil
bls_unemployment_json_error := nil
bls_unemployment_status_code := 0
bls_unemployment_ok := false
bls_unemployment_status := ""
bls_unemployment_series_count := 0
bls_unemployment_series_id := ""
bls_unemployment_observation_count := 0
bls_unemployment_year := ""
bls_unemployment_period := ""
bls_unemployment_period_name := ""
bls_unemployment_value := ""
bls_unemployment_latest := ""

headers := {}
headers["User-Agent"] = "Mozilla/5.0 Leia FinRobot live data smoke"
headers["Accept"] = "application/json"

resp, err := net.get("https://api.bls.gov/publicAPI/v2/timeseries/data/LNS14000000?latest=true", {
    headers: headers
    timeout: 30
})
if err != nil {
    bls_unemployment_request_error = err
} else {
    bls_unemployment_status_code = resp.status
    bls_unemployment_ok = resp.ok
    if resp.ok {
        data, json_err := resp.json()
        if json_err != nil {
            bls_unemployment_json_error = json_err
        } else {
            bls_unemployment_status = data.status
            if data.Results != nil && data.Results.series != nil {
                series := data.Results.series
                bls_unemployment_series_count = #series
                if bls_unemployment_series_count > 0 {
                    row := series[1]
                    bls_unemployment_series_id = row.seriesID
                    observations := row.data
                    if observations != nil {
                        bls_unemployment_observation_count = #observations
                        if bls_unemployment_observation_count > 0 {
                            obs := observations[1]
                            bls_unemployment_year = obs.year
                            bls_unemployment_period = obs.period
                            bls_unemployment_period_name = obs.periodName
                            bls_unemployment_value = obs.value
                            bls_unemployment_latest = obs.latest
                        }
                    }
                }
            }
        }
    } else {
        bls_unemployment_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	assertFinRobotPublicLiveDataOK(t, vm, "BLS unemployment rate", "bls_unemployment_status_code", "bls_unemployment_request_error", "bls_unemployment_json_error", "bls_unemployment_ok")
	status := mustGetString(t, vm, "bls_unemployment_status")
	seriesCount := mustGetInt(t, vm, "bls_unemployment_series_count")
	if status != "REQUEST_SUCCEEDED" && seriesCount == 0 {
		t.Skipf("BLS unemployment rate business response unavailable: status=%q", status)
	}
	seriesID := mustGetString(t, vm, "bls_unemployment_series_id")
	observationCount := mustGetInt(t, vm, "bls_unemployment_observation_count")
	year := mustGetString(t, vm, "bls_unemployment_year")
	period := mustGetString(t, vm, "bls_unemployment_period")
	periodName := mustGetString(t, vm, "bls_unemployment_period_name")
	value := mustGetString(t, vm, "bls_unemployment_value")
	latest := mustGetString(t, vm, "bls_unemployment_latest")
	fmt.Printf("bls_unemployment status=%q series=%d series_id=%q observations=%d year=%q period=%q period_name=%q value=%q latest=%q\n", status, seriesCount, seriesID, observationCount, year, period, periodName, value, latest)
	if status != "REQUEST_SUCCEEDED" || seriesCount <= 0 || seriesID != "LNS14000000" || observationCount <= 0 || year < "2020" || !strings.HasPrefix(period, "M") || periodName == "" || value == "" {
		t.Fatalf("unexpected BLS unemployment rate payload: status=%q series=%d series_id=%q observations=%d year=%q period=%q period_name=%q value=%q latest=%q", status, seriesCount, seriesID, observationCount, year, period, periodName, value, latest)
	}
}

func TestFinRobotLiveBLSTotalNonfarmPayrollDataIntegration(t *testing.T) {
	vm := newFinRobotLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
bls_payroll_request_error := nil
bls_payroll_json_error := nil
bls_payroll_status_code := 0
bls_payroll_ok := false
bls_payroll_status := ""
bls_payroll_series_count := 0
bls_payroll_series_id := ""
bls_payroll_observation_count := 0
bls_payroll_year := ""
bls_payroll_period := ""
bls_payroll_period_name := ""
bls_payroll_value := ""
bls_payroll_latest := ""

headers := {}
headers["User-Agent"] = "Mozilla/5.0 Leia FinRobot live data smoke"
headers["Accept"] = "application/json"

resp, err := net.get("https://api.bls.gov/publicAPI/v2/timeseries/data/CES0000000001?latest=true", {
    headers: headers
    timeout: 30
})
if err != nil {
    bls_payroll_request_error = err
} else {
    bls_payroll_status_code = resp.status
    bls_payroll_ok = resp.ok
    if resp.ok {
        data, json_err := resp.json()
        if json_err != nil {
            bls_payroll_json_error = json_err
        } else {
            bls_payroll_status = data.status
            if data.Results != nil && data.Results.series != nil {
                series := data.Results.series
                bls_payroll_series_count = #series
                if bls_payroll_series_count > 0 {
                    row := series[1]
                    bls_payroll_series_id = row.seriesID
                    observations := row.data
                    if observations != nil {
                        bls_payroll_observation_count = #observations
                        if bls_payroll_observation_count > 0 {
                            obs := observations[1]
                            bls_payroll_year = obs.year
                            bls_payroll_period = obs.period
                            bls_payroll_period_name = obs.periodName
                            bls_payroll_value = obs.value
                            bls_payroll_latest = obs.latest
                        }
                    }
                }
            }
        }
    } else {
        bls_payroll_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	assertFinRobotPublicLiveDataOK(t, vm, "BLS total nonfarm payroll", "bls_payroll_status_code", "bls_payroll_request_error", "bls_payroll_json_error", "bls_payroll_ok")
	status := mustGetString(t, vm, "bls_payroll_status")
	seriesCount := mustGetInt(t, vm, "bls_payroll_series_count")
	if status != "REQUEST_SUCCEEDED" && seriesCount == 0 {
		t.Skipf("BLS total nonfarm payroll business response unavailable: status=%q", status)
	}
	seriesID := mustGetString(t, vm, "bls_payroll_series_id")
	observationCount := mustGetInt(t, vm, "bls_payroll_observation_count")
	year := mustGetString(t, vm, "bls_payroll_year")
	period := mustGetString(t, vm, "bls_payroll_period")
	periodName := mustGetString(t, vm, "bls_payroll_period_name")
	value := mustGetString(t, vm, "bls_payroll_value")
	latest := mustGetString(t, vm, "bls_payroll_latest")
	fmt.Printf("bls_payroll status=%q series=%d series_id=%q observations=%d year=%q period=%q period_name=%q value=%q latest=%q\n", status, seriesCount, seriesID, observationCount, year, period, periodName, value, latest)
	if status != "REQUEST_SUCCEEDED" || seriesCount <= 0 || seriesID != "CES0000000001" || observationCount <= 0 || year < "2020" || !strings.HasPrefix(period, "M") || periodName == "" || value == "" {
		t.Fatalf("unexpected BLS total nonfarm payroll payload: status=%q series=%d series_id=%q observations=%d year=%q period=%q period_name=%q value=%q latest=%q", status, seriesCount, seriesID, observationCount, year, period, periodName, value, latest)
	}
}
