package leia_test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestFinRobotLiveOECDUSMacroDataIntegration(t *testing.T) {
	vm := newFinRobotLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
oecd_macro_request_error := nil
oecd_macro_json_error := nil
oecd_macro_status := 0
oecd_macro_ok := false
oecd_macro_body := ""
oecd_macro_body_len := 0
oecd_macro_content_type := ""
oecd_macro_header_present := false
oecd_macro_observation_count := 0
oecd_macro_data_line_present := false
oecd_macro_indicator := ""
oecd_macro_country := ""
oecd_macro_time := ""
oecd_macro_value := ""

headers := {}
headers["User-Agent"] = "Mozilla/5.0 Leia FinRobot live data smoke"
headers["Accept"] = "text/csv,text/plain,*/*"

resp, err := net.get("https://sdmx.oecd.org/public/rest/v1/data/OECD.SDD.TPS,DSD_G20_PRICES@DF_G20_PRICES/USA.M.N.CPI.IX._T.N._Z?startPeriod=2024-01&dimensionAtObservation=AllDimensions", {
    headers: headers
    timeout: 30
})
if err != nil {
    oecd_macro_request_error = err
} else {
    oecd_macro_status = resp.status
    oecd_macro_ok = resp.ok
    if resp.headers != nil && resp.headers["Content-Type"] != nil {
        oecd_macro_content_type = resp.headers["Content-Type"]
    }
    if resp.ok {
        body := resp.body
        oecd_macro_body = body
        oecd_macro_body_len = #body
        oecd_macro_header_present = string.find(body, "TIME_PERIOD,OBS_VALUE", 1, true) != nil

        lines := string.split(body, "\n")
        for i := 2; i <= #lines; i++ {
            line := string.trim(lines[i])
            if line != "" && !string.find(line, "DATAFLOW,REF_AREA", 1, true) {
                parts := string.split(line, ",")
                if #parts >= 11 && parts[2] == "USA" && parts[5] == "CPI" && parts[10] != "" && parts[11] != "" {
                    oecd_macro_observation_count = oecd_macro_observation_count + 1
                    oecd_macro_data_line_present = true
                    if parts[10] > oecd_macro_time {
                        oecd_macro_indicator = parts[5]
                        oecd_macro_country = parts[2]
                        oecd_macro_time = parts[10]
                        oecd_macro_value = parts[11]
                    }
                }
            }
        }
    } else {
        oecd_macro_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	assertFinRobotPublicLiveDataOK(t, vm, "OECD G20 US CPI macro data", "oecd_macro_status", "oecd_macro_request_error", "oecd_macro_json_error", "oecd_macro_ok")
	body := mustGetString(t, vm, "oecd_macro_body")
	bodyLen := mustGetInt(t, vm, "oecd_macro_body_len")
	contentType := mustGetString(t, vm, "oecd_macro_content_type")
	headerPresent := mustGetBool(t, vm, "oecd_macro_header_present")
	dataLinePresent := mustGetBool(t, vm, "oecd_macro_data_line_present")
	observationCount := mustGetInt(t, vm, "oecd_macro_observation_count")
	indicator := mustGetString(t, vm, "oecd_macro_indicator")
	country := mustGetString(t, vm, "oecd_macro_country")
	timePeriod := mustGetString(t, vm, "oecd_macro_time")
	valueText := mustGetString(t, vm, "oecd_macro_value")

	fmt.Printf("oecd_macro content_type=%q body_len=%d observations=%d indicator=%q country=%q time=%q value=%q\n", contentType, bodyLen, observationCount, indicator, country, timePeriod, valueText)
	if bodyLen <= 0 || strings.TrimSpace(body) == "" {
		t.Fatalf("OECD G20 US CPI body is empty: len=%d", bodyLen)
	}
	if contentType == "" {
		t.Fatalf("OECD G20 US CPI Content-Type header is empty")
	}
	if !headerPresent || !strings.Contains(body, "DATAFLOW,REF_AREA,FREQ,METHODOLOGY,MEASURE,UNIT_MEASURE,EXPENDITURE,ADJUSTMENT,TRANSFORMATION,TIME_PERIOD,OBS_VALUE") {
		t.Fatalf("OECD G20 US CPI body missing expected CSV header")
	}
	if !dataLinePresent || observationCount <= 0 || indicator != "CPI" || country != "USA" || timePeriod == "" || valueText == "" {
		t.Fatalf("unexpected OECD macro payload: indicator=%q country=%q observations=%d time=%q value=%q data_line=%t", indicator, country, observationCount, timePeriod, valueText, dataLinePresent)
	}
	period, err := time.Parse("2006-01", timePeriod)
	if err != nil {
		t.Fatalf("OECD macro time period = %q, want YYYY-MM: %v", timePeriod, err)
	}
	if period.Year() < 2024 || period.Year() > time.Now().Year()+1 {
		t.Fatalf("OECD macro time period = %q, want recent monthly CPI observation", timePeriod)
	}
	value, err := strconv.ParseFloat(valueText, 64)
	if err != nil {
		t.Fatalf("OECD macro value = %q, want numeric CPI index: %v", valueText, err)
	}
	if value <= 50 || value >= 300 {
		t.Fatalf("OECD macro CPI value = %f, want plausible index level", value)
	}
}
