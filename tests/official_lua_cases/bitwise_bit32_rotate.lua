print("case:bitwise_bit32_rotate")

local bit32 = bit32 or {}
bit32.lrotate = bit32.lrotate or function (x, disp)
  x = x & 0xffffffff
  disp = disp & 31
  return ((x << disp) | (x >> (32 - disp))) & 0xffffffff
end
bit32.rrotate = bit32.rrotate or function (x, disp)
  return bit32.lrotate(x, -disp)
end

assert(bit32.lrotate(0, -1) == 0)
assert(bit32.lrotate(0, 7) == 0)
assert(bit32.lrotate(305419896, 0) == 305419896)
assert(bit32.lrotate(305419896, 32) == 305419896)
assert(bit32.lrotate(305419896, 4) == 591751041)
assert(bit32.rrotate(305419896, -4) == 591751041)
assert(bit32.lrotate(305419896, -8) == 2014458966)
assert(bit32.rrotate(305419896, 8) == 2014458966)
assert(bit32.lrotate(2863311530, 2) == 2863311530)
assert(bit32.lrotate(2863311530, -2) == 2863311530)

print("ok")
