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

func TestFinRobotLiveWorldBankUSInflationDataIntegration(t *testing.T) {
	vm := newFinRobotLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
worldbank_inflation_request_error := nil
worldbank_inflation_json_error := nil
worldbank_inflation_status := 0
worldbank_inflation_ok := false
worldbank_inflation_page := 0
worldbank_inflation_count := 0
worldbank_inflation_country_iso3 := ""
worldbank_inflation_indicator_id := ""
worldbank_inflation_date := ""
worldbank_inflation_value := 0.0
worldbank_inflation_value_present := false

headers := {}
headers["User-Agent"] = "Mozilla/5.0 Leia FinRobot live data smoke"
headers["Accept"] = "application/json"

resp, err := net.get("https://api.worldbank.org/v2/country/US/indicator/FP.CPI.TOTL.ZG?format=json&per_page=1&mrnev=1", {
    headers: headers
    timeout: 30
})
if err != nil {
    worldbank_inflation_request_error = err
} else {
    worldbank_inflation_status = resp.status
    worldbank_inflation_ok = resp.ok
    if resp.ok {
        data, json_err := resp.json()
        if json_err != nil {
            worldbank_inflation_json_error = json_err
        } else {
            metadata := data[1]
            rows := data[2]
            worldbank_inflation_page = metadata.page
            worldbank_inflation_count = #rows
            if worldbank_inflation_count > 0 {
                row := rows[1]
                worldbank_inflation_country_iso3 = row.countryiso3code
                worldbank_inflation_indicator_id = row.indicator.id
                worldbank_inflation_date = row.date
                if row.value != nil {
                    worldbank_inflation_value = row.value
                    worldbank_inflation_value_present = true
                }
            }
        }
    } else {
        worldbank_inflation_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	assertFinRobotPublicLiveDataOK(t, vm, "World Bank US inflation", "worldbank_inflation_status", "worldbank_inflation_request_error", "worldbank_inflation_json_error", "worldbank_inflation_ok")
	page := mustGetInt(t, vm, "worldbank_inflation_page")
	count := mustGetInt(t, vm, "worldbank_inflation_count")
	countryISO3 := mustGetString(t, vm, "worldbank_inflation_country_iso3")
	indicatorID := mustGetString(t, vm, "worldbank_inflation_indicator_id")
	dateValue := mustGetString(t, vm, "worldbank_inflation_date")
	inflationValue := mustGetFloat(t, vm, "worldbank_inflation_value")
	valuePresent := mustGetBool(t, vm, "worldbank_inflation_value_present")
	fmt.Printf("worldbank_inflation page=%d count=%d countryiso3code=%q indicator_id=%q date=%q value=%f value_present=%t\n", page, count, countryISO3, indicatorID, dateValue, inflationValue, valuePresent)
	if page != 1 || count <= 0 || countryISO3 != "USA" || indicatorID != "FP.CPI.TOTL.ZG" || !valuePresent || inflationValue <= -10 || inflationValue >= 30 {
		t.Fatalf("unexpected World Bank US inflation payload: page=%d count=%d countryiso3code=%q indicator_id=%q date=%q value=%f value_present=%t", page, count, countryISO3, indicatorID, dateValue, inflationValue, valuePresent)
	}
	year, err := strconv.Atoi(dateValue)
	if err != nil {
		t.Fatalf("World Bank US inflation date = %q, want numeric year: %v", dateValue, err)
	}
	if year < 2020 {
		t.Fatalf("World Bank US inflation date = %d, want >= 2020", year)
	}
}
