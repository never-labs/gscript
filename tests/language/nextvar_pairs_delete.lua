print("case:nextvar_pairs_delete")

local k1 = {1}
local k2 = {2}
local t = {}
t[k1] = 1
t[k2] = 2
t[string.rep("x ", 4)] = 3
t[100.3] = 4
t[4] = 5

local n = 0
for k, v in pairs(t) do
  n = n + 1
  assert(t[k] == v)
  t[k] = nil
  assert(t[k] == nil)
end

assert(n == 5)
assert(next(t) == nil)

print("ok")
