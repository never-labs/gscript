print("case:nextvar_pairs_string_keys")

local t = {a = 10, b = 20, c = 30}
local sum = 0
local seen = {}

for k, v in pairs(t) do
  seen[k] = true
  sum = sum + v
end

assert(sum == 60)
assert(seen.a and seen.b and seen.c)

local count = 0
local k = nil
repeat
  k = next(t, k)
  if k ~= nil then count = count + 1 end
until k == nil

assert(count == 3)

print("ok")
