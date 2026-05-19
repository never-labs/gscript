print("case:coroutine_status_yieldable_more")

assert(coroutine.isyieldable() == false)

local flag = false
local co = coroutine.create(function(a)
  assert(coroutine.isyieldable() == true)
  flag = true
  local x = coroutine.yield("yield", a + 1)
  return x * 2
end)

assert(coroutine.status(co) == "suspended")
local ok, tag, value = coroutine.resume(co, 10)
assert(ok and tag == "yield" and value == 11)
assert(flag and coroutine.status(co) == "suspended")
ok, value = coroutine.resume(co, 21)
assert(ok and value == 42)
assert(coroutine.status(co) == "dead")

print("ok")
