print("case:coroutine_tail_yield_more")

local x = nil

local function foo(i)
  return coroutine.yield(i)
end

local f = coroutine.wrap(function()
  for i = 1, 10 do
    assert(foo(i) == x)
  end
  return "a"
end)

for i = 1, 10 do
  x = i
  assert(f(i) == i)
end

x = "xuxu"
assert(f("xuxu") == "a")

print("ok")
