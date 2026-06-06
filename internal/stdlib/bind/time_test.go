package bind

import (
	"testing"

	"github.com/never-labs/leia/internal/runtime"
)

func timeInterp(t *testing.T, src string) *runtime.Interpreter {
	t.Helper()
	interp := runtime.NewCore()
	interp.InstallRuntimeStdlib()
	installTestModule(interp, "context", runtime.TableValue(BuildContext()))
	installTestModule(interp, "time", runtime.TableValue(BuildTime()))
	execOnInterp(t, interp, src)
	return interp
}

func TestTimeModuleCoreOperations(t *testing.T) {
	interp := timeInterp(t, `
t := time.unix(0, 500000000)
formatted := time.format(t, "%Y-%m-%d %H:%M:%S")
parsed, parseErr := time.parse("2023-06-15 10:30:00", "%Y-%m-%d %H:%M:%S")
diff := time.diff(time.unix(1000), time.add(time.unix(1000), 60))
dated := time.date(2023, 6, 15, 10, 30, 45)
weekday := time.weekday(time.unix(0))
month := time.month(time.unix(0))
before := time.isBefore(time.unix(1), time.unix(2))
after := time.isAfter(time.unix(2), time.unix(1))
`)
	if got := interp.GetGlobal("formatted"); !got.IsString() || got.Str() != "1970-01-01 00:00:00" {
		t.Fatalf("formatted = %v, want epoch format", got)
	}
	if got := interp.GetGlobal("parseErr"); !got.IsNil() {
		t.Fatalf("parseErr = %v, want nil", got)
	}
	if got := interp.GetGlobal("parsed").Table().RawGetString("year"); got.Int() != 2023 {
		t.Fatalf("parsed.year = %v, want 2023", got)
	}
	if got := interp.GetGlobal("diff"); !got.IsFloat() || got.Number() != 60 {
		t.Fatalf("diff = %v, want 60", got)
	}
	if got := interp.GetGlobal("dated").Table().RawGetString("sec"); got.Int() != 45 {
		t.Fatalf("dated.sec = %v, want 45", got)
	}
	if got := interp.GetGlobal("weekday"); !got.IsString() || got.Str() != "Thursday" {
		t.Fatalf("weekday = %v, want Thursday", got)
	}
	if got := interp.GetGlobal("month"); !got.IsString() || got.Str() != "January" {
		t.Fatalf("month = %v, want January", got)
	}
	if got := interp.GetGlobal("before"); !got.IsBool() || !got.Bool() {
		t.Fatalf("before = %v, want true", got)
	}
	if got := interp.GetGlobal("after"); !got.IsBool() || !got.Bool() {
		t.Fatalf("after = %v, want true", got)
	}
}

func TestTimeModuleAfterAndContextSleep(t *testing.T) {
	interp := timeInterp(t, `
deadline := time.after(0.001)
afterResult := "none"
select {
case <-deadline:
	afterResult = "timeout"
}

ctx, cancel := context.withTimeout(0.001)
_ := cancel
t0 := time.now()
ok, err := time.sleep(ctx, 0.05)
elapsed := time.since(t0)
`)
	if got := interp.GetGlobal("afterResult"); !got.IsString() || got.Str() != "timeout" {
		t.Fatalf("afterResult = %v, want timeout", got)
	}
	if got := interp.GetGlobal("ok"); !got.IsBool() || got.Bool() {
		t.Fatalf("ok = %v, want false", got)
	}
	if got := interp.GetGlobal("err"); !got.IsString() || got.Str() != "deadline exceeded" {
		t.Fatalf("err = %v, want deadline exceeded", got)
	}
	if got := interp.GetGlobal("elapsed"); !got.IsFloat() || got.Number() >= 0.04 {
		t.Fatalf("elapsed = %v, want cancellation before full sleep", got)
	}
}
