print("case:control_coroutine_defer_more")

order := ""

func mark(s) {
	order = order .. s .. "|"
}

func returnWithDefer(label) {
	defer mark(label .. ":first")
	defer mark(label .. ":second")
	mark(label .. ":body")
	return label .. ":value"
}

rv := returnWithDefer("ret")
assert(rv == "ret:value")
assert(order == "ret:body|ret:second|ret:first|")

const cfg = {count: 1, note: "outer"}
cfg.count = 2

func mutateCaptured(delta) {
	cfg.count = cfg.count + delta
	return cfg.count
}

assert(mutateCaptured(3) == 5)
assert(!pcall(func() {
	cfg = {count: 99}
}))

if true {
	const cfg = {count: 10}
	cfg.count = 11
	assert(cfg.count == 11)
}
assert(cfg.count == 5 && cfg.note == "outer")

cachedYield := coroutine.yield
cachedIsYieldable := coroutine.isyieldable
co := coroutine.create(func(seed) {
	assert(cachedIsYieldable())
	mark("co:start")
	a, b := cachedYield("cached", seed + 1)
	assert(cachedIsYieldable())
	mark("co:after")
	return a + b, cfg.count
})

ok, tag, val := coroutine.resume(co, 41)
assert(ok && tag == "cached" && val == 42)
assert(coroutine.status(co) == "suspended")

ok, sum, capturedCount := coroutine.resume(co, 7, 8)
assert(ok && sum == 15 && capturedCount == 5)
assert(coroutine.status(co) == "dead")

func protectedStep() {
	defer mark("pcall:defer1")
	defer mark("pcall:defer2")
	mark("pcall:before")
	error("inner")
	mark("pcall:after")
}

okInner, errInner := pcall(protectedStep)
assert(!okInner && errInner != nil)
mark("pcall:continued")

func outerFailure() {
	defer mark("outer:first")
	defer mark("outer:second")
	val := returnWithDefer("nested")
	assert(val == "nested:value")
	mark("outer:before-error")
	error("outer")
}

okOuter, errOuter := pcall(outerFailure)
assert(!okOuter && errOuter != nil)

expected := "ret:body|ret:second|ret:first|" ..
	"co:start|co:after|" ..
	"pcall:before|pcall:defer2|pcall:defer1|pcall:continued|" ..
	"nested:body|nested:second|nested:first|outer:before-error|outer:second|outer:first|"
assert(order == expected)

print("ok")
