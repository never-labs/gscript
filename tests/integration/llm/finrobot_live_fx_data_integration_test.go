package leia_test

import (
	"fmt"
	"testing"
)

func TestFinRobotLiveFrankfurterFXDataIntegration(t *testing.T) {
	vm := newFinRobotLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
fx_request_error := nil
fx_json_error := nil
fx_status := 0
fx_ok := false
fx_amount := 0.0
fx_base := ""
fx_date := ""
fx_eur := 0.0
fx_jpy := 0.0

headers := {}
headers["User-Agent"] = "Mozilla/5.0 Leia FinRobot live data smoke"
headers["Accept"] = "application/json"

resp, err := net.get("https://api.frankfurter.app/latest?from=USD&to=EUR,JPY", {
    headers: headers
    timeout: 30
})
if err != nil {
    fx_request_error = err
} else {
    fx_status = resp.status
    fx_ok = resp.ok
    if resp.ok {
        data, json_err := resp.json()
        if json_err != nil {
            fx_json_error = json_err
        } else {
            fx_amount = data.amount
            fx_base = data.base
            fx_date = data.date
            fx_eur = data.rates.EUR
            fx_jpy = data.rates.JPY
        }
    } else {
        fx_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	assertFinRobotPublicLiveDataOK(t, vm, "Frankfurter FX", "fx_status", "fx_request_error", "fx_json_error", "fx_ok")
	amount := mustGetFloat(t, vm, "fx_amount")
	base := mustGetString(t, vm, "fx_base")
	date := mustGetString(t, vm, "fx_date")
	eur := mustGetFloat(t, vm, "fx_eur")
	jpy := mustGetFloat(t, vm, "fx_jpy")
	fmt.Printf("frankfurter_fx amount=%f base=%q date=%q eur=%f jpy=%f\n", amount, base, date, eur, jpy)
	if amount != 1 || base != "USD" || date == "" || eur <= 0 || jpy <= 0 {
		t.Fatalf("unexpected Frankfurter FX payload: amount=%f base=%q date=%q eur=%f jpy=%f", amount, base, date, eur, jpy)
	}
}
