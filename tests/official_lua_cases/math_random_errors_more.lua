print("case:math_random_errors_more")

assert(not pcall(math.random, 1, 2, 3))
assert(not pcall(math.random, 2, 1))
assert(not pcall(math.random, 10, -10))

for i = 1, 20 do
  local r = math.random(1, 6)
  assert(1 <= r and r <= 6 and math.type(r) == "integer")
end

print("ok")
