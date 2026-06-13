package leia_test

import (
	"fmt"
	"strings"
	"testing"
)

func TestFinRobotLiveCoinbaseBTCTickerDataIntegration(t *testing.T) {
	vm := newFinRobotLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
coinbase_btc_request_error := nil
coinbase_btc_json_error := nil
coinbase_btc_status := 0
coinbase_btc_ok := false
coinbase_btc_price := ""
coinbase_btc_bid := ""
coinbase_btc_ask := ""
coinbase_btc_volume := ""
coinbase_btc_time := ""
coinbase_btc_trade_id := 0

headers := {}
headers["User-Agent"] = "Mozilla/5.0 Leia FinRobot live data smoke"
headers["Accept"] = "application/json"

resp, err := net.get("https://api.exchange.coinbase.com/products/BTC-USD/ticker", {
    headers: headers
    timeout: 30
})
if err != nil {
    coinbase_btc_request_error = err
} else {
    coinbase_btc_status = resp.status
    coinbase_btc_ok = resp.ok
    if resp.ok {
        data, json_err := resp.json()
        if json_err != nil {
            coinbase_btc_json_error = json_err
        } else {
            coinbase_btc_price = data.price
            coinbase_btc_bid = data.bid
            coinbase_btc_ask = data.ask
            coinbase_btc_volume = data.volume
            coinbase_btc_time = data.time
            coinbase_btc_trade_id = data.trade_id
        }
    } else {
        coinbase_btc_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	assertFinRobotPublicLiveDataOK(t, vm, "Coinbase BTC ticker", "coinbase_btc_status", "coinbase_btc_request_error", "coinbase_btc_json_error", "coinbase_btc_ok")
	price := mustGetString(t, vm, "coinbase_btc_price")
	bid := mustGetString(t, vm, "coinbase_btc_bid")
	ask := mustGetString(t, vm, "coinbase_btc_ask")
	volume := mustGetString(t, vm, "coinbase_btc_volume")
	timestamp := mustGetString(t, vm, "coinbase_btc_time")
	tradeID := mustGetInt(t, vm, "coinbase_btc_trade_id")
	fmt.Printf("coinbase_btc price=%q bid=%q ask=%q volume=%q time=%q trade_id=%d\n", price, bid, ask, volume, timestamp, tradeID)
	if price == "" || bid == "" || ask == "" || volume == "" || tradeID <= 0 || !strings.Contains(timestamp, "T") {
		t.Fatalf("unexpected Coinbase BTC ticker payload: price=%q bid=%q ask=%q volume=%q time=%q trade_id=%d", price, bid, ask, volume, timestamp, tradeID)
	}
}
