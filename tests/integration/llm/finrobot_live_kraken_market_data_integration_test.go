package leia_test

import (
	"fmt"
	"strconv"
	"testing"
)

func TestFinRobotLiveKrakenBTCTickerDataIntegration(t *testing.T) {
	vm := newFinRobotLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
kraken_btc_request_error := nil
kraken_btc_json_error := nil
kraken_btc_status := 0
kraken_btc_ok := false
kraken_btc_error_count := 0
kraken_btc_pair := ""
kraken_btc_ask := ""
kraken_btc_bid := ""
kraken_btc_last := ""
kraken_btc_volume_today := ""
kraken_btc_volume_24h := ""
kraken_btc_vwap_today := ""
kraken_btc_trades_today := 0
kraken_btc_low_24h := ""
kraken_btc_high_24h := ""
kraken_btc_open := ""

headers := {}
headers["User-Agent"] = "Mozilla/5.0 Leia FinRobot live data smoke"
headers["Accept"] = "application/json"

resp, err := net.get("https://api.kraken.com/0/public/Ticker?pair=XBTUSD", {
    headers: headers
    timeout: 30
})
if err != nil {
    kraken_btc_request_error = err
} else {
    kraken_btc_status = resp.status
    kraken_btc_ok = resp.ok
    if resp.ok {
        data, json_err := resp.json()
        if json_err != nil {
            kraken_btc_json_error = json_err
        } else {
            if data.error != nil {
                kraken_btc_error_count = #data.error
            }
            if data.result != nil && data.result.XXBTZUSD != nil {
                row := data.result.XXBTZUSD
                kraken_btc_pair = "XXBTZUSD"
                if row.a != nil && #row.a > 0 {
                    kraken_btc_ask = row.a[1]
                }
                if row.b != nil && #row.b > 0 {
                    kraken_btc_bid = row.b[1]
                }
                if row.c != nil && #row.c > 0 {
                    kraken_btc_last = row.c[1]
                }
                if row.v != nil && #row.v >= 2 {
                    kraken_btc_volume_today = row.v[1]
                    kraken_btc_volume_24h = row.v[2]
                }
                if row.p != nil && #row.p > 0 {
                    kraken_btc_vwap_today = row.p[1]
                }
                if row.t != nil && #row.t > 0 {
                    kraken_btc_trades_today = row.t[1]
                }
                if row.l != nil && #row.l >= 2 {
                    kraken_btc_low_24h = row.l[2]
                }
                if row.h != nil && #row.h >= 2 {
                    kraken_btc_high_24h = row.h[2]
                }
                kraken_btc_open = row.o
            }
        }
    } else {
        kraken_btc_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	assertFinRobotPublicLiveDataOK(t, vm, "Kraken BTC ticker", "kraken_btc_status", "kraken_btc_request_error", "kraken_btc_json_error", "kraken_btc_ok")
	errorCount := mustGetInt(t, vm, "kraken_btc_error_count")
	pair := mustGetString(t, vm, "kraken_btc_pair")
	askText := mustGetString(t, vm, "kraken_btc_ask")
	bidText := mustGetString(t, vm, "kraken_btc_bid")
	lastText := mustGetString(t, vm, "kraken_btc_last")
	volumeTodayText := mustGetString(t, vm, "kraken_btc_volume_today")
	volume24hText := mustGetString(t, vm, "kraken_btc_volume_24h")
	vwapTodayText := mustGetString(t, vm, "kraken_btc_vwap_today")
	tradesToday := mustGetInt(t, vm, "kraken_btc_trades_today")
	low24hText := mustGetString(t, vm, "kraken_btc_low_24h")
	high24hText := mustGetString(t, vm, "kraken_btc_high_24h")
	openText := mustGetString(t, vm, "kraken_btc_open")

	ask := mustParsePositiveMarketFloat(t, "Kraken BTC ask", askText)
	bid := mustParsePositiveMarketFloat(t, "Kraken BTC bid", bidText)
	last := mustParsePositiveMarketFloat(t, "Kraken BTC last", lastText)
	volumeToday := mustParsePositiveMarketFloat(t, "Kraken BTC today volume", volumeTodayText)
	volume24h := mustParsePositiveMarketFloat(t, "Kraken BTC 24h volume", volume24hText)
	vwapToday := mustParsePositiveMarketFloat(t, "Kraken BTC today VWAP", vwapTodayText)
	low24h := mustParsePositiveMarketFloat(t, "Kraken BTC 24h low", low24hText)
	high24h := mustParsePositiveMarketFloat(t, "Kraken BTC 24h high", high24hText)
	open := mustParsePositiveMarketFloat(t, "Kraken BTC open", openText)

	fmt.Printf("kraken_btc pair=%q ask=%f bid=%f last=%f volume_today=%f volume_24h=%f vwap_today=%f trades_today=%d low_24h=%f high_24h=%f open=%f errors=%d\n", pair, ask, bid, last, volumeToday, volume24h, vwapToday, tradesToday, low24h, high24h, open, errorCount)
	if errorCount != 0 || pair != "XXBTZUSD" {
		t.Fatalf("unexpected Kraken BTC envelope: pair=%q errors=%d", pair, errorCount)
	}
	if bid > ask || last < low24h || last > high24h || open < low24h || open > high24h {
		t.Fatalf("unexpected Kraken BTC market ordering: bid=%f ask=%f last=%f low_24h=%f high_24h=%f open=%f", bid, ask, last, low24h, high24h, open)
	}
	if tradesToday <= 0 || volumeToday <= 0 || volume24h <= 0 || vwapToday <= 0 {
		t.Fatalf("unexpected Kraken BTC liquidity metrics: trades_today=%d volume_today=%f volume_24h=%f vwap_today=%f", tradesToday, volumeToday, volume24h, vwapToday)
	}
}

func mustParsePositiveMarketFloat(t *testing.T, label, value string) float64 {
	t.Helper()
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		t.Fatalf("%s = %q, want numeric string: %v", label, value, err)
	}
	if parsed <= 0 {
		t.Fatalf("%s = %f, want positive", label, parsed)
	}
	return parsed
}
