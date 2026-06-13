package leia_test

import (
	"fmt"
	"strings"
	"testing"
)

func TestFinRobotLiveGDELTFinanceNewsDataIntegration(t *testing.T) {
	vm := newFinRobotLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
gdelt_news_request_error := nil
gdelt_news_json_error := nil
gdelt_news_status := 0
gdelt_news_ok := false
gdelt_news_count := 0
gdelt_news_title := ""
gdelt_news_domain := ""
gdelt_news_seendate := ""
gdelt_news_url := ""

headers := {}
headers["Accept"] = "application/json"

resp, err := net.get("https://api.gdeltproject.org/api/v2/doc/doc?query=AAPL&mode=artlist&format=json&maxrecords=5&sort=hybridrel", {
    headers: headers
    timeout: 30
})
if err != nil {
    gdelt_news_request_error = err
} else {
    gdelt_news_status = resp.status
    gdelt_news_ok = resp.ok
    if resp.ok {
        data, json_err := resp.json()
        if json_err != nil {
            gdelt_news_json_error = json_err
        } else {
            gdelt_news_count = #data.articles
            if gdelt_news_count > 0 {
                row := data.articles[1]
                gdelt_news_title = row.title
                gdelt_news_domain = row.domain
                gdelt_news_seendate = row.seendate
                gdelt_news_url = row.url
            }
        }
    } else {
        gdelt_news_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	assertFinRobotPublicLiveDataOK(t, vm, "GDELT finance news", "gdelt_news_status", "gdelt_news_request_error", "gdelt_news_json_error", "gdelt_news_ok")
	count := mustGetInt(t, vm, "gdelt_news_count")
	title := mustGetString(t, vm, "gdelt_news_title")
	domain := mustGetString(t, vm, "gdelt_news_domain")
	seendate := mustGetString(t, vm, "gdelt_news_seendate")
	url := mustGetString(t, vm, "gdelt_news_url")
	fmt.Printf("gdelt_news count=%d title=%q domain=%q seendate=%q url=%q\n", count, title, domain, seendate, url)
	if count <= 0 || title == "" || domain == "" || seendate == "" || !strings.HasPrefix(url, "http") {
		t.Fatalf("unexpected GDELT finance news payload: count=%d title=%q domain=%q seendate=%q url=%q", count, title, domain, seendate, url)
	}
}
