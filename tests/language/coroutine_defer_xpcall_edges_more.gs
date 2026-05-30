print("case:coroutine_defer_xpcall_edges_more")

order := ""

func mark(s) {
  order = order .. s .. "|"
}

func coReturnCleanup1() { mark("co-return:defer1") }
func coReturnCleanup2() { mark("co-return:defer2") }

co := coroutine.create(func(seed) {
  mark("co-return:start")
  defer coReturnCleanup1()
  defer coReturnCleanup2()
  yielded := coroutine.yield("yielded", seed + 1)
  mark("co-return:after-yield")
  return yielded * 2
})

ok, tag, value := coroutine.resume(co, 10)
assert(ok && tag == "yielded" && value == 11)
assert(coroutine.status(co) == "suspended")
assert(order == "co-return:start|")

ok, value = coroutine.resume(co, 21)
assert(ok && value == 42)
assert(coroutine.status(co) == "dead")
assert(order == "co-return:start|co-return:after-yield|co-return:defer2|co-return:defer1|")

func coErrorCleanup1() { mark("co-error:defer1") }
func coErrorCleanup2() { mark("co-error:defer2") }

errco := coroutine.create(func() {
  mark("co-error:start")
  defer coErrorCleanup1()
  defer coErrorCleanup2()
  coroutine.yield("pause")
  mark("co-error:after-yield")
  error("co-error-boom")
})

ok, tag = coroutine.resume(errco)
assert(ok && tag == "pause")
assert(coroutine.status(errco) == "suspended")
assert(order == "co-return:start|co-return:after-yield|co-return:defer2|co-return:defer1|co-error:start|")

ok, err := coroutine.resume(errco)
assert(!ok && err != nil)
assert(coroutine.status(errco) == "dead")
assert(order == "co-return:start|co-return:after-yield|co-return:defer2|co-return:defer1|co-error:start|co-error:after-yield|co-error:defer2|co-error:defer1|")

phase := "init"
drains := 0
currentPrefix := ""

func xpCleanup1() {
  drains = drains + 1
  phase = currentPrefix .. ":cleanup1"
  mark(currentPrefix .. ":defer1")
}

func xpCleanup2() {
  drains = drains + 1
  phase = currentPrefix .. ":cleanup2"
  mark(currentPrefix .. ":defer2")
}

func protectedBoom(label) {
  currentPrefix = label
  phase = label .. ":body"
  defer xpCleanup1()
  defer xpCleanup2()
  mark(label .. ":before-error")
  error(label .. ":boom")
}

res, msg := xpcall(func() {
  protectedBoom("xp-ok")
}, func(e) {
  mark("xp-ok:handler")
  assert(drains == 2)
  assert(phase == "xp-ok:cleanup1")
  return "handled:" .. phase
})

assert(!res && msg == "handled:xp-ok:cleanup1")

drains = 0
phase = "reset"
handlerCalls := 0
res, msg = xpcall(func() {
  protectedBoom("xp-handler-error")
}, func(e) {
  handlerCalls = handlerCalls + 1
  if handlerCalls > 1 {
    return "handler-recursed"
  }
  mark("xp-handler-error:handler")
  assert(drains == 2)
  assert(phase == "xp-handler-error:cleanup1")
  error("handler-boom")
})

assert(!res && drains == 2)

expected := "co-return:start|co-return:after-yield|co-return:defer2|co-return:defer1|" ..
  "co-error:start|co-error:after-yield|co-error:defer2|co-error:defer1|" ..
  "xp-ok:before-error|xp-ok:defer2|xp-ok:defer1|xp-ok:handler|" ..
  "xp-handler-error:before-error|xp-handler-error:defer2|xp-handler-error:defer1|xp-handler-error:handler|"
assert(order == expected)

print("ok")
