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

func newFinRobotLiveDataVM(t *testing.T) *leia.VM {
	t.Helper()
	if strings.EqualFold(strings.TrimSpace(os.Getenv("LEIA_FINROBOT_LIVE_DATA")), "0") {
		t.Skip("set LEIA_FINROBOT_LIVE_DATA to a non-zero value to run the FinRobot live data gate")
	}
	userAgent := firstNonEmptyEnv("LEIA_SEC_USER_AGENT", "SEC_USER_AGENT")
	if userAgent == "" {
		userAgent = "Leia FinRobot live data smoke contact=opensource@example.invalid"
	}
	t.Setenv("LEIA_SEC_USER_AGENT", userAgent)
	return leia.New(leia.WithLibs(leia.LibString | leia.LibNet | leia.LibJSON | leia.LibOS))
}

func requireFinRobotFMPAPIKey(t *testing.T) string {
	t.Helper()
	return finRobotOptionalLiveDataToken(t, "FMP", "LEIA_FMP_API_KEY", "FMP_API_KEY")
}

func requireFinRobotFinnhubToken(t *testing.T) string {
	t.Helper()
	return finRobotOptionalLiveDataToken(t, "Finnhub", "LEIA_FINNHUB_TOKEN", "FINNHUB_TOKEN", "FINNHUB_API_KEY")
}

func finRobotOptionalLiveDataToken(t *testing.T, provider string, envNames ...string) string {
	t.Helper()
	token := firstNonEmptyEnv(envNames...)
	if token == "" {
		t.Skipf("set %s to run optional %s live data gates", strings.Join(envNames, " or "), provider)
	}
	return token
}

func newFinRobotFMPLiveDataVM(t *testing.T) *leia.VM {
	t.Helper()
	apiKey := requireFinRobotFMPAPIKey(t)
	t.Setenv("LEIA_FMP_API_KEY", apiKey)
	return newFinRobotLiveDataVM(t)
}

func newFinRobotFinnhubLiveDataVM(t *testing.T) *leia.VM {
	t.Helper()
	token := requireFinRobotFinnhubToken(t)
	t.Setenv("LEIA_FINNHUB_TOKEN", token)
	return newFinRobotLiveDataVM(t)
}

func execFinRobotLiveDataScript(t *testing.T, vm *leia.VM, script string) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	if err := vm.ExecContext(ctx, script); err != nil {
		return fmt.Errorf("ExecContext: %w", err)
	}
	return nil
}

func skipUnavailableFinRobotLiveData(t *testing.T, provider string, status int64, requestErr any) {
	t.Helper()
	if status == 403 || status == 408 || status == 429 || status >= 500 {
		t.Skipf("%s live data endpoint unavailable for this run: status=%d error=%v", provider, status, requestErr)
	}
	if status == 0 && requestErr != nil {
		t.Skipf("%s live data request failed: status=%d error=%v", provider, status, requestErr)
	}
}

func skipUnavailableFinRobotCredentialedLiveData(t *testing.T, provider string, status int64, requestErr any) {
	t.Helper()
	if status == 408 || status == 429 || status >= 500 {
		t.Skipf("%s live data endpoint unavailable for this run: status=%d error=%v", provider, status, requestErr)
	}
	if status == 0 && requestErr != nil {
		t.Skipf("%s live data request failed: status=%d error=%v", provider, status, requestErr)
	}
}

func skipUnavailableFinRobotPublicLiveData(t *testing.T, provider string, status int64, requestErr any) {
	t.Helper()
	if status == 401 {
		t.Skipf("%s public live data endpoint requires auth or blocked this run: status=%d error=%v", provider, status, requestErr)
	}
	skipUnavailableFinRobotLiveData(t, provider, status, requestErr)
}

func assertFinRobotPublicLiveDataOK(t *testing.T, vm *leia.VM, provider, statusName, requestErrName, jsonErrName, okName string) {
	t.Helper()
	status := mustGetInt(t, vm, statusName)
	skipUnavailableFinRobotPublicLiveData(t, provider, status, getOrNil(t, vm, requestErrName))
	if status != 200 {
		t.Fatalf("%s status = %d, want 200", provider, status)
	}
	if got := getOrNil(t, vm, jsonErrName); got != nil {
		t.Fatalf("%s JSON decode failed: %v", provider, got)
	}
	if ok := mustGetBool(t, vm, okName); !ok {
		t.Fatalf("%s ok = false", provider)
	}
}

func assertFinRobotLiveDataOK(t *testing.T, vm *leia.VM, provider, statusName, requestErrName, jsonErrName, okName string) {
	t.Helper()
	status := mustGetInt(t, vm, statusName)
	skipUnavailableFinRobotCredentialedLiveData(t, provider, status, getOrNil(t, vm, requestErrName))
	if status != 200 {
		t.Fatalf("%s status = %d, want 200", provider, status)
	}
	if got := getOrNil(t, vm, jsonErrName); got != nil {
		t.Fatalf("%s JSON decode failed: %v", provider, got)
	}
	if ok := mustGetBool(t, vm, okName); !ok {
		t.Fatalf("%s ok = false", provider)
	}
}

func assertFinRobotLiveDataPrefixOK(t *testing.T, vm *leia.VM, provider, prefix string) {
	t.Helper()
	assertFinRobotLiveDataOK(t, vm, provider, prefix+"_status", prefix+"_request_error", prefix+"_json_error", prefix+"_ok")
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

func mustGetFloat(t *testing.T, vm *leia.VM, name string) float64 {
	t.Helper()
	got := getOrNil(t, vm, name)
	switch v := got.(type) {
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case float64:
		return v
	default:
		t.Fatalf("%s = %#v (%T), want number", name, got, got)
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
