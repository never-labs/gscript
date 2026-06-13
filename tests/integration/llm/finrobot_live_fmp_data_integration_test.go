package leia_test

import (
	"fmt"
	"strings"
	"testing"
)

func TestFinRobotLiveFMPCompanyProfileDataIntegration(t *testing.T) {
	vm := newFinRobotFMPLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
fmp_profile_request_error := nil
fmp_profile_json_error := nil
fmp_profile_status := 0
fmp_profile_ok := false
fmp_profile_symbol := ""
fmp_profile_name := ""
fmp_profile_exchange := ""
fmp_profile_sector := ""
fmp_profile_currency := ""

url := "https://financialmodelingprep.com/stable/profile?symbol=AAPL&apikey=" .. os.getenv("LEIA_FMP_API_KEY")
resp, err := net.get(url, {timeout: 30})
if err != nil {
    fmp_profile_request_error = err
} else {
    fmp_profile_status = resp.status
    fmp_profile_ok = resp.ok
    if resp.ok {
        data, json_err := resp.json()
        if json_err != nil {
            fmp_profile_json_error = json_err
        } else {
            row := data[1]
            fmp_profile_symbol = row.symbol
            fmp_profile_name = row.companyName
            fmp_profile_exchange = row.exchange
            fmp_profile_sector = row.sector
            fmp_profile_currency = row.currency
        }
    } else {
        fmp_profile_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	status := mustGetInt(t, vm, "fmp_profile_status")
	skipUnavailableFinRobotLiveData(t, "FMP profile", status, getOrNil(t, vm, "fmp_profile_request_error"))
	if status != 200 {
		t.Fatalf("FMP profile status = %d, want 200", status)
	}
	if got := getOrNil(t, vm, "fmp_profile_json_error"); got != nil {
		t.Fatalf("FMP profile JSON decode failed: %v", got)
	}
	if ok := mustGetBool(t, vm, "fmp_profile_ok"); !ok {
		t.Fatalf("FMP profile ok = false")
	}
	symbol := mustGetString(t, vm, "fmp_profile_symbol")
	name := mustGetString(t, vm, "fmp_profile_name")
	exchange := mustGetString(t, vm, "fmp_profile_exchange")
	sector := mustGetString(t, vm, "fmp_profile_sector")
	currency := mustGetString(t, vm, "fmp_profile_currency")
	fmt.Printf("fmp_profile symbol=%q name=%q exchange=%q sector=%q currency=%q\n", symbol, name, exchange, sector, currency)
	if symbol != "AAPL" || !strings.Contains(strings.ToLower(name), "apple") || exchange == "" || sector == "" || currency == "" {
		t.Fatalf("unexpected FMP profile payload: symbol=%q name=%q exchange=%q sector=%q currency=%q", symbol, name, exchange, sector, currency)
	}
}

func TestFinRobotLiveFMPHistoricalPriceLightDataIntegration(t *testing.T) {
	vm := newFinRobotFMPLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
fmp_price_request_error := nil
fmp_price_json_error := nil
fmp_price_status := 0
fmp_price_ok := false
fmp_price_date := ""
fmp_price_close := 0.0
fmp_price_volume := 0

url := "https://financialmodelingprep.com/stable/historical-price-eod/light?symbol=AAPL&apikey=" .. os.getenv("LEIA_FMP_API_KEY")
resp, err := net.get(url, {timeout: 30})
if err != nil {
    fmp_price_request_error = err
} else {
    fmp_price_status = resp.status
    fmp_price_ok = resp.ok
    if resp.ok {
        data, json_err := resp.json()
        if json_err != nil {
            fmp_price_json_error = json_err
        } else {
            row := data[1]
            fmp_price_date = row.date
            fmp_price_close = row.price
            fmp_price_volume = row.volume
        }
    } else {
        fmp_price_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	status := mustGetInt(t, vm, "fmp_price_status")
	skipUnavailableFinRobotLiveData(t, "FMP historical-price-eod light", status, getOrNil(t, vm, "fmp_price_request_error"))
	if status != 200 {
		t.Fatalf("FMP historical-price-eod light status = %d, want 200", status)
	}
	if got := getOrNil(t, vm, "fmp_price_json_error"); got != nil {
		t.Fatalf("FMP historical-price-eod light JSON decode failed: %v", got)
	}
	if ok := mustGetBool(t, vm, "fmp_price_ok"); !ok {
		t.Fatalf("FMP historical-price-eod light ok = false")
	}
	date := mustGetString(t, vm, "fmp_price_date")
	closePrice := mustGetFloat(t, vm, "fmp_price_close")
	volume := mustGetInt(t, vm, "fmp_price_volume")
	fmt.Printf("fmp_price date=%q price=%f volume=%d\n", date, closePrice, volume)
	if date == "" || closePrice <= 0 || volume < 0 {
		t.Fatalf("unexpected FMP historical-price-eod light payload: date=%q price=%f volume=%d", date, closePrice, volume)
	}
}
