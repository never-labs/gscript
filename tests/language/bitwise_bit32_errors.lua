print("case:bitwise_bit32_errors")

local bit32 = bit32 or {}
bit32.band = bit32.band or function (...)
  local n = select("#", ...)
  if n == 0 then return 0xffffffff end
  local r = select(1, ...) & 0xffffffff
  for i = 2, n do r = r & select(i, ...) end
  return r & 0xffffffff
end
bit32.bnot = bit32.bnot or function (n) return (~n) & 0xffffffff end
bit32.lshift = bit32.lshift or function (n, d)
  if d < 0 then return (n >> -d) & 0xffffffff end
  if d >= 32 then return 0 end
  return (n << d) & 0xffffffff
end
bit32.rshift = bit32.rshift or function (n, d)
  if d < 0 then return (n << -d) & 0xffffffff end
  if d >= 32 then return 0 end
  return (n >> d) & 0xffffffff
end

assert(not pcall(bit32.band, {}))
assert(not pcall(bit32.bnot, "a"))
assert(not pcall(bit32.lshift, 45))
assert(not pcall(bit32.lshift, 45, print))
assert(not pcall(bit32.rshift, 45, print))
assert(bit32.band(1, 3, 7) == 1)
assert(bit32.bnot(0) == 0xffffffff)

print("ok")
