package leia_test

import (
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestFinRobotLiveFederalReserveH15TreasuryYieldDataIntegration(t *testing.T) {
	vm := newFinRobotLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
fed_h15_request_error := nil
fed_h15_status := 0
fed_h15_ok := false
fed_h15_content_type := ""
fed_h15_body := ""
fed_h15_body_len := 0

headers := {}
headers["User-Agent"] = "Mozilla/5.0 Leia FinRobot live data smoke"
headers["Accept"] = "text/csv,*/*"

resp, err := net.get("https://www.federalreserve.gov/datadownload/Output.aspx?rel=H15&series=bf17364827e38702b42a58cf8eaa3f78&lastobs=5&from=&to=&filetype=csv&label=include&layout=seriescolumn&type=package", {
    headers: headers
    timeout: 30
})
if err != nil {
    fed_h15_request_error = err
} else {
    fed_h15_status = resp.status
    fed_h15_ok = resp.ok
    if resp.headers != nil && resp.headers["Content-Type"] != nil {
        fed_h15_content_type = resp.headers["Content-Type"]
    }
    if resp.ok {
        fed_h15_body = resp.body
        fed_h15_body_len = #resp.body
    } else {
        fed_h15_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	status := mustGetInt(t, vm, "fed_h15_status")
	skipUnavailableFinRobotPublicLiveData(t, "Federal Reserve H.15 Treasury yields", status, getOrNil(t, vm, "fed_h15_request_error"))
	if status != 200 {
		t.Fatalf("Federal Reserve H.15 status = %d, want 200", status)
	}
	if ok := mustGetBool(t, vm, "fed_h15_ok"); !ok {
		t.Fatalf("Federal Reserve H.15 ok = false")
	}
	contentType := mustGetString(t, vm, "fed_h15_content_type")
	body := mustGetString(t, vm, "fed_h15_body")
	bodyLen := mustGetInt(t, vm, "fed_h15_body_len")
	if bodyLen <= 0 || strings.TrimSpace(body) == "" {
		t.Fatalf("Federal Reserve H.15 body is empty: len=%d", bodyLen)
	}
	if contentType == "" || !strings.Contains(contentType, "text/csv") {
		t.Fatalf("Federal Reserve H.15 Content-Type = %q, want text/csv", contentType)
	}

	records, err := csv.NewReader(strings.NewReader(body)).ReadAll()
	if err != nil {
		t.Fatalf("Federal Reserve H.15 CSV decode failed: %v", err)
	}
	if len(records) < 7 {
		t.Fatalf("Federal Reserve H.15 CSV rows = %d, want metadata + observations", len(records))
	}
	header := records[5]
	if len(header) < 10 || header[0] != "Time Period" || header[9] != "RIFLGFCY10_N.B" {
		t.Fatalf("unexpected Federal Reserve H.15 header: %#v", header)
	}
	last := records[len(records)-1]
	if len(last) < len(header) {
		t.Fatalf("unexpected Federal Reserve H.15 last row: %#v", last)
	}
	observedAt, err := time.Parse("2006-01-02", last[0])
	if err != nil {
		t.Fatalf("Federal Reserve H.15 observation date = %q, want YYYY-MM-DD: %v", last[0], err)
	}
	if observedAt.Year() < time.Now().Year()-2 || observedAt.After(time.Now().AddDate(0, 0, 2)) {
		t.Fatalf("Federal Reserve H.15 observation date = %s, want recent observation", last[0])
	}
	tenYear, err := strconv.ParseFloat(last[9], 64)
	if err != nil {
		t.Fatalf("Federal Reserve H.15 10Y value = %q, want numeric yield: %v", last[9], err)
	}
	if tenYear <= 0 || tenYear >= 20 {
		t.Fatalf("Federal Reserve H.15 10Y yield = %f, want plausible percent yield", tenYear)
	}
	fmt.Printf("fed_h15 content_type=%q rows=%d date=%q ten_year=%f\n", contentType, len(records), last[0], tenYear)
}
