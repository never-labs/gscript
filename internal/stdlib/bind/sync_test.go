package bind

import (
	"testing"

	"github.com/never-labs/leia/internal/runtime"
)

func syncInterp(t *testing.T, src string) *runtime.Interpreter {
	t.Helper()
	interp := runtime.NewCore()
	interp.InstallRuntimeStdlib()
	installTestModule(interp, "sync", runtime.TableValue(BuildSync(ConcurrencyOptions{Call: interp.CallFunction, Launch: interp.LaunchFunction})))
	installTestModule(interp, "time", runtime.TableValue(BuildTime()))
	execOnInterp(t, interp, src)
	return interp
}

func TestSyncModuleWaitGroup(t *testing.T) {
	interp := syncInterp(t, `
wg := sync.waitgroup()
ch := make(chan, 4)
for i := 1; i <= 4; i++ {
    wg.add(1)
    go func(v) {
        ch <- v
        wg.done()
    }(i)
}
wg.wait()
total := 0
for i := 1; i <= 4; i++ {
    total = total + <-ch
}
`)
	if got := interp.GetGlobal("total"); !got.IsInt() || got.Int() != 10 {
		t.Fatalf("total = %v, want 10", got)
	}
}

func TestSyncModuleTaskGroup(t *testing.T) {
	interp := syncInterp(t, `
group := sync.group()
for i := 1; i <= 4; i++ {
    group.start(func(ctx, v) {
        if v == 3 {
            error("bad task")
        }
    }, i)
}
ok, err, count := group.wait()
ctxCancelled := group.context().cancelled()
`)
	if got := interp.GetGlobal("ok"); !got.IsBool() || got.Bool() {
		t.Fatalf("ok = %v, want false", got)
	}
	if got := interp.GetGlobal("err"); !got.IsString() || got.Str() == "" {
		t.Fatalf("err = %v, want non-empty string", got)
	}
	if got := interp.GetGlobal("count"); !got.IsInt() || got.Int() != 1 {
		t.Fatalf("count = %v, want 1", got)
	}
	if got := interp.GetGlobal("ctxCancelled"); !got.IsBool() || !got.Bool() {
		t.Fatalf("ctxCancelled = %v, want true", got)
	}
}

func TestSyncModuleTaskGroupCancelsSiblingTasks(t *testing.T) {
	interp := syncInterp(t, `
group := sync.group()
ctx := group.context()
out := make(chan, 2)

group.start(func(ctx) {
    error("first failure")
})

group.start(func(ctx) {
    ok, err := time.sleep(ctx, 0.05)
    out <- ok
    out <- err
})

ok, err, count := group.wait()
sleptOK := <-out
sleepErr := <-out
cancelled := ctx.cancelled()
`)
	if got := interp.GetGlobal("ok"); !got.IsBool() || got.Bool() {
		t.Fatalf("ok = %v, want false", got)
	}
	if got := interp.GetGlobal("cancelled"); !got.IsBool() || !got.Bool() {
		t.Fatalf("cancelled = %v, want true", got)
	}
	if got := interp.GetGlobal("sleptOK"); !got.IsBool() || got.Bool() {
		t.Fatalf("sleptOK = %v, want false", got)
	}
	if got := interp.GetGlobal("sleepErr"); !got.IsString() || got.Str() != "cancelled" {
		t.Fatalf("sleepErr = %v, want cancelled", got)
	}
	if got := interp.GetGlobal("count"); !got.IsInt() || got.Int() != 1 {
		t.Fatalf("count = %v, want 1", got)
	}
}

func TestSyncModuleMutexOnceAndRWMutex(t *testing.T) {
	interp := syncInterp(t, `
mu := sync.mutex()
wg := sync.waitgroup()
state := {value: 0}
for i := 1; i <= 4; i++ {
    wg.add(1)
    go func() {
        for j := 1; j <= 100; j++ {
            mu.lock()
            state.value = state.value + 1
            mu.unlock()
        }
        wg.done()
    }()
}
wg.wait()

once := sync.once()
initCount := 0
for i := 1; i <= 5; i++ {
    once.do(func() {
        initCount = initCount + 1
    })
}

rw := sync.rwmutex()
rw.lock()
state.value = state.value + 10
rw.unlock()
rw.rlock()
snapshot := state.value
rw.runlock()
`)
	state := interp.GetGlobal("state")
	if !state.IsTable() {
		t.Fatalf("state = %v, want table", state)
	}
	if got := state.Table().RawGetString("value"); !got.IsInt() || got.Int() != 410 {
		t.Fatalf("state.value = %v, want 410", got)
	}
	if got := interp.GetGlobal("initCount"); !got.IsInt() || got.Int() != 1 {
		t.Fatalf("initCount = %v, want 1", got)
	}
	if got := interp.GetGlobal("snapshot"); !got.IsInt() || got.Int() != 410 {
		t.Fatalf("snapshot = %v, want 410", got)
	}
}
