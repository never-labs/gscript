print("case:nextvar_pairs_metamethod")

local a = {}
local function foo(e, i)
  assert(e == a)
  if i <= 10 then return i + 1, i + 2 end
end

local mt = {}
mt.__pairs = function(x)
  return foo, x, 0
end
setmetatable(a, mt)

local i = 0
for k, v in pairs(a) do
  i = i + 1
  assert(k == i and v == k + 1)
end

assert(i == 11)

print("ok")
