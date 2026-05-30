print("case:coroutine_wrap_sieve_more")

local function gen(n)
  return coroutine.wrap(function()
    for i = 2, n do
      coroutine.yield(i)
    end
  end)
end

local function filter(p, g)
  return coroutine.wrap(function()
    while true do
      local n = g()
      if n == nil then return end
      if math.fmod(n, p) ~= 0 then coroutine.yield(n) end
    end
  end)
end

local x = gen(80)
local a = {}
while true do
  local n = x()
  if n == nil then break end
  table.insert(a, n)
  x = filter(n, x)
end

assert(#a == 22 and a[#a] == 79)

print("ok")
