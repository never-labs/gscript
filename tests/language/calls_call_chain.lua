print("case:calls_call_chain")

local n = 5
local u = table.pack
for i = 1, n do
  local t = {i}
  local mt = {__call = u}
  u = setmetatable(t, mt)
end

local res = u("a", "b", "c")
assert(res.n == n + 3)
for i = 1, n do
  assert(res[i][1] == i)
end
assert(res[n + 1] == "a" and res[n + 2] == "b" and res[n + 3] == "c")

print("ok")
