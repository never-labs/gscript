package leia_test

import (
	"fmt"
	"strings"
	"testing"
)

func TestFinRobotLiveOpenMeteoClimateRiskDataIntegration(t *testing.T) {
	vm := newFinRobotLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
openmeteo_climate_request_error := nil
openmeteo_climate_json_error := nil
openmeteo_climate_status := 0
openmeteo_climate_ok := false
openmeteo_climate_latitude := 0.0
openmeteo_climate_longitude := 0.0
openmeteo_climate_timezone := ""
openmeteo_climate_time_unit := ""
openmeteo_climate_temperature_unit := ""
openmeteo_climate_precipitation_unit := ""
openmeteo_climate_wind_unit := ""
openmeteo_climate_day_count := 0
openmeteo_climate_first_day := ""
openmeteo_climate_last_day := ""
openmeteo_climate_first_temperature_max := 0.0
openmeteo_climate_first_precipitation_sum := 0.0
openmeteo_climate_first_wind_speed_max := 0.0
openmeteo_climate_hot_day_present := false
openmeteo_climate_wet_day_present := false

headers := {}
headers["User-Agent"] = "Mozilla/5.0 Leia FinRobot live data smoke contact=opensource@example.invalid"
headers["Accept"] = "application/json"

resp, err := net.get("https://archive-api.open-meteo.com/v1/archive?latitude=40.7128&longitude=-74.0060&start_date=2024-07-01&end_date=2024-07-07&daily=temperature_2m_max,precipitation_sum,wind_speed_10m_max&timezone=UTC", {
    headers: headers
    timeout: 30
})
if err != nil {
    openmeteo_climate_request_error = err
} else {
    openmeteo_climate_status = resp.status
    openmeteo_climate_ok = resp.ok
    if resp.ok {
        data, json_err := resp.json()
        if json_err != nil {
            openmeteo_climate_json_error = json_err
        } else {
            openmeteo_climate_latitude = data.latitude
            openmeteo_climate_longitude = data.longitude
            openmeteo_climate_timezone = data.timezone
            openmeteo_climate_time_unit = data.daily_units.time
            openmeteo_climate_temperature_unit = data.daily_units.temperature_2m_max
            openmeteo_climate_precipitation_unit = data.daily_units.precipitation_sum
            openmeteo_climate_wind_unit = data.daily_units.wind_speed_10m_max
            openmeteo_climate_day_count = #data.daily.time
            if openmeteo_climate_day_count > 0 {
                openmeteo_climate_first_day = data.daily.time[1]
                openmeteo_climate_last_day = data.daily.time[openmeteo_climate_day_count]
                openmeteo_climate_first_temperature_max = data.daily.temperature_2m_max[1]
                openmeteo_climate_first_precipitation_sum = data.daily.precipitation_sum[1]
                openmeteo_climate_first_wind_speed_max = data.daily.wind_speed_10m_max[1]
                for _, value := range pairs(data.daily.temperature_2m_max) {
                    if value >= 30.0 {
                        openmeteo_climate_hot_day_present = true
                    }
                }
                for _, value := range pairs(data.daily.precipitation_sum) {
                    if value > 0.0 {
                        openmeteo_climate_wet_day_present = true
                    }
                }
            }
        }
    } else {
        openmeteo_climate_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	assertFinRobotPublicLiveDataOK(t, vm, "Open-Meteo climate risk archive", "openmeteo_climate_status", "openmeteo_climate_request_error", "openmeteo_climate_json_error", "openmeteo_climate_ok")
	latitude := mustGetFloat(t, vm, "openmeteo_climate_latitude")
	longitude := mustGetFloat(t, vm, "openmeteo_climate_longitude")
	timezone := mustGetString(t, vm, "openmeteo_climate_timezone")
	timeUnit := mustGetString(t, vm, "openmeteo_climate_time_unit")
	temperatureUnit := mustGetString(t, vm, "openmeteo_climate_temperature_unit")
	precipitationUnit := mustGetString(t, vm, "openmeteo_climate_precipitation_unit")
	windUnit := mustGetString(t, vm, "openmeteo_climate_wind_unit")
	dayCount := mustGetInt(t, vm, "openmeteo_climate_day_count")
	firstDay := mustGetString(t, vm, "openmeteo_climate_first_day")
	lastDay := mustGetString(t, vm, "openmeteo_climate_last_day")
	firstTemperatureMax := mustGetFloat(t, vm, "openmeteo_climate_first_temperature_max")
	firstPrecipitationSum := mustGetFloat(t, vm, "openmeteo_climate_first_precipitation_sum")
	firstWindSpeedMax := mustGetFloat(t, vm, "openmeteo_climate_first_wind_speed_max")
	hotDayPresent := mustGetBool(t, vm, "openmeteo_climate_hot_day_present")
	wetDayPresent := mustGetBool(t, vm, "openmeteo_climate_wet_day_present")
	fmt.Printf("openmeteo_climate lat=%f lon=%f timezone=%q days=%d first=%q last=%q temp_max=%f precip=%f wind=%f units=%q/%q/%q/%q hot_day=%t wet_day=%t\n", latitude, longitude, timezone, dayCount, firstDay, lastDay, firstTemperatureMax, firstPrecipitationSum, firstWindSpeedMax, timeUnit, temperatureUnit, precipitationUnit, windUnit, hotDayPresent, wetDayPresent)
	if latitude < 40 || latitude > 41 || longitude < -75 || longitude > -73 || timezone == "" {
		t.Fatalf("unexpected Open-Meteo location metadata: lat=%f lon=%f timezone=%q", latitude, longitude, timezone)
	}
	if dayCount != 7 || firstDay != "2024-07-01" || lastDay != "2024-07-07" {
		t.Fatalf("unexpected Open-Meteo daily window: days=%d first=%q last=%q", dayCount, firstDay, lastDay)
	}
	if timeUnit != "iso8601" || !strings.Contains(temperatureUnit, "C") || precipitationUnit != "mm" || windUnit != "km/h" {
		t.Fatalf("unexpected Open-Meteo daily units: time=%q temperature=%q precipitation=%q wind=%q", timeUnit, temperatureUnit, precipitationUnit, windUnit)
	}
	if firstTemperatureMax < -30 || firstTemperatureMax > 50 || firstPrecipitationSum < 0 || firstWindSpeedMax <= 0 {
		t.Fatalf("unexpected Open-Meteo first daily metrics: temp_max=%f precipitation=%f wind=%f", firstTemperatureMax, firstPrecipitationSum, firstWindSpeedMax)
	}
	if !hotDayPresent || !wetDayPresent {
		t.Fatalf("Open-Meteo archive missing expected July climate risk signals: hot_day=%t wet_day=%t", hotDayPresent, wetDayPresent)
	}
}
