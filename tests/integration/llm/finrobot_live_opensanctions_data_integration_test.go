package leia_test

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestFinRobotLiveOpenSanctionsDatasetIndexDataIntegration(t *testing.T) {
	vm := newFinRobotLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
opensanctions_index_request_error := nil
opensanctions_index_json_error := nil
opensanctions_index_status := 0
opensanctions_index_ok := false
opensanctions_index_content_type := ""
opensanctions_index_dataset_count := 0
opensanctions_index_default_found := false
opensanctions_index_default_title := ""
opensanctions_index_default_updated_at := ""
opensanctions_index_default_last_export := ""
opensanctions_index_default_entity_count := 0
opensanctions_index_default_thing_count := 0
opensanctions_index_default_resource_count := 0
opensanctions_index_default_entities_resource := false
opensanctions_index_default_targets_resource := false
opensanctions_index_default_statements_resource := false

headers := {}
headers["User-Agent"] = "Mozilla/5.0 Leia FinRobot live data smoke contact=opensource@example.invalid"
headers["Accept"] = "application/json"

resp, err := net.get("https://data.opensanctions.org/datasets/latest/index.json", {
    headers: headers
    timeout: 30
})
if err != nil {
    opensanctions_index_request_error = err
} else {
    opensanctions_index_status = resp.status
    opensanctions_index_ok = resp.ok
    if resp.headers != nil && resp.headers["Content-Type"] != nil {
        opensanctions_index_content_type = resp.headers["Content-Type"]
    }
    if resp.ok {
        data, json_err := resp.json()
        if json_err != nil {
            opensanctions_index_json_error = json_err
        } else {
            opensanctions_index_dataset_count = #data.datasets
            for _, dataset := range pairs(data.datasets) {
                if dataset.name == "default" {
                    opensanctions_index_default_found = true
                    opensanctions_index_default_title = dataset.title
                    opensanctions_index_default_updated_at = dataset.updated_at
                    opensanctions_index_default_last_export = dataset.last_export
                    opensanctions_index_default_entity_count = dataset.entity_count
                    opensanctions_index_default_thing_count = dataset.thing_count
                    opensanctions_index_default_resource_count = #dataset.resources
                    for _, resource := range pairs(dataset.resources) {
                        if resource.name == "entities.ftm.json" && resource.mime_type == "application/json+ftm" {
                            opensanctions_index_default_entities_resource = true
                        }
                        if resource.name == "targets.nested.json" && resource.mime_type == "application/json" {
                            opensanctions_index_default_targets_resource = true
                        }
                        if resource.name == "statements.csv" && string.contains(resource.mime_type, "text/csv") {
                            opensanctions_index_default_statements_resource = true
                        }
                    }
                }
            }
        }
    } else {
        opensanctions_index_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	assertFinRobotPublicLiveDataOK(t, vm, "OpenSanctions dataset index", "opensanctions_index_status", "opensanctions_index_request_error", "opensanctions_index_json_error", "opensanctions_index_ok")
	contentType := mustGetString(t, vm, "opensanctions_index_content_type")
	datasetCount := mustGetInt(t, vm, "opensanctions_index_dataset_count")
	defaultFound := mustGetBool(t, vm, "opensanctions_index_default_found")
	defaultTitle := mustGetString(t, vm, "opensanctions_index_default_title")
	updatedAt := mustGetString(t, vm, "opensanctions_index_default_updated_at")
	lastExport := mustGetString(t, vm, "opensanctions_index_default_last_export")
	entityCount := mustGetInt(t, vm, "opensanctions_index_default_entity_count")
	thingCount := mustGetInt(t, vm, "opensanctions_index_default_thing_count")
	resourceCount := mustGetInt(t, vm, "opensanctions_index_default_resource_count")
	hasEntities := mustGetBool(t, vm, "opensanctions_index_default_entities_resource")
	hasTargets := mustGetBool(t, vm, "opensanctions_index_default_targets_resource")
	hasStatements := mustGetBool(t, vm, "opensanctions_index_default_statements_resource")

	fmt.Printf("opensanctions_index datasets=%d default_title=%q updated_at=%q last_export=%q entities=%d things=%d resources=%d content_type=%q\n", datasetCount, defaultTitle, updatedAt, lastExport, entityCount, thingCount, resourceCount, contentType)
	if !strings.Contains(contentType, "application/json") {
		t.Fatalf("OpenSanctions index Content-Type = %q, want JSON", contentType)
	}
	if datasetCount < 100 || !defaultFound || defaultTitle != "OpenSanctions Default" {
		t.Fatalf("unexpected OpenSanctions dataset catalog: datasets=%d default_found=%t default_title=%q", datasetCount, defaultFound, defaultTitle)
	}
	if entityCount < 1000000 || thingCount < 100000 || resourceCount < 3 {
		t.Fatalf("unexpected OpenSanctions default dataset scale: entities=%d things=%d resources=%d", entityCount, thingCount, resourceCount)
	}
	if !hasEntities || !hasTargets || !hasStatements {
		t.Fatalf("OpenSanctions default resources missing expected public bulk exports: entities=%t targets=%t statements=%t", hasEntities, hasTargets, hasStatements)
	}
	for label, value := range map[string]string{"updated_at": updatedAt, "last_export": lastExport} {
		parsed, err := time.Parse("2006-01-02T15:04:05", value)
		if err != nil {
			t.Fatalf("OpenSanctions default %s = %q, want timestamp: %v", label, value, err)
		}
		if parsed.Before(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)) || parsed.After(time.Now().AddDate(0, 0, 2)) {
			t.Fatalf("OpenSanctions default %s = %s, want plausible public dataset timestamp", label, value)
		}
	}
}
