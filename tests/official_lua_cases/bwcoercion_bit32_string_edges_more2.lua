print("case:bwcoercion_bit32_string_edges_more2")

local function toint(x)
  x = tonumber(x)
  if not x then return false end
  return math.tointeger(x) or false
end

local inputs = {" 15 ", "0xf0", "-1", "4294967296", "3.0"}
assert(toint(inputs[1]) == 15)
assert(toint(inputs[2]) == 240)
assert(toint(inputs[3]) == -1)
assert(toint(inputs[4]) == 4294967296)
assert(toint(inputs[5]) == 3)

local a = toint(inputs[1])
local b = toint(inputs[2])
assert(((a | b) & 0xffffffff) == 255)
assert(((b ~ a) & 0xffffffff) == 255)
assert(((toint(inputs[3]) & b) & 0xffffffff) == 240)
assert(((toint(inputs[5]) << 8) & 0xffffffff) == 768)
assert(((toint(inputs[4]) | 7) & 0xffffffff) == 7)
assert(toint("3.25") == false)

print("ok")
