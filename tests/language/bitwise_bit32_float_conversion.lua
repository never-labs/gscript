print("case:bitwise_bit32_float_conversion")

local bit32 = bit32 or {}
bit32.bor = bit32.bor or function (...)
  local r = 0
  for i = 1, select("#", ...) do r = r | select(i, ...) end
  return r & 0xffffffff
end

assert(bit32.bor(3.0) == 3)
assert(bit32.bor(-4.0) == 0xfffffffc)
assert(bit32.bor(2.0^32 - 5.0) == 0xfffffffb)
assert(bit32.bor(-2.0^32 - 6.0) == 0xfffffffa)
assert(bit32.bor(2.0^48 - 5.0) == 0xfffffffb)
assert(bit32.bor(-2.0^48 - 6.0) == 0xfffffffa)

print("ok")
