print("case:coroutine_self_resume_status_more")

local function co_func()
  coroutine.yield(10)
  coroutine.yield(23)
  return 10
end

local co = coroutine.create(co_func)
assert(coroutine.status(co) == "suspended")

local a, b, c = coroutine.resume(co)
assert(a == true and b == 10 and c == nil)
assert(coroutine.status(co) == "suspended")

a, b = coroutine.resume(co)
assert(a == true and b == 23)

a, b = coroutine.resume(co)
assert(a == true and b == 10)
assert(coroutine.status(co) == "dead")

assert(coroutine.resume(co) == false)
assert(coroutine.resume(co) == false)

print("ok")
