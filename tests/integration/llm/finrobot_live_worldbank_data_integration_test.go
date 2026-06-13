package leia_test

import (
	"fmt"
	"strconv"
	"testing"
)

func TestFinRobotLiveWorldBankUSGDPDataIntegration(t *testing.T) {
	vm := newFinRobotLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
worldbank_gdp_request_error := nil
worldbank_gdp_json_error := nil
worldbank_gdp_status := 0
worldbank_gdp_ok := false
worldbank_gdp_page := 0
worldbank_gdp_count := 0
worldbank_gdp_country_iso3 := ""
worldbank_gdp_indicator_id := ""
worldbank_gdp_date := ""
worldbank_gdp_value := 0.0

headers := {}
headers["User-Agent"] = "Mozilla/5.0 Leia FinRobot live data smoke"
headers["Accept"] = "application/json"

resp, err := net.get("https://api.worldbank.org/v2/country/US/indicator/NY.GDP.MKTP.CD?format=json&per_page=1&mrnev=1", {
    headers: headers
    timeout: 30
})
if err != nil {
    worldbank_gdp_request_error = err
} else {
    worldbank_gdp_status = resp.status
    worldbank_gdp_ok = resp.ok
    if resp.ok {
        data, json_err := resp.json()
        if json_err != nil {
            worldbank_gdp_json_error = json_err
        } else {
            metadata := data[1]
            rows := data[2]
            worldbank_gdp_page = metadata.page
            worldbank_gdp_count = #rows
            if worldbank_gdp_count > 0 {
                row := rows[1]
                worldbank_gdp_country_iso3 = row.countryiso3code
                worldbank_gdp_indicator_id = row.indicator.id
                worldbank_gdp_date = row.date
                worldbank_gdp_value = row.value
            }
        }
    } else {
        worldbank_gdp_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	assertFinRobotPublicLiveDataOK(t, vm, "World Bank US GDP", "worldbank_gdp_status", "worldbank_gdp_request_error", "worldbank_gdp_json_error", "worldbank_gdp_ok")
	page := mustGetInt(t, vm, "worldbank_gdp_page")
	count := mustGetInt(t, vm, "worldbank_gdp_count")
	countryISO3 := mustGetString(t, vm, "worldbank_gdp_country_iso3")
	indicatorID := mustGetString(t, vm, "worldbank_gdp_indicator_id")
	dateValue := mustGetString(t, vm, "worldbank_gdp_date")
	gdpValue := mustGetFloat(t, vm, "worldbank_gdp_value")
	fmt.Printf("worldbank_gdp page=%d count=%d countryiso3code=%q indicator_id=%q date=%q value=%f\n", page, count, countryISO3, indicatorID, dateValue, gdpValue)
	if page != 1 || count <= 0 || countryISO3 != "USA" || indicatorID != "NY.GDP.MKTP.CD" || gdpValue <= 10000000000000 {
		t.Fatalf("unexpected World Bank US GDP payload: page=%d count=%d countryiso3code=%q indicator_id=%q date=%q value=%f", page, count, countryISO3, indicatorID, dateValue, gdpValue)
	}
	year, err := strconv.Atoi(dateValue)
	if err != nil {
		t.Fatalf("World Bank US GDP date = %q, want numeric year: %v", dateValue, err)
	}
	if year < 2020 {
		t.Fatalf("World Bank US GDP date = %d, want >= 2020", year)
	}
}
