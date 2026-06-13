package leia_test

import (
	"fmt"
	"strings"
	"testing"
)

func TestFinRobotLiveNASAEONETEventCategoriesDataIntegration(t *testing.T) {
	vm := newFinRobotLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
nasa_eonet_request_error := nil
nasa_eonet_json_error := nil
nasa_eonet_status := 0
nasa_eonet_ok := false
nasa_eonet_title := ""
nasa_eonet_description := ""
nasa_eonet_category_count := 0
nasa_eonet_first_category_id := ""
nasa_eonet_first_category_title := ""
nasa_eonet_first_category_link := ""
nasa_eonet_first_category_description := ""
nasa_eonet_first_category_layers := ""
nasa_eonet_wildfire_present := false
nasa_eonet_severe_storms_present := false

headers := {}
headers["User-Agent"] = "Mozilla/5.0 Leia FinRobot live data smoke"
headers["Accept"] = "application/json,*/*"

resp, err := net.get("https://eonet.gsfc.nasa.gov/api/v3/categories", {
    headers: headers
    timeout: 30
})
if err != nil {
    nasa_eonet_request_error = err
} else {
    nasa_eonet_status = resp.status
    nasa_eonet_ok = resp.ok
    if resp.ok {
        data, json_err := resp.json()
        if json_err != nil {
            nasa_eonet_json_error = json_err
        } else {
            nasa_eonet_title = data.title
            nasa_eonet_description = data.description
            nasa_eonet_category_count = #data.categories
            if nasa_eonet_category_count > 0 {
                first := data.categories[1]
                nasa_eonet_first_category_id = first.id
                nasa_eonet_first_category_title = first.title
                nasa_eonet_first_category_link = first.link
                nasa_eonet_first_category_description = first.description
                nasa_eonet_first_category_layers = first.layers
                for _, category := range pairs(data.categories) {
                    if category.id == "wildfires" {
                        nasa_eonet_wildfire_present = true
                    }
                    if category.id == "severeStorms" {
                        nasa_eonet_severe_storms_present = true
                    }
                }
            }
        }
    } else {
        nasa_eonet_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	assertFinRobotPublicLiveDataOK(t, vm, "NASA EONET event categories", "nasa_eonet_status", "nasa_eonet_request_error", "nasa_eonet_json_error", "nasa_eonet_ok")
	title := mustGetString(t, vm, "nasa_eonet_title")
	description := mustGetString(t, vm, "nasa_eonet_description")
	categoryCount := mustGetInt(t, vm, "nasa_eonet_category_count")
	categoryID := mustGetString(t, vm, "nasa_eonet_first_category_id")
	categoryTitle := mustGetString(t, vm, "nasa_eonet_first_category_title")
	categoryLink := mustGetString(t, vm, "nasa_eonet_first_category_link")
	categoryDescription := mustGetString(t, vm, "nasa_eonet_first_category_description")
	categoryLayers := mustGetString(t, vm, "nasa_eonet_first_category_layers")
	wildfirePresent := mustGetBool(t, vm, "nasa_eonet_wildfire_present")
	severeStormsPresent := mustGetBool(t, vm, "nasa_eonet_severe_storms_present")
	fmt.Printf("nasa_eonet title=%q description=%q categories=%d first=%q/%q wildfire=%t severe_storms=%t\n", title, description, categoryCount, categoryID, categoryTitle, wildfirePresent, severeStormsPresent)
	if title != "EONET Event Categories" || description == "" || categoryCount <= 0 {
		t.Fatalf("unexpected NASA EONET metadata: title=%q description=%q categories=%d", title, description, categoryCount)
	}
	if categoryID == "" || categoryTitle == "" || !strings.HasPrefix(categoryLink, "https://eonet.gsfc.nasa.gov/api/v3/categories/") ||
		categoryDescription == "" || !strings.HasPrefix(categoryLayers, "https://eonet.gsfc.nasa.gov/api/v3/layers/") {
		t.Fatalf("unexpected NASA EONET first category: id=%q title=%q link=%q description=%q layers=%q", categoryID, categoryTitle, categoryLink, categoryDescription, categoryLayers)
	}
	if !wildfirePresent || !severeStormsPresent {
		t.Fatalf("NASA EONET category set missing expected risk categories: wildfire=%t severe_storms=%t", wildfirePresent, severeStormsPresent)
	}
}
