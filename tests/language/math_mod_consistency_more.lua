print("case:math_mod_consistency_more")

for i = -10, 10 do
  for j = -10, 10 do
    if j ~= 0 then
      assert((i + 0.0) % j == i % j)
    end
  end
end
assert(math.mininteger % math.mininteger == 0)
assert(math.maxinteger % math.maxinteger == 0)

print("ok")
