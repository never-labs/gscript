package leia_test

import (
	"fmt"
	"strings"
	"testing"
)

func TestFinRobotLiveNWSWeatherRiskDataIntegration(t *testing.T) {
	vm := newFinRobotLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
nws_weather_request_error := nil
nws_weather_json_error := nil
nws_weather_status := 0
nws_weather_ok := false
nws_weather_content_type := ""
nws_weather_collection_type := ""
nws_weather_title := ""
nws_weather_updated := ""
nws_weather_alert_count := 0
nws_weather_first_id := ""
nws_weather_first_type := ""
nws_weather_first_event := ""
nws_weather_first_status := ""
nws_weather_first_message_type := ""
nws_weather_first_category := ""
nws_weather_first_severity := ""
nws_weather_first_certainty := ""
nws_weather_first_urgency := ""
nws_weather_first_headline := ""
nws_weather_first_sender_name := ""

headers := {}
headers["User-Agent"] = "Mozilla/5.0 Leia FinRobot live data smoke contact=opensource@example.invalid"
headers["Accept"] = "application/geo+json,application/json"

resp, err := net.get("https://api.weather.gov/alerts/active?area=CA", {
    headers: headers
    timeout: 30
})
if err != nil {
    nws_weather_request_error = err
} else {
    nws_weather_status = resp.status
    nws_weather_ok = resp.ok
    if resp.headers != nil && resp.headers["Content-Type"] != nil {
        nws_weather_content_type = resp.headers["Content-Type"]
    }
    if resp.ok {
        data, json_err := resp.json()
        if json_err != nil {
            nws_weather_json_error = json_err
        } else {
            nws_weather_collection_type = data.type
            nws_weather_title = data.title
            nws_weather_updated = data.updated
            nws_weather_alert_count = #data.features
            if nws_weather_alert_count > 0 {
                first := data.features[1]
                nws_weather_first_id = first.id
                nws_weather_first_type = first.type
                nws_weather_first_event = first.properties.event
                nws_weather_first_status = first.properties.status
                nws_weather_first_message_type = first.properties.messageType
                nws_weather_first_category = first.properties.category
                nws_weather_first_severity = first.properties.severity
                nws_weather_first_certainty = first.properties.certainty
                nws_weather_first_urgency = first.properties.urgency
                nws_weather_first_headline = first.properties.headline
                nws_weather_first_sender_name = first.properties.senderName
            }
        }
    } else {
        nws_weather_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	assertFinRobotPublicLiveDataOK(t, vm, "NOAA/NWS California active weather alerts", "nws_weather_status", "nws_weather_request_error", "nws_weather_json_error", "nws_weather_ok")
	contentType := mustGetString(t, vm, "nws_weather_content_type")
	collectionType := mustGetString(t, vm, "nws_weather_collection_type")
	title := mustGetString(t, vm, "nws_weather_title")
	updated := mustGetString(t, vm, "nws_weather_updated")
	alertCount := mustGetInt(t, vm, "nws_weather_alert_count")
	firstID := mustGetString(t, vm, "nws_weather_first_id")
	firstType := mustGetString(t, vm, "nws_weather_first_type")
	firstEvent := mustGetString(t, vm, "nws_weather_first_event")
	firstStatus := mustGetString(t, vm, "nws_weather_first_status")
	firstMessageType := mustGetString(t, vm, "nws_weather_first_message_type")
	firstCategory := mustGetString(t, vm, "nws_weather_first_category")
	firstSeverity := mustGetString(t, vm, "nws_weather_first_severity")
	firstCertainty := mustGetString(t, vm, "nws_weather_first_certainty")
	firstUrgency := mustGetString(t, vm, "nws_weather_first_urgency")
	firstHeadline := mustGetString(t, vm, "nws_weather_first_headline")
	firstSenderName := mustGetString(t, vm, "nws_weather_first_sender_name")

	fmt.Printf("nws_weather content_type=%q type=%q title=%q updated=%q alerts=%d first_event=%q first_status=%q first_message_type=%q first_sender=%q\n", contentType, collectionType, title, updated, alertCount, firstEvent, firstStatus, firstMessageType, firstSenderName)
	if contentType == "" || !strings.Contains(contentType, "geo+json") {
		t.Fatalf("NOAA/NWS active alerts Content-Type = %q, want geo+json", contentType)
	}
	if collectionType != "FeatureCollection" || title == "" || !strings.Contains(title, "California") || updated == "" || alertCount < 0 {
		t.Fatalf("unexpected NOAA/NWS active alerts metadata: type=%q title=%q updated=%q alerts=%d", collectionType, title, updated, alertCount)
	}
	if alertCount > 0 {
		if firstID == "" || !strings.HasPrefix(firstID, "https://api.weather.gov/alerts/") || firstType != "Feature" ||
			firstEvent == "" || firstStatus == "" || firstMessageType == "" || firstCategory == "" ||
			firstSeverity == "" || firstCertainty == "" || firstUrgency == "" || firstHeadline == "" || firstSenderName == "" {
			t.Fatalf("unexpected NOAA/NWS first alert: id=%q type=%q event=%q status=%q message_type=%q category=%q severity=%q certainty=%q urgency=%q headline=%q sender=%q", firstID, firstType, firstEvent, firstStatus, firstMessageType, firstCategory, firstSeverity, firstCertainty, firstUrgency, firstHeadline, firstSenderName)
		}
		if !isNWSAlertStatus(firstStatus) || !isNWSAlertMessageType(firstMessageType) {
			t.Fatalf("unexpected NOAA/NWS alert status/message type: status=%q message_type=%q", firstStatus, firstMessageType)
		}
	}
}

func isNWSAlertStatus(status string) bool {
	switch status {
	case "Actual", "Exercise", "System", "Test", "Draft":
		return true
	default:
		return false
	}
}

func isNWSAlertMessageType(messageType string) bool {
	switch messageType {
	case "Alert", "Update", "Cancel", "Ack", "Error":
		return true
	default:
		return false
	}
}
