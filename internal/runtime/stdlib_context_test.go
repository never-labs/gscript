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
