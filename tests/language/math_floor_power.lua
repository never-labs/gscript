print("case:math_floor_power")

local function eq (a, b, limit)
  limit = limit or 1E-11
  return a == b or math.abs(a - b) <= limit
end

assert(0e12 == 0 and .0 == 0 and 0. == 0 and .2e2 == 20 and 2.E-1 == 0.2)

for _, i in pairs{-16, -15, -3, -2, -1, 0, 1, 2, 3, 15} do
  for _, j in pairs{-16, -15, -3, -2, -1, 1, 2, 3, 15} do
    assert(i // j == math.floor(i / j))
  end
end

assert(2^-3 == 1 / 2^3)
assert(eq((-3)^-3, 1 / (-3)^3))
for i = -3, 3 do
  for j = -3, 3 do
    if i ~= 0 or j > 0 then
      assert(eq(i^j, 1 / i^(-j)))
    end
  end
end

print("ok")
