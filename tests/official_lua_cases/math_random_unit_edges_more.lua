print("case:math_random_unit_edges_more")

local minint = math.mininteger
local maxint = math.maxinteger
for i = 1, 20 do
  assert(math.random(-10, -10) == -10)
  assert(math.random(minint, minint) == minint)
  assert(math.random(maxint, maxint) == maxint)
end
for i = 1, 50 do
  local t = math.random(minint, minint + 9)
  assert(minint <= t and t <= minint + 9)
  t = math.random(maxint - 3, maxint)
  assert(maxint - 3 <= t and t <= maxint)
end

print("ok")
