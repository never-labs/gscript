package leia_test

import (
	"fmt"
	"strings"
	"testing"
)

func TestFinRobotLiveSECCurrentFilingsAtomDataIntegration(t *testing.T) {
	vm := newFinRobotLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
sec_current_filings_request_error := nil
sec_current_filings_status := 0
sec_current_filings_ok := false
sec_current_filings_body_len := 0
sec_current_filings_content_type := ""
sec_current_filings_has_feed := false
sec_current_filings_has_entry := false
sec_current_filings_has_10k := false
sec_current_filings_has_browse_edgar := false
sec_current_filings_has_archive := false

headers := {}
headers["User-Agent"] = os.getenv("LEIA_SEC_USER_AGENT")
headers["Accept"] = "application/atom+xml,text/xml"

resp, err := net.get("https://www.sec.gov/cgi-bin/browse-edgar?action=getcurrent&type=10-K&count=10&output=atom", {
    headers: headers
    timeout: 30
})
if err != nil {
    sec_current_filings_request_error = err
} else {
    sec_current_filings_status = resp.status
    sec_current_filings_ok = resp.ok
    if resp.headers != nil && resp.headers["Content-Type"] != nil {
        sec_current_filings_content_type = resp.headers["Content-Type"]
    }
    if resp.ok {
        body := resp.body
        sec_current_filings_body_len = #body
        sec_current_filings_has_feed = string.find(body, "<feed", 1, true) != nil
        sec_current_filings_has_entry = string.find(body, "<entry>", 1, true) != nil
        sec_current_filings_has_10k = string.find(body, "10-K", 1, true) != nil
        sec_current_filings_has_browse_edgar = string.find(body, "browse-edgar", 1, true) != nil
        sec_current_filings_has_archive = string.find(body, "Archives/edgar", 1, true) != nil
    } else {
        sec_current_filings_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	status := mustGetInt(t, vm, "sec_current_filings_status")
	skipUnavailableFinRobotLiveData(t, "SEC current filings Atom", status, getOrNil(t, vm, "sec_current_filings_request_error"))
	if status != 200 {
		t.Fatalf("SEC current filings Atom status = %d, want 200", status)
	}
	if ok := mustGetBool(t, vm, "sec_current_filings_ok"); !ok {
		t.Fatalf("SEC current filings Atom ok = false")
	}
	bodyLen := mustGetInt(t, vm, "sec_current_filings_body_len")
	contentType := mustGetString(t, vm, "sec_current_filings_content_type")
	fmt.Printf("sec_current_filings_content_type=%q body_len=%d\n", contentType, bodyLen)
	if bodyLen <= 500 {
		t.Fatalf("SEC current filings Atom body length = %d, want > 500", bodyLen)
	}

	for name, ok := range map[string]bool{
		"<feed":            mustGetBool(t, vm, "sec_current_filings_has_feed"),
		"<entry>":          mustGetBool(t, vm, "sec_current_filings_has_entry"),
		"10-K":             mustGetBool(t, vm, "sec_current_filings_has_10k"),
		"browse-edgar/url": mustGetBool(t, vm, "sec_current_filings_has_browse_edgar") || mustGetBool(t, vm, "sec_current_filings_has_archive"),
	} {
		if !ok {
			t.Fatalf("SEC current filings Atom body missing %s", name)
		}
	}
	if ct := strings.ToLower(contentType); ct != "" {
		t.Logf("SEC current filings Atom Content-Type: %s", ct)
	}
}
