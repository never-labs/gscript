print("case:bitwise_bit32_rotate")

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
