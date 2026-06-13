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
	if requestErr != nil {
		t.Skipf("%s live data request failed: status=%d error=%v", provider, status, requestErr)
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
