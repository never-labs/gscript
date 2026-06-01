print("case:coroutine_create_gofunction")

co := coroutine.create(assert)
assert(coroutine.status(co) == "suspended")

ok, a, b := coroutine.resume(co, true, "native-ok")
assert(ok && a == true && b == "native-ok")
assert(coroutine.status(co) == "dead")

ok, a = coroutine.resume(co, true)
assert(!ok && string.find(a, "dead"))

errco := coroutine.create(error)
ok, a = coroutine.resume(errco, "native_error")
assert(!ok && string.find(a, "native_error"))
assert(coroutine.status(errco) == "dead")

print("ok")
