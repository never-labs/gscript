print("case:locals_block_assignment_scope_more")

local outer = 10
local a
do
  local outer
  a, outer = 1, 2
  assert(a + 1 == outer)
end
assert(a == 1 and outer == 10)

do
  local i = 2
  local p = 4
  for j = -3, 3 do
    local a = j
    a = a + (p - j)
    assert(a == 2 ^ i)
    local b = -j
    local c = b - (p - j)
    assert(c == -(2 ^ i))
  end
end

print("ok")
