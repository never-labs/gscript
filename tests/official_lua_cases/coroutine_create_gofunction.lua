print("case:coroutine_create_gofunction")

local co = coroutine.create(assert)
assert(coroutine.status(co) == "suspended")

local ok, a, b = coroutine.resume(co, true, "native-ok")
assert(ok and a == true and b == "native-ok")
assert(coroutine.status(co) == "dead")

ok, a = coroutine.resume(co, true)
assert(not ok and string.find(a, "dead"))

local errco = coroutine.create(error)
ok, a = coroutine.resume(errco, "native_error")
assert(not ok and string.find(a, "native_error"))
assert(coroutine.status(errco) == "dead")

print("ok")
