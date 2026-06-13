package leia_test

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestFinRobotLiveECBDailyFXReferenceRatesIntegration(t *testing.T) {
	vm := newFinRobotLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
ecb_fx_request_error := nil
ecb_fx_json_error := nil
ecb_fx_status := 0
ecb_fx_ok := false
ecb_fx_body := ""
ecb_fx_body_len := 0
ecb_fx_content_type := ""

headers := {}
headers["User-Agent"] = "Mozilla/5.0 Leia FinRobot live data smoke"
headers["Accept"] = "application/xml,text/xml,*/*"

resp, err := net.get("https://www.ecb.europa.eu/stats/eurofxref/eurofxref-daily.xml", {
    headers: headers
    timeout: 30
})
if err != nil {
    ecb_fx_request_error = err
} else {
    ecb_fx_status = resp.status
    ecb_fx_ok = resp.ok
    if resp.headers != nil && resp.headers["Content-Type"] != nil {
        ecb_fx_content_type = resp.headers["Content-Type"]
    }
    if resp.ok {
        body := resp.body
        ecb_fx_body = body
        ecb_fx_body_len = #body
    } else {
        ecb_fx_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	assertFinRobotPublicLiveDataOK(t, vm, "ECB daily FX reference rates", "ecb_fx_status", "ecb_fx_request_error", "ecb_fx_json_error", "ecb_fx_ok")
	body := mustGetString(t, vm, "ecb_fx_body")
	bodyLen := mustGetInt(t, vm, "ecb_fx_body_len")
	contentType := mustGetString(t, vm, "ecb_fx_content_type")

	fmt.Printf("ecb_fx content_type=%q body_len=%d\n", contentType, bodyLen)
	if bodyLen <= 0 || strings.TrimSpace(body) == "" {
		t.Fatalf("ECB daily FX reference rates body is empty: len=%d", bodyLen)
	}
	if !strings.Contains(body, "gesmes:Envelope") {
		t.Fatalf("ECB daily FX reference rates body missing gesmes:Envelope")
	}
	if !strings.Contains(body, "currency='USD'") && !strings.Contains(body, `currency="USD"`) {
		t.Fatalf("ECB daily FX reference rates body missing USD currency entry")
	}
	if !strings.Contains(body, "rate='") && !strings.Contains(body, `rate="`) {
		t.Fatalf("ECB daily FX reference rates body missing rate attribute")
	}

	match := regexp.MustCompile(`currency=['"]USD['"][^>]*rate=['"]([0-9]+(?:\.[0-9]+)?)['"]`).FindStringSubmatch(body)
	if len(match) != 2 {
		t.Fatalf("ECB daily FX reference rates body missing parseable USD rate")
	}
	usdRate, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		t.Fatalf("ECB daily FX reference rates USD rate %q is not numeric: %v", match[1], err)
	}
	if usdRate <= 0 {
		t.Fatalf("ECB daily FX reference rates USD rate = %f, want > 0", usdRate)
	}
	fmt.Printf("ecb_fx usd_rate=%f\n", usdRate)
}
