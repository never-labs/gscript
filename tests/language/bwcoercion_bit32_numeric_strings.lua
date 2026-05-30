print("case:bwcoercion_bit32_numeric_strings")

local function toint(x)
  x = tonumber(x)
  if not x then return false end
  return math.tointeger(x) or false
end

local function checkargs(x, y)
  local xi = toint(x)
  local yi = toint(y)
  if xi and yi then
    return xi, yi
  end
  error("not integer")
end

local function bandstr(x, y)
  local xi, yi = checkargs(x, y)
  return (xi & yi) & 0xffffffff
end

local function borstr(x, y)
  local xi, yi = checkargs(x, y)
  return (xi | yi) & 0xffffffff
end

local function xorstr(x, y)
  local xi, yi = checkargs(x, y)
  return (xi ~ yi) & 0xffffffff
end

local function shlstr(x, y)
  local xi, yi = checkargs(x, y)
  return (xi << yi) & 0xffffffff
end

local function shrstr(x, y)
  local xi, yi = checkargs(x, y)
  return (xi >> yi) & 0xffffffff
end

local function bnotstr(x)
  local xi = toint(x)
  if not xi then error("not integer") end
  return (~xi) & 0xffffffff
end

assert(bandstr("7", "3.0") == 3)
assert(borstr("0x10", " 5 ") == 21)
assert(xorstr("255", "15") == 240)
assert(shlstr("3", "4") == 48)
assert(shrstr("256", "3") == 32)
assert(bnotstr("0") == 4294967295)
assert(toint("3.5") == false)
assert(toint("not a number") == false)
assert(not pcall(bandstr, "3.5", "1"))

print("ok")
