package runtime

import "testing"

func TestTestkitMemorySnapshotDiffAndCheck(t *testing.T) {
	interp := NewCore()
	testkitModule := TableValue(buildTestkitLib(interp))
	interp.SetGlobal("testkit", testkitModule)
	interp.SetModule("testkit", testkitModule)

	execBinaryIOTest(t, interp, `
		before := testkit.snapshot()
		collectgarbage("collect")
		after := testkit.memory()
		diff := testkit.diff(before, after)
		ok, report := testkit.checkMemory(before, {maxAllocBytesGrowth: 1 << 62, maxHeapObjectsGrowth: 1 << 62, collect: true})
	`)

	if got := interp.GetGlobal("before").Table().RawGetString("allocBytes"); !got.IsNumber() {
		t.Fatalf("before.allocBytes = %v, want number", got)
	}
	if got := interp.GetGlobal("diff").Table().RawGetString("numGC"); !got.IsNumber() {
		t.Fatalf("diff.numGC = %v, want number", got)
	}
	if got := interp.GetGlobal("ok"); !got.IsBool() || !got.Bool() {
		t.Fatalf("ok = %v, want true", got)
	}
	if got := interp.GetGlobal("report").Table().RawGetString("ok"); !got.IsBool() || !got.Bool() {
		t.Fatalf("report.ok = %v, want true", got)
	}
}

func TestTestkitValueAndFunctionInspectors(t *testing.T) {
	interp := NewCore()
	testkitModule := TableValue(buildTestkitLib(interp))
	interp.SetGlobal("testkit", testkitModule)
	interp.SetModule("testkit", testkitModule)

	execBinaryIOTest(t, interp, `
		func sample(a, b, ...) {
			return a + b, "done"
		}
		numberInfo := testkit.value(42)
		tableInfo := testkit.value({1, 2, 3})
		fnInfo := testkit.functionInfo(sample)
		valueInfo := testkit.value(sample)
		sameSample := testkit.sameFunction(sample, sample)
		sameMixed := testkit.sameFunction(sample, print)
		isNumber := testkit.typeOf(1.5)
		rawEqual := testkit.equal(sample, sample)
	`)

	if got := interp.GetGlobal("numberInfo").Table().RawGetString("numberKind").Str(); got != "int" {
		t.Fatalf("numberKind = %q, want int", got)
	}
	if got := interp.GetGlobal("tableInfo").Table().RawGetString("len").Int(); got != 3 {
		t.Fatalf("table len = %d, want 3", got)
	}
	if got := interp.GetGlobal("fnInfo").Table().RawGetString("kind").Str(); got != "script" {
		t.Fatalf("fn kind = %q, want script", got)
	}
	if got := interp.GetGlobal("valueInfo").Table().RawGetString("functionKind").Str(); got != "script" {
		t.Fatalf("value functionKind = %q, want script", got)
	}
	if got := interp.GetGlobal("sameSample").Bool(); !got {
		t.Fatalf("sameSample = false, want true")
	}
	if got := interp.GetGlobal("sameMixed").Bool(); got {
		t.Fatalf("sameMixed = true, want false")
	}
	if got := interp.GetGlobal("isNumber").Str(); got != "number" {
		t.Fatalf("typeOf = %q, want number", got)
	}
	if got := interp.GetGlobal("rawEqual").Bool(); !got {
		t.Fatalf("rawEqual = false, want true")
	}
}

func TestTestkitProtect(t *testing.T) {
	interp := NewCore()
	testkitModule := TableValue(buildTestkitLib(interp))
	interp.SetGlobal("testkit", testkitModule)
	interp.SetModule("testkit", testkitModule)

	execBinaryIOTest(t, interp, `
		func ok(a, b) {
			return a + b, "ok"
		}
		func fail() {
			error({code: "boom"})
		}
		good := testkit.protect(ok, 2, 3)
		bad := testkit.protect(fail)
	`)

	good := interp.GetGlobal("good").Table()
	if !good.RawGetString("ok").Bool() {
		t.Fatalf("good.ok = false, want true")
	}
	if got := good.RawGetString("values").Table().RawGet(IntValue(1)).Int(); got != 5 {
		t.Fatalf("good.values[1] = %d, want 5", got)
	}
	bad := interp.GetGlobal("bad").Table()
	if bad.RawGetString("ok").Bool() {
		t.Fatalf("bad.ok = true, want false")
	}
	if got := bad.RawGetString("error").Table().RawGetString("code").Str(); got != "boom" {
		t.Fatalf("bad.error.code = %q, want boom", got)
	}
}
