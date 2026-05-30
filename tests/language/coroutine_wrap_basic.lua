print("case:coroutine_wrap_basic")

local function foo (i) return coroutine.yield(i) end
local f = coroutine.wrap(function ()
  for i = 1, 10 do
    assert(foo(i) == _G.x)
  end
  return "a"
end)

for i = 1, 10 do
  _G.x = i
  assert(f(i) == i)
end
_G.x = "xuxu"
assert(f("xuxu") == "a")
_G.x = nil

local function gen (n)
  return coroutine.wrap(function ()
    for i = 2, n do coroutine.yield(i) end
  end)
end

local x = gen(10)
local a = {}
while true do
  local n = x()
  if n == nil then break end
  table.insert(a, n)
end
assert(#a == 9 and a[1] == 2 and a[#a] == 10)

print("ok")
