package leia_test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestFinRobotLiveBankOfCanadaFXDataIntegration(t *testing.T) {
	vm := newFinRobotLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
boc_fx_request_error := nil
boc_fx_json_error := nil
boc_fx_status := 0
boc_fx_ok := false
boc_fx_series_label := ""
boc_fx_series_description := ""
boc_fx_count := 0
boc_fx_date := ""
boc_fx_value := ""

headers := {}
headers["User-Agent"] = "Mozilla/5.0 Leia FinRobot live data smoke"
headers["Accept"] = "application/json"

resp, err := net.get("https://www.bankofcanada.ca/valet/observations/FXUSDCAD/json?recent=5", {
    headers: headers
    timeout: 30
})
if err != nil {
    boc_fx_request_error = err
} else {
    boc_fx_status = resp.status
    boc_fx_ok = resp.ok
    if resp.ok {
        data, json_err := resp.json()
        if json_err != nil {
            boc_fx_json_error = json_err
        } else {
            boc_fx_series_label = data.seriesDetail.FXUSDCAD.label
            boc_fx_series_description = data.seriesDetail.FXUSDCAD.description
            boc_fx_count = #data.observations
            if boc_fx_count > 0 {
                row := data.observations[1]
                boc_fx_date = row.d
                boc_fx_value = row.FXUSDCAD.v
            }
        }
    } else {
        boc_fx_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	assertFinRobotPublicLiveDataOK(t, vm, "Bank of Canada USD/CAD FX", "boc_fx_status", "boc_fx_request_error", "boc_fx_json_error", "boc_fx_ok")
	label := mustGetString(t, vm, "boc_fx_series_label")
	description := mustGetString(t, vm, "boc_fx_series_description")
	count := mustGetInt(t, vm, "boc_fx_count")
	dateText := mustGetString(t, vm, "boc_fx_date")
	valueText := mustGetString(t, vm, "boc_fx_value")
	fmt.Printf("boc_fx label=%q count=%d date=%q value=%q description=%q\n", label, count, dateText, valueText, description)
	if label != "USD/CAD" || !strings.Contains(description, "US dollar") || count <= 0 || dateText == "" || valueText == "" {
		t.Fatalf("unexpected Bank of Canada FX payload: label=%q description=%q count=%d date=%q value=%q", label, description, count, dateText, valueText)
	}
	observedAt, err := time.Parse("2006-01-02", dateText)
	if err != nil {
		t.Fatalf("Bank of Canada FX date = %q, want YYYY-MM-DD: %v", dateText, err)
	}
	if observedAt.Year() < time.Now().Year()-2 || observedAt.After(time.Now().AddDate(0, 0, 2)) {
		t.Fatalf("Bank of Canada FX date = %s, want recent observation", dateText)
	}
	value, err := strconv.ParseFloat(valueText, 64)
	if err != nil {
		t.Fatalf("Bank of Canada FX value = %q, want numeric rate: %v", valueText, err)
	}
	if value <= 0.5 || value >= 3.0 {
		t.Fatalf("Bank of Canada USD/CAD value = %f, want plausible FX rate", value)
	}
}
