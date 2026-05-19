print("case:coroutine_status_yieldable_more")

assert(coroutine.isyieldable() == false)

flag := false
co := coroutine.create(func(a) {
  assert(coroutine.isyieldable() == true)
  flag = true
  x := coroutine.yield("yield", a + 1)
  return x * 2
})

assert(coroutine.status(co) == "suspended")
ok, tag, value := coroutine.resume(co, 10)
assert(ok && tag == "yield" && value == 11)
assert(flag && coroutine.status(co) == "suspended")
ok, value = coroutine.resume(co, 21)
assert(ok && value == 42)
assert(coroutine.status(co) == "dead")

print("ok")
