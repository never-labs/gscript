package leia_test

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestFinRobotLiveEnergyChartsPowerPriceDataIntegration(t *testing.T) {
	vm := newFinRobotLiveDataVM(t)
	end := time.Now().UTC().AddDate(0, 0, -1).Format(time.DateOnly)
	start := time.Now().UTC().AddDate(0, 0, -3).Format(time.DateOnly)
	url := fmt.Sprintf("https://api.energy-charts.info/price?bzn=DE-LU&start=%s&end=%s", start, end)
	if err := execFinRobotLiveDataScript(t, vm, fmt.Sprintf(`
energy_charts_request_error := nil
energy_charts_json_error := nil
energy_charts_status := 0
energy_charts_ok := false
energy_charts_license := ""
energy_charts_unit := ""
energy_charts_deprecated := true
energy_charts_timestamp_count := 0
energy_charts_price_count := 0
energy_charts_first_timestamp := 0
energy_charts_first_price := 0.0

headers := {}
headers["User-Agent"] = "Mozilla/5.0 Leia FinRobot live data smoke"
headers["Accept"] = "application/json"

resp, err := net.get(%q, {
    headers: headers
    timeout: 30
})
if err != nil {
    energy_charts_request_error = err
} else {
    energy_charts_status = resp.status
    energy_charts_ok = resp.ok
    if resp.ok {
        data, json_err := resp.json()
        if json_err != nil {
            energy_charts_json_error = json_err
        } else {
            energy_charts_license = data.license_info
            energy_charts_unit = data.unit
            energy_charts_deprecated = data.deprecated
            if data.unix_seconds != nil {
                energy_charts_timestamp_count = #data.unix_seconds
                if energy_charts_timestamp_count > 0 {
                    energy_charts_first_timestamp = data.unix_seconds[1]
                }
            }
            if data.price != nil {
                energy_charts_price_count = #data.price
                if energy_charts_price_count > 0 {
                    energy_charts_first_price = data.price[1]
                }
            }
        }
    } else {
        energy_charts_request_error = resp.statusText
    }
}
`, url)); err != nil {
		t.Fatal(err)
	}

	assertFinRobotPublicLiveDataOK(t, vm, "Energy-Charts power price", "energy_charts_status", "energy_charts_request_error", "energy_charts_json_error", "energy_charts_ok")
	license := mustGetString(t, vm, "energy_charts_license")
	unit := mustGetString(t, vm, "energy_charts_unit")
	deprecated := mustGetBool(t, vm, "energy_charts_deprecated")
	timestampCount := mustGetInt(t, vm, "energy_charts_timestamp_count")
	priceCount := mustGetInt(t, vm, "energy_charts_price_count")
	firstTimestamp := mustGetInt(t, vm, "energy_charts_first_timestamp")
	firstPrice := mustGetFloat(t, vm, "energy_charts_first_price")

	fmt.Printf("energy_charts start=%q end=%q timestamps=%d prices=%d first_timestamp=%d first_price=%f unit=%q deprecated=%t license=%q\n", start, end, timestampCount, priceCount, firstTimestamp, firstPrice, unit, deprecated, license)
	if !strings.Contains(license, "Bundesnetzagentur") || unit != "EUR / MWh" || deprecated {
		t.Fatalf("unexpected Energy-Charts metadata: license=%q unit=%q deprecated=%t", license, unit, deprecated)
	}
	if timestampCount <= 0 || priceCount <= 0 || timestampCount != priceCount {
		t.Fatalf("unexpected Energy-Charts series lengths: timestamps=%d prices=%d", timestampCount, priceCount)
	}
	if firstTimestamp <= 0 || firstPrice < -1000 || firstPrice > 5000 {
		t.Fatalf("unexpected Energy-Charts first observation: timestamp=%d price=%f", firstTimestamp, firstPrice)
	}
}
