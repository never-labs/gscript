print("case:math_negative_powers_more")

local function eq(a, b)
  return a == b or math.abs(a - b) <= 1e-11
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
