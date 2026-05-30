print("case:strings_format_large32_subset_more")

local max, min = 0x7fffffff, -0x80000000

assert(string.format("%x", max) == "7fffffff")
assert(string.sub(string.format("%x", min), -8) == "80000000")
assert(string.format("%d", max) == "2147483647")
assert(string.format("%d", min) == "-2147483648")
assert(string.format("%u", 0xffffffff) == "4294967295")
assert(string.format("%o", 0xABCD) == "125715")

print("ok")
