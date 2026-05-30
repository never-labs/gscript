print("case:math_random_range_more")

local random = math.random
for i = 1, 100 do
  local t = random()
  assert(0 <= t and t < 1)
end
for i = 1, 100 do
  local r = random(6)
  assert(math.type(r) == "integer" and 1 <= r and r <= 6)
end
for i = 1, 100 do
  local r = random(-10, 10)
  assert(math.type(r) == "integer" and -10 <= r and r <= 10)
end

print("ok")
