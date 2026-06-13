package leia_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	leia "github.com/never-labs/leia"
)

func TestFinRobotLiveSECCompanyConceptDataIntegration(t *testing.T) {
	if strings.EqualFold(strings.TrimSpace(os.Getenv("LEIA_FINROBOT_LIVE_DATA")), "0") {
		t.Skip("set LEIA_FINROBOT_LIVE_DATA to a non-zero value to run the FinRobot live data gate")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	vm := leia.New(leia.WithLibs(leia.LibString | leia.LibNet | leia.LibJSON))
	if err := vm.ExecContext(ctx, `
sec_live_data_error := nil
sec_live_data_status := 0
sec_live_data_ok := false
sec_live_data_entity := ""
sec_live_data_tag := ""
sec_live_data_taxonomy := ""
sec_live_data_unit_count := 0

headers := {}
headers["User-Agent"] = "Leia FinRobot live data smoke contact=opensource@example.invalid"
headers["Accept"] = "application/json"

resp, err := net.get("https://data.sec.gov/api/xbrl/companyconcept/CIK0000320193/us-gaap/Assets.json", {
    headers: headers
    timeout: 30
})
if err != nil {
    sec_live_data_error = err
} else {
    sec_live_data_status = resp.status
    sec_live_data_ok = resp.ok
    if resp.ok {
        data, json_err := resp.json()
        if json_err != nil {
            sec_live_data_error = json_err
        } else {
            sec_live_data_entity = data.entityName
            sec_live_data_tag = data.tag
            sec_live_data_taxonomy = data.taxonomy
            sec_live_data_unit_count = #data.units.USD
        }
    } else {
        sec_live_data_error = resp.statusText
    }
}
`); err != nil {
		t.Fatalf("ExecContext: %v", err)
	}

	status := mustGetInt(t, vm, "sec_live_data_status")
	if status == 403 || status == 429 || status >= 500 {
		t.Skipf("SEC live data endpoint unavailable for this run: status=%d error=%v", status, getOrNil(t, vm, "sec_live_data_error"))
	}
	if got := getOrNil(t, vm, "sec_live_data_error"); got != nil {
		t.Skipf("SEC live data request failed: status=%d error=%v", status, got)
	}
	if status != 200 {
		t.Fatalf("SEC live data status = %d, want 200", status)
	}
	if ok := mustGetBool(t, vm, "sec_live_data_ok"); !ok {
		t.Fatalf("SEC live data ok = false")
	}
	entity := mustGetString(t, vm, "sec_live_data_entity")
	tag := mustGetString(t, vm, "sec_live_data_tag")
	taxonomy := mustGetString(t, vm, "sec_live_data_taxonomy")
	unitCount := mustGetInt(t, vm, "sec_live_data_unit_count")
	fmt.Printf("sec_entity=%q tag=%q taxonomy=%q units=%d\n", entity, tag, taxonomy, unitCount)
	if !strings.Contains(entity, "Apple") || tag != "Assets" || taxonomy != "us-gaap" || unitCount <= 0 {
		t.Fatalf("unexpected SEC companyconcept payload: entity=%q tag=%q taxonomy=%q units=%d", entity, tag, taxonomy, unitCount)
	}
}

func getOrNil(t *testing.T, vm *leia.VM, name string) any {
	t.Helper()
	got, err := vm.Get(name)
	if err != nil {
		t.Fatalf("Get %s: %v", name, err)
	}
	return got
}

func mustGetString(t *testing.T, vm *leia.VM, name string) string {
	t.Helper()
	return fmt.Sprint(getOrNil(t, vm, name))
}

func mustGetInt(t *testing.T, vm *leia.VM, name string) int64 {
	t.Helper()
	got := getOrNil(t, vm, name)
	switch v := got.(type) {
	case int:
		return int64(v)
	case int64:
		return v
	case float64:
		return int64(v)
	default:
		t.Fatalf("%s = %#v (%T), want integer", name, got, got)
		return 0
	}
}

func mustGetBool(t *testing.T, vm *leia.VM, name string) bool {
	t.Helper()
	got := getOrNil(t, vm, name)
	value, ok := got.(bool)
	if !ok {
		t.Fatalf("%s = %#v (%T), want bool", name, got, got)
	}
	return value
}
