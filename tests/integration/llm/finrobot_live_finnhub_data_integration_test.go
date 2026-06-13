package leia_test

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestFinRobotLiveFinnhubProfileDataIntegration(t *testing.T) {
	vm := newFinRobotFinnhubLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
finnhub_profile_request_error := nil
finnhub_profile_json_error := nil
finnhub_profile_status := 0
finnhub_profile_ok := false
finnhub_profile_ticker := ""
finnhub_profile_name := ""
finnhub_profile_exchange := ""
finnhub_profile_industry := ""
finnhub_profile_currency := ""

url := "https://finnhub.io/api/v1/stock/profile2?symbol=AAPL&token=" .. os.getenv("LEIA_FINNHUB_TOKEN")
resp, err := net.get(url, {timeout: 30})
if err != nil {
    finnhub_profile_request_error = err
} else {
    finnhub_profile_status = resp.status
    finnhub_profile_ok = resp.ok
    if resp.ok {
        data, json_err := resp.json()
        if json_err != nil {
            finnhub_profile_json_error = json_err
        } else {
            finnhub_profile_ticker = data.ticker
            finnhub_profile_name = data.name
            finnhub_profile_exchange = data.exchange
            finnhub_profile_industry = data.finnhubIndustry
            finnhub_profile_currency = data.currency
        }
    } else {
        finnhub_profile_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	assertFinRobotLiveDataOK(t, vm, "Finnhub profile", "finnhub_profile_status", "finnhub_profile_request_error", "finnhub_profile_json_error", "finnhub_profile_ok")
	ticker := mustGetString(t, vm, "finnhub_profile_ticker")
	name := mustGetString(t, vm, "finnhub_profile_name")
	exchange := mustGetString(t, vm, "finnhub_profile_exchange")
	industry := mustGetString(t, vm, "finnhub_profile_industry")
	currency := mustGetString(t, vm, "finnhub_profile_currency")
	fmt.Printf("finnhub_profile ticker=%q name=%q exchange=%q industry=%q currency=%q\n", ticker, name, exchange, industry, currency)
	if ticker != "AAPL" || !strings.Contains(strings.ToLower(name), "apple") || exchange == "" || industry == "" || currency == "" {
		t.Fatalf("unexpected Finnhub profile payload: ticker=%q name=%q exchange=%q industry=%q currency=%q", ticker, name, exchange, industry, currency)
	}
}

func TestFinRobotLiveFinnhubQuoteDataIntegration(t *testing.T) {
	vm := newFinRobotFinnhubLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
finnhub_quote_request_error := nil
finnhub_quote_json_error := nil
finnhub_quote_status := 0
finnhub_quote_ok := false
finnhub_quote_current := 0.0
finnhub_quote_high := 0.0
finnhub_quote_low := 0.0
finnhub_quote_open := 0.0
finnhub_quote_previous_close := 0.0
finnhub_quote_time := 0

url := "https://finnhub.io/api/v1/quote?symbol=AAPL&token=" .. os.getenv("LEIA_FINNHUB_TOKEN")
resp, err := net.get(url, {timeout: 30})
if err != nil {
    finnhub_quote_request_error = err
} else {
    finnhub_quote_status = resp.status
    finnhub_quote_ok = resp.ok
    if resp.ok {
        data, json_err := resp.json()
        if json_err != nil {
            finnhub_quote_json_error = json_err
        } else {
            finnhub_quote_current = data.c
            finnhub_quote_high = data.h
            finnhub_quote_low = data.l
            finnhub_quote_open = data.o
            finnhub_quote_previous_close = data.pc
            finnhub_quote_time = data.t
        }
    } else {
        finnhub_quote_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	assertFinRobotLiveDataOK(t, vm, "Finnhub quote", "finnhub_quote_status", "finnhub_quote_request_error", "finnhub_quote_json_error", "finnhub_quote_ok")
	current := mustGetFloat(t, vm, "finnhub_quote_current")
	high := mustGetFloat(t, vm, "finnhub_quote_high")
	low := mustGetFloat(t, vm, "finnhub_quote_low")
	open := mustGetFloat(t, vm, "finnhub_quote_open")
	previousClose := mustGetFloat(t, vm, "finnhub_quote_previous_close")
	timestamp := mustGetInt(t, vm, "finnhub_quote_time")
	fmt.Printf("finnhub_quote current=%f high=%f low=%f open=%f previous_close=%f time=%d\n", current, high, low, open, previousClose, timestamp)
	if current <= 0 || high <= 0 || low <= 0 || open <= 0 || previousClose <= 0 || low > high || timestamp <= 0 {
		t.Fatalf("unexpected Finnhub quote payload: current=%f high=%f low=%f open=%f previous_close=%f time=%d", current, high, low, open, previousClose, timestamp)
	}
}

func TestFinRobotLiveFinnhubMetricsDataIntegration(t *testing.T) {
	vm := newFinRobotFinnhubLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
finnhub_metrics_request_error := nil
finnhub_metrics_json_error := nil
finnhub_metrics_status := 0
finnhub_metrics_ok := false
finnhub_metrics_symbol := ""
finnhub_metrics_type := ""
finnhub_metrics_52w_high := 0.0
finnhub_metrics_52w_low := 0.0
finnhub_metrics_market_cap := 0.0

url := "https://finnhub.io/api/v1/stock/metric?symbol=AAPL&metric=all&token=" .. os.getenv("LEIA_FINNHUB_TOKEN")
resp, err := net.get(url, {timeout: 30})
if err != nil {
    finnhub_metrics_request_error = err
} else {
    finnhub_metrics_status = resp.status
    finnhub_metrics_ok = resp.ok
    if resp.ok {
        data, json_err := resp.json()
        if json_err != nil {
            finnhub_metrics_json_error = json_err
        } else {
            finnhub_metrics_symbol = data.symbol
            finnhub_metrics_type = data.metricType
            finnhub_metrics_52w_high = data.metric["52WeekHigh"]
            finnhub_metrics_52w_low = data.metric["52WeekLow"]
            finnhub_metrics_market_cap = data.metric.marketCapitalization
        }
    } else {
        finnhub_metrics_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	assertFinRobotLiveDataOK(t, vm, "Finnhub metrics", "finnhub_metrics_status", "finnhub_metrics_request_error", "finnhub_metrics_json_error", "finnhub_metrics_ok")
	symbol := mustGetString(t, vm, "finnhub_metrics_symbol")
	metricType := mustGetString(t, vm, "finnhub_metrics_type")
	high52 := mustGetFloat(t, vm, "finnhub_metrics_52w_high")
	low52 := mustGetFloat(t, vm, "finnhub_metrics_52w_low")
	marketCap := mustGetFloat(t, vm, "finnhub_metrics_market_cap")
	fmt.Printf("finnhub_metrics symbol=%q metric_type=%q 52w_high=%f 52w_low=%f market_cap=%f\n", symbol, metricType, high52, low52, marketCap)
	if symbol != "AAPL" || metricType != "all" || high52 <= 0 || low52 <= 0 || low52 > high52 || marketCap <= 0 {
		t.Fatalf("unexpected Finnhub metrics payload: symbol=%q metric_type=%q 52w_high=%f 52w_low=%f market_cap=%f", symbol, metricType, high52, low52, marketCap)
	}
}

func TestFinRobotLiveFinnhubCompanyNewsDataIntegration(t *testing.T) {
	vm := newFinRobotFinnhubLiveDataVM(t)
	to := time.Now().UTC().Format("2006-01-02")
	from := time.Now().UTC().AddDate(0, 0, -14).Format("2006-01-02")
	if err := execFinRobotLiveDataScript(t, vm, fmt.Sprintf(`
finnhub_news_request_error := nil
finnhub_news_json_error := nil
finnhub_news_status := 0
finnhub_news_ok := false
finnhub_news_count := 0
finnhub_news_category := ""
finnhub_news_datetime := 0
finnhub_news_headline := ""
finnhub_news_source := ""
finnhub_news_summary := ""
finnhub_news_url := ""

url := "https://finnhub.io/api/v1/company-news?symbol=AAPL&from=%s&to=%s&token=" .. os.getenv("LEIA_FINNHUB_TOKEN")
resp, err := net.get(url, {timeout: 30})
if err != nil {
    finnhub_news_request_error = err
} else {
    finnhub_news_status = resp.status
    finnhub_news_ok = resp.ok
    if resp.ok {
        data, json_err := resp.json()
        if json_err != nil {
            finnhub_news_json_error = json_err
        } else {
            finnhub_news_count = #data
            row := data[1]
            finnhub_news_category = row.category
            finnhub_news_datetime = row.datetime
            finnhub_news_headline = row.headline
            finnhub_news_source = row.source
            finnhub_news_summary = row.summary
            finnhub_news_url = row.url
        }
    } else {
        finnhub_news_request_error = resp.statusText
    }
}
`, from, to)); err != nil {
		t.Fatal(err)
	}

	assertFinRobotLiveDataOK(t, vm, "Finnhub company-news", "finnhub_news_status", "finnhub_news_request_error", "finnhub_news_json_error", "finnhub_news_ok")
	count := mustGetInt(t, vm, "finnhub_news_count")
	category := mustGetString(t, vm, "finnhub_news_category")
	datetime := mustGetInt(t, vm, "finnhub_news_datetime")
	headline := mustGetString(t, vm, "finnhub_news_headline")
	source := mustGetString(t, vm, "finnhub_news_source")
	summary := mustGetString(t, vm, "finnhub_news_summary")
	url := mustGetString(t, vm, "finnhub_news_url")
	fmt.Printf("finnhub_news from=%q to=%q count=%d category=%q source=%q headline=%q\n", from, to, count, category, source, headline)
	if count <= 0 || category == "" || datetime <= 0 || headline == "" || source == "" || summary == "" || !strings.HasPrefix(url, "http") {
		t.Fatalf("unexpected Finnhub company-news payload: count=%d category=%q datetime=%d headline=%q source=%q summary=%q url=%q", count, category, datetime, headline, source, summary, url)
	}
}
