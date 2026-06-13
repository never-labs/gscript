package leia_test

import (
	"fmt"
	"strings"
	"testing"
)

func TestFinRobotLiveFREDTreasuryYieldCSVDataIntegration(t *testing.T) {
	vm := newFinRobotLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
fred_dgs10_request_error := nil
fred_dgs10_json_error := nil
fred_dgs10_status := 0
fred_dgs10_ok := false
fred_dgs10_body := ""
fred_dgs10_body_len := 0
fred_dgs10_content_type := ""
fred_dgs10_header_present := false
fred_dgs10_line_count := 0
fred_dgs10_data_line_present := false

headers := {}
headers["User-Agent"] = "Mozilla/5.0 Leia FinRobot live data smoke"
headers["Accept"] = "text/csv,text/plain,*/*"

resp, err := net.get("https://fred.stlouisfed.org/graph/fredgraph.csv?id=DGS10", {
    headers: headers
    timeout: 10
})
if err != nil {
    fred_dgs10_request_error = err
} else {
    fred_dgs10_status = resp.status
    fred_dgs10_ok = resp.ok
    if resp.headers != nil && resp.headers["Content-Type"] != nil {
        fred_dgs10_content_type = resp.headers["Content-Type"]
    }
    if resp.ok {
        body := resp.body
        fred_dgs10_body = body
        fred_dgs10_body_len = #body
        fred_dgs10_header_present = string.find(body, "DATE,DGS10", 1, true) != nil

        lines := string.split(body, "\n")
        fred_dgs10_line_count = #lines
        for i := 2; i <= #lines; i++ {
            line := lines[i]
            if line != "" && string.find(line, ",", 1, true) != nil && !string.find(line, "DATE,DGS10", 1, true) {
                fred_dgs10_data_line_present = true
                break
            }
        }
    } else {
        fred_dgs10_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	assertFinRobotPublicLiveDataOK(t, vm, "FRED DGS10 CSV", "fred_dgs10_status", "fred_dgs10_request_error", "fred_dgs10_json_error", "fred_dgs10_ok")
	body := mustGetString(t, vm, "fred_dgs10_body")
	bodyLen := mustGetInt(t, vm, "fred_dgs10_body_len")
	contentType := mustGetString(t, vm, "fred_dgs10_content_type")
	lineCount := mustGetInt(t, vm, "fred_dgs10_line_count")
	headerPresent := mustGetBool(t, vm, "fred_dgs10_header_present")
	dataLinePresent := mustGetBool(t, vm, "fred_dgs10_data_line_present")

	fmt.Printf("fred_dgs10 content_type=%q body_len=%d lines=%d\n", contentType, bodyLen, lineCount)
	if bodyLen <= 0 || strings.TrimSpace(body) == "" {
		t.Fatalf("FRED DGS10 CSV body is empty: len=%d", bodyLen)
	}
	if contentType == "" {
		t.Fatalf("FRED DGS10 CSV Content-Type header is empty")
	}
	if !headerPresent || !strings.Contains(body, "DATE,DGS10") {
		t.Fatalf("FRED DGS10 CSV body missing DATE,DGS10 header")
	}
	if lineCount <= 100 {
		t.Fatalf("FRED DGS10 CSV line count = %d, want > 100", lineCount)
	}
	if !dataLinePresent {
		t.Fatalf("FRED DGS10 CSV body missing comma-delimited data row")
	}
}
