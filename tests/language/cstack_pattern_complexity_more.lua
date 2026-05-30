print("case:cstack_pattern_complexity_more")

local function f (size)
  local s = string.rep("a", size)
  local p = string.rep(".?", size)
  return string.match(s, p)
end

local m = f(80)
assert(#m == 80)

print("ok")
