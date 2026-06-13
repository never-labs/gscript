package leia_test

import (
	"fmt"
	"strings"
	"testing"
)

func TestFinRobotLiveYahooNewsSearchDataIntegration(t *testing.T) {
	vm := newFinRobotLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
yahoo_news_request_error := nil
yahoo_news_json_error := nil
yahoo_news_status := 0
yahoo_news_ok := false
yahoo_news_count := 0
yahoo_news_title := ""
yahoo_news_publisher := ""
yahoo_news_link := ""
yahoo_news_type := ""
yahoo_news_publish_time := 0

headers := {}
headers["User-Agent"] = "Mozilla/5.0 Leia FinRobot live data smoke"
headers["Accept"] = "application/json"

resp, err := net.get("https://query2.finance.yahoo.com/v1/finance/search?q=AAPL&quotesCount=0&newsCount=5", {
    headers: headers
    timeout: 30
})
if err != nil {
    yahoo_news_request_error = err
} else {
    yahoo_news_status = resp.status
    yahoo_news_ok = resp.ok
    if resp.ok {
        data, json_err := resp.json()
        if json_err != nil {
            yahoo_news_json_error = json_err
        } else {
            yahoo_news_count = #data.news
            if yahoo_news_count > 0 {
                row := data.news[1]
                yahoo_news_title = row.title
                yahoo_news_publisher = row.publisher
                yahoo_news_link = row.link
                yahoo_news_type = row.type
                yahoo_news_publish_time = row.providerPublishTime
            }
        }
    } else {
        yahoo_news_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	assertFinRobotPublicLiveDataOK(t, vm, "Yahoo news search", "yahoo_news_status", "yahoo_news_request_error", "yahoo_news_json_error", "yahoo_news_ok")
	count := mustGetInt(t, vm, "yahoo_news_count")
	title := mustGetString(t, vm, "yahoo_news_title")
	publisher := mustGetString(t, vm, "yahoo_news_publisher")
	link := mustGetString(t, vm, "yahoo_news_link")
	newsType := mustGetString(t, vm, "yahoo_news_type")
	publishTime := mustGetInt(t, vm, "yahoo_news_publish_time")
	fmt.Printf("yahoo_news count=%d title=%q publisher=%q type=%q publish_time=%d link=%q\n", count, title, publisher, newsType, publishTime, link)
	if count == 0 {
		t.Skip("Yahoo news search returned no live news rows for this run")
	}
	if count <= 0 || title == "" || publisher == "" || newsType == "" || publishTime <= 0 || !strings.HasPrefix(link, "https://") {
		t.Fatalf("unexpected Yahoo news payload: count=%d title=%q publisher=%q type=%q publish_time=%d link=%q", count, title, publisher, newsType, publishTime, link)
	}
}
