package leia_test

import (
	"fmt"
	"strings"
	"testing"
)

func TestFinRobotLiveUSGSEarthquakeRiskDataIntegration(t *testing.T) {
	vm := newFinRobotLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
usgs_quake_request_error := nil
usgs_quake_json_error := nil
usgs_quake_status := 0
usgs_quake_ok := false
usgs_quake_collection_type := ""
usgs_quake_title := ""
usgs_quake_count := 0
usgs_quake_feature_count := 0
usgs_quake_first_mag := 0.0
usgs_quake_first_place := ""
usgs_quake_first_time := 0
usgs_quake_first_url := ""

headers := {}
headers["User-Agent"] = "Mozilla/5.0 Leia FinRobot live data smoke"
headers["Accept"] = "application/geo+json,application/json"

resp, err := net.get("https://earthquake.usgs.gov/earthquakes/feed/v1.0/summary/4.5_day.geojson", {
    headers: headers
    timeout: 30
})
if err != nil {
    usgs_quake_request_error = err
} else {
    usgs_quake_status = resp.status
    usgs_quake_ok = resp.ok
    if resp.ok {
        data, json_err := resp.json()
        if json_err != nil {
            usgs_quake_json_error = json_err
        } else {
            usgs_quake_collection_type = data.type
            usgs_quake_title = data.metadata.title
            usgs_quake_count = data.metadata.count
            usgs_quake_feature_count = #data.features
            if usgs_quake_feature_count > 0 {
                first := data.features[1]
                usgs_quake_first_mag = first.properties.mag
                usgs_quake_first_place = first.properties.place
                usgs_quake_first_time = first.properties.time
                usgs_quake_first_url = first.properties.url
            }
        }
    } else {
        usgs_quake_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	assertFinRobotPublicLiveDataOK(t, vm, "USGS earthquake risk", "usgs_quake_status", "usgs_quake_request_error", "usgs_quake_json_error", "usgs_quake_ok")
	collectionType := mustGetString(t, vm, "usgs_quake_collection_type")
	title := mustGetString(t, vm, "usgs_quake_title")
	count := mustGetInt(t, vm, "usgs_quake_count")
	featureCount := mustGetInt(t, vm, "usgs_quake_feature_count")
	mag := mustGetFloat(t, vm, "usgs_quake_first_mag")
	place := mustGetString(t, vm, "usgs_quake_first_place")
	eventTime := mustGetInt(t, vm, "usgs_quake_first_time")
	url := mustGetString(t, vm, "usgs_quake_first_url")
	fmt.Printf("usgs_quake type=%q title=%q count=%d features=%d first_mag=%f first_place=%q first_time=%d first_url=%q\n", collectionType, title, count, featureCount, mag, place, eventTime, url)
	if collectionType != "FeatureCollection" || title == "" || count < 0 || featureCount < 0 {
		t.Fatalf("unexpected USGS earthquake risk metadata: type=%q title=%q count=%d features=%d", collectionType, title, count, featureCount)
	}
	if featureCount > 0 && (mag < 4.5 || place == "" || eventTime <= 0 || !strings.HasPrefix(url, "https://earthquake.usgs.gov/earthquakes/eventpage/")) {
		t.Fatalf("unexpected USGS earthquake risk first feature: mag=%f place=%q time=%d url=%q", mag, place, eventTime, url)
	}
}
