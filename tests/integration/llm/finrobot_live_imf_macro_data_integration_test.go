package leia_test

import (
	"fmt"
	"strconv"
	"testing"
	"time"
)

func TestFinRobotLiveIMFUSMacroIndicatorDataIntegration(t *testing.T) {
	vm := newFinRobotLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
imf_macro_request_error := nil
imf_macro_json_error := nil
imf_macro_status := 0
imf_macro_ok := false
imf_macro_indicator := ""
imf_macro_country := ""
imf_macro_year := ""
imf_macro_value := 0.0
imf_macro_value_present := false
imf_macro_observation_count := 0

headers := {}
headers["User-Agent"] = "Mozilla/5.0 Leia FinRobot live data smoke"
headers["Accept"] = "application/json"

resp, err := net.get("https://www.imf.org/external/datamapper/api/v1/NGDP_RPCH/USA", {
    headers: headers
    timeout: 30
})
if err != nil {
    imf_macro_request_error = err
} else {
    imf_macro_status = resp.status
    imf_macro_ok = resp.ok
    if resp.ok {
        data, json_err := resp.json()
        if json_err != nil {
            imf_macro_json_error = json_err
        } else if data.values != nil && data.values.NGDP_RPCH != nil && data.values.NGDP_RPCH.USA != nil {
            imf_macro_indicator = "NGDP_RPCH"
            imf_macro_country = "USA"
            for year, value := range pairs(data.values.NGDP_RPCH.USA) {
                imf_macro_observation_count = imf_macro_observation_count + 1
                year_text := tostring(year)
                if value != nil && year_text > imf_macro_year {
                    imf_macro_year = year_text
                    imf_macro_value = value
                    imf_macro_value_present = true
                }
            }
        }
    } else {
        imf_macro_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	assertFinRobotPublicLiveDataOK(t, vm, "IMF DataMapper US real GDP growth", "imf_macro_status", "imf_macro_request_error", "imf_macro_json_error", "imf_macro_ok")
	indicator := mustGetString(t, vm, "imf_macro_indicator")
	country := mustGetString(t, vm, "imf_macro_country")
	yearText := mustGetString(t, vm, "imf_macro_year")
	value := mustGetFloat(t, vm, "imf_macro_value")
	valuePresent := mustGetBool(t, vm, "imf_macro_value_present")
	observationCount := mustGetInt(t, vm, "imf_macro_observation_count")
	fmt.Printf("imf_macro indicator=%q country=%q observations=%d year=%q value=%f value_present=%t\n", indicator, country, observationCount, yearText, value, valuePresent)
	if indicator != "NGDP_RPCH" || country != "USA" || observationCount <= 0 || !valuePresent {
		t.Fatalf("unexpected IMF macro payload: indicator=%q country=%q observations=%d year=%q value=%f value_present=%t", indicator, country, observationCount, yearText, value, valuePresent)
	}
	year, err := strconv.Atoi(yearText)
	if err != nil {
		t.Fatalf("IMF macro year = %q, want numeric year: %v", yearText, err)
	}
	if year < 2020 || year > time.Now().Year()+10 {
		t.Fatalf("IMF macro year = %d, want recent historical or near forecast year", year)
	}
	if value <= -50 || value >= 50 {
		t.Fatalf("IMF macro value = %f, want plausible annual percent change", value)
	}
}
