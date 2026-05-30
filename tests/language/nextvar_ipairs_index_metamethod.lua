print("case:nextvar_ipairs_index_metamethod")

local a = {n = 10}
local mt = {}
mt.__index = function(t, k)
  if k <= t.n then return k * 10 end
end
setmetatable(a, mt)

local i = 0
for k, v in ipairs(a) do
  i = i + 1
  assert(k == i and v == i * 10)
end

assert(i == a.n)

print("ok")
