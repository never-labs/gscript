print("case:code_constant_branch_matrix_more2")

local function choose(x)
  if x == -2 then return "neg" end
  if x == 0 then return "zero" end
  if x == 3 then return "three" end
  if x == 42 then return "forty-two" end
  return "other"
end

local seen = {}
for i = -3, 43 do
  seen[#seen + 1] = choose(i)
end

assert(seen[1] == "other")
assert(seen[2] == "neg")
assert(seen[4] == "zero")
assert(seen[7] == "three")
assert(seen[46] == "forty-two")
assert((2 ^ 10) + (3 * 7) - 5 == 1040)

print("ok")
