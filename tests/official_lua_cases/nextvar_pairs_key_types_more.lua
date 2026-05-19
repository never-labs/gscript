print("case:nextvar_pairs_key_types_more")

local fkey = function() return 6 end
local tkey = {}
local long = string.rep('x', 1000)
local a = {
  [1] = 1,
  [1.1] = 2,
  ['x'] = 3,
  [long] = 4,
  [fkey] = 5,
  [true] = 6,
  [tkey] = 7,
}
local b = {}
for i = 1, 7 do b[i] = true end
for k, v in pairs(a) do
  assert(b[v])
  b[v] = nil
  assert(a[k] == v)
end
assert(next(b) == nil)

local n = 0
local t = {[{}] = 1, [string.rep("x ", 4)] = 3, [100.3] = 4, [4] = 5}
for k, v in pairs(t) do
  n = n + 1
  assert(t[k] == v)
  t[k] = nil
  assert(t[k] == nil)
end
assert(n == 4 and next(t) == nil)

print("ok")
