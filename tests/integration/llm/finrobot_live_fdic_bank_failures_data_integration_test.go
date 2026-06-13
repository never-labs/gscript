package leia_test

import (
	"fmt"
	"testing"
)

func TestFinRobotLiveFDICBankFailuresDataIntegration(t *testing.T) {
	vm := newFinRobotLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
fdic_failures_request_error := nil
fdic_failures_json_error := nil
fdic_failures_status := 0
fdic_failures_ok := false
fdic_failures_total := 0
fdic_failures_count := 0
fdic_failures_bank_name := ""
fdic_failures_city := ""
fdic_failures_fail_date := ""
fdic_failures_cert := 0
fdic_failures_savr := ""

headers := {}
headers["User-Agent"] = "Mozilla/5.0 Leia FinRobot live data smoke"
headers["Accept"] = "application/json"

resp, err := net.get("https://api.fdic.gov/banks/failures?fields=NAME,CITY,ST,CERT,FAILDATE,SAVR&sort_by=FAILDATE&sort_order=DESC&limit=5&format=json", {
    headers: headers
    timeout: 30
})
if err != nil {
    fdic_failures_request_error = err
} else {
    fdic_failures_status = resp.status
    fdic_failures_ok = resp.ok
    if resp.ok {
        data, json_err := resp.json()
        if json_err != nil {
            fdic_failures_json_error = json_err
        } else {
            fdic_failures_total = data.totals.count
            fdic_failures_count = #data.data
            if fdic_failures_count > 0 {
                row := data.data[1].data
                fdic_failures_bank_name = row.NAME
                fdic_failures_city = row.CITY
                fdic_failures_fail_date = row.FAILDATE
                fdic_failures_cert = row.CERT
                fdic_failures_savr = row.SAVR
            }
        }
    } else {
        fdic_failures_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	assertFinRobotPublicLiveDataOK(t, vm, "FDIC bank failures", "fdic_failures_status", "fdic_failures_request_error", "fdic_failures_json_error", "fdic_failures_ok")
	total := mustGetInt(t, vm, "fdic_failures_total")
	count := mustGetInt(t, vm, "fdic_failures_count")
	name := mustGetString(t, vm, "fdic_failures_bank_name")
	city := mustGetString(t, vm, "fdic_failures_city")
	failDate := mustGetString(t, vm, "fdic_failures_fail_date")
	cert := mustGetInt(t, vm, "fdic_failures_cert")
	savr := mustGetString(t, vm, "fdic_failures_savr")
	fmt.Printf("fdic_failures total=%d count=%d name=%q city=%q fail_date=%q cert=%d savr=%q\n", total, count, name, city, failDate, cert, savr)
	if total <= 0 || count <= 0 || name == "" || city == "" || failDate == "" || cert <= 0 || savr == "" {
		t.Fatalf("unexpected FDIC bank failures payload: total=%d count=%d name=%q city=%q fail_date=%q cert=%d savr=%q", total, count, name, city, failDate, cert, savr)
	}
}
