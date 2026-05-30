print("case:locals_repeat_shadow_more2")

local function nilarg(x)
  x = nil
  local y
  return x, y
end

local a, b = nilarg(10)
assert(a == nil and b == nil)

do
  local i = 10
  do local i = 100; assert(i == 100) end
  do local i = 1000; assert(i == 1000) end
  assert(i == 10)
  if i ~= 10 then
    local i = 20
  else
    local i = 30
    assert(i == 30)
  end
  assert(i == 10)
end

local rb = 10
local ra
repeat
  local rb
  ra, rb = 1, 2
  assert(ra + 1 == rb)
until ra + rb == 3
assert(rb == 10)

print("ok")
