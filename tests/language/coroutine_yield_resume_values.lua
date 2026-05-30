print("case:coroutine_yield_resume_values")

local co = coroutine.wrap(function ()
  local a = coroutine.yield("first")
  return "done", a
end)

assert(co() == "first")
local a, b = co("arg")
assert(a == "done" and b == "arg")

local count = 0
local gen = coroutine.wrap(function ()
  for i = 1, 3 do
    count = count + i
    coroutine.yield(count)
  end
end)

assert(gen() == 1)
assert(gen() == 3)
assert(gen() == 6)

print("ok")
