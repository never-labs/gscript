print("case:bitwise_bit32_shifts")

local bit32 = bit32 or {}
bit32.band = bit32.band or function (...)
  local n = select("#", ...)
  if n == 0 then return 0xffffffff end
  local r = select(1, ...) & 0xffffffff
  for i = 2, n do r = r & select(i, ...) end
  return r & 0xffffffff
end
bit32.lshift = bit32.lshift or function (n, d)
  n = n & 0xffffffff
  if d < 0 then return (n >> -d) & 0xffffffff end
  if d >= 32 then return 0 end
  return (n << d) & 0xffffffff
end
bit32.rshift = bit32.rshift or function (n, d)
  n = n & 0xffffffff
  if d < 0 then return (n << -d) & 0xffffffff end
  if d >= 32 then return 0 end
  return (n >> d) & 0xffffffff
end
bit32.arshift = bit32.arshift or function (n, d)
  n = n & 0xffffffff
  if d < 0 then return (n << -d) & 0xffffffff end
  if d >= 32 then return n >= 0x80000000 and 0xffffffff or 0 end
  if n >= 0x80000000 then n = n - 0x100000000 end
  return (n >> d) & 0xffffffff
end

assert(bit32.lshift(0x12345678, 4) == 0x23456780)
assert(bit32.lshift(0x12345678, 8) == 0x34567800)
assert(bit32.lshift(0x12345678, -4) == 0x01234567)
assert(bit32.lshift(0x12345678, -8) == 0x00123456)
assert(bit32.lshift(0x12345678, 32) == 0)
assert(bit32.lshift(0x12345678, -32) == 0)
assert(bit32.rshift(0x12345678, 4) == 0x01234567)
assert(bit32.rshift(0x12345678, 8) == 0x00123456)
assert(bit32.rshift(0x12345678, 32) == 0)
assert(bit32.rshift(0x12345678, -32) == 0)
assert(bit32.arshift(0x12345678, 0) == 0x12345678)
assert(bit32.arshift(0x12345678, 1) == 0x12345678 // 2)
assert(bit32.arshift(0x12345678, -1) == 0x12345678 * 2)
assert(bit32.arshift(-1, 1) == 0xffffffff)
assert(bit32.arshift(-1, 24) == 0xffffffff)
assert(bit32.arshift(-1, 32) == 0xffffffff)
assert(bit32.arshift(-1, -1) == bit32.band(-1 * 2, 0xffffffff))

print("ok")
