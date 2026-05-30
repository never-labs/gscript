print("case:math_fmod_integer_more")

local function eqT(a, b)
  return a == b and math.type(a) == math.type(b)
end

for i = -6, 6 do
  for j = -6, 6 do
    if j ~= 0 then
      local mi = math.fmod(i, j)
      local mf = math.fmod(i + 0.0, j)
      assert(mi == mf)
      assert(math.type(mi) == "integer" and math.type(mf) == "float")
      if (i >= 0 and j >= 0) or (i <= 0 and j <= 0) or mi == 0 then
        assert(eqT(mi, i % j))
      end
    end
  end
end

local s, err = pcall(math.fmod, 3, 0)
assert(not s and string.find(err, "zero"))

print("ok")
