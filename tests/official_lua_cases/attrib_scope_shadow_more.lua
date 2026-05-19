print("case:attrib_scope_shadow_more")

local a = "outer"
do
  local a = "inner"
  local function f()
    return a
  end
  assert(f() == "inner")
  a = "changed"
  assert(f() == "changed")
end
assert(a == "outer")

local function make()
  local x = 10
  return function(delta)
    x = x + delta
    return x
  end
end

local c1 = make()
local c2 = make()
assert(c1(1) == 11)
assert(c1(2) == 13)
assert(c2(5) == 15)

print("ok")
