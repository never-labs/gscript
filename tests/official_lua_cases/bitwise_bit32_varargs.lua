print("case:bitwise_bit32_varargs")

local bit32 = bit32 or {}
bit32.band = bit32.band or function (...)
  local n = select("#", ...)
  if n == 0 then return 0xffffffff end
  local r = select(1, ...) & 0xffffffff
  for i = 2, n do r = r & select(i, ...) end
  return r & 0xffffffff
end
bit32.bnot = bit32.bnot or function (n) return (~n) & 0xffffffff end
bit32.bor = bit32.bor or function (...)
  local r = 0
  for i = 1, select("#", ...) do r = r | select(i, ...) end
  return r & 0xffffffff
end
bit32.bxor = bit32.bxor or function (...)
  local r = 0
  for i = 1, select("#", ...) do r = r ~ select(i, ...) end
  return r & 0xffffffff
end
bit32.btest = bit32.btest or function (...) return bit32.band(...) ~= 0 end

assert(bit32.band() == bit32.bnot(0))
assert(bit32.btest() == true)
assert(bit32.bor() == 0)
assert(bit32.bxor() == 0)
assert(bit32.band() == bit32.band(0xffffffff))
assert(bit32.band(1, 2) == 0)
assert(bit32.bor(1, 2, 4, 8) == 15)
assert(bit32.bxor(1, 2, 4, 8) == 15)
assert(bit32.bxor(1, 2, 4, 8, 15) == 0)

print("ok")
