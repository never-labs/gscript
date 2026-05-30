print("case:calls_tail_call_metamethod_more")

local function foo(x, ...)
  local a = {...}
  return x, a[1], a[2]
end

local t = setmetatable({}, {__call = foo})

local function call_table(x)
  return t(10, x)
end

local a, b, c = call_table(100)
assert(a == t and b == 10 and c == 100)

local n = 12
local function done()
  if n == 0 then
    return 1023
  else
    n = n - 1
    return done()
  end
end

local u = done
for i = 1, 8 do
  u = setmetatable({}, {__call = u})
end

assert(u() == 1023)

print("ok")
