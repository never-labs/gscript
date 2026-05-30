print("case:coroutine_multi_yield_resume_more")

local co = coroutine.create(function()
  local a, b = coroutine.yield("a", 10, nil, "z")
  return "done", a, b
end)

local ok, a, b, c, d = coroutine.resume(co)
assert(ok and a == "a" and b == 10 and c == nil and d == "z")

ok, a, b, c = coroutine.resume(co, 20, "resume")
assert(ok and a == "done" and b == 20 and c == "resume")

print("ok")
