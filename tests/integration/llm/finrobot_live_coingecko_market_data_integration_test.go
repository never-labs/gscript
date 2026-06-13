package leia_test

import (
	"fmt"
	"testing"
)

func TestFinRobotLiveCoinGeckoCryptoMarketDataIntegration(t *testing.T) {
	vm := newFinRobotLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
coingecko_market_request_error := nil
coingecko_market_json_error := nil
coingecko_market_status := 0
coingecko_market_ok := false
coingecko_market_btc_usd := 0.0
coingecko_market_btc_market_cap := 0.0
coingecko_market_btc_volume := 0.0
coingecko_market_eth_usd := 0.0
coingecko_market_eth_market_cap := 0.0
coingecko_market_eth_volume := 0.0

headers := {}
headers["User-Agent"] = "Mozilla/5.0 Leia FinRobot live data smoke"
headers["Accept"] = "application/json"

resp, err := net.get("https://api.coingecko.com/api/v3/simple/price?ids=bitcoin,ethereum&vs_currencies=usd&include_market_cap=true&include_24hr_vol=true&include_24hr_change=true", {
    headers: headers
    timeout: 30
})
if err != nil {
    coingecko_market_request_error = err
} else {
    coingecko_market_status = resp.status
    coingecko_market_ok = resp.ok
    if resp.ok {
        data, json_err := resp.json()
        if json_err != nil {
            coingecko_market_json_error = json_err
        } else {
            coingecko_market_btc_usd = data.bitcoin.usd
            coingecko_market_btc_market_cap = data.bitcoin.usd_market_cap
            coingecko_market_btc_volume = data.bitcoin.usd_24h_vol
            coingecko_market_eth_usd = data.ethereum.usd
            coingecko_market_eth_market_cap = data.ethereum.usd_market_cap
            coingecko_market_eth_volume = data.ethereum.usd_24h_vol
        }
    } else {
        coingecko_market_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	assertFinRobotPublicLiveDataOK(t, vm, "CoinGecko crypto market", "coingecko_market_status", "coingecko_market_request_error", "coingecko_market_json_error", "coingecko_market_ok")
	btcUSD := mustGetFloat(t, vm, "coingecko_market_btc_usd")
	btcMarketCap := mustGetFloat(t, vm, "coingecko_market_btc_market_cap")
	btcVolume := mustGetFloat(t, vm, "coingecko_market_btc_volume")
	ethUSD := mustGetFloat(t, vm, "coingecko_market_eth_usd")
	ethMarketCap := mustGetFloat(t, vm, "coingecko_market_eth_market_cap")
	ethVolume := mustGetFloat(t, vm, "coingecko_market_eth_volume")
	fmt.Printf("coingecko_market btc_usd=%f btc_market_cap=%f btc_volume=%f eth_usd=%f eth_market_cap=%f eth_volume=%f\n", btcUSD, btcMarketCap, btcVolume, ethUSD, ethMarketCap, ethVolume)
	if btcUSD <= 0 || btcMarketCap <= 0 || btcVolume <= 0 || ethUSD <= 0 || ethMarketCap <= 0 || ethVolume <= 0 {
		t.Fatalf("unexpected CoinGecko crypto market payload: btc_usd=%f btc_market_cap=%f btc_volume=%f eth_usd=%f eth_market_cap=%f eth_volume=%f", btcUSD, btcMarketCap, btcVolume, ethUSD, ethMarketCap, ethVolume)
	}
}
