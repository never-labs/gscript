print("case:math_fmod_more2")

for i = -6, 6 do
  for j = -6, 6 do
    if j ~= 0 then
      local mi = math.fmod(i, j)
      local mf = math.fmod(i + 0.0, j)
      assert(mi == mf)
      assert(math.type(mi) == "integer" and math.type(mf) == "float")
    end
  end
end
assert(math.fmod(math.mininteger, math.mininteger) == 0)
assert(math.fmod(math.maxinteger, math.maxinteger) == 0)

print("ok")
