print("case:strings_byte_range_multi_more")

local a, b, c, d = string.byte("abcdef", 2, 5)
assert(a == 98 and b == 99 and c == 100 and d == 101)

a, b, c = string.byte("abcdef", -3, -1)
assert(a == 100 and b == 101 and c == 102)

a, b = string.byte("abcdef", 4, 3)
assert(a == nil and b == nil)

print("ok")
