package leia_test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
)

func TestFinRobotLiveNasdaqTraderSymbolDirectoryDataIntegration(t *testing.T) {
	vm := newFinRobotLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
nasdaq_trader_request_error := nil
nasdaq_trader_json_error := nil
nasdaq_trader_status := 0
nasdaq_trader_ok := false
nasdaq_trader_body := ""
nasdaq_trader_body_len := 0
nasdaq_trader_content_type := ""

headers := {}
headers["User-Agent"] = "Mozilla/5.0 Leia FinRobot live data smoke"
headers["Accept"] = "text/plain,*/*"

resp, err := net.get("https://www.nasdaqtrader.com/dynamic/SymDir/nasdaqlisted.txt", {
    headers: headers
    timeout: 30
})
if err != nil {
    nasdaq_trader_request_error = err
} else {
    nasdaq_trader_status = resp.status
    nasdaq_trader_ok = resp.ok
    if resp.headers != nil && resp.headers["Content-Type"] != nil {
        nasdaq_trader_content_type = resp.headers["Content-Type"]
    }
    if resp.ok {
        body := resp.body
        nasdaq_trader_body = body
        nasdaq_trader_body_len = #body
    } else {
        nasdaq_trader_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	assertFinRobotPublicLiveDataOK(t, vm, "Nasdaq Trader Symbol Directory", "nasdaq_trader_status", "nasdaq_trader_request_error", "nasdaq_trader_json_error", "nasdaq_trader_ok")
	body := mustGetString(t, vm, "nasdaq_trader_body")
	bodyLen := mustGetInt(t, vm, "nasdaq_trader_body_len")
	contentType := mustGetString(t, vm, "nasdaq_trader_content_type")
	fmt.Printf("nasdaq_trader content_type=%q body_len=%d\n", contentType, bodyLen)

	if bodyLen <= 0 || strings.TrimSpace(body) == "" {
		t.Fatalf("Nasdaq Trader Symbol Directory body is empty: len=%d", bodyLen)
	}
	if contentType != "" && !strings.Contains(contentType, "text/plain") {
		t.Fatalf("Nasdaq Trader Symbol Directory content_type=%q, want text/plain", contentType)
	}

	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	if len(lines) < 100 {
		t.Fatalf("Nasdaq Trader Symbol Directory rows = %d, want many listed securities", len(lines))
	}
	header := strings.Split(lines[0], "|")
	wantHeader := []string{"Symbol", "Security Name", "Market Category", "Test Issue", "Financial Status", "Round Lot Size", "ETF", "NextShares"}
	if len(header) != len(wantHeader) {
		t.Fatalf("Nasdaq Trader Symbol Directory header columns = %d, want %d: %q", len(header), len(wantHeader), lines[0])
	}
	for i, want := range wantHeader {
		if header[i] != want {
			t.Fatalf("Nasdaq Trader Symbol Directory header[%d] = %q, want %q", i, header[i], want)
		}
	}

	var appleRow []string
	var fileCreationTime string
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "File Creation Time:") {
			fileCreationTime = strings.TrimPrefix(line, "File Creation Time:")
			continue
		}
		fields := strings.Split(line, "|")
		if len(fields) == len(wantHeader) && fields[0] == "AAPL" {
			appleRow = fields
		}
	}
	if len(appleRow) != len(wantHeader) {
		t.Fatalf("Nasdaq Trader Symbol Directory missing AAPL row")
	}

	roundLot, err := strconv.Atoi(appleRow[5])
	if err != nil {
		t.Fatalf("Nasdaq Trader Symbol Directory AAPL round lot %q is not numeric: %v", appleRow[5], err)
	}
	fmt.Printf("nasdaq_trader symbol=%q security=%q market=%q test_issue=%q financial_status=%q round_lot=%d etf=%q nextshares=%q file_creation_time=%q\n",
		appleRow[0], appleRow[1], appleRow[2], appleRow[3], appleRow[4], roundLot, appleRow[6], appleRow[7], fileCreationTime)

	if !strings.Contains(appleRow[1], "Apple") || appleRow[2] == "" || appleRow[3] != "N" || appleRow[4] == "" || roundLot <= 0 || appleRow[6] != "N" || appleRow[7] != "N" {
		t.Fatalf("unexpected Nasdaq Trader AAPL payload: row=%q round_lot=%d", strings.Join(appleRow, "|"), roundLot)
	}
	if strings.TrimSpace(fileCreationTime) == "" {
		t.Fatalf("Nasdaq Trader Symbol Directory missing file creation time footer")
	}
}
