print("case:bitwise_bit32_identities")

local bit32 = bit32 or {}
bit32.band = bit32.band or function (...)
  local n = select("#", ...)
  if n == 0 then return 0xffffffff end
  local r = select(1, ...) & 0xffffffff
  for i = 2, n do r = r & select(i, ...) end
  return r & 0xffffffff
end
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
bit32.bnot = bit32.bnot or function (n) return (~n) & 0xffffffff end
bit32.btest = bit32.btest or function (...) return bit32.band(...) ~= 0 end
bit32.lrotate = bit32.lrotate or function (x, disp)
  x = x & 0xffffffff
  disp = disp & 31
  return ((x << disp) | (x >> (32 - disp))) & 0xffffffff
end
bit32.rrotate = bit32.rrotate or function (x, disp) return bit32.lrotate(x, -disp) end
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

local c = {0, 1, 2, 3, 10, 0x80000000, 0xaaaaaaaa, 0x55555555,
           0xffffffff, 0x7fffffff}

for _, b in pairs(c) do
  assert(bit32.band(b) == b)
  assert(bit32.band(b, b) == b)
  assert(bit32.band(b, b, b, b) == b)
  assert(bit32.btest(b, b) == (b ~= 0))
  assert(bit32.band(b, b, b) == b)
  assert(bit32.band(b, b, b, bit32.bnot(b)) == 0)
  assert(bit32.btest(b, b, b) == (b ~= 0))
  assert(bit32.band(b, bit32.bnot(b)) == 0)
  assert(bit32.bor(b, bit32.bnot(b)) == bit32.bnot(0))
  assert(bit32.bor(b) == b)
  assert(bit32.bor(b, b) == b)
  assert(bit32.bor(b, b, b) == b)
  assert(bit32.bor(b, b, 0, bit32.bnot(b)) == 0xffffffff)
  assert(bit32.bxor(b) == b)
  assert(bit32.bxor(b, b) == 0)
  assert(bit32.bxor(b, b, b) == b)
  assert(bit32.bxor(b, b, b, b) == 0)
  assert(bit32.bxor(b, 0) == b)
  assert(bit32.bnot(b) ~= b)
  assert(bit32.bnot(bit32.bnot(b)) == b)
  assert(bit32.bnot(b) == 0xffffffff - b)
  assert(bit32.lrotate(b, 32) == b)
  assert(bit32.rrotate(b, 32) == b)
  assert(bit32.lshift(bit32.lshift(b, -4), 4) == bit32.band(b, bit32.bnot(0xf)))
  assert(bit32.rshift(bit32.rshift(b, 4), -4) == bit32.band(b, bit32.bnot(0xf)))
end

print("ok")
