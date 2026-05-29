package runtime

import "testing"

func runSyncTestScript(t *testing.T, code string) *Interpreter {
	t.Helper()
	interp := New()
	tokens, err := lexerNew(code)
	if err != nil {
		t.Fatalf("lexer: %v", err)
	}
	prog, err := parserNew(tokens)
	if err != nil {
		t.Fatalf("parser: %v", err)
	}
	if err := interp.Exec(prog); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	return interp
}

func TestSyncWaitGroup(t *testing.T) {
	interp := runSyncTestScript(t, `
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

func TestSyncTaskGroup(t *testing.T) {
	interp := runSyncTestScript(t, `
group := sync.group()
for i := 1; i <= 4; i++ {
    group.start(func(v) {
        if v == 3 {
            error("bad task")
        }
    }, i)
}
ok, err, count := group.wait()
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
}

func TestSyncMutexAndOnce(t *testing.T) {
	interp := runSyncTestScript(t, `
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
`)
	state := interp.GetGlobal("state")
	if !state.IsTable() {
		t.Fatalf("state = %v, want table", state)
	}
	if got := state.Table().RawGetString("value"); !got.IsInt() || got.Int() != 400 {
		t.Fatalf("state.value = %v, want 400", got)
	}
	if got := interp.GetGlobal("initCount"); !got.IsInt() || got.Int() != 1 {
		t.Fatalf("initCount = %v, want 1", got)
	}
}

func TestSyncRWMutex(t *testing.T) {
	interp := runSyncTestScript(t, `
mu := sync.rwmutex()
value := 0
mu.lock()
value = value + 10
mu.unlock()
mu.rlock()
snapshot := value
mu.runlock()
`)
	if got := interp.GetGlobal("snapshot"); !got.IsInt() || got.Int() != 10 {
		t.Fatalf("snapshot = %v, want 10", got)
	}
}
