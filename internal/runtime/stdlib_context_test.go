package runtime

import "testing"

func TestContextWithCancel(t *testing.T) {
	interp := runSyncTestScript(t, `
ctx, cancel := context.withCancel()
cancel()
result := "none"
select {
case <-ctx.done:
    result = ctx.err()
case <-time.after(0.01):
    result = "timeout"
}
`)
	if got := interp.GetGlobal("result"); !got.IsString() || got.Str() != "cancelled" {
		t.Fatalf("result = %v, want cancelled", got)
	}
}

func TestContextWithTimeout(t *testing.T) {
	interp := runSyncTestScript(t, `
ctx, cancel := context.withTimeout(0.001)
_ = cancel
result := "none"
select {
case <-ctx.done:
    result = ctx.err()
case <-time.after(0.05):
    result = "missed"
}
`)
	if got := interp.GetGlobal("result"); !got.IsString() || got.Str() != "deadline exceeded" {
		t.Fatalf("result = %v, want deadline exceeded", got)
	}
}

func TestContextSleepCompletes(t *testing.T) {
	interp := runSyncTestScript(t, `
ctx := context.background()
ok, err := time.sleep(ctx, 0.001)
`)
	if got := interp.GetGlobal("ok"); !got.IsBool() || !got.Bool() {
		t.Fatalf("ok = %v, want true", got)
	}
	if got := interp.GetGlobal("err"); !got.IsNil() {
		t.Fatalf("err = %v, want nil", got)
	}
}

func TestContextSleepCancelled(t *testing.T) {
	interp := runSyncTestScript(t, `
ctx, cancel := context.withTimeout(0.001)
_ = cancel
t0 := time.now()
ok, err := time.sleep(ctx, 0.05)
elapsed := time.since(t0)
`)
	if got := interp.GetGlobal("ok"); !got.IsBool() || got.Bool() {
		t.Fatalf("ok = %v, want false", got)
	}
	if got := interp.GetGlobal("err"); !got.IsString() || got.Str() != "deadline exceeded" {
		t.Fatalf("err = %v, want deadline exceeded", got)
	}
	if got := interp.GetGlobal("elapsed"); !got.IsFloat() || got.Number() >= 0.04 {
		t.Fatalf("elapsed = %v, want cancelled before full sleep", got)
	}
}
