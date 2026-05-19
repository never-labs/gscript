print("case:strings_byte_range_multi_more")

a, b, c, d := string.byte("abcdef", 2, 5)
assert(a == 98 && b == 99 && c == 100 && d == 101)

a, b, c = string.byte("abcdef", -3, -1)
assert(a == 100 && b == 101 && c == 102)

a, b = string.byte("abcdef", 4, 3)
assert(a == nil && b == nil)

print("ok")
