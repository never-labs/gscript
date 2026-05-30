print("case:bitwise_bit32_shift_sweep")

local bit32 = bit32 or {}
bit32.lshift = bit32.lshift or function (n, d)
  n = n & 0xffffffff
  if d < 0 then return (n >> -d) & 0xffffffff end
  if d >= 32 then return 0 end
  return (n << d) & 0xffffffff
end

local c = {0, 1, 2, 3, 10, 0x800000, 0xaaaaaa, 0x555555, 0xffffff, 0x7fffff}
local shifts = {-40, -32, -8, -1, 0, 1, 7, 16, 31, 32, 40}

for _, b in pairs(c) do
  for _, i in pairs(shifts) do
    local x = bit32.lshift(b, i)
    local y = math.floor(math.fmod(b * 2.0^i, 2.0^32))
    assert(math.fmod(x - y, 2.0^32) == 0)
  end
end

print("ok")
