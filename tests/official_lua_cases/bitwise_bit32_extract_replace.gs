print("case:bitwise_bit32_extract_replace")

assert(bit32.btest(4294967295, 16))
assert(!bit32.btest(16, 32))

assert(bit32.extract(305419896, 0, 4) == 8)
assert(bit32.extract(305419896, 4, 4) == 7)
assert(bit32.extract(2684358929, 28, 4) == 10)
assert(bit32.extract(2684358929, 31, 1) == 1)
assert(bit32.extract(1342181649, 31, 1) == 0)
assert(bit32.extract(4063516281, 0, 32) == 4063516281)

assert(!pcall(bit32.extract, 0, -1))
assert(!pcall(bit32.extract, 0, 32))
assert(!pcall(bit32.extract, 0, 0, 33))
assert(!pcall(bit32.extract, 0, 31, 2))

assert(bit32.replace(305419896, 5, 28, 4) == 1379161720)
assert(bit32.replace(305419896, 2271560481, 0, 32) == 2271560481)
assert(bit32.replace(0, 1, 2) == 4)
assert(bit32.replace(0, -1, 4) == 16)
assert(bit32.replace(-1, 0, 31) == 2147483647)
assert(bit32.replace(-1, 0, 1, 2) == 4294967289)

print("ok")
