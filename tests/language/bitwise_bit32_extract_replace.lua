print("case:bitwise_bit32_extract_replace")

local bit32 = bit32 or {}
bit32.band = bit32.band or function (...)
  local n = select("#", ...)
  if n == 0 then return 0xffffffff end
  local r = select(1, ...) & 0xffffffff
  for i = 2, n do r = r & select(i, ...) end
  return r & 0xffffffff
end
bit32.btest = bit32.btest or function (...)
  return bit32.band(...) ~= 0
end
bit32.extract = bit32.extract or function (n, field, width)
  width = width or 1
  if field < 0 or field >= 32 or width <= 0 or width > 32 - field then error("bad field or width") end
  return (n >> field) & ((1 << width) - 1)
end
bit32.replace = bit32.replace or function (n, v, field, width)
  width = width or 1
  if field < 0 or field >= 32 or width <= 0 or width > 32 - field then error("bad field or width") end
  local mask = ((1 << width) - 1) << field
  return ((n & ~mask) | ((v << field) & mask)) & 0xffffffff
end

assert(bit32.btest(0xffffffff, 0x10))
assert(not bit32.btest(0x10, 0x20))

assert(bit32.extract(0x12345678, 0, 4) == 8)
assert(bit32.extract(0x12345678, 4, 4) == 7)
assert(bit32.extract(0xa0001111, 28, 4) == 0xa)
assert(bit32.extract(0xa0001111, 31, 1) == 1)
assert(bit32.extract(0x50000111, 31, 1) == 0)
assert(bit32.extract(0xf2345679, 0, 32) == 0xf2345679)

assert(not pcall(bit32.extract, 0, -1))
assert(not pcall(bit32.extract, 0, 32))
assert(not pcall(bit32.extract, 0, 0, 33))
assert(not pcall(bit32.extract, 0, 31, 2))

assert(bit32.replace(0x12345678, 5, 28, 4) == 0x52345678)
assert(bit32.replace(0x12345678, 0x87654321, 0, 32) == 0x87654321)
assert(bit32.replace(0, 1, 2) == 4)
assert(bit32.replace(0, -1, 4) == 16)
assert(bit32.replace(-1, 0, 31) == 0x7fffffff)
assert(bit32.replace(-1, 0, 1, 2) == 0xfffffff9)

print("ok")
