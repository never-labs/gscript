print("case:bitwise_bit32_arshift_more")

local bit32 = bit32 or {}
bit32.band = bit32.band or function(...)
  local r = 0xffffffff
  for i = 1, select("#", ...) do
    r = r & select(i, ...)
  end
  return r & 0xffffffff
end
bit32.arshift = bit32.arshift or function(a, b)
  a = a & 0xffffffff
  if b <= 0 or (a & 0x80000000) == 0 then
    return (a >> b) & 0xffffffff
  else
    return ((a >> b) | ~(0xffffffff >> b)) & 0xffffffff
  end
end

assert(bit32.arshift(0x12345678, 0) == 0x12345678)
assert(bit32.arshift(0x12345678, 1) == 0x12345678 // 2)
assert(bit32.arshift(0x12345678, -1) == 0x12345678 * 2)
assert(bit32.arshift(-1, 1) == 0xffffffff)
assert(bit32.arshift(-1, 24) == 0xffffffff)
assert(bit32.arshift(-1, 32) == 0xffffffff)
assert(bit32.arshift(-1, -1) == bit32.band(-1 * 2, 0xffffffff))

print("ok")
